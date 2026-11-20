package dto

import (
	"encoding/json"
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

func TestProtectedPendingEvidenceOnlyAppearsInAdminDTO(t *testing.T) {
	input := 12345
	log := &service.UsageLog{BillingStatus: service.UsageBillingStatusPendingReview, BillingPendingReason: service.UsagePendingReasonCustomerInputUnavailable,
		InputTokensSource: service.UsageTokenSourceUnreported, PendingEvidence: &service.PendingUsageEvidence{PendingProviderUsage: service.PendingProviderUsage{InputTokens: &input}}}
	user, err := json.Marshal(UsageLogFromService(log))
	require.NoError(t, err)
	require.Contains(t, string(user), `"billing_pending_reason":"customer_input_unavailable"`)
	require.NotContains(t, string(user), "12345")
	require.NotContains(t, string(user), "pending_provider_evidence")
	admin, err := json.Marshal(UsageLogFromServiceAdmin(log))
	require.NoError(t, err)
	require.Contains(t, string(admin), `"pending_provider_evidence":{"input_tokens":12345}`)
}
