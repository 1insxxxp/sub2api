package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFirstOutputTraversalVisitsDifferentAccountsBeforeRetry(t *testing.T) {
	fs := NewFailoverState(1, false)
	unscheduler := &mockTempUnscheduler{}
	for _, accountID := range []int64{11, 22, 33, 44, 55, 66} {
		err := &service.UpstreamFailoverError{
			StatusCode:             http.StatusBadGateway,
			PreferNextAccount:      true,
			RetryableOnSameAccount: true,
		}
		require.Equal(t, FailoverContinue, fs.HandleFailoverError(context.Background(), unscheduler, accountID, service.PlatformGemini, 3, err))
		require.Contains(t, fs.FailedAccountIDs, accountID)
	}
	require.Len(t, fs.FailedAccountIDs, 6)
	require.Empty(t, fs.SameAccountRetryCount)
	require.Empty(t, unscheduler.calls, "skipping retries must not pretend that the configured retries were exhausted")
}

func TestFirstOutputTraversalExhaustionDoesNotRestartFailedPool(t *testing.T) {
	fs := NewFailoverState(9, false)
	err := &service.UpstreamFailoverError{StatusCode: http.StatusServiceUnavailable, PreferNextAccount: true}
	require.Equal(t, FailoverContinue, fs.HandleFailoverError(context.Background(), &mockTempUnscheduler{}, 11, service.PlatformGemini, 3, err))
	require.Equal(t, FailoverExhausted, fs.HandleSelectionExhausted(context.Background()))
	require.Contains(t, fs.FailedAccountIDs, int64(11))
}

func TestFirstOutputTraversalBudgetStopWinsOverAccountPreference(t *testing.T) {
	fs := NewFailoverState(9, false)
	err := &service.UpstreamFailoverError{StatusCode: http.StatusGatewayTimeout, PreferNextAccount: true, NextAccountAction: service.NextAccountStop}
	require.Equal(t, FailoverExhausted, fs.HandleFailoverError(context.Background(), &mockTempUnscheduler{}, 11, service.PlatformGemini, 3, err))
	require.Empty(t, fs.FailedAccountIDs)
	require.Zero(t, fs.SwitchCount)
}
