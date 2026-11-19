//go:build integration

package repository

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func acceptanceRepoFixture(t *testing.T, accounts []int64, costs []float64, budget *float64) (service.ChannelAcceptanceRepository, *service.AcceptancePreview) {
	t.Helper()
	r := NewChannelAcceptanceRepository(integrationDB)
	now := time.Now().UTC()
	p := &service.AcceptancePreview{ID: uuid.NewString(), ActorID: 123, RuleVersion: "test-v1", CreatedAt: now, ExpiresAt: now.Add(time.Hour), CanStart: true, Request: service.AcceptancePreviewRequest{BudgetUSD: budget}}
	for i, accountID := range accounts {
		p.Samples = append(p.Samples, service.AcceptanceSample{ID: uuid.NewString(), Ordinal: i, Snapshot: service.AcceptanceSnapshot{AccountID: accountID, Model: "gemini-test", UpstreamModel: "gemini-test", Fingerprint: "frozen"}, Protocol: "gemini", Category: "protocol", Prompt: "synthetic", Status: "pending", EstimatedCostUSD: costs[i], TimeoutSeconds: 180})
		p.EstimatedCostUSD += costs[i]
	}
	require.NoError(t, r.SavePreview(context.Background(), p))
	t.Cleanup(func() {
		_, _ = integrationDB.Exec("DELETE FROM channel_acceptance_runs WHERE preview_id=$1", p.ID)
		_, _ = integrationDB.Exec("DELETE FROM channel_acceptance_previews WHERE id=$1", p.ID)
	})
	return r, p
}

func TestChannelAcceptanceRepositoryPreviewOwnershipAndIdempotentCreate(t *testing.T) {
	ctx := context.Background()
	r, p := acceptanceRepoFixture(t, []int64{101}, []float64{0.25}, nil)
	_, err := r.GetPreview(ctx, p.ID, p.ActorID+1)
	require.ErrorIs(t, err, service.ErrAcceptanceNotFound)
	got, err := r.GetPreview(ctx, p.ID, p.ActorID)
	require.NoError(t, err)
	require.Equal(t, p.ActorID, got.ActorID)
	require.Equal(t, p.Samples, got.Samples)
	// Caller-supplied sample changes must never replace the persisted preview.
	p.Samples[0].Prompt = "tampered"
	const n = 8
	runs := make([]*service.AcceptanceRun, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) { defer wg.Done(); runs[i], errs[i] = r.CreateRun(ctx, p) }(i)
	}
	wg.Wait()
	for i := range n {
		require.NoError(t, errs[i])
		require.Equal(t, runs[0].ID, runs[i].ID)
		require.Equal(t, "synthetic", runs[i].Samples[0].Prompt)
	}
	_, err = integrationDB.Exec("UPDATE channel_acceptance_previews SET expires_at=now()-interval '1 hour' WHERE id=$1", p.ID)
	require.NoError(t, err)
	retry, err := r.CreateRun(ctx, p)
	require.NoError(t, err, "an accepted preview stays idempotent even after its expiry")
	require.Equal(t, runs[0].ID, retry.ID)
	foreign := *p
	foreign.ActorID++
	_, err = r.CreateRun(ctx, &foreign)
	require.ErrorIs(t, err, service.ErrAcceptanceNotFound)
}

func TestChannelAcceptanceRepositoryFindRunByPreview(t *testing.T) {
	ctx := context.Background()
	r, p := acceptanceRepoFixture(t, []int64{102}, []float64{.1}, nil)
	notStarted, err := r.FindRunByPreview(ctx, p.ID, p.ActorID)
	require.NoError(t, err)
	require.Nil(t, notStarted)
	created, err := r.CreateRun(ctx, p)
	require.NoError(t, err)
	_, err = integrationDB.Exec("UPDATE channel_acceptance_previews SET expires_at=now()-interval '1 hour' WHERE id=$1", p.ID)
	require.NoError(t, err)
	found, err := r.FindRunByPreview(ctx, p.ID, p.ActorID)
	require.NoError(t, err, "lookup must return an accepted run even after preview expiry")
	require.Equal(t, created.ID, found.ID)
	require.Equal(t, created.Samples, found.Samples)
	require.NoError(t, r.CancelRun(ctx, created.ID))
	finished, err := r.FindRunByPreview(ctx, p.ID, p.ActorID)
	require.NoError(t, err)
	require.Equal(t, created.ID, finished.ID)
	require.Equal(t, "cancelled", finished.Status, "a completed cancellation must not create a new run")
	foreign, err := r.FindRunByPreview(ctx, p.ID, p.ActorID+1)
	require.NoError(t, err)
	require.Nil(t, foreign, "lookup must never expose another actor's run")
	absent, err := r.FindRunByPreview(ctx, uuid.NewString(), p.ActorID)
	require.NoError(t, err)
	require.Nil(t, absent)
}

func TestChannelAcceptanceRepositoryConcurrentClaimsAndLeaseOwnership(t *testing.T) {
	ctx := context.Background()
	r, p := acceptanceRepoFixture(t, []int64{201, 201, 202, 203}, []float64{.1, .1, .1, .1}, nil)
	run, err := r.CreateRun(ctx, p)
	require.NoError(t, err)
	now := time.Now().UTC()
	const n = 10
	claims := make([]*service.AcceptanceSample, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			claims[i], errs[i] = NewChannelAcceptanceRepository(integrationDB).ClaimNext(ctx, fmt.Sprintf("worker-%d", i), now)
		}(i)
	}
	wg.Wait()
	active := make([]*service.AcceptanceSample, 0)
	seen := map[int64]bool{}
	for i, s := range claims {
		require.NoError(t, errs[i])
		if s == nil {
			continue
		}
		require.False(t, seen[s.Snapshot.AccountID])
		seen[s.Snapshot.AccountID] = true
		require.NotEmpty(t, s.LeaseToken)
		require.WithinDuration(t, now.Add(240*time.Second), *s.LeaseUntil, time.Second)
		active = append(active, s)
	}
	require.Len(t, active, 2, "global cap must survive claims from independent repository instances")
	run, err = r.GetRun(ctx, run.ID)
	require.NoError(t, err)
	require.InDelta(t, .2, run.ReservedCostUSD, 1e-9)
	cost := .04
	result := &service.AcceptanceResult{Status: "passed", CostUSD: &cost}
	require.ErrorIs(t, r.CompleteSample(ctx, active[0].ID, "foreign-lease", result), service.ErrAcceptanceStale)
	require.NoError(t, r.CompleteSample(ctx, active[0].ID, active[0].LeaseToken, result))
	require.NoError(t, r.CompleteSample(ctx, active[0].ID, active[0].LeaseToken, result), "retry must not double-settle")
	run, err = r.GetRun(ctx, run.ID)
	require.NoError(t, err)
	require.InDelta(t, .1, run.ReservedCostUSD, 1e-9)
	require.InDelta(t, .04, run.SpentCostUSD, 1e-9)
}

func TestChannelAcceptanceRepositoryBudgetCancelExpiryAndCleanup(t *testing.T) {
	ctx := context.Background()
	t.Run("reserve before dispatch and stop rest on insufficient budget", func(t *testing.T) {
		budget := .3
		r, p := acceptanceRepoFixture(t, []int64{301, 301}, []float64{.2, .2}, &budget)
		run, err := r.CreateRun(ctx, p)
		require.NoError(t, err)
		s, err := r.ClaimNext(ctx, "budget", time.Now())
		require.NoError(t, err)
		require.NotNil(t, s)
		require.NoError(t, r.CompleteSample(ctx, s.ID, s.LeaseToken, &service.AcceptanceResult{Status: "error"}))
		next, err := r.ClaimNext(ctx, "budget", time.Now())
		require.NoError(t, err)
		require.Nil(t, next)
		run, err = r.GetRun(ctx, run.ID)
		require.NoError(t, err)
		require.Equal(t, "budget_exhausted", run.Status)
		require.InDelta(t, .2, run.ReservedCostUSD, 1e-9, "missing cost retains reservation")
		require.Zero(t, run.SpentCostUSD)
		require.Equal(t, "cancelled", run.Samples[1].Status)
	})
	t.Run("cancel does not free inflight lane or override its result", func(t *testing.T) {
		r, p := acceptanceRepoFixture(t, []int64{302, 302}, []float64{.2, .2}, nil)
		run, err := r.CreateRun(ctx, p)
		require.NoError(t, err)
		s, err := r.ClaimNext(ctx, "cancel", time.Now())
		require.NoError(t, err)
		require.NotNil(t, s)
		require.NoError(t, r.CancelRun(ctx, run.ID))
		next, err := r.ClaimNext(ctx, "cancel", time.Now())
		require.NoError(t, err)
		require.Nil(t, next)
		zero := 0.0
		require.NoError(t, r.CompleteSample(ctx, s.ID, s.LeaseToken, &service.AcceptanceResult{Status: "cancelled", CostUSD: &zero}))
		run, err = r.GetRun(ctx, run.ID)
		require.NoError(t, err)
		require.Equal(t, "cancelled", run.Status)
		require.Equal(t, 2, run.Completed)
		require.Zero(t, run.ReservedCostUSD)
	})
	t.Run("expired work is unknown never replayed even after restart", func(t *testing.T) {
		r, p := acceptanceRepoFixture(t, []int64{303}, []float64{.2}, nil)
		run, err := r.CreateRun(ctx, p)
		require.NoError(t, err)
		s, err := r.ClaimNext(ctx, "crashed", time.Now().Add(-10*time.Minute))
		require.NoError(t, err)
		require.NotNil(t, s)
		restarted := NewChannelAcceptanceRepository(integrationDB)
		require.NoError(t, restarted.RecoverExpired(ctx, time.Now()))
		next, err := restarted.ClaimNext(ctx, "restarted", time.Now())
		require.NoError(t, err)
		require.Nil(t, next)
		require.ErrorIs(t, restarted.CompleteSample(ctx, s.ID, s.LeaseToken, &service.AcceptanceResult{Status: "passed"}), service.ErrAcceptanceStale)
		run, err = r.GetRun(ctx, run.ID)
		require.NoError(t, err)
		require.Equal(t, "unknown", run.Samples[0].Status)
		require.InDelta(t, .2, run.ReservedCostUSD, 1e-9)
		require.NoError(t, restarted.RecoverExpired(ctx, time.Now()))
		require.NoError(t, restarted.Cleanup(ctx, time.Now().Add(31*24*time.Hour)))
		_, err = restarted.GetRun(ctx, run.ID)
		require.ErrorIs(t, err, service.ErrAcceptanceNotFound)
	})
}

func TestChannelAcceptanceRepositoryRejectsInvalidPreviewAndRollsBackRun(t *testing.T) {
	ctx := context.Background()
	r, p := acceptanceRepoFixture(t, []int64{401}, []float64{.1}, nil)
	_, err := integrationDB.Exec("UPDATE channel_acceptance_previews SET expires_at=now()-interval '1 minute' WHERE id=$1", p.ID)
	require.NoError(t, err)
	_, err = r.CreateRun(ctx, p)
	require.ErrorIs(t, err, service.ErrAcceptanceStale)
	_, fresh := acceptanceRepoFixture(t, []int64{401}, []float64{.1}, nil)
	run, err := r.CreateRun(ctx, fresh)
	require.NoError(t, err)
	duplicate := *fresh
	duplicate.ID = uuid.NewString()
	require.NoError(t, r.SavePreview(ctx, &duplicate))
	t.Cleanup(func() { _, _ = integrationDB.Exec("DELETE FROM channel_acceptance_previews WHERE id=$1", duplicate.ID) })
	_, err = r.CreateRun(ctx, &duplicate)
	require.Error(t, err, "sample IDs cannot be reused by a different run")
	var n int
	require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM channel_acceptance_runs WHERE preview_id=$1", duplicate.ID).Scan(&n))
	require.Zero(t, n, "failed insertion must roll back the whole run")
	stillThere, err := r.GetRun(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, fresh.Samples[0].ID, stillThere.Samples[0].ID)
	for _, bad := range []float64{-1, math.NaN(), math.Inf(1), 1e12} {
		invalid := *fresh
		invalid.ID, invalid.EstimatedCostUSD = uuid.NewString(), bad
		require.ErrorIs(t, r.SavePreview(ctx, &invalid), service.ErrAcceptanceInvalid)
	}
}

func TestChannelAcceptanceRepositorySerialAccountAcrossRunsAndOrdering(t *testing.T) {
	ctx := context.Background()
	r, first := acceptanceRepoFixture(t, []int64{501, 501}, []float64{.1, .1}, nil)
	_, err := r.CreateRun(ctx, first)
	require.NoError(t, err)
	_, second := acceptanceRepoFixture(t, []int64{501}, []float64{.1}, nil)
	_, err = r.CreateRun(ctx, second)
	require.NoError(t, err)
	for _, expected := range []string{first.Samples[0].ID, first.Samples[1].ID, second.Samples[0].ID} {
		s, err := r.ClaimNext(ctx, "serial", time.Now())
		require.NoError(t, err)
		require.NotNil(t, s)
		require.Equal(t, expected, s.ID)
		other, err := NewChannelAcceptanceRepository(integrationDB).ClaimNext(ctx, "other-process", time.Now())
		require.NoError(t, err)
		require.Nil(t, other, "same account cannot overlap across separate runs")
		zero := 0.0
		require.NoError(t, r.CompleteSample(ctx, s.ID, s.LeaseToken, &service.AcceptanceResult{Status: "unsupported", CostUSD: &zero}))
	}
}

func TestChannelAcceptanceRepositoryBudgetIsAtomicAndDecimal(t *testing.T) {
	ctx := context.Background()
	t.Run("two workers cannot reserve past one shared budget", func(t *testing.T) {
		budget := .3
		r, p := acceptanceRepoFixture(t, []int64{601, 602}, []float64{.2, .2}, &budget)
		run, err := r.CreateRun(ctx, p)
		require.NoError(t, err)
		var wg sync.WaitGroup
		claims := make([]*service.AcceptanceSample, 2)
		errs := make([]error, 2)
		for i := range 2 {
			wg.Add(1)
			go func(i int) { defer wg.Done(); claims[i], errs[i] = r.ClaimNext(ctx, fmt.Sprint(i), time.Now()) }(i)
		}
		wg.Wait()
		var s *service.AcceptanceSample
		for i, c := range claims {
			require.NoError(t, errs[i])
			if c != nil {
				require.Nil(t, s)
				s = c
			}
		}
		require.NotNil(t, s)
		run, err = r.GetRun(ctx, run.ID)
		require.NoError(t, err)
		require.Equal(t, "budget_exhausted", run.Status)
		require.InDelta(t, .2, run.ReservedCostUSD, 1e-9)
		cost := .5 // Actual upstream consumption may exceed the reservation.
		require.NoError(t, r.CompleteSample(ctx, s.ID, s.LeaseToken, &service.AcceptanceResult{Status: "passed", CostUSD: &cost}))
		run, err = r.GetRun(ctx, run.ID)
		require.NoError(t, err)
		require.Equal(t, "budget_exhausted", run.Status)
		require.Zero(t, run.ReservedCostUSD)
		require.InDelta(t, .5, run.SpentCostUSD, 1e-9)
		require.NoError(t, r.CancelRun(ctx, run.ID))
		finished, err := r.GetRun(ctx, run.ID)
		require.NoError(t, err)
		require.Equal(t, "budget_exhausted", finished.Status, "terminal runs without in-flight work keep their original reason")
	})
	t.Run("decimal sums at the exact budget are permitted", func(t *testing.T) {
		budget := .3
		r, p := acceptanceRepoFixture(t, []int64{603, 603}, []float64{.1, .2}, &budget)
		run, err := r.CreateRun(ctx, p)
		require.NoError(t, err)
		for _, cost := range []float64{.1, .2} {
			s, err := r.ClaimNext(ctx, "decimal", time.Now())
			require.NoError(t, err)
			require.NotNil(t, s)
			require.NoError(t, r.CompleteSample(ctx, s.ID, s.LeaseToken, &service.AcceptanceResult{Status: "passed", CostUSD: &cost}))
		}
		run, err = r.GetRun(ctx, run.ID)
		require.NoError(t, err)
		require.Equal(t, "completed", run.Status)
		require.InDelta(t, .3, run.SpentCostUSD, 1e-12)
	})
}

func TestChannelAcceptanceRepositoryHistoryAndRetention(t *testing.T) {
	ctx := context.Background()
	r, p := acceptanceRepoFixture(t, []int64{701}, []float64{.1}, nil)
	run, err := r.CreateRun(ctx, p)
	require.NoError(t, err)
	s, err := r.ClaimNext(ctx, "history", time.Now())
	require.NoError(t, err)
	require.NotNil(t, s)
	zero, input := int64(0), int64(42)
	require.NoError(t, r.CompleteSample(ctx, s.ID, s.LeaseToken, &service.AcceptanceResult{Status: "anomaly", RawUsage: service.AcceptanceUsage{Input: &input, Output: &zero}, ClientUsage: service.AcceptanceUsage{Input: &input}, OutputExcerpt: "synthetic reply"}))
	loaded, err := r.GetRun(ctx, run.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded.Samples[0].Result.RawUsage.Output)
	require.Zero(t, *loaded.Samples[0].Result.RawUsage.Output)
	require.Nil(t, loaded.Samples[0].Result.ClientUsage.Output, "null is not a reported zero")
	list, err := r.ListRuns(ctx, []int64{701, 701}, 50)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Empty(t, list[0].Samples, "history must not return large synthetic prompts")
	empty, err := r.ListRuns(ctx, []int64{99999999}, 50)
	require.NoError(t, err)
	require.Empty(t, empty)
	_, activePreview := acceptanceRepoFixture(t, []int64{702, 702}, []float64{.1, .1}, nil)
	active, err := r.CreateRun(ctx, activePreview)
	require.NoError(t, err)
	_, err = r.ClaimNext(ctx, "live", time.Now())
	require.NoError(t, err)
	require.NoError(t, r.CancelRun(ctx, active.ID))
	_, err = integrationDB.Exec("UPDATE channel_acceptance_runs SET updated_at=now()-interval '31 days' WHERE id IN ($1,$2)", run.ID, active.ID)
	require.NoError(t, err)
	require.NoError(t, r.Cleanup(ctx, time.Now().Add(-30*24*time.Hour)))
	_, err = r.GetRun(ctx, run.ID)
	require.ErrorIs(t, err, service.ErrAcceptanceNotFound)
	kept, err := r.GetRun(ctx, active.ID)
	require.NoError(t, err)
	require.Equal(t, "running", kept.Samples[0].Status, "retention cannot delete a cancelled run with an in-flight request")
	// Reading nonexistent records must use the service's stable not-found error.
	require.ErrorIs(t, r.CancelRun(ctx, uuid.NewString()), service.ErrAcceptanceNotFound)
}

func TestChannelAcceptanceRepositoryCancelBudgetExhaustedInflight(t *testing.T) {
	ctx := context.Background()
	budget := .3
	r, p := acceptanceRepoFixture(t, []int64{801, 802}, []float64{.2, .2}, &budget)
	run, err := r.CreateRun(ctx, p)
	require.NoError(t, err)
	s, err := r.ClaimNext(ctx, "inflight", time.Now())
	require.NoError(t, err)
	require.NotNil(t, s)
	next, err := r.ClaimNext(ctx, "over-budget", time.Now())
	require.NoError(t, err)
	require.Nil(t, next)
	run, err = r.GetRun(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, "budget_exhausted", run.Status)
	require.NoError(t, r.CancelRun(ctx, run.ID))
	run, err = r.GetRun(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", run.Status, "worker cancellation monitor must see the request")
	require.Equal(t, "running", run.Samples[0].Status)
	require.Equal(t, s.LeaseToken, run.Samples[0].LeaseToken)
	require.InDelta(t, .2, run.ReservedCostUSD, 1e-9)
	_, waiting := acceptanceRepoFixture(t, []int64{801}, []float64{.1}, nil)
	_, err = r.CreateRun(ctx, waiting)
	require.NoError(t, err)
	blocked, err := r.ClaimNext(ctx, "same-account", time.Now())
	require.NoError(t, err)
	require.Nil(t, blocked, "cancellation must not release a live account lane")
	require.NoError(t, r.CompleteSample(ctx, s.ID, s.LeaseToken, &service.AcceptanceResult{Status: "cancelled"}))
	run, err = r.GetRun(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", run.Status)
	require.Equal(t, 2, run.Completed)
	require.InDelta(t, .2, run.ReservedCostUSD, 1e-9, "unknown consumption keeps its reservation")
	next, err = r.ClaimNext(ctx, "same-account", time.Now())
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Equal(t, waiting.Samples[0].ID, next.ID)
}

type acceptanceWorkerAccountReader struct{ account *service.Account }

func (r acceptanceWorkerAccountReader) GetByID(_ context.Context, id int64) (*service.Account, error) {
	if r.account.ID != id {
		return nil, service.ErrAcceptanceNotFound
	}
	// Account adapters maintain per-instance mapping caches. Give each service
	// read its own value, as a repository read would, instead of sharing caches.
	copyAccount := *r.account
	return &copyAccount, nil
}

type acceptanceWorkerExecutorFunc func(context.Context, service.AcceptanceSample) (*service.AcceptanceResult, error)

func (f acceptanceWorkerExecutorFunc) Execute(ctx context.Context, sample service.AcceptanceSample) (*service.AcceptanceResult, error) {
	return f(ctx, sample)
}

func acceptanceWorkerCleanup(t *testing.T, svc *service.ChannelAcceptanceService, previewID string) {
	t.Helper()
	t.Cleanup(func() {
		svc.Stop() // No worker may outlive its database fixtures.
		_, _ = integrationDB.Exec("DELETE FROM channel_acceptance_runs WHERE preview_id=$1", previewID)
		_, _ = integrationDB.Exec("DELETE FROM channel_acceptance_previews WHERE id=$1", previewID)
	})
}

func TestChannelAcceptanceRepositoryServiceWorkersEndToEnd(t *testing.T) {
	// Only the provider/account reader is faked. Preview confirmation, workers,
	// scheduling, reservations and completion use the production service and PG.
	account := &service.Account{ID: 901, Name: "worker fixture", Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey,
		Credentials: map[string]any{"model_mapping": map[string]any{"gemini-worker-test": "gemini-worker-test"}}}
	price := service.AcceptancePrice{Source: "manual", InputPerMillion: 0, OutputPerMillion: 1, CachePerMillion: 0}
	target := service.AcceptanceTargetRequest{AccountID: account.ID, Model: "gemini-worker-test", Price: &price}
	manualPreview := func() *service.AcceptancePreview {
		now := time.Now().UTC()
		return &service.AcceptancePreview{ID: uuid.NewString(), ActorID: 123, RuleVersion: service.ChannelAcceptanceRuleVersion,
			CreatedAt: now, ExpiresAt: now.Add(time.Hour), CanStart: true, GenerationCalls: 1, EstimatedCostUSD: .002048,
			Request: service.AcceptancePreviewRequest{Targets: []service.AcceptanceTargetRequest{target}},
			Samples: []service.AcceptanceSample{{ID: uuid.NewString(), Ordinal: 0, Status: "pending", Category: "protocol", Protocol: "gemini",
				Prompt: "Return worker verification", Expected: "worker verification", MaxOutputTokens: 2048, TimeoutSeconds: 180, EstimatedCostUSD: .002048,
				Snapshot: service.AcceptanceSnapshot{
					AccountID: account.ID, AccountName: account.Name, Platform: account.Platform, AccountType: account.Type,
					Protocols: []string{"gemini", "chat", "messages"},
					Model:     target.Model, UpstreamModel: target.Model, Price: price,
					Fingerprint: service.AcceptanceAccountFingerprint(account, nil),
				}}}}
	}

	for _, fullPreview := range []bool{false, true} {
		name := "single_sample_completes"
		if fullPreview {
			name = "full_preview_stops_after_one_sample_at_budget"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			repo := NewChannelAcceptanceRepository(integrationDB)
			var calls atomic.Int32
			executor := acceptanceWorkerExecutorFunc(func(_ context.Context, s service.AcceptanceSample) (*service.AcceptanceResult, error) {
				calls.Add(1)
				cost := s.EstimatedCostUSD
				return &service.AcceptanceResult{Status: "passed", Completed: true, UpstreamUsageFinal: true, HasText: true, OutputExcerpt: s.Expected, CostUSD: &cost,
					Findings: []service.AcceptanceFinding{{Code: "worker_fixture", Level: "info", Message: "synthetic provider response"}}}, nil
			})
			svc := service.NewChannelAcceptanceService(repo, acceptanceWorkerAccountReader{account}, nil, nil, executor)
			var p *service.AcceptancePreview
			var err error
			if fullPreview {
				budget := .003 // One 2048-token reservation fits, two do not.
				p, err = svc.Preview(ctx, 123, service.AcceptancePreviewRequest{Targets: []service.AcceptanceTargetRequest{target}, BudgetUSD: &budget})
			} else {
				p = manualPreview()
				err = repo.SavePreview(ctx, p)
			}
			require.NoError(t, err)
			acceptanceWorkerCleanup(t, svc, p.ID)
			require.True(t, p.CanStart)
			run, err := svc.Start(ctx, p.ActorID, p.ID)
			require.NoError(t, err)
			again, err := svc.Start(ctx, p.ActorID, p.ID)
			require.NoError(t, err)
			require.Equal(t, run.ID, again.ID)
			svc.StartWorkers()
			svc.StartWorkers() // Starting the service twice must not duplicate work.
			wantStatus := "completed"
			if fullPreview {
				wantStatus = "budget_exhausted"
			}
			require.Eventually(t, func() bool {
				got, e := repo.GetRun(ctx, run.ID)
				return e == nil && got.Status == wantStatus && got.Completed == got.Total
			}, 8*time.Second, 25*time.Millisecond)
			svc.Stop()
			require.Equal(t, int32(1), calls.Load())
			report, err := svc.Get(ctx, run.ID)
			require.NoError(t, err)
			require.False(t, report.StaleConfig)
			require.True(t, report.Samples[0].Result.Completed)
			require.Equal(t, "passed", report.Samples[0].Status)
			require.Contains(t, report.Samples[0].Result.Findings, service.AcceptanceFinding{Code: "worker_fixture", Level: "info", Message: "synthetic provider response"})
			require.Zero(t, report.ReservedCostUSD)
			require.InDelta(t, p.Samples[0].EstimatedCostUSD, report.SpentCostUSD, 1e-12)
			finalRetry, err := svc.Start(ctx, p.ActorID, p.ID)
			require.NoError(t, err)
			require.Equal(t, run.ID, finalRetry.ID)
			var count int
			require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM channel_acceptance_runs WHERE preview_id=$1", p.ID).Scan(&count))
			require.Equal(t, 1, count)
		})
	}

	for _, cancelRun := range []bool{false, true} {
		name := "stop_cancels_inflight_partial_usage_keeps_reservation"
		if cancelRun {
			name = "cancel_run_cancels_inflight_and_pending_partial_usage_keeps_reservation"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			repo := NewChannelAcceptanceRepository(integrationDB)
			entered, cancelled := make(chan struct{}, 2), make(chan error, 2)
			var calls atomic.Int32
			executor := acceptanceWorkerExecutorFunc(func(ctx context.Context, _ service.AcceptanceSample) (*service.AcceptanceResult, error) {
				calls.Add(1)
				entered <- struct{}{}
				<-ctx.Done()
				cancelled <- ctx.Err()
				// All scalar fields are present, so accidentally treating this interim
				// usage as final would calculate a tiny fee and release the reservation.
				input, output, zero, total := int64(100), int64(1), int64(0), int64(101)
				return &service.AcceptanceResult{Status: "cancelled", UpstreamUsageFinal: false,
					RawUsage: service.AcceptanceUsage{Input: &input, Output: &output, Thoughts: &zero, CacheRead: &zero, Total: &total, Source: "upstream_gemini_reported"}}, ctx.Err()
			})
			svc := service.NewChannelAcceptanceService(repo, acceptanceWorkerAccountReader{account}, nil, nil, executor)
			p := manualPreview()
			if cancelRun {
				queued := p.Samples[0]
				queued.ID, queued.Ordinal = uuid.NewString(), 1
				p.Samples = append(p.Samples, queued)
				p.GenerationCalls++
				p.EstimatedCostUSD += queued.EstimatedCostUSD
			}
			require.NoError(t, repo.SavePreview(ctx, p))
			acceptanceWorkerCleanup(t, svc, p.ID)
			run, err := svc.Start(ctx, p.ActorID, p.ID)
			require.NoError(t, err)
			svc.StartWorkers()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("worker did not dispatch")
			}
			stopped := make(chan struct{})
			if cancelRun {
				_, err = svc.Cancel(ctx, run.ID)
				require.NoError(t, err)
			} else {
				go func() { svc.Stop(); close(stopped) }()
			}
			select {
			case err := <-cancelled:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(3 * time.Second):
				t.Fatal("Stop/Cancel did not cancel executor context")
			}
			if cancelRun {
				require.Eventually(t, func() bool {
					got, e := repo.GetRun(ctx, run.ID)
					return e == nil && got.Completed == got.Total
				}, 5*time.Second, 25*time.Millisecond)
				svc.Stop()
			} else {
				select {
				case <-stopped:
				case <-time.After(5 * time.Second):
					t.Fatal("Stop did not finish persisting the in-flight result")
				}
			}
			report, err := svc.Get(ctx, run.ID)
			require.NoError(t, err)
			wantStatus := "completed"
			if cancelRun {
				wantStatus = "cancelled"
			}
			require.Equal(t, wantStatus, report.Status)
			require.Equal(t, len(p.Samples), report.Completed)
			require.Equal(t, "cancelled", report.Samples[0].Status)
			require.NotNil(t, report.Samples[0].Result.RawUsage.Output)
			require.Equal(t, int64(1), *report.Samples[0].Result.RawUsage.Output)
			require.False(t, report.Samples[0].Result.UpstreamUsageFinal)
			require.Nil(t, report.Samples[0].Result.CostUSD)
			require.InDelta(t, p.Samples[0].EstimatedCostUSD, report.ReservedCostUSD, 1e-12)
			if cancelRun {
				require.Equal(t, "cancelled", report.Samples[1].Status)
			}
			require.Zero(t, report.SpentCostUSD)
			require.Equal(t, int32(1), calls.Load())
			retry, err := svc.Start(ctx, p.ActorID, p.ID)
			require.NoError(t, err)
			require.Equal(t, run.ID, retry.ID)
			next, err := repo.ClaimNext(ctx, "restarted", time.Now())
			require.NoError(t, err)
			require.Nil(t, next, "stopped request must not be replayed")
		})
	}

	t.Run("configuration_changes_after_start_prevent_dispatch", func(t *testing.T) {
		ctx := context.Background()
		repo := NewChannelAcceptanceRepository(integrationDB)
		changed := *account
		var calls atomic.Int32
		executor := acceptanceWorkerExecutorFunc(func(context.Context, service.AcceptanceSample) (*service.AcceptanceResult, error) {
			calls.Add(1)
			return &service.AcceptanceResult{Status: "passed", UpstreamUsageFinal: true}, nil
		})
		svc := service.NewChannelAcceptanceService(repo, acceptanceWorkerAccountReader{&changed}, nil, nil, executor)
		p := manualPreview()
		require.NoError(t, repo.SavePreview(ctx, p))
		acceptanceWorkerCleanup(t, svc, p.ID)
		run, err := svc.Start(ctx, p.ActorID, p.ID)
		require.NoError(t, err)
		// Change only before spawning workers: exercise revalidation without a
		// data race on the stub account shared by two worker goroutines.
		changed.ModelSystemPrompts = map[string]string{target.Model: "changed after confirmation"}
		svc.StartWorkers()
		require.Eventually(t, func() bool {
			got, e := repo.GetRun(ctx, run.ID)
			return e == nil && got.Status == "completed"
		}, 5*time.Second, 25*time.Millisecond)
		svc.Stop()
		require.Zero(t, calls.Load(), "changed configuration must not reach the provider")
		report, err := svc.Get(ctx, run.ID)
		require.NoError(t, err)
		require.True(t, report.StaleConfig)
		require.Equal(t, "inconclusive", report.Samples[0].Status)
		require.NotNil(t, report.Samples[0].Result.CostUSD)
		require.Zero(t, *report.Samples[0].Result.CostUSD)
		require.Zero(t, report.SpentCostUSD)
		require.Zero(t, report.ReservedCostUSD)
		require.NotEmpty(t, report.Samples[0].Result.Findings)
		require.Equal(t, "configuration_changed", report.Samples[0].Result.Findings[0].Code)
	})
}
