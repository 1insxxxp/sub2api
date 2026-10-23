//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUsagePersistenceWorkerConstructionAndLifecycle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := newUsagePersistenceWorkerRepo(1)
		worker := NewUsagePersistenceWorker(repo)
		synctest.Wait()
		require.Zero(t, repo.snapshot().claims, "construction must not start delivery")
		worker.Start()
		worker.Start()
		synctest.Wait()
		require.Equal(t, []int64{1}, repo.snapshot().delivered)
		worker.Stop()
		worker.Stop()
		claims := repo.snapshot().claims
		worker.Start()
		time.Sleep(time.Second)
		synctest.Wait()
		require.Equal(t, claims, repo.snapshot().claims, "stopped instance must not restart")

		stopped := NewUsagePersistenceWorker(repo)
		stopped.Stop()
		stopped.Start()
		synctest.Wait()
		require.Equal(t, claims, repo.snapshot().claims)
		var nilWorker *UsagePersistenceWorker
		require.NotPanics(t, nilWorker.Start)
		require.NotPanics(t, nilWorker.Stop)
		require.NotPanics(t, NewUsagePersistenceWorker(nil).Start)
	})
}

func TestProvideUsagePersistenceWorkerStartsImmediately(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := newUsagePersistenceWorkerRepo(1)
		worker := ProvideUsagePersistenceWorker(repo)
		defer worker.Stop()
		synctest.Wait()
		require.Equal(t, []int64{1}, repo.snapshot().delivered)
	})
}

func TestUsagePersistenceWorkerBoundsBatchAndConcurrentDeliveries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := newUsagePersistenceWorkerRepo(1, 2, 3, 4, 5, 6, 7, 8)
		release := make(chan struct{})
		repo.deliver = func(ctx context.Context, _ int64, _ int) error {
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		worker := ProvideUsagePersistenceWorker(repo)
		defer worker.Stop()
		synctest.Wait()
		state := repo.snapshot()
		require.Equal(t, 4, state.active)
		require.Equal(t, 4, state.maxActive)
		require.Equal(t, 8, state.claimLimits[0])
		require.Equal(t, 60*time.Second, state.claimLeases[0])
		require.Equal(t, 5*time.Second, state.claimBudgets[0])
		close(release)
		synctest.Wait()
		state = repo.snapshot()
		require.ElementsMatch(t, []int64{1, 2, 3, 4, 5, 6, 7, 8}, state.delivered)
		require.Equal(t, 4, state.maxActive)
		for _, budget := range state.deliverBudgets {
			require.Equal(t, 10*time.Second, budget)
		}
	})
}

func TestUsagePersistenceWorkerRetriesFailedDeliveryWithoutLosingEvent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := newUsagePersistenceWorkerRepo(7)
		repo.deliver = func(_ context.Context, _ int64, call int) error {
			if call == 1 {
				return errors.New("database error containing private material")
			}
			return nil
		}
		worker := ProvideUsagePersistenceWorker(repo)
		defer worker.Stop()
		synctest.Wait()
		state := repo.snapshot()
		require.Empty(t, state.delivered)
		require.Equal(t, 1, state.pending)
		require.Equal(t, 1, state.retries)
		require.Equal(t, time.Second, state.retryDelays[0])
		require.Equal(t, "usage_persistence_delivery_failed", state.retryCodes[0])
		time.Sleep(1500 * time.Millisecond)
		synctest.Wait()
		state = repo.snapshot()
		require.Equal(t, []int64{7}, state.delivered)
		require.Zero(t, state.pending)
		require.Equal(t, 2, state.deliverCalls)
	})
}

func TestUsagePersistenceWorkerDeliveryTimeoutUsesFreshRetryDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := newUsagePersistenceWorkerRepo(8)
		repo.deliver = func(ctx context.Context, _ int64, call int) error {
			if call == 1 {
				<-ctx.Done()
				return ctx.Err()
			}
			return nil
		}
		worker := ProvideUsagePersistenceWorker(repo)
		defer worker.Stop()
		synctest.Wait()
		time.Sleep(10 * time.Second)
		synctest.Wait()
		state := repo.snapshot()
		require.Equal(t, 1, state.retries)
		require.Equal(t, []time.Duration{5 * time.Second}, state.retryBudgets)
		require.Equal(t, []string{"usage_persistence_delivery_timeout"}, state.retryCodes)
		time.Sleep(1500 * time.Millisecond)
		synctest.Wait()
		state = repo.snapshot()
		require.Equal(t, []int64{8}, state.delivered)
		require.Equal(t, []time.Duration{10 * time.Second, 10 * time.Second}, state.deliverBudgets)
	})
}

func TestUsagePersistenceWorkerStopCancelsDeliveryAndRestartReclaimsLease(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := newUsagePersistenceWorkerRepo(9)
		repo.deliver = func(ctx context.Context, _ int64, call int) error {
			if call == 1 {
				<-ctx.Done()
				return ctx.Err()
			}
			return nil
		}
		first := ProvideUsagePersistenceWorker(repo)
		synctest.Wait()
		first.Stop()
		state := repo.snapshot()
		require.Zero(t, state.active)
		require.Zero(t, state.retries, "shutdown should leave the durable lease for recovery")
		require.Equal(t, 1, state.pending)
		second := ProvideUsagePersistenceWorker(repo)
		defer second.Stop()
		synctest.Wait()
		require.Empty(t, repo.snapshot().delivered)
		time.Sleep(60 * time.Second)
		synctest.Wait()
		state = repo.snapshot()
		require.Equal(t, []int64{9}, state.delivered)
		require.Len(t, state.deliveryOwners, 2)
		require.NotEmpty(t, state.deliveryOwners[0])
		require.NotEqual(t, state.deliveryOwners[0], state.deliveryOwners[1])
	})
}

func TestUsagePersistenceWorkerFailedRetryWriteIsRecoveredAfterLease(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := newUsagePersistenceWorkerRepo(10)
		repo.deliver = func(_ context.Context, _ int64, call int) error {
			if call == 1 {
				return errors.New("temporary failure")
			}
			return nil
		}
		repo.retry = func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}
		first := ProvideUsagePersistenceWorker(repo)
		synctest.Wait()
		time.Sleep(5 * time.Second)
		synctest.Wait()
		first.Stop()
		require.Equal(t, 1, repo.snapshot().pending)
		second := ProvideUsagePersistenceWorker(repo)
		defer second.Stop()
		time.Sleep(55 * time.Second)
		synctest.Wait()
		require.Equal(t, []int64{10}, repo.snapshot().delivered)
	})
}

func TestUsagePersistenceWorkerClaimHasIndependentBoundedDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := newUsagePersistenceWorkerRepo(11)
		repo.claim = func(ctx context.Context, call int) error {
			if call == 1 {
				<-ctx.Done()
				return ctx.Err()
			}
			return nil
		}
		worker := ProvideUsagePersistenceWorker(repo)
		defer worker.Stop()
		synctest.Wait()
		time.Sleep(6 * time.Second)
		synctest.Wait()
		state := repo.snapshot()
		require.GreaterOrEqual(t, state.claims, 2)
		require.Equal(t, []int64{11}, state.delivered)
		for _, budget := range state.claimBudgets {
			require.Equal(t, 5*time.Second, budget)
		}
		require.Equal(t, []time.Duration{10 * time.Second}, state.deliverBudgets)
	})
}

func TestUsagePersistenceWorkerConcurrentLifecycle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := newUsagePersistenceWorkerRepo()
		worker := NewUsagePersistenceWorker(repo)
		var callers sync.WaitGroup
		for range 20 {
			callers.Add(2)
			go func() { defer callers.Done(); worker.Start() }()
			go func() { defer callers.Done(); worker.Stop() }()
		}
		callers.Wait()
		synctest.Wait()
		worker.Stop()
		claims := repo.snapshot().claims
		time.Sleep(time.Second)
		synctest.Wait()
		require.Equal(t, claims, repo.snapshot().claims)
	})
}

func TestUsagePersistenceRetryDelayIsBoundedAndDoesNotDiscard(t *testing.T) {
	for _, tc := range []struct {
		attempts int
		want     time.Duration
	}{
		{-1, time.Second}, {0, time.Second}, {1, 2 * time.Second},
		{2, 4 * time.Second}, {5, 32 * time.Second}, {6, time.Minute},
		{100, time.Minute}, {math.MaxInt, time.Minute},
	} {
		t.Run(fmt.Sprint(tc.attempts), func(t *testing.T) {
			require.Equal(t, tc.want, usagePersistenceRetryDelay(tc.attempts))
		})
	}
	synctest.Test(t, func(t *testing.T) {
		repo := newUsagePersistenceWorkerRepo(12)
		repo.deliver = func(context.Context, int64, int) error { return errors.New("permanent") }
		worker := ProvideUsagePersistenceWorker(repo)
		defer worker.Stop()
		time.Sleep(130 * time.Second)
		synctest.Wait()
		state := repo.snapshot()
		require.Equal(t, 1, state.pending, "failed events must never be discarded")
		require.Empty(t, state.delivered)
		require.GreaterOrEqual(t, state.retries, 8)
		for _, delay := range state.retryDelays {
			require.GreaterOrEqual(t, delay, time.Second)
			require.LessOrEqual(t, delay, time.Minute)
		}
	})
}

type usagePersistenceWorkerEntry struct {
	event     UsagePersistenceEvent
	owner     string
	lease     time.Time
	available time.Time
}

type usagePersistenceWorkerState struct {
	claims, deliverCalls, retries, active, maxActive, pending int
	delivered                                                 []int64
	claimLimits                                               []int
	claimLeases, claimBudgets, deliverBudgets                 []time.Duration
	retryBudgets, retryDelays                                 []time.Duration
	retryCodes, deliveryOwners                                []string
}

type usagePersistenceWorkerRepo struct {
	mu      sync.Mutex
	entries map[int64]*usagePersistenceWorkerEntry
	state   usagePersistenceWorkerState
	claim   func(context.Context, int) error
	deliver func(context.Context, int64, int) error
	retry   func(context.Context) error
}

func newUsagePersistenceWorkerRepo(ids ...int64) *usagePersistenceWorkerRepo {
	r := &usagePersistenceWorkerRepo{entries: make(map[int64]*usagePersistenceWorkerEntry)}
	for _, id := range ids {
		r.entries[id] = &usagePersistenceWorkerEntry{event: UsagePersistenceEvent{ID: id, CreatedAt: time.Now()}}
	}
	return r
}

func (r *usagePersistenceWorkerRepo) Claim(ctx context.Context, owner string, limit int, lease time.Duration) ([]UsagePersistenceEvent, error) {
	r.mu.Lock()
	r.state.claims++
	call := r.state.claims
	r.state.claimLimits = append(r.state.claimLimits, limit)
	r.state.claimLeases = append(r.state.claimLeases, lease)
	r.state.claimBudgets = append(r.state.claimBudgets, usagePersistenceWorkerBudget(ctx))
	r.mu.Unlock()
	if r.claim != nil {
		if err := r.claim(ctx, call); err != nil {
			return nil, err
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var events []UsagePersistenceEvent
	for _, entry := range r.entries {
		if len(events) == limit {
			break
		}
		if time.Now().Before(entry.lease) || time.Now().Before(entry.available) {
			continue
		}
		entry.owner, entry.lease = owner, time.Now().Add(lease)
		events = append(events, entry.event)
	}
	return events, nil
}

func (r *usagePersistenceWorkerRepo) Deliver(ctx context.Context, id int64, owner string) error {
	r.mu.Lock()
	r.state.deliverCalls++
	call := r.state.deliverCalls
	r.state.active++
	r.state.maxActive = max(r.state.maxActive, r.state.active)
	r.state.deliverBudgets = append(r.state.deliverBudgets, usagePersistenceWorkerBudget(ctx))
	r.state.deliveryOwners = append(r.state.deliveryOwners, owner)
	r.mu.Unlock()
	defer func() { r.mu.Lock(); r.state.active--; r.mu.Unlock() }()
	if r.deliver != nil {
		if err := r.deliver(ctx, id, call); err != nil {
			return err
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry := r.entries[id]; entry == nil || entry.owner != owner {
		return errors.New("lease mismatch")
	}
	delete(r.entries, id)
	r.state.delivered = append(r.state.delivered, id)
	return nil
}

func (r *usagePersistenceWorkerRepo) RetryClaimed(ctx context.Context, id int64, owner string, available time.Time, code string) error {
	r.mu.Lock()
	r.state.retries++
	r.state.retryBudgets = append(r.state.retryBudgets, usagePersistenceWorkerBudget(ctx))
	r.state.retryDelays = append(r.state.retryDelays, available.Sub(time.Now()))
	r.state.retryCodes = append(r.state.retryCodes, code)
	r.mu.Unlock()
	if r.retry != nil {
		if err := r.retry(ctx); err != nil {
			return err
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := r.entries[id]
	if entry == nil || entry.owner != owner {
		return errors.New("lease mismatch")
	}
	entry.event.Attempts++
	entry.available, entry.lease, entry.owner = available, time.Time{}, ""
	return nil
}

func (r *usagePersistenceWorkerRepo) snapshot() usagePersistenceWorkerState {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.state
	state.pending = len(r.entries)
	state.delivered = append([]int64(nil), state.delivered...)
	state.claimLimits = append([]int(nil), state.claimLimits...)
	state.claimLeases = append([]time.Duration(nil), state.claimLeases...)
	state.claimBudgets = append([]time.Duration(nil), state.claimBudgets...)
	state.deliverBudgets = append([]time.Duration(nil), state.deliverBudgets...)
	state.retryBudgets = append([]time.Duration(nil), state.retryBudgets...)
	state.retryDelays = append([]time.Duration(nil), state.retryDelays...)
	state.retryCodes = append([]string(nil), state.retryCodes...)
	state.deliveryOwners = append([]string(nil), state.deliveryOwners...)
	return state
}

func usagePersistenceWorkerBudget(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok || ctx.Err() != nil {
		return -1
	}
	return time.Until(deadline)
}
