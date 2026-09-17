//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestChatCompletionsAnthropicThinkingDisabledForwarding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const preset = "<think>preset scene outline</think>Dialogue."
	cases := []struct {
		name         string
		controls     string
		groupEffort  string
		wantThinking string
		wantEffort   string
	}{
		{name: "unspecified"},
		{name: "group default remains enabled", groupEffort: "high", wantThinking: "enabled", wantEffort: "high"},
		{name: "disabled", controls: `"thinking":{"type":"disabled"},`, wantThinking: "disabled"},
		{name: "disabled overrides group default", controls: `"thinking":{"type":"disabled"},`, groupEffort: "high", wantThinking: "disabled"},
		{name: "disabled overrides reasoning effort", controls: `"thinking":{"type":"disabled"},"reasoning_effort":"high",`, wantThinking: "disabled"},
		{name: "reasoning remains enabled", controls: `"reasoning_effort":"high",`, wantThinking: "enabled", wantEffort: "high"},
	}
	for _, native := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			for _, tt := range cases {
				if native && tt.groupEffort != "" {
					continue
				}
				t.Run(fmt.Sprintf("native=%t/stream=%t/%s", native, stream, tt.name), func(t *testing.T) {
					body := []byte(fmt.Sprintf(`{"model":"cursor/claude-opus-4-6","max_tokens":16384,"stream":%t,%s"messages":[{"role":"system","content":"Keep the preset outline."},{"role":"assistant","content":"%s"},{"role":"user","content":"Continue."}]}`, stream, tt.controls, preset))
					response := nativeAnthropicStreamResponse()
					sse, err := io.ReadAll(response.Body)
					require.NoError(t, err)
					response.Body = io.NopCloser(strings.NewReader(strings.ReplaceAll(string(sse), "pong", preset)))
					upstream := &httpUpstreamRecorder{resp: response}
					recorder := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(recorder)
					c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(string(body)))
					var effort *string
					if native {
						svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
						account := nativeAnthropicTestAccount()
						account.Credentials["model_mapping"] = map[string]any{"cursor/claude-opus-4-6": "claude-opus-4-6"}
						result, forwardErr := svc.forwardChatCompletionsViaNativeAnthropic(context.Background(), c, account, body, "")
						require.NoError(t, forwardErr)
						effort = result.ReasoningEffort
					} else {
						svc := &GatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}}
						account := newAnthropicAPIKeyAccountForTest()
						account.Credentials["model_mapping"] = map[string]any{"cursor/claude-opus-4-6": "claude-opus-4-6"}
						group := &Group{ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true, DefaultReasoningEffort: tt.groupEffort}
						ctx := svc.withGroupContext(context.Background(), group)
						require.Same(t, group, svc.groupFromContext(ctx, group.ID))
						result, forwardErr := svc.ForwardAsChatCompletions(ctx, c, account, body, &ParsedRequest{GroupID: &group.ID})
						require.NoError(t, forwardErr)
						effort = result.ReasoningEffort
					}

					require.Equal(t, "claude-opus-4-6", gjson.GetBytes(upstream.lastBody, "model").String())
					require.Equal(t, tt.wantThinking, gjson.GetBytes(upstream.lastBody, "thinking.type").String())
					require.Equal(t, tt.wantEffort, gjson.GetBytes(upstream.lastBody, "output_config.effort").String())
					if tt.wantThinking == "disabled" {
						require.False(t, gjson.GetBytes(upstream.lastBody, "thinking.budget_tokens").Exists())
					}
					if tt.wantEffort == "" {
						require.Nil(t, effort)
					} else {
						require.NotNil(t, effort)
						require.Equal(t, tt.wantEffort, *effort)
					}
					require.Contains(t, gjson.GetBytes(upstream.lastBody, "system").String(), "Keep the preset outline.")
					require.Contains(t, string(upstream.lastBody), "preset scene outline")

					if stream {
						var content strings.Builder
						for _, line := range strings.Split(recorder.Body.String(), "\n") {
							if !strings.HasPrefix(line, "data: ") || strings.HasSuffix(line, "[DONE]") {
								continue
							}
							chunk := strings.TrimPrefix(line, "data: ")
							content.WriteString(gjson.Get(chunk, "choices.0.delta.content").String())
							require.False(t, gjson.Get(chunk, "choices.0.delta.reasoning_content").Exists())
						}
						require.Equal(t, preset, content.String())
					} else {
						require.Equal(t, preset, gjson.GetBytes(recorder.Body.Bytes(), "choices.0.message.content").String())
						require.False(t, gjson.GetBytes(recorder.Body.Bytes(), "choices.0.message.reasoning_content").Exists())
					}
				})
			}
		}
	}
}
