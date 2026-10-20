//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Exercise the real protocol adapters and billing boundary: an upstream can
// send content while omitting both its initial input count and final usage.
func TestAnthropicForwardMissingUsageStillBillsGeneratedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "responses", "native", "passthrough"} {
		for _, complete := range []bool{true, false} {
			name := protocol + "/interrupted"
			if complete {
				name = protocol + "/complete"
			}
			t.Run(name, func(t *testing.T) {
				body := []byte(`{"model":"claude-sonnet-4-5","stream":true,"max_tokens":128,"messages":[{"role":"user","content":"Please explain how HTTP streaming works in a few sentences."}]}`)
				path := "/v1/messages"
				if protocol == "chat" {
					path = "/v1/chat/completions"
				} else if protocol == "responses" {
					path = "/v1/responses"
					body = []byte(`{"model":"claude-sonnet-4-5","stream":true,"max_output_tokens":128,"input":"Please explain how HTTP streaming works in a few sentences."}`)
				}
				stream := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"missing-usage\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-5\",\"content\":[],\"usage\":{\"input_tokens\":0,\"output_tokens\":0}}}\n\n" +
					"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
					"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"HTTP streams deliver the response incrementally while the connection remains open.\"}}\n\n"
				if complete {
					stream += "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
						"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":0}}\n\n" +
						"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
				}
				upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{StatusCode: 200,
					Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(stream))}}
				svc := &GatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}}
				account := newAnthropicAPIKeyAccountForTest()
				account.Extra["anthropic_passthrough"] = protocol == "passthrough"
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
				collector, restore := AttachDownstreamOutputTokenCollector(c, "claude-sonnet-4-5")
				defer restore()
				var result *ForwardResult
				var err error
				switch protocol {
				case "chat":
					result, err = svc.ForwardAsChatCompletions(c.Request.Context(), c, account, body, nil)
				case "responses":
					result, err = svc.ForwardAsResponses(c.Request.Context(), c, account, body, nil)
				default:
					parsed, parseErr := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
					require.NoError(t, parseErr)
					result, err = svc.Forward(c.Request.Context(), c, account, parsed)
				}
				if complete {
					require.NoError(t, err)
				}
				require.NotNil(t, result)
				ApplyDeliveredOutputTokens(result, collector)
				require.Greater(t, recordedOutputTokens(result), 0)
				logs := &openAIRecordUsageLogRepoStub{inserted: true}
				billing := &openAIRecordUsageBillingRepoStub{}
				billingService := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
				err = billingService.RecordUsage(context.Background(), &RecordUsageInput{Result: result, APIKey: &APIKey{ID: 501}, User: &User{ID: 601}, Account: account})
				require.NoError(t, err)
				require.NotNil(t, logs.lastLog)
				require.Positive(t, logs.lastLog.InputTokens, "generated response must not lose its entire input bill when upstream usage is missing")
				require.Positive(t, logs.lastLog.InputCost)
				require.Positive(t, logs.lastLog.OutputTokens)
				require.Zero(t, logs.lastLog.UpstreamOutputTokens, "estimates must not be presented as provider measurements")
				require.Positive(t, billing.lastCmd.BalanceCost)
			})
		}
	}
}
