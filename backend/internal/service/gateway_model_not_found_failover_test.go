//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const gatewayUnsupportedModelBody = `{"error":{"type":"model_not_found","message":"Model \"claude-opus-4-6\" is not supported by any configured account in this group"}}`

func TestGatewayModelNotFoundFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/v1/chat/completions", "/v1/responses", "/v1/messages", "/v1/messages-passthrough"} {
		for _, poolMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/pool=%t", path, poolMode), func(t *testing.T) {
				upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{{
					StatusCode: http.StatusNotFound,
					Header:     http.Header{"X-Request-Id": {"missing-model-attempt"}},
					Body:       io.NopCloser(strings.NewReader(gatewayUnsupportedModelBody)),
				}}}
				repo := &modelNotFoundAccountRepoStub{}
				svc := &GatewayService{
					cfg:                 &config.Config{},
					httpUpstream:        upstream,
					tlsFPProfileService: &TLSFingerprintProfileService{},
					rateLimitService:    &RateLimitService{accountRepo: repo},
				}
				account := newAnthropicAPIKeyAccountForTest()
				account.Extra["anthropic_passthrough"] = strings.HasSuffix(path, "-passthrough")
				account.Credentials["model_mapping"] = map[string]any{"public-opus": "claude-opus-4-6"}
				account.Credentials["pool_mode"] = poolMode
				account.Credentials["pool_mode_retry_status_codes"] = []any{float64(http.StatusNotFound)}
				// Even custom retry settings must not keep trying a missing model on this account.
				account.Credentials["custom_error_codes_enabled"] = true
				account.Credentials["custom_error_codes"] = []any{float64(http.StatusTooManyRequests)}
				c, recorder := gatewayModelNotFoundContext(path)

				result, err := forwardGatewayModelNotFoundTest(svc, c, account, path)

				require.Nil(t, result)
				var failoverErr *UpstreamFailoverError
				require.ErrorAs(t, err, &failoverErr)
				require.Equal(t, http.StatusNotFound, failoverErr.StatusCode)
				require.JSONEq(t, gatewayUnsupportedModelBody, string(failoverErr.ResponseBody))
				require.True(t, failoverErr.ShouldRetryNextAccount())
				require.False(t, failoverErr.RetryableOnSameAccount)
				require.False(t, failoverErr.PreferNextAccount, "missing models must respect the normal account-switch budget")
				require.Equal(t, 1, upstream.callCount)
				require.Equal(t, "claude-opus-4-6", gjson.GetBytes(upstream.requestBodies[0], "model").String())
				require.False(t, c.Writer.Written())
				require.Empty(t, recorder.Body.String())
				require.Zero(t, repo.tempCalls)
				require.Empty(t, repo.modelRateLimitCalls, "custom error-code exclusions must still disable persistent cooldown")
				events := c.MustGet(OpsUpstreamErrorsKey).([]*OpsUpstreamErrorEvent)
				require.Len(t, events, 1)
				require.Equal(t, account.ID, events[0].AccountID)
				require.Equal(t, http.StatusNotFound, events[0].UpstreamStatusCode)
				require.Equal(t, "missing-model-attempt", events[0].UpstreamRequestID)
				require.Contains(t, events[0].Message, "not supported")
				require.Equal(t, strings.HasSuffix(path, "-passthrough"), events[0].Passthrough)
			})
		}
	}
}

func TestGatewayModelNotFoundFailoverPreservesTerminalErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/v1/chat/completions", "/v1/responses", "/v1/messages", "/v1/messages-passthrough"} {
		for _, tc := range []struct {
			name   string
			status int
			body   string
		}{
			{"endpoint missing", http.StatusNotFound, `{"error":{"message":"endpoint not found"}}`},
			{"invalid parameter", http.StatusBadRequest, `{"error":{"message":"temperature is invalid"}}`},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{{
					StatusCode: tc.status,
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader(tc.body)),
				}}}
				svc := &GatewayService{cfg: &config.Config{}, httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}, rateLimitService: &RateLimitService{}}
				account := newAnthropicAPIKeyAccountForTest()
				account.Extra["anthropic_passthrough"] = strings.HasSuffix(path, "-passthrough")
				c, recorder := gatewayModelNotFoundContext(path)

				_, err := forwardGatewayModelNotFoundTest(svc, c, account, path)

				require.Error(t, err)
				var failoverErr *UpstreamFailoverError
				require.False(t, errors.As(err, &failoverErr))
				require.Equal(t, 1, upstream.callCount)
				require.True(t, c.Writer.Written())
				require.NotEmpty(t, recorder.Body.String())
				require.Contains(t, err.Error(), gjson.Get(tc.body, "error.message").String())
			})
		}
	}
}

func TestGatewayModelNotFoundFailoverCoolsOnlyMappedModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/v1/chat/completions", "/v1/responses", "/v1/messages", "/v1/messages-passthrough"} {
		t.Run(path, func(t *testing.T) {
			upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{{
				StatusCode: http.StatusNotFound, Header: http.Header{},
				Body: io.NopCloser(strings.NewReader(gatewayUnsupportedModelBody)),
			}}}
			repo := &modelNotFoundAccountRepoStub{}
			svc := &GatewayService{cfg: &config.Config{}, httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}, rateLimitService: &RateLimitService{accountRepo: repo}}
			account := newAnthropicAPIKeyAccountForTest()
			account.Extra["anthropic_passthrough"] = strings.HasSuffix(path, "-passthrough")
			account.Credentials["model_mapping"] = map[string]any{"public-opus": "claude-opus-4-6"}
			account.Credentials["pool_mode"] = true
			c, _ := gatewayModelNotFoundContext(path)

			_, err := forwardGatewayModelNotFoundTest(svc, c, account, path)

			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Len(t, repo.modelRateLimitCalls, 1)
			require.Equal(t, "claude-opus-4-6", repo.modelRateLimitCalls[0].scope)
			require.Equal(t, upstreamModelNotFoundReason, repo.modelRateLimitCalls[0].reason)
			require.WithinDuration(t, time.Now().Add(upstreamModelNotFoundCooldown), repo.modelRateLimitCalls[0].resetAt, time.Second)
			require.Zero(t, repo.tempCalls)
		})
	}
}

func gatewayModelNotFoundContext(path string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, strings.TrimSuffix(path, "-passthrough"), nil)
	return c, recorder
}

func TestGatewayModelNotFoundFailoverAfterRectification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name     string
		failures []string
	}{
		{"budget", []string{`{"error":{"message":"thinking.budget_tokens must be greater than or equal to 1024"}}`}},
		{"thinking signature", []string{`{"error":{"message":"Invalid signature in thinking block"}}`}},
		{"tool signature", []string{`{"error":{"message":"Invalid signature in thinking block"}}`, `{"error":{"message":"Invalid signature in tool_use block"}}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &queuedHTTPUpstreamStub{}
			for _, body := range tc.failures {
				upstream.responses = append(upstream.responses, &http.Response{StatusCode: http.StatusBadRequest, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))})
			}
			upstream.responses = append(upstream.responses, &http.Response{
				StatusCode: http.StatusNotFound, Header: http.Header{"X-Request-Id": {"rectified-model-missing"}},
				Body: io.NopCloser(strings.NewReader(gatewayUnsupportedModelBody)),
			})
			settings, err := json.Marshal(&RectifierSettings{Enabled: true, ThinkingSignatureEnabled: true, APIKeySignatureEnabled: true, ThinkingBudgetEnabled: true})
			require.NoError(t, err)
			svc := &GatewayService{
				cfg: &config.Config{}, httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{},
				settingService:   NewSettingService(&gatewayTTLSettingRepo{data: map[string]string{SettingKeyRectifierSettings: string(settings)}}, nil),
				rateLimitService: &RateLimitService{},
			}
			account := newAnthropicAPIKeyAccountForTest()
			account.Extra["anthropic_passthrough"] = false
			account.Credentials["pool_mode"] = true
			account.Credentials["model_mapping"] = map[string]any{"public-opus": "claude-opus-4-6"}
			c, recorder := gatewayModelNotFoundContext("/v1/messages")

			_, err = forwardGatewayModelNotFoundTest(svc, c, account, "/v1/messages")

			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, http.StatusNotFound, failoverErr.StatusCode)
			require.False(t, failoverErr.RetryableOnSameAccount)
			require.Equal(t, len(tc.failures)+1, upstream.callCount)
			require.False(t, c.Writer.Written())
			require.Empty(t, recorder.Body.String())
			events := c.MustGet(OpsUpstreamErrorsKey).([]*OpsUpstreamErrorEvent)
			require.Equal(t, "failover", events[len(events)-1].Kind)
			require.Equal(t, "rectified-model-missing", events[len(events)-1].UpstreamRequestID)
		})
	}
}

func forwardGatewayModelNotFoundTest(svc *GatewayService, c *gin.Context, account *Account, path string) (*ForwardResult, error) {
	body := []byte(`{"model":"public-opus","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"hello"}]}`)
	switch path {
	case "/v1/chat/completions":
		return svc.ForwardAsChatCompletions(context.Background(), c, account, body, nil)
	case "/v1/responses":
		return svc.ForwardAsResponses(context.Background(), c, account, []byte(`{"model":"public-opus","stream":true,"input":"hello"}`), nil)
	default:
		parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
		if err != nil {
			return nil, err
		}
		return svc.Forward(context.Background(), c, account, parsed)
	}
}
