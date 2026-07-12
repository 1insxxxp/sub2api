//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type affiliateQualificationRecoveryReconcilerStub struct {
	mu     sync.Mutex
	calls  int
	errors []error
	called chan struct{}
}

func (s *affiliateQualificationRecoveryReconcilerStub) ReconcilePendingAffiliateQualifications(context.Context) error {
	s.mu.Lock()
	call := s.calls
	s.calls++
	var err error
	if call < len(s.errors) {
		err = s.errors[call]
	}
	s.mu.Unlock()
	select {
	case s.called <- struct{}{}:
	default:
	}
	return err
}

func (s *affiliateQualificationRecoveryReconcilerStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func TestAffiliateQualificationRecoveryWorkerImmediatePeriodicAndCancel(t *testing.T) {
	reconciler := &affiliateQualificationRecoveryReconcilerStub{
		errors: []error{errors.New("first drain failed")},
		called: make(chan struct{}, 4),
	}
	worker := newAffiliateQualificationRecoveryWorker(reconciler, 10*time.Millisecond, time.Second)
	require.Zero(t, reconciler.callCount(), "constructor must not start a goroutine")
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)

	waitForAffiliateRecoveryCall(t, reconciler.called)
	waitForAffiliateRecoveryCall(t, reconciler.called)
	require.GreaterOrEqual(t, reconciler.callCount(), 2, "periodic cycle must retry after an error")
	cancel()
	require.Eventually(t, worker.stopped, time.Second, 5*time.Millisecond)
	callsAfterCancel := reconciler.callCount()
	time.Sleep(30 * time.Millisecond)
	require.Equal(t, callsAfterCancel, reconciler.callCount(), "cancelled worker must not leak periodic calls")
	worker.Stop()
}

func TestAffiliateQualificationRecoveryWorkerStopIsIdempotent(t *testing.T) {
	reconciler := &affiliateQualificationRecoveryReconcilerStub{called: make(chan struct{}, 2)}
	worker := newAffiliateQualificationRecoveryWorker(reconciler, time.Hour, time.Second)
	worker.Start(context.Background())
	waitForAffiliateRecoveryCall(t, reconciler.called)

	worker.Stop()
	worker.Stop()
	require.True(t, worker.stopped())
}

func TestProvideAffiliateQualificationRecoveryWorkerStartsAndStops(t *testing.T) {
	repo := &affiliateTierServiceRepoStub{
		reconcileRequired: true,
		generation:        1,
		reconcileStarted:  make(chan struct{}),
		releaseReconcile:  make(chan struct{}),
	}
	affiliateService := NewAffiliateService(repo, NewSettingService(newAffiliateTierServiceSettingRepo(), nil), nil, nil)
	worker := ProvideAffiliateQualificationRecoveryWorker(affiliateService)

	select {
	case <-repo.reconcileStarted:
	case <-time.After(time.Second):
		t.Fatal("production provider did not run immediate affiliate recovery")
	}
	close(repo.releaseReconcile)
	worker.Stop()
	require.True(t, worker.stopped())
}

func waitForAffiliateRecoveryCall(t *testing.T, called <-chan struct{}) {
	t.Helper()
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for affiliate qualification recovery call")
	}
}
