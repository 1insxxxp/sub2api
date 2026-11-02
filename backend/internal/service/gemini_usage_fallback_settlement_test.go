//go:build unit

package service

import (
	"context"
	"encoding/json"
	"strings"
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

func TestGeminiFallbackSettlementZeroCountPreservesCacheBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name            string
		externalCache   bool
		fullyCached     bool
		measuredCount   bool
		upstreamAccepts bool
	}{
		{name: "local_text", upstreamAccepts: true},
		{name: "explicit_cache_excludes_cached_content", externalCache: true, upstreamAccepts: true},
		{name: "fully_cached_local_estimate", fullyCached: true, upstreamAccepts: true},
		{name: "fully_cached_positive_provider_count", fullyCached: true, measuredCount: true, upstreamAccepts: true},
		{name: "no_acceptance"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, input, upstream := newGeminiCountFallbackFixture(t, `{"totalTokens":0}`)
			body := geminiFallbackInputBody
			if tc.externalCache {
				body = strings.Replace(body, `"contents":`, `"cachedContent":"cachedContents/test-cache","contents":`, 1)
			}
			fallback := prepareGeminiInputUsageFallback(newGeminiFallbackInputRequest(t, geminiFallbackInputURL, body), input.Account.ID, geminiInputUsageRegressionModel)
			require.True(t, fallback.Known)
			require.Positive(t, fallback.Tokens)
			fallback.CalibrationKey = "" // This test isolates count/cache precedence from calibration training.
			cache := 0
			if tc.externalCache || tc.fullyCached {
				cache = fallback.Tokens
			}
			if tc.measuredCount {
				raw, err := json.Marshal(map[string]int{"totalTokens": cache})
				require.NoError(t, err)
				upstream.resp = geminiUsageRecoveryResponse(200, string(raw))
			}
			result := input.Result
			result.Model = geminiInputUsageRegressionModel
			result.Usage.CacheReadInputTokens = cache
			result.GeminiUsageFallback.Input = fallback
			result.GeminiUsageFallback.UpstreamAccepted = tc.upstreamAccepts
			result.GeminiUsageFallback.CandidateTokens = 71
			applyGeminiMeasuredUsageEvidence(result, 8)
			svc.FinalizeGeminiUsageFallback(context.Background(), input)
			wantInput, wantOutput, wantCalls := fallback.Tokens, 71, 1
			if tc.fullyCached {
				wantInput = 0
			}
			if !tc.upstreamAccepts {
				wantInput, wantOutput, wantCalls = 0, 0, 0
			}
			require.Equal(t, wantInput, result.Usage.InputTokens)
			require.Equal(t, wantOutput, result.Usage.OutputTokens)
			require.Equal(t, cache, result.Usage.CacheReadInputTokens)
			require.Equal(t, UsageTokenSourceEstimated, result.InputTokensSource)
			require.False(t, result.MeasuredUsageComplete)
			require.True(t, result.GeminiUsageFinalized)
			require.Nil(t, fallback.CountRequest)
			require.Len(t, upstream.requests, wantCalls)
			svc.FinalizeGeminiUsageFallback(context.Background(), input)
			require.Equal(t, wantInput, result.Usage.InputTokens)
			require.Len(t, upstream.requests, wantCalls, "finalization must not repeat the count query")
		})
	}
}
