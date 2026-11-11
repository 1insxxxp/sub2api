//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestForwardAsChatCompletions_AccountModelSystemPrompt(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, match := range []bool{false, true} {
			t.Run(fmt.Sprintf("stream=%t/match=%t", stream, match), func(t *testing.T) {
				account := newAnthropicAPIKeyAccountForTest()
				account.Credentials["model_mapping"] = map[string]any{"ccA2/public-opus": "claude-opus-4-6"}
				promptModel := "claude-sonnet-4-6"
				if match {
					promptModel = "claude-opus-4-6"
				}
				account.ModelSystemPrompts = map[string]string{promptModel: "Keep emotional continuity for {{user}}."}
				body := []byte(fmt.Sprintf(`{"model":"ccA2/public-opus","max_tokens":8192,"stream":%t,"messages":[{"role":"system","content":"Client character remains reserved."},{"role":"user","content":"Continue."}]}`, stream))
				forwarded, _, err := forwardClaudeCCControlsForTest(t, body, account)
				require.NoError(t, err)
				require.Equal(t, "claude-opus-4-6", gjson.GetBytes(forwarded, "model").String())
				require.Contains(t, string(forwarded), "Client character remains reserved.")
				if match {
					system := gjson.GetBytes(forwarded, "system").Raw
					require.Contains(t, system, "Keep emotional continuity for {{user}}.")
					require.Less(t, strings.Index(system, "Keep emotional continuity"), strings.Index(system, "Client character"))
				} else {
					require.NotContains(t, string(forwarded), "Keep emotional continuity")
				}
			})
		}
	}
}

func TestForwardAsChatCompletions_ExplicitAnthropicControls(t *testing.T) {
	cases := []struct {
		name     string
		model    string
		controls string
		thinking string
		budget   int64
		effort   string
	}{
		{name: "default low", thinking: "enabled", budget: 1024, effort: "low"},
		{name: "null remains default", controls: `"thinking":null,"output_config":null,`, thinking: "enabled", budget: 1024, effort: "low"},
		{name: "enabled budget", controls: `"thinking":{"type":"enabled","budget_tokens":4096},`, thinking: "enabled", budget: 4096},
		{name: "adaptive and effort", controls: `"thinking":{"type":"adaptive","display":"omitted"},"output_config":{"effort":"medium"},`, thinking: "adaptive", effort: "medium"},
		{name: "effort without thinking", controls: `"output_config":{"effort":"high"},`, effort: "high"},
		{name: "disabled wins", controls: `"thinking":{"type":"disabled","budget_tokens":4096},"output_config":{"effort":"medium"},"reasoning_effort":"high",`, thinking: "disabled"},
		{name: "explicit controls win over reasoning alias", controls: `"thinking":{"type":"enabled","budget_tokens":2048},"output_config":{"effort":"medium"},"reasoning_effort":"high",`, thinking: "enabled", budget: 2048, effort: "medium"},
		{name: "opus55 native effort", model: "claude-opus-5-5", controls: `"thinking":{"type":"adaptive"},"output_config":{"effort":"high"},`, thinking: "adaptive", effort: "high"},
		{name: "native enabled overrides none alias", controls: `"thinking":{"type":"enabled","budget_tokens":2048},"reasoning_effort":"none",`, thinking: "enabled", budget: 2048},
		{name: "native adaptive overrides none alias", controls: `"thinking":{"type":"adaptive"},"reasoning_effort":"none",`, thinking: "adaptive"},
		{name: "native effort avoids legacy thinking", controls: `"output_config":{"effort":"medium"},"reasoning_effort":"high",`, effort: "medium"},
	}
	for _, stream := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("stream=%t/%s", stream, tc.name), func(t *testing.T) {
				model := tc.model
				if model == "" {
					model = "claude-opus-4-6"
				}
				body := []byte(fmt.Sprintf(`{"model":%q,"max_tokens":8192,"stream":%t,%s"messages":[{"role":"user","content":"Continue."}]}`, model, stream, tc.controls))
				forwarded, _, err := forwardClaudeCCControlsForTest(t, body, newAnthropicAPIKeyAccountForTest())
				require.NoError(t, err)
				require.Equal(t, tc.thinking, gjson.GetBytes(forwarded, "thinking.type").String())
				require.Equal(t, tc.budget, gjson.GetBytes(forwarded, "thinking.budget_tokens").Int())
				require.Equal(t, tc.effort, gjson.GetBytes(forwarded, "output_config.effort").String())
				require.EqualValues(t, 8192, gjson.GetBytes(forwarded, "max_tokens").Int())
				if tc.name == "adaptive and effort" {
					require.Equal(t, "omitted", gjson.GetBytes(forwarded, "thinking.display").String())
				}
			})
		}
	}
}

func TestForwardAsChatCompletions_ThirdPartyNativeThinking(t *testing.T) {
	for _, model := range []string{"kimi-k3", "private-story-model"} {
		for _, controls := range []string{`"thinking":{"type":"enabled"},`, `"thinking":{"type":"enabled","budget_tokens":128},`} {
			t.Run(model+"/"+controls, func(t *testing.T) {
				body := []byte(fmt.Sprintf(`{"model":%q,"max_tokens":8192,%s"messages":[{"role":"user","content":"Continue."}]}`, model, controls))
				forwarded, _, err := forwardClaudeCCControlsForTest(t, body, newAnthropicAPIKeyAccountForTest())
				require.NoError(t, err)
				require.JSONEq(t, gjson.GetBytes(body, "thinking").Raw, gjson.GetBytes(forwarded, "thinking").Raw)
				require.EqualValues(t, 8192, gjson.GetBytes(forwarded, "max_tokens").Int())
			})
		}
	}
}

func TestForwardAsChatCompletions_ContinuationPreservesManualBudget(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, remaining := range []int{1096, 1024, 2048, 2049} {
			t.Run(fmt.Sprintf("stream=%t/remaining=%d", stream, remaining), func(t *testing.T) {
				responses := []*http.Response{}
				for i, text := range []string{"alpha", "beta"} {
					reason, outputTokens := "max_tokens", 4096-remaining
					if i == 1 {
						reason, outputTokens = "end_turn", 5
					}
					responses = append(responses, &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(anthropicTextStream(text, reason, 100, outputTokens)))})
				}
				upstream := &httpUpstreamRecorder{responses: responses}
				svc := &GatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}}
				account := newAnthropicAPIKeyAccountForTest()
				account.Extra["anti_truncation"] = accountContinuationTestAccount(true).Extra["anti_truncation"]
				account.Credentials["model_mapping"] = map[string]any{"long-text": "claude-opus-4-6"}
				body := []byte(fmt.Sprintf(`{"model":"long-text","max_tokens":4096,"stream":%t,"thinking":{"type":"enabled","budget_tokens":2048},"messages":[{"role":"user","content":"write"}]}`, stream))
				c, rec := newGeminiOutputLimitContext(body)
				result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, nil)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rec.Code)
				require.Contains(t, rec.Body.String(), "alpha")
				require.NotContains(t, rec.Body.String(), `"error"`)
				if remaining <= 2048 {
					require.Len(t, upstream.bodies, 1)
					require.Equal(t, 4096-remaining, result.Usage.OutputTokens)
					require.Contains(t, rec.Body.String(), `"finish_reason":"length"`)
					return
				}
				require.Len(t, upstream.bodies, 2)
				require.EqualValues(t, 2048, gjson.GetBytes(upstream.bodies[1], "thinking.budget_tokens").Int())
				require.EqualValues(t, remaining, gjson.GetBytes(upstream.bodies[1], "max_tokens").Int())
				require.EqualValues(t, 4096, gjson.GetBytes(body, "max_tokens").Int())
			})
		}
	}
}

func TestForwardAsChatCompletions_ContinuationRejectsInvalidInitialBudget(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: nativeAnthropicStreamResponse()}
	svc := &GatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}}
	account := newAnthropicAPIKeyAccountForTest()
	account.Extra["anti_truncation"] = accountContinuationTestAccount(true).Extra["anti_truncation"]
	account.Credentials["model_mapping"] = map[string]any{"long-text": "claude-opus-4-6"}
	body := []byte(`{"model":"long-text","max_tokens":4096,"thinking":{"type":"enabled","budget_tokens":4096},"messages":[{"role":"user","content":"write"}]}`)
	c, rec := newGeminiOutputLimitContext(body)
	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, nil)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, upstream.bodies)
}

func TestForwardAsChatCompletions_ThirdPartyContinuationKeepsProviderBudget(t *testing.T) {
	for _, model := range []string{"kimi-k3", "private-story-model"} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", model, stream), func(t *testing.T) {
				responses := []*http.Response{}
				for i, text := range []string{"alpha", "beta"} {
					reason, outputTokens := "max_tokens", 4000
					if i == 1 {
						reason, outputTokens = "end_turn", 5
					}
					responses = append(responses, &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(anthropicTextStream(text, reason, 100, outputTokens)))})
				}
				upstream := &httpUpstreamRecorder{responses: responses}
				svc := &GatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}}
				account := newAnthropicAPIKeyAccountForTest()
				account.Extra["anti_truncation"] = accountContinuationTestAccount(true).Extra["anti_truncation"]
				account.Credentials["model_mapping"] = map[string]any{"long-text": model}
				body := []byte(fmt.Sprintf(`{"model":"long-text","max_tokens":4096,"stream":%t,"thinking":{"type":"enabled","budget_tokens":128},"messages":[{"role":"user","content":"write"}]}`, stream))
				c, rec := newGeminiOutputLimitContext(body)
				result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, nil)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rec.Code)
				require.Len(t, upstream.bodies, 2)
				require.EqualValues(t, 96, gjson.GetBytes(upstream.bodies[1], "max_tokens").Int())
				require.EqualValues(t, 128, gjson.GetBytes(upstream.bodies[1], "thinking.budget_tokens").Int())
				require.Equal(t, 4005, result.Usage.OutputTokens)
				require.Contains(t, rec.Body.String(), "beta")
			})
		}
	}
}

func TestForwardAsChatCompletions_StopSequences(t *testing.T) {
	cases := []struct {
		name string
		stop string
		want []string
	}{
		{name: "string", stop: `"\nUser:"`, want: []string{"\nUser:"}},
		{name: "list", stop: `["\nUser:","<end>"]`, want: []string{"\nUser:", "<end>"}},
		{name: "null", stop: `null`},
		{name: "empty list", stop: `[]`},
	}
	for _, stream := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("stream=%t/%s", stream, tc.name), func(t *testing.T) {
				body := []byte(fmt.Sprintf(`{"model":"claude-opus-4-6","max_tokens":8192,"stream":%t,"stop":%s,"messages":[{"role":"user","content":"Continue."}]}`, stream, tc.stop))
				forwarded, _, err := forwardClaudeCCControlsForTest(t, body, newAnthropicAPIKeyAccountForTest())
				require.NoError(t, err)
				var got []string
				for _, value := range gjson.GetBytes(forwarded, "stop_sequences").Array() {
					got = append(got, value.String())
				}
				require.Equal(t, tc.want, got)
			})
		}
	}
}

func TestForwardAsChatCompletions_Claude55NativeReasoningPriority(t *testing.T) {
	cases := []struct {
		name      string
		model     string
		controls  string
		effort    string
		wantError bool
	}{
		{name: "native effort overrides none alias", model: "claude-opus-5-5", controls: `"thinking":{"type":"adaptive"},"output_config":{"effort":"high"},"reasoning_effort":"none",`, effort: "high"},
		{name: "effort alone overrides none alias", model: "claude-opus-5-5", controls: `"output_config":{"effort":"high"},"reasoning_effort":"none",`, effort: "high"},
		{name: "native adaptive skips group low", model: "claude-opus-5-5", controls: `"thinking":{"type":"adaptive"},`},
		{name: "native adaptive overrides none alias", model: "claude-opus-5-5", controls: `"thinking":{"type":"adaptive"},"reasoning_effort":"none",`},
		{name: "valid alias still supplies effort", model: "claude-opus-5-5", controls: `"thinking":{"type":"adaptive"},"reasoning_effort":"high",`, effort: "high"},
		{name: "sonnet native adaptive keeps default omitted", model: "claude-sonnet-5-5", controls: `"thinking":{"type":"adaptive"},`},
		{name: "sonnet native adaptive overrides none alias", model: "claude-sonnet-5-5", controls: `"thinking":{"type":"adaptive"},"reasoning_effort":"none",`},
		{name: "haiku native adaptive keeps default omitted", model: "claude-haiku-5-5", controls: `"thinking":{"type":"adaptive"},`},
		{name: "sonnet between tools rejects xhigh alias", model: "claude-sonnet-5-5", controls: `"thinking":{"type":"between_tools"},"reasoning_effort":"xhigh",`, wantError: true},
		{name: "legacy none without native controls still rejected", model: "claude-opus-5-5", controls: `"reasoning_effort":"none",`, wantError: true},
	}
	for _, stream := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("stream=%t/%s", stream, tc.name), func(t *testing.T) {
				body := []byte(fmt.Sprintf(`{"model":%q,"max_tokens":8192,"stream":%t,%s"messages":[{"role":"user","content":"Continue."}]}`, tc.model, stream, tc.controls))
				forwarded, rec, err := forwardClaudeCCControlsForTest(t, body, newAnthropicAPIKeyAccountForTest())
				if tc.wantError {
					require.Error(t, err)
					require.Equal(t, http.StatusBadRequest, rec.Code)
					require.Empty(t, forwarded)
					return
				}
				require.NoError(t, err)
				require.Equal(t, "adaptive", gjson.GetBytes(forwarded, "thinking.type").String())
				require.Equal(t, tc.effort, gjson.GetBytes(forwarded, "output_config.effort").String())
				require.EqualValues(t, 8192, gjson.GetBytes(forwarded, "max_tokens").Int())
			})
		}
	}
}

func TestForwardAsChatCompletions_InvalidAnthropicControls(t *testing.T) {
	cases := map[string]string{
		"thinking string":          `"thinking":"enabled",`,
		"thinking missing type":    `"thinking":{"budget_tokens":2048},`,
		"thinking unknown type":    `"thinking":{"type":"invalid"},`,
		"thinking budget type":     `"thinking":{"type":"enabled","budget_tokens":"2048"},`,
		"thinking budget absent":   `"thinking":{"type":"enabled"},`,
		"thinking budget over cap": `"thinking":{"type":"enabled","budget_tokens":8192},`,
		"output config string":     `"output_config":"high",`,
		"effort number":            `"output_config":{"effort":2},`,
		"effort invalid":           `"output_config":{"effort":"invalid"},`,
		"stop number":              `"stop":7,`,
		"stop object":              `"stop":{"value":"User:"},`,
		"stop mixed list":          `"stop":["User:",7],`,
		"stop empty string":        `"stop":"",`,
	}
	for name, controls := range cases {
		t.Run(name, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"model":"claude-opus-4-6","max_tokens":8192,%s"messages":[{"role":"user","content":"Continue."}]}`, controls))
			forwarded, rec, err := forwardClaudeCCControlsForTest(t, body, newAnthropicAPIKeyAccountForTest())
			require.Error(t, err)
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Empty(t, forwarded, "invalid controls must be rejected before upstream dispatch")
		})
	}
}

func forwardClaudeCCControlsForTest(t *testing.T, body []byte, account *Account) ([]byte, *httptest.ResponseRecorder, error) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	delete(account.Extra, "anthropic_passthrough")
	upstream := &anthropicHTTPUpstreamRecorder{resp: nativeAnthropicStreamResponse()}
	svc := &GatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
	group := &Group{ID: 84, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true, DefaultReasoningEffort: "low"}
	ctx := svc.withGroupContext(context.Background(), group)
	_, err := svc.ForwardAsChatCompletions(ctx, c, account, body, &ParsedRequest{GroupID: &group.ID})
	return upstream.lastBody, rec, err
}
