//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiFallbackSettlementPreservesMeasurementsAndEstimatesOnlyMissingFields(t *testing.T) {
	for _, tc := range []struct {
		name                                string
		fields                              uint8
		prompt, candidates, thoughts, cache int
		accepted                            bool
		wantInput, wantOutput               int
		wantInputSource, wantOutputSource   UsageTokenSource
	}{
		{"missing all", 0, 0, 0, 0, 0, true, 21916, 4055, UsageTokenSourceEstimated, UsageTokenSourceEstimated},
		{"prompt only", 1, 17005, 0, 0, 0, true, 17005, 4055, UsageTokenSourceUpstream, UsageTokenSourceEstimated},
		{"explicit zero prompt", 1, 0, 0, 0, 0, true, 0, 4055, UsageTokenSourceUpstream, UsageTokenSourceEstimated},
		{"candidates only", 2, 0, 4000, 0, 0, true, 21916, 4000, UsageTokenSourceEstimated, UsageTokenSourceUpstream},
		{"explicit zero candidates", 2, 0, 0, 0, 0, true, 21916, 0, UsageTokenSourceEstimated, UsageTokenSourceUpstream},
		{"thoughts only", 4, 0, 0, 73, 0, true, 21916, 4073, UsageTokenSourceEstimated, UsageTokenSourceEstimated},
		{"complete output", 2 | 4, 0, 4100, 73, 0, true, 21916, 4173, UsageTokenSourceEstimated, UsageTokenSourceUpstream},
		{"known cache", 8, 0, 0, 0, 12000, true, 9916, 4055, UsageTokenSourceEstimated, UsageTokenSourceEstimated},
		{"cache larger than estimate", 8, 0, 0, 0, 30000, true, 0, 4055, UsageTokenSourceEstimated, UsageTokenSourceEstimated},
		{"no upstream acceptance", 0, 0, 0, 0, 0, false, 0, 0, UsageTokenSourceEstimated, UsageTokenSourceEstimated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			billing, logs, commands := newGeminiInputUsageRegressionBilling()
			result := &ForwardResult{
				RequestID: "fallback-" + tc.name, Model: geminiInputUsageRegressionModel,
				Usage: ClaudeUsage{InputTokens: max(0, tc.prompt-tc.cache), OutputTokens: tc.candidates + tc.thoughts, CacheReadInputTokens: tc.cache},
				GeminiUsageFallback: &GeminiUsageFallback{
					Input:           &GeminiInputUsageFallback{Tokens: 21916, Known: true},
					CandidateTokens: 4000, ThoughtTokens: 55, UsageFields: tc.fields,
					MeasuredCandidates: tc.candidates, MeasuredThoughts: tc.thoughts,
					UpstreamAccepted: tc.accepted,
				},
			}
			applyGeminiMeasuredUsageEvidence(result, tc.fields)
			input := &RecordUsageInput{Result: result, Account: &Account{ID: 5957, Platform: PlatformGemini, Type: AccountTypeAPIKey},
				APIKey: &APIKey{ID: 501, Quota: 100}, User: &User{ID: 601}, APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}}
			require.True(t, NeedsUsageMeasurementReview(result))
			billing.FinalizeGeminiUsageFallback(context.Background(), input)
			require.False(t, NeedsUsageMeasurementReview(result))
			require.False(t, result.MeasuredUsageComplete, "an estimate must not become measured usage")
			require.Equal(t, tc.wantInput, result.Usage.InputTokens)
			require.Equal(t, tc.wantOutput, result.Usage.OutputTokens)
			require.Equal(t, tc.wantInputSource, result.InputTokensSource)
			require.Equal(t, tc.wantOutputSource, result.OutputTokensSource)
			require.NoError(t, billing.RecordUsage(context.Background(), input))
			require.Equal(t, 1, commands.calls)
			require.NotNil(t, logs.lastLog)
			require.Equal(t, tc.wantInput, logs.lastLog.InputTokens)
			require.Equal(t, tc.wantOutput, logs.lastLog.OutputTokens)
			require.Equal(t, UsageBillingStatusSettled, logs.lastLog.BillingStatus.Normalize())
			require.InDelta(t, logs.lastLog.ActualCost, commands.lastCmd.BalanceCost, 1e-8)
		})
	}
}

func TestGeminiFallbackSettlementNeverReplacesCompleteProviderUsage(t *testing.T) {
	input, upstream := newGeminiUsageRecoveryFixture()
	billing, _, _ := newGeminiInputUsageRegressionBilling()
	billing.httpUpstream = upstream
	input.Result.Usage = ClaudeUsage{InputTokens: 21, OutputTokens: 7}
	input.Result.InputTokensSource, input.Result.OutputTokensSource = UsageTokenSourceUpstream, UsageTokenSourceUpstream
	input.Result.MeasuredUsageComplete = true
	input.Result.GeminiUsageFallback = &GeminiUsageFallback{Input: &GeminiInputUsageFallback{Tokens: 90000, Known: true}, CandidateTokens: 999, UpstreamAccepted: true}
	billing.FinalizeGeminiUsageFallback(context.Background(), input)
	require.Equal(t, 21, input.Result.Usage.InputTokens)
	require.Equal(t, 7, input.Result.Usage.OutputTokens)
	require.Empty(t, upstream.requests)
}

func TestGeminiFallbackSettlementExplicitCacheLocalEstimateIsAlreadyUncached(t *testing.T) {
	billing, _, _ := newGeminiInputUsageRegressionBilling()
	result := &ForwardResult{RequireMeasuredUsage: true, Model: geminiInputUsageRegressionModel,
		Usage: ClaudeUsage{CacheReadInputTokens: 12000},
		GeminiUsageFallback: &GeminiUsageFallback{UpstreamAccepted: true, UsageFields: 8,
			Input: &GeminiInputUsageFallback{Tokens: 200, Known: true, LocalTokensExcludeCachedContent: true}},
	}
	billing.FinalizeGeminiUsageFallback(context.Background(), &RecordUsageInput{Result: result})
	require.Equal(t, 200, result.Usage.InputTokens, "cachedContent data was not included in the local input estimate")
	require.Equal(t, 12000, result.Usage.CacheReadInputTokens)
}

func TestGeminiFallbackSettlementSnapshotCannotBeSerialized(t *testing.T) {
	result := &ForwardResult{GeminiUsageFallback: &GeminiUsageFallback{Input: &GeminiInputUsageFallback{
		CountRequest: &geminiInputCountRequest{Body: []byte("private prompt text"), URL: "https://private-count.example"},
	}}}
	raw, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "GeminiUsageFallback")
	require.NotContains(t, string(raw), "private-count")
}
