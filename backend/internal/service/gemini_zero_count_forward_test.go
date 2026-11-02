//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiZeroCountForwardUsesLocalInputAtSettlement(t *testing.T) {
	testGeminiZeroCountForwardSettlement(t, false)
}

func TestGeminiZeroCountForwardPreservesExplicitGenerationZero(t *testing.T) {
	testGeminiZeroCountForwardSettlement(t, true)
}

func testGeminiZeroCountForwardSettlement(t *testing.T, reportedZero bool) {
	t.Helper()
	for _, adapter := range []string{"native", "chat", "messages"} {
		for _, stream := range []bool{false, true} {
			t.Run(adapter+"/"+map[bool]string{false: "sync", true: "stream"}[stream], func(t *testing.T) {
				countService, input, upstream := newGeminiCountFallbackFixture(t, `{"totalTokens":0}`)
				usage := `"candidatesTokenCount":71`
				if reportedZero {
					usage = `"promptTokenCount":0,` + usage
				}
				payload := `{"candidates":[{"content":{"parts":[{"text":"A complete answer."}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{` + usage + `}}`
				contentType := "application/json"
				if stream {
					contentType = "text/event-stream"
					payload = "data: " + payload + "\n\ndata: [DONE]\n\n"
				}
				upstream.responses = []*http.Response{{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{contentType}},
					Body:       io.NopCloser(strings.NewReader(payload)),
				}}
				forward := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: countService.cfg}
				c := newGeminiAttemptTestContext(t)
				result, err := callGeminiTraversalPath(t, adapter, forward, c, input.Account, geminiInputUsageRegressionModel, stream)
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, http.StatusOK, c.Writer.Status())
				require.True(t, result.RequireMeasuredUsage)
				require.Equal(t, 71, result.Usage.OutputTokens)
				require.Equal(t, UsageTokenSourceUpstream, result.OutputTokensSource)
				require.NotNil(t, result.GeminiUsageFallback)
				require.NotNil(t, result.GeminiUsageFallback.Input)
				require.True(t, result.GeminiUsageFallback.Input.Known)
				localInput := result.GeminiUsageFallback.Input.Tokens
				require.Positive(t, localInput, "the final inference request contains semantic input")
				require.Len(t, upstream.requests, 1, "forwarding must generate only once")

				wantInput, wantSource, wantCountCalls := localInput, UsageTokenSourceEstimated, 1
				if reportedZero {
					wantInput, wantSource, wantCountCalls = 0, UsageTokenSourceUpstream, 0
					require.True(t, result.MeasuredUsageComplete)
					require.Equal(t, UsageTokenSourceUpstream, result.InputTokensSource)
				} else {
					require.True(t, NeedsUsageMeasurementReview(result))
					require.Equal(t, UsageTokenSourceUnreported, result.InputTokensSource)
					require.NotNil(t, result.GeminiUsageFallback.Input.CountRequest)
				}

				result.RequestID = "zero-count-" + adapter + "-" + map[bool]string{false: "sync", true: "stream"}[stream]
				input.Result = result
				input.APIKey = &APIKey{ID: 501, Quota: 100}
				input.User = &User{ID: 601}
				input.APIKeyService = &openAIRecordUsageAPIKeyQuotaStub{}
				billing, logs, commands := newGeminiInputUsageRegressionBilling()
				billing.cfg.Default.RateMultiplier = 1
				billing.httpUpstream = upstream
				billing.FinalizeGeminiUsageFallback(context.Background(), input)
				require.Equal(t, wantInput, result.Usage.InputTokens, "a zero count response cannot erase known nonempty input")
				require.Equal(t, wantSource, result.InputTokensSource)
				require.Equal(t, 71, result.Usage.OutputTokens, "input recovery cannot rewrite measured output")
				require.Equal(t, UsageTokenSourceUpstream, result.OutputTokensSource)
				require.Equal(t, reportedZero, result.MeasuredUsageComplete, "local fallback must not become measured usage")
				require.True(t, result.GeminiUsageFinalized)
				require.False(t, NeedsUsageMeasurementReview(result))
				require.Nil(t, result.GeminiUsageFallback.Input.CountRequest)

				require.NoError(t, billing.RecordUsage(context.Background(), input))
				require.Equal(t, 1, commands.calls)
				require.NotNil(t, commands.lastCmd)
				require.NotNil(t, logs.lastLog)
				require.Equal(t, wantInput, commands.lastCmd.InputTokens)
				require.Equal(t, wantInput, logs.lastLog.InputTokens)
				require.Equal(t, wantSource, logs.lastLog.InputTokensSource)
				require.Equal(t, 71, commands.lastCmd.OutputTokens)
				require.Equal(t, 71, logs.lastLog.OutputTokens)
				wantCost := float64(wantInput)*1e-6 + 71*5e-6
				require.InDelta(t, float64(wantInput)*1e-6, logs.lastLog.InputCost, 1e-12)
				require.InDelta(t, 71*5e-6, logs.lastLog.OutputCost, 1e-12)
				require.InDelta(t, wantCost, logs.lastLog.ActualCost, 1e-12)
				require.InDelta(t, wantCost, commands.lastCmd.BalanceCost, 1e-12)
				require.InDelta(t, wantCost, commands.lastCmd.APIKeyQuotaCost, 1e-12)

				inferenceCalls, countCalls := 0, 0
				for _, req := range upstream.requests {
					switch {
					case strings.HasSuffix(req.URL.Path, ":countTokens"):
						countCalls++
					case strings.HasSuffix(req.URL.Path, ":generateContent"), strings.HasSuffix(req.URL.Path, ":streamGenerateContent"):
						inferenceCalls++
					default:
						t.Fatalf("unexpected upstream request path: %s", req.URL.Path)
					}
				}
				require.Equal(t, 1, inferenceCalls, "settlement cannot repeat inference")
				require.Equal(t, wantCountCalls, countCalls, "reported generation zero must not trigger a count request")
			})
		}
	}
}
