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
					request := map[string]any{"model": "public-opus", "max_tokens": 65536, "max_output_tokens": 65536}
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

// Verify the emitted request, after model mapping and group defaults, rather than
// only testing the budget helper in isolation.
func TestGatewayGroupDefaultPreservesOutputAllowance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"messages", "chat_completions", "responses"} {
		for _, stream := range []bool{true, false} {
			for _, tc := range []struct {
				name, model         string
				limit               *int
				metadata            int64
				wantMax, wantBudget int64
				wantThinking        string
			}{
				{name: "missing limit uses final model capability", model: "claude-opus-4-6", wantMax: 128000, wantBudget: 10240, wantThinking: "enabled"},
				{name: "selected account capability", model: "private-model", metadata: 96000, wantMax: 96000, wantBudget: 10240, wantThinking: "enabled"},
				{name: "small capability skips automatic thinking", model: "private-model", metadata: 512, wantMax: 512},
				{name: "unknown limit remains absent", model: "private-model", wantBudget: 10240, wantThinking: "enabled"},
				{name: "explicit large limit", model: "claude-opus-4-6", limit: groupOutputLimitPtr(262144), wantMax: 262144, wantBudget: 10240, wantThinking: "enabled"},
				{name: "automatic thinking fits explicit limit", model: "claude-opus-4-6", limit: groupOutputLimitPtr(4096), wantMax: 4096, wantBudget: 4095, wantThinking: "enabled"},
				{name: "tiny limit skips automatic thinking", model: "claude-opus-4-6", limit: groupOutputLimitPtr(256), wantMax: 256},
			} {
				t.Run(fmt.Sprintf("%s/stream=%t/%s", protocol, stream, tc.name), func(t *testing.T) {
					request := map[string]any{"model": "public-opus", "stream": stream}
					field := "max_tokens"
					if protocol == "responses" {
						request["input"] = "hello"
						field = "max_output_tokens"
					} else {
						request["messages"] = []map[string]string{{"role": "user", "content": "hello"}}
					}
					if tc.limit != nil {
						request[field] = *tc.limit
					}
					body, err := json.Marshal(request)
					require.NoError(t, err)
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest("POST", "/v1/"+protocol, strings.NewReader(string(body)))
					response := nativeAnthropicStreamResponse()
					if protocol == "messages" && !stream {
						response = nativeAnthropicBufferedResponse()
					}
					upstream := &anthropicHTTPUpstreamRecorder{resp: response}
					svc := newForwardPartialUsageServiceForTest(upstream)
					account := newAnthropicAPIKeyAccountForTest()
					account.Extra["anthropic_passthrough"] = false
					account.Credentials["model_mapping"] = map[string]any{"public-opus": tc.model}
					if tc.metadata > 0 {
						account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{tc.model: {ID: tc.model, MaxOutputTokens: tc.metadata}}})
					}
					group := &Group{ID: 84, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true, DefaultReasoningEffort: "high"}
					parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
					require.NoError(t, err)
					parsed.GroupID = &group.ID
					ctx := svc.withGroupContext(context.Background(), group)
					switch protocol {
					case "messages":
						_, err = svc.Forward(ctx, c, account, parsed)
					case "chat_completions":
						_, err = svc.ForwardAsChatCompletions(ctx, c, account, body, parsed)
					case "responses":
						_, err = svc.ForwardAsResponses(ctx, c, account, body, parsed)
					}
					require.NoError(t, err)
					require.Equal(t, tc.model, gjson.GetBytes(upstream.lastBody, "model").String())
					require.Equal(t, tc.wantMax, gjson.GetBytes(upstream.lastBody, "max_tokens").Int())
					require.Equal(t, tc.wantMax > 0, gjson.GetBytes(upstream.lastBody, "max_tokens").Exists())
					require.Equal(t, tc.wantBudget, gjson.GetBytes(upstream.lastBody, "thinking.budget_tokens").Int())
					require.Equal(t, tc.wantThinking, gjson.GetBytes(upstream.lastBody, "thinking.type").String())
					require.Equal(t, tc.limit != nil, gjson.GetBytes(body, field).Exists(), "group defaults must not mutate ingress bytes")
				})
			}
		}
	}
}
func groupOutputLimitPtr(value int) *int { return &value }
