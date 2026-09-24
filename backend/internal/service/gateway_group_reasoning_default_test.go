//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGatewayGroupReasoningDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name, model, groupEffort, clientEffort, wantEffort, wantThinking string
		wantBudget                                                       int64
	}{
		{"opus55 low", "claude-opus-5-5", "low", "", "low", "adaptive", 0},
		{"opus55 provider default", "claude-opus-5-5", "", "", "medium", "adaptive", 0},
		{"opus55 explicit wins", "claude-opus-5-5", "low", "high", "high", "adaptive", 0},
		{"legacy low unchanged", "claude-opus-4-6", "low", "", "low", "enabled", 1024},
		{"legacy high budget", "claude-opus-4-6", "high", "", "high", "enabled", 10240},
	}
	for _, protocol := range []string{"chat_completions", "responses"} {
		for _, fromRepo := range []bool{false, true} {
			for _, tc := range cases {
				t.Run(fmt.Sprintf("%s/repo=%t/%s", protocol, fromRepo, tc.name), func(t *testing.T) {
					request := map[string]any{"model": "public-opus", "max_tokens": 256, "max_output_tokens": 256}
					if protocol == "chat_completions" {
						request["messages"] = []map[string]string{{"role": "user", "content": "hello"}}
						if tc.clientEffort != "" {
							request["reasoning_effort"] = tc.clientEffort
						}
					} else {
						request["input"] = "hello"
						if tc.clientEffort != "" {
							request["reasoning"] = map[string]string{"effort": tc.clientEffort}
						}
					}
					body, err := json.Marshal(request)
					require.NoError(t, err)
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest("POST", "/v1/"+protocol, strings.NewReader(string(body)))
					upstream := &anthropicHTTPUpstreamRecorder{resp: nativeAnthropicStreamResponse()}
					svc := newForwardPartialUsageServiceForTest(upstream)
					account := newAnthropicAPIKeyAccountForTest()
					account.Credentials["model_mapping"] = map[string]any{"public-opus": tc.model}
					group := &Group{ID: 84, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true, DefaultReasoningEffort: tc.groupEffort}
					repo := &mockGroupRepoForGateway{groups: map[int64]*Group{group.ID: group}}
					svc.groupRepo = repo
					ctx := context.Background()
					if !fromRepo {
						ctx = svc.withGroupContext(ctx, group)
					}
					parsed := &ParsedRequest{GroupID: &group.ID}
					var result *ForwardResult
					if protocol == "chat_completions" {
						result, err = svc.ForwardAsChatCompletions(ctx, c, account, body, parsed)
					} else {
						result, err = svc.ForwardAsResponses(ctx, c, account, body, parsed)
					}
					require.NoError(t, err)
					require.Equal(t, tc.model, gjson.GetBytes(upstream.lastBody, "model").String())
					require.Equal(t, tc.wantThinking, gjson.GetBytes(upstream.lastBody, "thinking.type").String())
					require.Equal(t, tc.wantEffort, gjson.GetBytes(upstream.lastBody, "output_config.effort").String())
					require.Equal(t, tc.wantBudget, gjson.GetBytes(upstream.lastBody, "thinking.budget_tokens").Int())
					if tc.wantBudget == 0 {
						require.False(t, gjson.GetBytes(upstream.lastBody, "thinking.budget_tokens").Exists())
					} else {
						require.Greater(t, gjson.GetBytes(upstream.lastBody, "max_tokens").Int(), tc.wantBudget)
					}
					require.NotNil(t, result.ReasoningEffort)
					require.Equal(t, tc.wantEffort, *result.ReasoningEffort)
					wantReads := 0
					if fromRepo {
						wantReads = 1
					}
					require.Equal(t, wantReads, repo.getByIDLiteCalls)
				})
			}
		}
	}
}

func TestForward_GroupDefaultUsesMappedOpus55(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	upstream := &anthropicHTTPUpstreamRecorder{resp: nativeAnthropicStreamResponse()}
	svc := newForwardPartialUsageServiceForTest(upstream)
	account := newAnthropicAPIKeyAccountForTest()
	account.Extra["anthropic_passthrough"] = false
	account.Credentials["model_mapping"] = map[string]any{"public-opus": "claude-opus-5-5"}
	group := &Group{ID: 84, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true, DefaultReasoningEffort: "low"}
	parsed, err := ParseGatewayRequest(NewRequestBodyRef([]byte(`{"model":"public-opus","max_tokens":256,"stream":true,"messages":[{"role":"user","content":"hello"}]}`)), PlatformAnthropic)
	require.NoError(t, err)
	parsed.GroupID = &group.ID
	_, err = svc.Forward(svc.withGroupContext(context.Background(), group), c, account, parsed)
	require.NoError(t, err)
	require.Equal(t, "adaptive", gjson.GetBytes(upstream.lastBody, "thinking.type").String())
	require.Equal(t, "low", gjson.GetBytes(upstream.lastBody, "output_config.effort").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "thinking.budget_tokens").Exists())
	require.Equal(t, int64(256), gjson.GetBytes(upstream.lastBody, "max_tokens").Int())
}
