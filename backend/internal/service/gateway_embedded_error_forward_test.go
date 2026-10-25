//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicForwardNonStreamingEmbeddedErrorDoesNotReturnUsageResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		mode := "normal"
		if passthrough {
			mode = "passthrough"
		}
		t.Run(mode, func(t *testing.T) {
			const upstreamBody = `{"type":"error","error":{"type":"api_error","status_code":502,"message":"bad response"}}`
			svc, account, c, recorder, parsed := newEmbeddedErrorForwardFixture(t, passthrough, upstreamBody)
			ctx, outcomeCollector := WithResponseOutcomeCollector(c.Request.Context(), 0, 0)
			c.Request = c.Request.WithContext(ctx)

			result, err := svc.Forward(ctx, c, account, parsed)

			require.Nil(t, result, "a failed attempt must not produce a zero-usage success result")
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
			require.JSONEq(t, upstreamBody, string(failoverErr.ResponseBody))
			require.Equal(t, "rid-embedded-error", failoverErr.ResponseHeaders.Get("x-request-id"))
			require.True(t, failoverErr.ShouldRetryNextAccount())
			require.False(t, c.Writer.Written(), "the error envelope must remain replaceable by the next attempt")
			require.Empty(t, recorder.Body.String())
			outcome := outcomeCollector.Snapshot()
			require.False(t, outcome.HasEffectiveOutput())
			require.False(t, outcome.StreamCompleted)
			require.NotEqual(t, UsageOutcomeSuccess, ClassifyUsageOutcome(&outcome))

			value, ok := c.Get(OpsUpstreamErrorsKey)
			require.True(t, ok, "the rejected upstream attempt must remain visible in Ops")
			events, ok := value.([]*OpsUpstreamErrorEvent)
			require.True(t, ok)
			require.Len(t, events, 1)
			require.Equal(t, account.ID, events[0].AccountID)
			require.Equal(t, http.StatusBadGateway, events[0].UpstreamStatusCode)
			require.Equal(t, "rid-embedded-error", events[0].UpstreamRequestID)
			require.Equal(t, passthrough, events[0].Passthrough)
			require.Contains(t, events[0].Message, "bad response")
		})
	}
}

func TestAnthropicForwardNonStreamingContentWithoutUsageStillBills(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		mode := "normal"
		if passthrough {
			mode = "passthrough"
		}
		t.Run(mode, func(t *testing.T) {
			const upstreamBody = `{"id":"msg-no-usage","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"HTTP streams deliver response content incrementally while the connection remains open."}],"stop_reason":"end_turn"}`
			svc, account, c, recorder, parsed := newEmbeddedErrorForwardFixture(t, passthrough, upstreamBody)
			collector, restore := AttachDownstreamOutputTokenCollector(c, parsed.Model)
			defer restore()

			result, err := svc.Forward(c.Request.Context(), c, account, parsed)

			require.NoError(t, err)
			require.NotNil(t, result)
			require.JSONEq(t, upstreamBody, recorder.Body.String())
			require.NotNil(t, result.Outcome)
			require.Equal(t, UsageOutcomeSuccess, ClassifyUsageOutcome(result.Outcome))
			ApplyDeliveredOutputTokens(result, collector)
			require.Positive(t, recordedOutputTokens(result))
			require.Zero(t, result.Usage.OutputTokens, "a local estimate must not become provider-reported usage")
			logs := &openAIRecordUsageLogRepoStub{inserted: true}
			billing := &openAIRecordUsageBillingRepoStub{}
			billingService := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
			err = billingService.RecordUsage(context.Background(), &RecordUsageInput{
				Result: result, APIKey: &APIKey{ID: 501}, User: &User{ID: 601}, Account: account,
			})
			require.NoError(t, err)
			require.NotNil(t, logs.lastLog)
			require.Positive(t, logs.lastLog.InputTokens)
			require.Positive(t, logs.lastLog.OutputTokens)
			require.Zero(t, logs.lastLog.UpstreamOutputTokens)
			require.Positive(t, billing.lastCmd.BalanceCost)
		})
	}
}

func newEmbeddedErrorForwardFixture(t *testing.T, passthrough bool, upstreamBody string) (*GatewayService, *Account, *gin.Context, *httptest.ResponseRecorder, *ParsedRequest) {
	t.Helper()
	const requestBody = `{"model":"claude-sonnet-4-5","stream":false,"max_tokens":128,"messages":[{"role":"user","content":"Please explain how HTTP streaming works in a few sentences."}]}`
	parsed, err := ParseGatewayRequest(NewRequestBodyRef([]byte(requestBody)), PlatformAnthropic)
	require.NoError(t, err)
	upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": {"application/json"},
			"X-Request-Id": {"rid-embedded-error"},
		},
		Body: io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	cfg := &config.Config{}
	svc := &GatewayService{
		cfg:                  cfg,
		httpUpstream:         upstream,
		tlsFPProfileService:  &TLSFingerprintProfileService{},
		rateLimitService:     &RateLimitService{},
		responseHeaderFilter: compileResponseHeaderFilter(cfg),
	}
	account := newAnthropicAPIKeyAccountForTest()
	account.Extra["anthropic_passthrough"] = passthrough
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(requestBody))
	return svc, account, c, recorder, parsed
}
