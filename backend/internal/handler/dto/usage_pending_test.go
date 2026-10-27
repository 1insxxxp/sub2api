package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageDTOSeparatesPendingFromSettledAndSuccess(t *testing.T) {
	log := &service.UsageLog{ID: -1, BillingStatus: service.UsageBillingStatusPendingReview, BillingPendingReason: service.UsagePendingReasonUpstreamMissing, OutputTokens: 42, OutputTokensSource: service.UsageTokenSourceUpstream}
	require.Equal(t, service.UsageBillingStatusPendingReview, UsageLogFromService(log).BillingStatus)
	admin := UsageLogFromServiceAdmin(log)
	require.Equal(t, service.UsageBillingStatusPendingReview, admin.BillingStatus)
	require.Equal(t, "unknown", admin.OutcomeStatus, "observed output on an unsettled request is not completion evidence")
	require.Equal(t, service.UsagePendingReasonUpstreamMissing, admin.BillingPendingReason)
	require.Equal(t, service.UsageBillingStatusSettled, UsageLogFromService(&service.UsageLog{ID: 1}).BillingStatus)
}
