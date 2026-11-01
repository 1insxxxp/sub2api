//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAntigravityOutputLimitPreservesClientIntentOnWire(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "responses", "messages"} {
		for _, model := range []string{"gemini-3.1-pro-high", "claude-sonnet-4-6"} {
			for _, stream := range []bool{false, true} {
				for _, tc := range []struct {
					limit   int
					present bool
				}{{}, {limit: 0, present: true}, {limit: -1, present: true}, {limit: 8, present: true}, {limit: 8192, present: true}, {limit: 64001, present: true}, {limit: 131072, present: true}} {
					limit := tc.limit
					t.Run(protocol+"/"+model+"/stream="+boolString(stream)+"/present="+boolString(tc.present)+"/limit="+strconv.Itoa(limit), func(t *testing.T) {
						var captured []byte
						upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{antigravityCompatSuccessResponse()}, onCall: func(req *http.Request, _ *queuedHTTPUpstreamStub) {
							var err error
							captured, err = io.ReadAll(req.Body)
							require.NoError(t, err)
						}}
						svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)
						account := newAntigravityCompatAccount(AccountTypeOAuth)
						request := map[string]any{"model": model, "stream": stream, "messages": []map[string]string{{"role": "user", "content": "hello"}}}
						path := "/v1/chat/completions"
						if protocol == "responses" {
							delete(request, "messages")
							request["input"], path = "hello", "/v1/responses"
						} else if protocol == "messages" {
							path = "/v1/messages"
						}
						if tc.present {
							field := "max_tokens"
							if protocol == "responses" {
								field = "max_output_tokens"
							}
							request[field] = limit
						}
						body, err := json.Marshal(request)
						require.NoError(t, err)
						original := string(body)
						c, recorder := newAntigravityCompatContext(http.MethodPost, path, body)
						var result *ForwardResult
						switch protocol {
						case "chat":
							result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body, nil)
						case "responses":
							result, err = svc.ForwardAsResponses(context.Background(), c, account, body, nil)
						case "messages":
							result, err = svc.Forward(context.Background(), c, account, body, false)
						}
						require.NoError(t, err)
						require.NotNil(t, result)
						require.Equal(t, http.StatusOK, recorder.Code)
						require.Contains(t, recorder.Body.String(), "ok")
						require.Len(t, upstream.requestBodies, 1, "changing an output limit must not cause an extra generation")
						wireLimit := gjson.GetBytes(captured, "request.generationConfig.maxOutputTokens")
						if tc.present {
							require.Equal(t, int64(limit), wireLimit.Int())
						} else {
							require.False(t, wireLimit.Exists(), "the optional upstream output limit must remain omitted")
						}
						require.Equal(t, original, string(body), "preserve the original body for reservation and continuation")
					})
				}
			}
		}
	}
}

func TestAntigravityBudgetRectifierPreservesOutputLimitOnWire(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{false, true} {
		for _, limit := range []int{0, 1024, 8192, 131072} {
			t.Run("stream="+boolString(stream)+"/limit="+strconv.Itoa(limit), func(t *testing.T) {
				upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{
					{StatusCode: http.StatusBadRequest, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"max_tokens must be greater than thinking.budget_tokens"}}`))},
					antigravityCompatSuccessResponse(),
				}}
				svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)
				account := newAntigravityCompatAccount(AccountTypeOAuth)
				request := map[string]any{"model": "claude-sonnet-4-6", "stream": stream, "messages": []map[string]string{{"role": "user", "content": "hello"}}}
				if limit > 0 {
					request["max_tokens"] = limit
				}
				body, err := json.Marshal(request)
				require.NoError(t, err)
				c, recorder := newAntigravityCompatContext(http.MethodPost, "/v1/messages", body)
				result, err := svc.Forward(context.Background(), c, account, body, false)
				if limit == 1024 {
					require.Error(t, err, "a cap too small for thinking must not be enlarged to make a retry possible")
					require.Len(t, upstream.requestBodies, 1)
					require.Equal(t, int64(limit), gjson.GetBytes(upstream.requestBodies[0], "request.generationConfig.maxOutputTokens").Int())
					return
				}
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, http.StatusOK, recorder.Code)
				require.Len(t, upstream.requestBodies, 2)
				for _, sent := range upstream.requestBodies {
					wireLimit := gjson.GetBytes(sent, "request.generationConfig.maxOutputTokens")
					if limit > 0 {
						require.Equal(t, int64(limit), wireLimit.Int(), "a budget correction cannot change the output cap")
					} else {
						require.False(t, wireLimit.Exists(), "a budget correction cannot supply an omitted output cap")
					}
				}
				retryThinking := gjson.GetBytes(upstream.requestBodies[1], "request.generationConfig.thinkingConfig")
				wantBudget := 32000
				if limit > 0 && limit <= wantBudget {
					wantBudget = limit - 1
				}
				require.Equal(t, int64(wantBudget), retryThinking.Get("thinkingBudget").Int())
			})
		}
	}
}
