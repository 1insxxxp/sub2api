//go:build unit

package service

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// A transport error can accompany a complete, unterminated usage JSON line.
// Keep that provider measurement without changing each adapter's delivery policy.
func TestGeminiStreamUsageTailPreservesProviderMeasurement(t *testing.T) {
	const model = "gemini-2.5-pro"
	const priorUsage = `data: {"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":1}}` + "\n\n"
	const finalUsage = `"usageMetadata":{"promptTokenCount":55779,"candidatesTokenCount":96146}`
	for _, path := range []string{"chat", "messages", "native"} {
		for _, tc := range []struct {
			name                 string
			prior, tail          string
			readErr              error
			wantInput, wantOut   int
			complete, tailAnswer bool
		}{
			{name: "first_measurement_with_read_error", tail: "data: {" + finalUsage + "}", readErr: io.ErrUnexpectedEOF, wantInput: 55779, wantOut: 96146},
			{name: "revised_measurement_with_read_error", prior: priorUsage, tail: "data: {" + finalUsage + "}", readErr: io.ErrUnexpectedEOF, wantInput: 55779, wantOut: 96146},
			{name: "invalid_json_cannot_replace_measurement", prior: priorUsage, tail: `data: {"usageMetadata":{"promptTokenCount":557`, readErr: io.ErrUnexpectedEOF, wantInput: 100, wantOut: 1},
			{name: "usage_recovery_preserves_delivery_policy", prior: priorUsage, tail: `data: {"candidates":[{"content":{"parts":[{"text":"tail answer"}]}}],` + finalUsage + "}", readErr: io.ErrUnexpectedEOF, wantInput: 55779, wantOut: 96146, tailAnswer: true},
			{name: "normal_eof_terminal_tail", prior: priorUsage, tail: `data: {"candidates":[{"finishReason":"STOP"}],` + finalUsage + "}", readErr: io.EOF, wantInput: 55779, wantOut: 96146, complete: true},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				svc, c, rec := newGeminiUsageRecoveryService(t, geminiAbortTextChunk+tc.prior+tc.tail, tc.readErr)
				collector, restore := AttachDownstreamOutputTokenCollector(c, model)
				defer restore()
				account := geminiVertexAPIKeyTestAccount("")
				result, err := callGeminiTraversalPath(t, path, svc, c, account, model, true)
				if tc.complete {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, tc.readErr, "recovering usage must not turn a transport failure into success")
					var failover *UpstreamFailoverError
					require.False(t, errors.As(err, &failover), "already delivered output must never be replayed")
				}
				require.NotNil(t, result)
				require.NotNil(t, result.Outcome)
				require.Equal(t, tc.complete, result.Outcome.StreamCompleted)
				require.Contains(t, rec.Body.String(), "hello")
				if tc.tailAnswer {
					if path == "chat" {
						require.Contains(t, rec.Body.String(), "tail answer", "retain Chat's existing successful tail delivery")
					} else {
						require.NotContains(t, rec.Body.String(), "tail answer", "usage-only recovery must not newly deliver the failed tail")
					}
				}
				require.Equal(t, tc.wantInput, result.Usage.InputTokens, "retain the complete provider measurement, including revisions")
				require.Equal(t, tc.wantOut, result.Usage.OutputTokens, "retain provider evidence separately from delivered output")
				ApplyDeliveredOutputTokens(result, collector)
				require.NotNil(t, result.DeliveredOutputTokens)
				require.Positive(t, *result.DeliveredOutputTokens)
				require.Equal(t, tc.wantOut, recordedOutputTokens(result), "Gemini settlement retains measured output even when interrupted")

				billableOutput := recordedOutputTokens(result)
				result.RequestID = "gemini-tail-" + path + "-" + tc.name
				billing, logs, commands := newGeminiInputUsageRegressionBilling()
				billing.billingService.pricingService.pricingData[model] = &LiteLLMModelPricing{InputCostPerToken: 1e-6, OutputCostPerToken: 5e-6}
				require.NoError(t, billing.RecordUsage(c.Request.Context(), &RecordUsageInput{
					Result: result, APIKey: &APIKey{ID: 501, Quota: 100}, User: &User{ID: 601}, Account: account,
					APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{},
				}))
				require.Equal(t, 1, commands.calls, "settle the delivered request exactly once")
				require.NotNil(t, logs.lastLog)
				require.Equal(t, tc.wantInput, logs.lastLog.InputTokens)
				require.InDelta(t, float64(tc.wantInput)*1e-6, logs.lastLog.InputCost, 1e-10)
				require.Equal(t, tc.wantInput, commands.lastCmd.InputTokens)
				require.Equal(t, billableOutput, logs.lastLog.OutputTokens)
				require.Equal(t, tc.wantOut, logs.lastLog.UpstreamOutputTokens)
				require.Equal(t, billableOutput, commands.lastCmd.OutputTokens)
				require.InDelta(t, logs.lastLog.ActualCost, commands.lastCmd.BalanceCost, 1e-8)
				require.InDelta(t, logs.lastLog.ActualCost, commands.lastCmd.APIKeyQuotaCost, 1e-8)

				if !tc.complete {
					events := upstreamErrorEventsFromContext(t, c)
					require.Len(t, events, 1)
					require.Equal(t, "observed", events[0].UsageState)
					require.Equal(t, int64(tc.wantInput), events[0].InputTokens, "Ops must retain only valid provider measurements, not truncated JSON numbers")
					require.Equal(t, int64(tc.wantOut), events[0].OutputTokens)
				}
			})
		}
	}
}
