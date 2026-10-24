//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const geminiInputUsageRegressionModel = "gemini-2.5-flash"

// Exercise all real Gemini adapters and the normal-mode settlement boundary.
// No fabricated provider input count may stand in for a missing local estimate.
func TestGeminiInputUsageForward_MissingUsageBillsDeliveredPartialResponse(t *testing.T) {
	for _, path := range []string{"chat", "messages", "native"} {
		for _, clientCancel := range []bool{false, true} {
			ending := "upstream_eof"
			if clientCancel {
				ending = "client_cancel"
			}
			t.Run(path+"/"+ending, func(t *testing.T) {
				body := geminiInputUsageRegressionRequest(t, path)
				svc, c, rec := newGeminiUsageRecoveryService(t, geminiAbortTextChunk, io.EOF)
				if clientCancel {
					svc, c, rec = newGeminiAbortingStreamService(t, "/v1/"+path, body, geminiAbortTextChunk, true)
				}
				collector, restore := AttachDownstreamOutputTokenCollector(c, geminiInputUsageRegressionModel)
				defer restore()
				account := geminiVertexAPIKeyTestAccount("")
				result, err := forwardGeminiInputUsageRegression(t, path, svc, c, account, body)
				require.NotNil(t, result)
				if clientCancel {
					require.True(t, result.ClientDisconnect, "adapters may preserve cancellation in the result without returning an error")
				} else {
					require.Error(t, err, "EOF without a terminal marker remains an upstream interruption")
				}
				require.NotNil(t, result.Outcome)
				require.True(t, result.Outcome.HasText)
				require.False(t, result.Outcome.StreamCompleted)
				require.Contains(t, rec.Body.String(), "hello")
				require.Zero(t, result.Usage.InputTokens, "local estimates are not provider measurements")
				require.Zero(t, result.Usage.OutputTokens, "upstream omitted all usage")
				t.Logf("path=%s ending=%s estimated_input=%d provider_input=%d", path, ending, result.EstimatedInputTokens, result.Usage.InputTokens)
				require.Positive(t, result.EstimatedInputTokens, "each adapter must retain an estimate of its final upstream request")
				ApplyDeliveredOutputTokens(result, collector)
				require.Positive(t, recordedOutputTokens(result), "only text actually written downstream is billable")
				delivered := recordedOutputTokens(result)
				estimated := result.EstimatedInputTokens
				result.RequestID = "gemini-input-" + path + "-" + ending

				billing, logs, commands := newGeminiInputUsageRegressionBilling()
				require.NoError(t, billing.RecordUsage(c.Request.Context(), &RecordUsageInput{
					Result: result, APIKey: &APIKey{ID: 501, Quota: 100}, User: &User{ID: 601}, Account: account,
					APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{},
				}))
				require.Equal(t, 1, commands.calls)
				require.NoError(t, commands.lastCtxErr, "client cancellation must not lose settlement")
				require.NotNil(t, logs.lastLog)
				require.Equal(t, estimated, logs.lastLog.InputTokens)
				require.Positive(t, logs.lastLog.InputCost)
				require.True(t, result.InputTokensEstimated)
				require.Equal(t, delivered, logs.lastLog.OutputTokens)
				require.Zero(t, logs.lastLog.UpstreamOutputTokens)
				require.Equal(t, estimated, commands.lastCmd.InputTokens)
				require.Equal(t, delivered, commands.lastCmd.OutputTokens)
				require.Positive(t, commands.lastCmd.BalanceCost)
				require.InDelta(t, logs.lastLog.ActualCost, commands.lastCmd.BalanceCost, 1e-8)
				require.InDelta(t, logs.lastLog.ActualCost, commands.lastCmd.APIKeyQuotaCost, 1e-8)
			})
		}
	}
}

func TestGeminiInputUsageForward_ProviderInputAndFullyCachedInputWin(t *testing.T) {
	for _, path := range []string{"chat", "messages", "native"} {
		for _, tc := range []struct {
			name          string
			cached        int
			uncachedInput int
		}{
			{name: "provider_input", uncachedInput: 73},
			{name: "fully_cached_input", cached: 73},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				response, err := json.Marshal(map[string]any{
					"candidates": []any{map[string]any{
						"content":      map[string]any{"parts": []any{map[string]string{"text": "hello"}}},
						"finishReason": "STOP",
					}},
					"usageMetadata": map[string]int{
						"promptTokenCount": 73, "cachedContentTokenCount": tc.cached, "candidatesTokenCount": 5,
					},
				})
				require.NoError(t, err)
				svc, c, _ := newGeminiUsageRecoveryService(t, "data: "+string(response)+"\n\n", io.EOF)
				collector, restore := AttachDownstreamOutputTokenCollector(c, geminiInputUsageRegressionModel)
				defer restore()
				account := geminiVertexAPIKeyTestAccount("")
				result, err := forwardGeminiInputUsageRegression(t, path, svc, c, account, geminiInputUsageRegressionRequest(t, path))
				require.NoError(t, err)
				require.NotNil(t, result)
				require.True(t, result.Outcome.StreamCompleted)
				require.Equal(t, tc.uncachedInput, result.Usage.InputTokens)
				require.Equal(t, tc.cached, result.Usage.CacheReadInputTokens)
				require.Zero(t, result.EstimatedInputTokens, "a provider/cache measurement suppresses the local estimate")
				ApplyDeliveredOutputTokens(result, collector)
				result.RequestID = "gemini-input-" + path + "-" + tc.name
				billing, logs, commands := newGeminiInputUsageRegressionBilling()
				require.NoError(t, billing.RecordUsage(context.Background(), &RecordUsageInput{
					Result: result, APIKey: &APIKey{ID: 501, Quota: 100}, User: &User{ID: 601}, Account: account,
					APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{},
				}))
				require.Equal(t, 1, commands.calls)
				require.NotNil(t, logs.lastLog)
				require.Equal(t, tc.uncachedInput, logs.lastLog.InputTokens)
				require.Equal(t, tc.cached, logs.lastLog.CacheReadTokens)
				require.False(t, result.InputTokensEstimated)
				require.Equal(t, 5, logs.lastLog.OutputTokens)
				require.Equal(t, 5, logs.lastLog.UpstreamOutputTokens)
				if tc.cached > 0 {
					require.Zero(t, logs.lastLog.InputCost, "fully cached input is a legitimate zero uncached input")
					require.Positive(t, logs.lastLog.CacheReadCost)
				} else {
					require.Positive(t, logs.lastLog.InputCost)
				}
			})
		}
	}
}

func geminiInputUsageRegressionRequest(t *testing.T, path string) []byte {
	t.Helper()
	prompt := strings.Repeat("Explain how incremental HTTP streaming preserves already delivered content. ", 8)
	request := map[string]any{
		"model": geminiInputUsageRegressionModel, "stream": true, "max_tokens": 128,
		"messages": []any{map[string]string{"role": "user", "content": prompt}},
	}
	if path == "native" {
		request = map[string]any{
			"contents": []any{map[string]any{"role": "user", "parts": []any{map[string]string{"text": prompt}}}},
		}
	}
	body, err := json.Marshal(request)
	require.NoError(t, err)
	return body
}

func forwardGeminiInputUsageRegression(t *testing.T, path string, svc *GeminiMessagesCompatService, c *gin.Context, account *Account, body []byte) (*ForwardResult, error) {
	t.Helper()
	switch path {
	case "native":
		return svc.ForwardNative(c.Request.Context(), c, account, geminiInputUsageRegressionModel, "streamGenerateContent", true, body)
	case "messages":
		return svc.Forward(c.Request.Context(), c, account, body)
	default:
		return svc.ForwardAsChatCompletions(c.Request.Context(), c, account, body)
	}
}

func newGeminiInputUsageRegressionBilling() (*GatewayService, *openAIRecordUsageLogRepoStub, *openAIRecordUsageBillingRepoStub) {
	logs := &openAIRecordUsageLogRepoStub{inserted: true}
	commands := &openAIRecordUsageBillingRepoStub{}
	svc := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, commands, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
	svc.billingService.pricingService = &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		geminiInputUsageRegressionModel: {InputCostPerToken: 1e-6, OutputCostPerToken: 5e-6, CacheReadInputTokenCost: 1e-7},
	}}
	return svc, logs, commands
}
