//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGeminiChatOutputUsageProjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, parts, metadata             string
		wireOutput, providerOutput, cache int
	}{
		{"missing candidates", `{"text":"abcdefgh"}`, `{"promptTokenCount":10,"totalTokenCount":10}`, 2, 0, 0},
		{"zero candidates", `{"text":"abcdefgh"}`, `{"promptTokenCount":10,"candidatesTokenCount":0,"totalTokenCount":10}`, 2, 0, 0},
		{"missing metadata", `{"text":"abcdefgh"}`, `{}`, 2, 0, 0},
		{"measured candidates", `{"text":"abcdefgh"}`, `{"promptTokenCount":10,"candidatesTokenCount":73,"totalTokenCount":83}`, 73, 73, 0},
		{"measured thinking", `{"text":"abcdefgh"}`, `{"promptTokenCount":10,"candidatesTokenCount":0,"thoughtsTokenCount":5,"totalTokenCount":15}`, 5, 5, 0},
		{"tool output", `{"functionCall":{"name":"tool","args":{}}}`, `{"promptTokenCount":10,"candidatesTokenCount":0}`, 2, 0, 0},
		{"cached input", `{"text":"abcdefgh"}`, `{"promptTokenCount":10,"cachedContentTokenCount":4,"candidatesTokenCount":0,"totalTokenCount":10}`, 2, 0, 4},
		{"cached measured output", `{"text":"abcdefgh"}`, `{"promptTokenCount":10,"cachedContentTokenCount":4,"candidatesTokenCount":73,"totalTokenCount":83}`, 73, 73, 4},
	} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", tc.name, stream), func(t *testing.T) {
				raw := `{"candidates":[{"content":{"role":"model","parts":[` + tc.parts + `]},"finishReason":"STOP"}],"usageMetadata":` + tc.metadata + `}`
				contentType := "application/json"
				if stream {
					contentType = "text/event-stream"
					// Content is sent before the terminal frame, as in actual streams.
					raw = "data: " + `{"candidates":[{"content":{"role":"model","parts":[` + tc.parts + `]}}]}` + "\n\n" +
						"data: " + `{"candidates":[{"finishReason":"STOP"}],"usageMetadata":` + tc.metadata + `}` + "\n\n"
				}
				svc := newGeminiSignalService(contentType, raw)
				body := []byte(fmt.Sprintf(`{"model":"gemini-test","stream":%t,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hi"}]}`, stream))
				c, rec := newAntigravityCompatContext(http.MethodPost, "/v1/chat/completions", body)
				result, err := svc.ForwardAsChatCompletions(context.Background(), c, geminiSignalTestAccount(), body)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rec.Code)
				require.Equal(t, tc.providerOutput, result.Usage.OutputTokens, "client projection must not change upstream accounting evidence")
				require.Equal(t, tc.cache, result.Usage.CacheReadInputTokens)
				wire := gjson.Parse(rec.Body.String())
				if stream {
					wire = geminiChatLastUsageFrame(t, rec.Body.String())
					require.Contains(t, rec.Body.String(), "data: [DONE]")
				}
				u := wire.Get("usage")
				require.EqualValues(t, tc.wireOutput, u.Get("completion_tokens").Int())
				require.EqualValues(t, result.Usage.InputTokens+tc.cache, u.Get("prompt_tokens").Int())
				require.EqualValues(t, u.Get("prompt_tokens").Int()+int64(tc.wireOutput), u.Get("total_tokens").Int())
			})
		}
	}
}

func TestGeminiChatOutputUsagePreservesInputDiscount(t *testing.T) {
	for _, stream := range []bool{false, true} {
		result, rec := callGeminiInputCountingForward(t, "chat", stream, true,
			`{"promptTokenCount":1000,"cachedContentTokenCount":0,"candidatesTokenCount":0,"totalTokenCount":1000}`)
		require.Zero(t, result.Usage.OutputTokens)
		require.Equal(t, 1000, result.Usage.InputTokens)
		wire := gjson.Parse(rec.Body.String())
		if stream {
			wire = geminiChatLastUsageFrame(t, rec.Body.String())
		}
		u := wire.Get("usage")
		require.NotNil(t, result.GeminiInputCounting)
		require.EqualValues(t, result.GeminiInputCounting.CandidateInputTokens, u.Get("prompt_tokens").Int())
		require.Positive(t, u.Get("completion_tokens").Int())
		require.Equal(t, u.Get("prompt_tokens").Int()+u.Get("completion_tokens").Int(), u.Get("total_tokens").Int())
	}
}

func TestGeminiChatOutputUsageDoesNotInventTextForEmptyOrMediaResponse(t *testing.T) {
	for _, parts := range []string{`[]`, `[{"text":"thinking only","thought":true}]`, `[{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}]`} {
		raw := []byte(`{"candidates":[{"content":{"parts":` + parts + `},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10}}`)
		var response map[string]any
		require.NoError(t, json.Unmarshal(raw, &response))
		chat, usage, err := geminiResponseToChatCompletions(response, "gemini-test", raw, nil, nil)
		require.NoError(t, err)
		require.Zero(t, usage.OutputTokens)
		require.Zero(t, chat.Usage.CompletionTokens)
	}
}

func TestGeminiChatOutputUsageStreamAccumulatesUniqueOutput(t *testing.T) {
	frame := `data: {"candidates":[{"content":{"parts":[{"text":"abcdefgh"}]}}],"usageMetadata":{"promptTokenCount":10}}` + "\n\n"
	svc := newGeminiSignalService("text/event-stream", frame+frame+`data: {"candidates":[{"finishReason":"STOP"}],"usageMetadata":{"cachedContentTokenCount":4}}`+"\n\n")
	body := []byte(`{"model":"gemini-test","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hi"}]}`)
	c, rec := newAntigravityCompatContext(http.MethodPost, "/v1/chat/completions", body)
	result, err := svc.ForwardAsChatCompletions(context.Background(), c, geminiSignalTestAccount(), body)
	require.NoError(t, err)
	require.Zero(t, result.Usage.OutputTokens)
	u := geminiChatLastUsageFrame(t, rec.Body.String()).Get("usage")
	require.EqualValues(t, 2, u.Get("completion_tokens").Int(), "cumulative frames must not double count output")
	require.EqualValues(t, 10, u.Get("prompt_tokens").Int())
	require.EqualValues(t, 12, u.Get("total_tokens").Int())
}

func TestGeminiChatOutputUsageDoesNotForceUsageOption(t *testing.T) {
	svc := newGeminiSignalService("text/event-stream", `data: {"candidates":[{"content":{"parts":[{"text":"abcdefgh"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10}}`+"\n\n")
	body := []byte(`{"model":"gemini-test","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	c, rec := newAntigravityCompatContext(http.MethodPost, "/v1/chat/completions", body)
	_, err := svc.ForwardAsChatCompletions(context.Background(), c, geminiSignalTestAccount(), body)
	require.NoError(t, err)
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if payload, ok := strings.CutPrefix(line, "data: "); ok {
			require.False(t, gjson.Get(payload, "usage").IsObject())
		}
	}
}

func TestGeminiChatOutputUsageCollectedResponse(t *testing.T) {
	raw := []byte(`{"candidates":[{"content":{"parts":[{"text":"abcdefgh"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10}}`)
	var response map[string]any
	require.NoError(t, json.Unmarshal(raw, &response))
	for _, reported := range []int{0, 43} {
		provider := &ClaudeUsage{InputTokens: 10, OutputTokens: reported}
		chat, usage, err := geminiResponseToChatCompletions(response, "gemini-test", raw, provider, nil)
		require.NoError(t, err)
		require.Same(t, provider, usage)
		require.Equal(t, reported, usage.OutputTokens)
		require.Equal(t, max(2, reported), chat.Usage.CompletionTokens)
		require.Equal(t, 10+max(2, reported), chat.Usage.TotalTokens)
	}
}

func TestGeminiChatOutputUsageCollectedSSEDeduplicatesCumulativeFrames(t *testing.T) {
	frame := `data: {"response":{"candidates":[{"content":{"parts":[{"text":"abcdefgh"}]}}]}}` + "\n\n"
	svc := newGeminiSignalService("text/event-stream", frame+frame+`data: {"response":{"candidates":[{"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":0}}}`+"\n\n")
	svc.tokenProvider = NewGeminiTokenProvider(nil, nil, nil)
	account := geminiSignalTestAccount()
	account.Type = AccountTypeOAuth
	account.Credentials = map[string]any{"access_token": "test-token", "project_id": "test-project"}
	body := []byte(`{"model":"gemini-test","stream":false,"messages":[{"role":"user","content":"hi"}]}`)
	c, rec := newAntigravityCompatContext(http.MethodPost, "/v1/chat/completions", body)
	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body)
	require.NoError(t, err)
	require.Zero(t, result.Usage.OutputTokens)
	u := gjson.Get(rec.Body.String(), "usage")
	require.EqualValues(t, 2, u.Get("completion_tokens").Int())
	require.EqualValues(t, 12, u.Get("total_tokens").Int())
}

func geminiChatLastUsageFrame(t *testing.T, sse string) gjson.Result {
	t.Helper()
	var last gjson.Result
	for _, line := range strings.Split(sse, "\n") {
		if payload, ok := strings.CutPrefix(line, "data: "); ok && gjson.Get(payload, "usage").IsObject() {
			last = gjson.Parse(payload)
		}
	}
	require.True(t, last.Exists(), "requested usage must be emitted before [DONE]")
	return last
}
