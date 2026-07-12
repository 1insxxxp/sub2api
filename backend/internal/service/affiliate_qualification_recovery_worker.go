package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const (
	affiliateQualificationRecoveryInterval = time.Minute
	affiliateQualificationRecoveryTimeout  = 30 * time.Second
)

type affiliateQualificationPendingReconciler interface {
	ReconcilePendingAffiliateQualifications(context.Context) error
}

// AffiliateQualificationRecoveryWorker drains durable qualification work at
// startup and periodically afterwards. Construction has no runtime side effects.
type AffiliateQualificationRecoveryWorker struct {
	reconciler affiliateQualificationPendingReconciler
	interval   time.Duration
	timeout    time.Duration

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	done    chan struct{}
}

func NewAffiliateQualificationRecoveryWorker(affiliateService *AffiliateService) *AffiliateQualificationRecoveryWorker {
	return newAffiliateQualificationRecoveryWorker(
		affiliateService,
		affiliateQualificationRecoveryInterval,
		affiliateQualificationRecoveryTimeout,
	)
}

func newAffiliateQualificationRecoveryWorker(reconciler affiliateQualificationPendingReconciler, interval, timeout time.Duration) *AffiliateQualificationRecoveryWorker {
	return &AffiliateQualificationRecoveryWorker{
		reconciler: reconciler,
		interval:   interval,
		timeout:    timeout,
	}
}

func (w *AffiliateQualificationRecoveryWorker) Start(parent context.Context) {
	if w == nil || w.reconciler == nil || w.interval <= 0 || w.timeout <= 0 {
		return
	}
	if parent == nil {
		parent = context.Background()
	}

	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	w.started = true
	w.cancel = cancel
	w.done = done
	w.mu.Unlock()

	go w.run(ctx, done)
}

func (w *AffiliateQualificationRecoveryWorker) Stop() {
	if w == nil {
		return
	}
	w.mu.Lock()
	cancel := w.cancel
	done := w.done
	w.mu.Unlock()
	if cancel == nil || done == nil {
		return
	}
	cancel()
	<-done
}

func (w *AffiliateQualificationRecoveryWorker) stopped() bool {
	if w == nil {
		return true
	}
	w.mu.Lock()
	done := w.done
	w.mu.Unlock()
	if done == nil {
		return true
	}
	select {
	case <-done:
		return true
	default:
		return false
	}
}

func (w *AffiliateQualificationRecoveryWorker) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	if ctx.Err() != nil {
		return
	}
	w.runOnce(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.runOnce(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (w *AffiliateQualificationRecoveryWorker) runOnce(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, w.timeout)
	defer cancel()
	if err := w.reconciler.ReconcilePendingAffiliateQualifications(ctx); err != nil {
		slog.Warn("affiliate qualification recovery failed", "error", err)
	}
}
