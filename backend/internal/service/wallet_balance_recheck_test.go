//go:build unit

package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type walletBalanceReaderFunc func(context.Context, int64) (*User, error)

func (f walletBalanceReaderFunc) GetByID(ctx context.Context, id int64) (*User, error) {
	return f(ctx, id)
}

func TestWalletBalanceRecheck_DoesNotCacheZeroAcrossRecharge(t *testing.T) {
	var balance atomic.Int64
	reader := walletBalanceReaderFunc(func(_ context.Context, id int64) (*User, error) {
		return &User{ID: id, Balance: float64(balance.Load())}, nil
	})
	var recheck walletBalanceRechecker
	first, err := recheck.load(context.Background(), reader, 1)
	require.NoError(t, err)
	require.Zero(t, first.Balance)
	balance.Store(1)
	second, err := recheck.load(context.Background(), reader, 1)
	require.NoError(t, err)
	require.Equal(t, 1.0, second.Balance)
}

func TestWalletBalanceRecheck_BoundsConcurrentDatabaseLoads(t *testing.T) {
	started := make(chan struct{}, walletBalanceRecheckConcurrency)
	release := make(chan struct{})
	reader := walletBalanceReaderFunc(func(ctx context.Context, id int64) (*User, error) {
		started <- struct{}{}
		select {
		case <-release:
			return &User{ID: id, Balance: 1}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	var recheck walletBalanceRechecker
	done := make(chan error, walletBalanceRecheckConcurrency)
	for i := range walletBalanceRecheckConcurrency {
		go func() {
			_, err := recheck.load(context.Background(), reader, int64(i+1))
			done <- err
		}()
	}
	for range walletBalanceRecheckConcurrency {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("database rechecks did not start")
		}
	}
	_, err := recheck.load(context.Background(), reader, 100)
	require.ErrorContains(t, err, "overloaded")
	close(release)
	for range walletBalanceRecheckConcurrency {
		require.NoError(t, <-done)
	}
}

func TestWalletBalanceRecheck_CancelledWaiterDoesNotCancelSharedRead(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	reader := walletBalanceReaderFunc(func(ctx context.Context, id int64) (*User, error) {
		close(started)
		select {
		case <-release:
			return &User{ID: id, Balance: 1}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	var recheck walletBalanceRechecker
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := recheck.load(ctx, reader, 1); done <- err }()
	<-started
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	// The original read is still running and can be joined by another request.
	joined := recheck.loads.DoChan("1", func() (any, error) {
		return nil, errors.New("cancelled caller discarded the shared read")
	})
	close(release)
	result := <-joined
	require.NoError(t, result.Err)
	require.Equal(t, 1.0, result.Val.(*User).Balance)
}

func TestWalletBalanceRecheck_MissingReaderOrWalletIsUnavailable(t *testing.T) {
	var recheck walletBalanceRechecker
	_, err := recheck.load(context.Background(), nil, 1)
	require.Error(t, err)
	_, err = recheck.load(context.Background(), walletBalanceReaderFunc(func(context.Context, int64) (*User, error) { return nil, nil }), 1)
	require.Error(t, err)
	_, err = recheck.load(context.Background(), walletBalanceReaderFunc(func(context.Context, int64) (*User, error) { return nil, errors.New("database unavailable") }), 1)
	require.ErrorContains(t, err, "database unavailable")
}
