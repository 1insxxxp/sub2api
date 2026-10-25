//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicBufferedResponse_EmbeddedError(t *testing.T) {
	for _, route := range []string{"standard", "passthrough", "native"} {
		t.Run(route, func(t *testing.T) {
			for _, tt := range []struct {
				name   string
				body   string
				status int
			}{
				{"explicit provider status", `{"type":"error","error":{"type":"api_error","status_code":503,"message":"temporarily unavailable"}}`, 503},
				{"new api gateway error", `{"type":"error","error":{"type":"new_api_error","message":"status_code=502, bad response status code 502"}}`, 502},
				{"overloaded", `{"type":"error","error":{"type":"overloaded_error","message":"overloaded"}}`, 529},
				{"invalid parameters", `{"type":"error","error":{"type":"invalid_request_error","message":"max_tokens must be positive"}}`, 400},
				{"unknown error", `{"error":{"message":"upstream failed"}}`, 502},
			} {
				t.Run(tt.name, func(t *testing.T) {
					c, rec := anthropicBufferedErrorContext()
					result, err := runAnthropicBufferedRoute(route, c, []byte(tt.body))
					require.Error(t, err, "HTTP 200 error JSON must not become a successful zero-token result")
					require.Nil(t, result)
					var failover *UpstreamFailoverError
					if tt.status == http.StatusBadRequest {
						require.False(t, errors.As(err, &failover), "deterministic client errors must not cycle through accounts")
						require.Equal(t, tt.status, rec.Code)
						return
					}
					require.ErrorAs(t, err, &failover)
					require.Equal(t, tt.status, failover.StatusCode)
					require.Equal(t, tt.body, string(failover.ResponseBody))
					require.False(t, c.Writer.Written(), "failover must happen before any response bytes are sent")
				})
			}
		})
	}
}

func TestAnthropicBufferedResponse_RealOutputPreserved(t *testing.T) {
	for _, route := range []string{"standard", "passthrough", "native"} {
		t.Run(route, func(t *testing.T) {
			for _, content := range []string{
				`{"type":"text","text":"The upstream returned status_code=502 earlier."}`,
				`{"type":"tool_use","id":"tool_1","name":"lookup","input":{}}`,
				`{"type":"thinking","thinking":"Considering the question"}`,
				`{"type":"redacted_thinking","data":"opaque"}`,
				`{"type":"image","source":{"type":"base64","data":"opaque"}}`,
			} {
				body := []byte(`{"type":"message","content":[` + content + `],"error":{"type":"api_error","message":"diagnostic metadata"},"stop_reason":"end_turn","usage":{"input_tokens":12,"output_tokens":7}}`)
				c, rec := anthropicBufferedErrorContext()
				result, err := runAnthropicBufferedRoute(route, c, body)
				require.NoError(t, err, "real message output must not be retried or discarded")
				require.NotNil(t, result)
				require.JSONEq(t, string(body), rec.Body.String())
			}
		})
	}
}

func TestAnthropicBufferedErrorStatus(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want int
	}{
		{"explicit status beats error type", `{"error":{"type":"invalid_request_error","status_code":503}}`, 503},
		{"numeric status string", `{"error":{"status":"429"}}`, 429},
		{"root numeric code", `{"code":502,"error":"upstream failed"}`, 502},
		{"numeric error type", `{"error":{"type":"503","message":"upstream failed"}}`, 503},
		{"auth type", `{"error":{"type":"authentication_error"}}`, 401},
		{"permission type", `{"error":{"type":"permission_error"}}`, 403},
		{"missing model type", `{"error":{"type":"not_found_error"}}`, 404},
		{"request size type", `{"error":{"type":"request_too_large"}}`, 413},
		{"limit type", `{"error":{"type":"rate_limit_error"}}`, 429},
		{"generic api type", `{"error":{"type":"api_error"}}`, 500},
		{"generic wrapper message status", `{"error":{"type":"new_api_error","message":"status code: 503 unavailable"}}`, 503},
		{"typed validation ignores quoted auth code", `{"error":{"type":"invalid_request_error","message":"messages.0.content must not contain status_code=401"}}`, 400},
		{"typed validation ignores quoted server code", `{"error":{"type":"invalid_request_error","message":"unsupported text: status_code=503"}}`, 400},
		{"invalid structured status ignored", `{"error":{"status_code":"503junk","type":"invalid_request_error"}}`, 400},
		{"fractional status ignored", `{"error":{"status_code":503.5,"type":"invalid_request_error"}}`, 400},
		{"success status ignored", `{"error":{"status_code":200}}`, 502},
		{"array not an envelope", `[{"error":{"message":"failed"}}]`, 0},
		{"trailing invalid JSON", `{"error":{"message":"failed"}}junk`, 0},
		{"empty object", `{}`, 0},
		{"null error", `{"type":"message","error":null}`, 0},
		{"false error", `{"type":"message","error":false}`, 0},
		{"empty error object", `{"error":{}}`, 0},
		{"text mentioning error", `{"type":"message","content":[{"type":"text","text":"error: status_code=502"}]}`, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, anthropicBufferedErrorStatus([]byte(tt.body)))
		})
	}
}

type anthropicBufferedErrorAccountRepo struct {
	nonJSONTempUnschedAccountRepo
}

func (*anthropicBufferedErrorAccountRepo) SetOverloaded(context.Context, int64, time.Time) error {
	return nil
}

func anthropicBufferedErrorContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	return c, rec
}

func runAnthropicBufferedRoute(route string, c *gin.Context, body []byte) (any, error) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}, "X-Request-Id": {"embedded-error-request"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	account := &Account{ID: 201, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	svc := &GatewayService{cfg: &config.Config{}, rateLimitService: NewRateLimitService(&anthropicBufferedErrorAccountRepo{}, nil, &config.Config{}, nil, nil)}
	switch route {
	case "standard":
		return svc.handleNonStreamingResponse(context.Background(), resp, c, account, "claude-opus-4-6", "claude-opus-4-6")
	case "passthrough":
		return svc.handleNonStreamingResponseAnthropicAPIKeyPassthrough(context.Background(), resp, c, account)
	default:
		native := &OpenAIGatewayService{cfg: &config.Config{}}
		return native.handleNativeAnthropicBufferedResponse(context.Background(), resp, c, account, "claude-opus-4-6", "claude-opus-4-6", "claude-opus-4-6", nil, time.Now())
	}
}
