package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEmptyResponseClaimDoesNotTreatMissingUsageAsMeasuredZero(t *testing.T) {
	now := time.Now()
	group := Group{EmptyResponseCompensationEnabled: true}
	base := UsageLog{ActualCost: 1, CreatedAt: now.Add(-time.Minute)}
	for _, tc := range []struct {
		name    string
		source  UsageTokenSource
		outcome *ResponseOutcome
	}{
		{"unreported without evidence", UsageTokenSourceUnreported, nil},
		{"unreported with output", UsageTokenSourceUnreported, &ResponseOutcome{HasText: true, StreamCompleted: true, CollectorVersion: 1}},
		{"estimated zero without evidence", UsageTokenSourceEstimated, nil},
		{"estimated zero with output", UsageTokenSourceEstimated, &ResponseOutcome{HasReasoning: true, CollectorVersion: 1}},
		{"estimated zero incomplete response", UsageTokenSourceEstimated, &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, CollectorVersion: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usage := base
			usage.OutputTokensSource = tc.source
			decision := EvaluateEmptyResponseClaim(now, usage, tc.outcome, group, 0)
			require.Equal(t, EmptyResponseClaimManualReview, decision.Status)
			require.Equal(t, EmptyResponseReasonMissingEvidence, decision.ReasonCode)
		})
	}
}

func TestEmptyResponseClaimKeepsKnownAndHistoricalLowOutputPolicy(t *testing.T) {
	now := time.Now()
	group := Group{EmptyResponseCompensationEnabled: true}
	for _, tc := range []struct {
		name   string
		source UsageTokenSource
		tokens int
	}{
		{"measured zero", UsageTokenSourceUpstream, 0},
		{"measured low output", UsageTokenSourceUpstream, 10},
		{"estimated received short output", UsageTokenSourceEstimated, 5},
		{"legacy unknown zero", UsageTokenSourceUnknown, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usage := UsageLog{ActualCost: 1, CreatedAt: now.Add(-time.Minute), OutputTokens: tc.tokens, OutputTokensSource: tc.source}
			decision := EvaluateEmptyResponseClaim(now, usage, nil, group, 0)
			require.Equal(t, EmptyResponseClaimApproved, decision.Status)
			require.Equal(t, EmptyResponseReasonLowOutput, decision.ReasonCode)
		})
	}
}

func TestEmptyResponseClaimStillAcceptsVerifiedEmptyWithoutUsage(t *testing.T) {
	now := time.Now()
	usage := UsageLog{ActualCost: 1, CreatedAt: now.Add(-time.Minute), OutputTokensSource: UsageTokenSourceUnreported}
	outcome := &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true, CollectorVersion: 1}
	decision := EvaluateEmptyResponseClaim(now, usage, outcome, Group{EmptyResponseCompensationEnabled: true}, 0)
	require.Equal(t, EmptyResponseClaimApproved, decision.Status)
	require.Equal(t, EmptyResponseReasonPureEmpty, decision.ReasonCode)
}
