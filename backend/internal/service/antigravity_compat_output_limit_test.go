//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAntigravityCompatOutputLimitDoesNotInherit8192(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "responses"} {
		for _, model := range []string{"gemini-3.1-pro-high", "claude-sonnet-4-6"} {
			for _, explicit := range []bool{false, true} {
				t.Run(protocol+"/"+model+"/explicit="+boolString(explicit), func(t *testing.T) {
					var captured []byte
					upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{antigravityCompatSuccessResponse()}, onCall: func(req *http.Request, _ *queuedHTTPUpstreamStub) {
						var err error
						captured, err = io.ReadAll(req.Body)
						require.NoError(t, err)
					}}
					svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)
					account := newAntigravityCompatAccount(AccountTypeOAuth)
					extra, content, path := "", `,"messages":[{"role":"user","content":"hello"}]`, "/v1/chat/completions"
					if protocol == "responses" {
						content, path = `,"input":"hello"`, "/v1/responses"
					}
					if explicit {
						extra = `,"max_tokens":8192`
						if protocol == "responses" {
							extra = `,"max_output_tokens":8192`
						}
					}
					body := []byte(`{"model":"` + model + `"` + extra + content + `}`)
					c, _ := newAntigravityCompatContext(http.MethodPost, path, body)
					var err error
					if protocol == "chat" {
						_, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body, nil)
					} else {
						_, err = svc.ForwardAsResponses(context.Background(), c, account, body, nil)
					}
					require.NoError(t, err)
					limit := gjson.GetBytes(captured, "request.generationConfig.maxOutputTokens")
					switch {
					case explicit:
						require.Equal(t, int64(8192), limit.Int())
					case model == "claude-sonnet-4-6":
						require.Equal(t, int64(64000), limit.Int(), "retain the native Antigravity transformer default")
					default:
						require.False(t, limit.Exists(), "native Gemini decides the default when the client omits it")
					}
				})
			}
		}
	}
}
