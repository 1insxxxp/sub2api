package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	usagePersistenceBatchSize      = 8
	usagePersistenceConcurrency    = 4
	usagePersistenceLease          = 60 * time.Second
	usagePersistenceClaimTimeout   = 5 * time.Second
	usagePersistenceDeliverTimeout = 10 * time.Second
	usagePersistenceRetryTimeout   = 5 * time.Second
	usagePersistencePollInterval   = 250 * time.Millisecond
	usagePersistenceIdleInterval   = 500 * time.Millisecond
)

// UsagePersistenceWorker recovers usage details from committed settlements.
// Its repository only delivers persisted snapshots; it cannot repeat billing.
type UsagePersistenceWorker struct {
	repo     UsagePersistenceOutboxRepository
	workerID string
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	started  bool
	stopped  bool
	wg       sync.WaitGroup
}

func NewUsagePersistenceWorker(repo UsagePersistenceOutboxRepository) *UsagePersistenceWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &UsagePersistenceWorker{
		repo: repo, workerID: uuid.NewString(), ctx: ctx, cancel: cancel,
	}
}

func ProvideUsagePersistenceWorker(repo UsagePersistenceOutboxRepository) *UsagePersistenceWorker {
	worker := NewUsagePersistenceWorker(repo)
	worker.Start()
	return worker
}

func (w *UsagePersistenceWorker) Start() {
	if w == nil || w.repo == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started || w.stopped {
		return
	}
	w.started = true
	w.wg.Add(1)
	go w.run()
}

func (w *UsagePersistenceWorker) Stop() {
	if w == nil {
		return
	}
	w.mu.Lock()
	w.stopped = true
	if w.cancel != nil {
		w.cancel()
	}
	w.mu.Unlock()
	// Start cannot add work after stopped is set while holding the same mutex.
	w.wg.Wait()
}

func (w *UsagePersistenceWorker) run() {
	defer w.wg.Done()
	for w.ctx.Err() == nil {
		count := w.processBatch(w.ctx)
		delay := usagePersistenceIdleInterval
		if count > 0 {
			delay = usagePersistencePollInterval
		}
		timer := time.NewTimer(delay)
		select {
		case <-w.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (w *UsagePersistenceWorker) processBatch(parent context.Context) int {
	claimCtx, cancel := context.WithTimeout(parent, usagePersistenceClaimTimeout)
	events, err := w.repo.Claim(claimCtx, w.workerID, usagePersistenceBatchSize, usagePersistenceLease)
	cancel()
	if err != nil {
		if parent.Err() == nil {
			usagePersistenceFailure(0, 0, usagePersistenceErrorCode("claim", err))
		}
		return 0
	}
	semaphore := make(chan struct{}, usagePersistenceConcurrency)
	var deliveries sync.WaitGroup
	for _, event := range events {
		if parent.Err() != nil {
			break
		}
		select {
		case <-parent.Done():
			deliveries.Wait()
			return len(events)
		case semaphore <- struct{}{}:
		}
		if parent.Err() != nil {
			<-semaphore
			break
		}
		deliveries.Add(1)
		go func(event UsagePersistenceEvent) {
			defer deliveries.Done()
			defer func() { <-semaphore }()
			w.processEvent(parent, event)
		}(event)
	}
	deliveries.Wait()
	return len(events)
}

func (w *UsagePersistenceWorker) processEvent(parent context.Context, event UsagePersistenceEvent) {
	if parent.Err() != nil {
		return
	}
	deliverCtx, cancel := context.WithTimeout(parent, usagePersistenceDeliverTimeout)
	err := w.repo.Deliver(deliverCtx, event.ID, w.workerID)
	cancel()
	if err == nil || parent.Err() != nil {
		// On shutdown, leave unfinished claims durable for lease-based recovery.
		return
	}
	code := usagePersistenceErrorCode("deliver", err)
	usagePersistenceFailure(event.ID, event.Attempts, code)
	retryAt := time.Now().UTC().Add(usagePersistenceRetryDelay(event.Attempts))
	// A timed-out delivery must not poison its independent retry update.
	retryCtx, retryCancel := context.WithTimeout(context.Background(), usagePersistenceRetryTimeout)
	err = w.repo.RetryClaimed(retryCtx, event.ID, w.workerID, retryAt, code)
	retryCancel()
	if err != nil {
		// If this update also fails, the existing lease still permits recovery.
		usagePersistenceFailure(event.ID, event.Attempts, usagePersistenceErrorCode("retry", err))
	}
}

func usagePersistenceRetryDelay(previousFailures int) time.Duration {
	if previousFailures < 0 {
		previousFailures = 0
	}
	if previousFailures >= 6 {
		return time.Minute
	}
	return time.Second * time.Duration(1<<previousFailures)
}

func usagePersistenceErrorCode(operation string, err error) string {
	if operation == "deliver" {
		operation = "usage_persistence_delivery"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return operation + "_timeout"
	}
	return operation + "_failed"
}

func usagePersistenceFailure(id int64, attempts int, code string) {
	// Do not log raw errors: database diagnostics may contain usage identities.
	slog.Warn("usage persistence outbox processing failed", "event_id", id, "attempts", attempts, "code", code)
}
