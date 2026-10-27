//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// Run each real Gemini adapter through normal-mode settlement. Missing provider
// input must never become a charge based on the local prompt-length estimate.
func TestGeminiMeasuredUsageBilling(t *testing.T) {
	const model = "gemini-2.5-pro"
	const measuredUsage = `"usageMetadata":{"promptTokenCount":55779,"candidatesTokenCount":96146}`
	const answer = `"candidates":[{"content":{"parts":[{"text":"hello"}],"role":"model"}}]`
	const completedAnswer = `"candidates":[{"content":{"parts":[{"text":"hello"}],"role":"model"},"finishReason":"STOP"}]`
	for _, path := range []string{"chat", "messages", "native"} {
		for _, tc := range []struct {
			name          string
			sse           string
			complete      bool
			input, cached int
			providerOut   int
			forceCache    bool
		}{
			{name: "missing_usage_interrupted", sse: "data: {" + answer + "}\n\n"},
			{name: "missing_usage_completed", sse: "data: {" + completedAnswer + "}\n\n", complete: true},
			{name: "measured_usage_interrupted", sse: "data: {" + answer + "," + measuredUsage + "}\n\n", input: 55779, providerOut: 96146},
			{name: "measured_usage_completed", sse: "data: {" + completedAnswer + "," + measuredUsage + "}\n\n", complete: true, input: 55779, providerOut: 96146},
			{name: "partly_cached_input", sse: "data: {" + completedAnswer + `,"usageMetadata":{"promptTokenCount":55779,"cachedContentTokenCount":12000,"candidatesTokenCount":96146}}` + "\n\n", complete: true, input: 43779, cached: 12000, providerOut: 96146},
			{name: "fully_cached_input", sse: "data: {" + completedAnswer + `,"usageMetadata":{"promptTokenCount":55779,"cachedContentTokenCount":55779,"candidatesTokenCount":96146}}` + "\n\n", complete: true, cached: 55779, providerOut: 96146},
			{name: "switched_account_measured_input", sse: "data: {" + answer + "," + measuredUsage + "}\n\n", input: 55779, providerOut: 96146, forceCache: true},
			{name: "switched_account_partly_cached_input", sse: "data: {" + completedAnswer + `,"usageMetadata":{"promptTokenCount":55779,"cachedContentTokenCount":12000,"candidatesTokenCount":96146}}` + "\n\n", complete: true, input: 43779, cached: 12000, providerOut: 96146, forceCache: true},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				svc, c, rec := newGeminiUsageRecoveryService(t, tc.sse, io.EOF)
				collector, restore := AttachDownstreamOutputTokenCollector(c, model)
				defer restore()
				account := geminiVertexAPIKeyTestAccount("")
				result, err := callGeminiTraversalPath(t, path, svc, c, account, model, true)
				if tc.complete {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					var failover *UpstreamFailoverError
					require.False(t, errors.As(err, &failover), "already delivered text must not be replayed on another account")
				}
				require.NotNil(t, result)
				require.NotNil(t, result.Outcome)
				require.Equal(t, tc.complete, result.Outcome.StreamCompleted)
				require.Contains(t, rec.Body.String(), "hello")
				require.Equal(t, tc.input, result.Usage.InputTokens)
				require.Equal(t, tc.cached, result.Usage.CacheReadInputTokens)
				require.Equal(t, tc.providerOut, result.Usage.OutputTokens)
				ApplyDeliveredOutputTokens(result, collector)
				require.NotNil(t, result.DeliveredOutputTokens)
				require.Positive(t, *result.DeliveredOutputTokens)
				billableOutput := tc.providerOut
				result.RequestID = "gemini-measured-" + path + "-" + tc.name

				logs := &openAIRecordUsageLogRepoStub{inserted: true}
				commands := &openAIRecordUsageBillingRepoStub{}
				billing := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, commands, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
				billing.billingService.pricingService = &PricingService{pricingData: map[string]*LiteLLMModelPricing{
					model: {InputCostPerToken: 1e-6, OutputCostPerToken: 5e-6, CacheReadInputTokenCost: 1e-7},
				}}
				billingErr := billing.RecordUsage(context.Background(), &RecordUsageInput{
					Result: result, APIKey: &APIKey{ID: 501, Quota: 100}, User: &User{ID: 601}, Account: account,
					APIKeyService:     &openAIRecordUsageAPIKeyQuotaStub{},
					ForceCacheBilling: tc.forceCache,
				})
				if tc.providerOut == 0 {
					require.ErrorIs(t, billingErr, ErrUsageMeasurementIncomplete)
					require.Zero(t, commands.calls)
					require.Nil(t, logs.lastLog)
					return
				}
				require.NoError(t, billingErr)
				require.Equal(t, 1, commands.calls, "settle the request exactly once")
				require.NotNil(t, logs.lastLog)
				require.Equal(t, tc.input, logs.lastLog.InputTokens, "provider input is the only billable Gemini input measurement")
				require.InDelta(t, float64(tc.input)*1e-6, logs.lastLog.InputCost, 1e-10)
				require.Equal(t, tc.cached, logs.lastLog.CacheReadTokens)
				require.InDelta(t, float64(tc.cached)*1e-7, logs.lastLog.CacheReadCost, 1e-10)
				require.Equal(t, billableOutput, logs.lastLog.OutputTokens)
				require.Equal(t, tc.providerOut, logs.lastLog.UpstreamOutputTokens)
				require.InDelta(t, float64(billableOutput)*5e-6, logs.lastLog.OutputCost, 1e-10)
				require.False(t, result.InputTokensEstimated)
				require.Equal(t, tc.input, commands.lastCmd.InputTokens)
				require.Equal(t, billableOutput, commands.lastCmd.OutputTokens)
				require.InDelta(t, logs.lastLog.ActualCost, commands.lastCmd.BalanceCost, 1e-8)
				require.InDelta(t, logs.lastLog.ActualCost, commands.lastCmd.APIKeyQuotaCost, 1e-8)
			})
		}
	}
}

func TestGeminiLedgerRecoveredUsagePreservesMeasuredInputAfterAccountSwitch(t *testing.T) {
	input, upstream := newGeminiUsageRecoveryFixture()
	billing, logs, commands := newGeminiInputUsageRegressionBilling()
	billing.httpUpstream = upstream
	input.APIKey = &APIKey{ID: 501, Quota: 100}
	input.User = &User{ID: 601}
	input.APIKeyService = &openAIRecordUsageAPIKeyQuotaStub{}
	input.ForceCacheBilling = true
	input.Result.RequestID = "gemini-ledger-switched-account"
	input.Result.ClientDisconnect = true
	deliveredEstimate := 5071
	input.Result.DeliveredOutputTokens = &deliveredEstimate

	recovered, err := billing.RecoverGeminiUsage(context.Background(), input)
	require.NoError(t, err)
	require.True(t, recovered)
	require.NoError(t, billing.RecordUsage(context.Background(), input))
	require.Equal(t, 1, commands.calls)
	require.NotNil(t, logs.lastLog)
	require.Equal(t, 21916, logs.lastLog.InputTokens, "a sticky account switch must not rewrite measured ledger input as cache")
	require.Zero(t, logs.lastLog.CacheReadTokens)
	require.Equal(t, 4055, logs.lastLog.OutputTokens, "provider output must not become the downstream estimate")
	require.Equal(t, UsageTokenSourceUpstream, logs.lastLog.InputTokensSource)
	require.Equal(t, UsageTokenSourceUpstream, logs.lastLog.OutputTokensSource)
	require.InDelta(t, 21916e-6, logs.lastLog.InputCost, 1e-10)
	require.Zero(t, logs.lastLog.CacheReadCost)
	require.InDelta(t, 4055*5e-6, logs.lastLog.OutputCost, 1e-10)
	require.Equal(t, 21916, commands.lastCmd.InputTokens)
	require.Equal(t, 4055, commands.lastCmd.OutputTokens)
	require.InDelta(t, logs.lastLog.ActualCost, commands.lastCmd.BalanceCost, 1e-8)
	require.InDelta(t, logs.lastLog.ActualCost, commands.lastCmd.APIKeyQuotaCost, 1e-8)
}
