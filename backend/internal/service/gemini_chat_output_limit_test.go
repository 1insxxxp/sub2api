//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGeminiChatOutputLimitPreservesClientIntent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{false, true} {
		for _, tc := range []struct {
			name     string
			extra    string
			want     int64
			explicit bool
		}{
			{name: "omitted"},
			{name: "null", extra: `,"max_tokens":null`},
			{name: "explicit zero", extra: `,"max_tokens":0`, explicit: true},
			{name: "explicit negative", extra: `,"max_tokens":-1`, want: -1},
			{name: "explicit small", extra: `,"max_tokens":8`, want: 8},
			{name: "explicit 8192", extra: `,"max_tokens":8192`, want: 8192},
			{name: "explicit large", extra: `,"max_tokens":32768`, want: 32768},
			{name: "completion precedence", extra: `,"max_tokens":32768,"max_completion_tokens":8192`, want: 8192},
		} {
			t.Run(tc.name+"/stream="+boolString(stream), func(t *testing.T) {
				body := []byte(`{"model":"paygo-vet-4a","stream":` + boolString(stream) + tc.extra + `,"messages":[{"role":"user","content":"Write a story."}]}`)
				original := string(body)
				upstream := &continuationBudgetUpstream{responses: []*http.Response{geminiOutputLimitResponse(t, stream, "hello", 5, "STOP")}}
				svc := &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: upstream}
				c, recorder := newGeminiOutputLimitContext(body)
				result, err := svc.ForwardAsChatCompletions(context.Background(), c, geminiOutputLimitAccount(), body)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, recorder.Code)
				require.Len(t, upstream.bodies, 1, "changing a default must not generate a second request")
				limit := gjson.GetBytes(upstream.bodies[0], "generationConfig.maxOutputTokens")
				if tc.want == 0 && !tc.explicit {
					require.False(t, limit.Exists(), "an intermediate Claude request must not add a Gemini output limit")
				} else {
					require.True(t, limit.Exists())
					require.Equal(t, tc.want, limit.Int())
				}
				require.Equal(t, "gemini-4-argon", result.UpstreamModel)
				require.Equal(t, 5, result.Usage.OutputTokens)
				require.Equal(t, original, string(body), "reservation and continuation read the original client body")
			})
		}
	}
}

func TestGeminiChatOutputLimitForwardsBeyond8192(t *testing.T) {
	gin.SetMode(gin.TestMode)
	text := strings.Repeat("word ", 10000)
	for _, stream := range []bool{false, true} {
		t.Run("stream="+boolString(stream), func(t *testing.T) {
			body := []byte(`{"model":"paygo-vet-4a","stream":` + boolString(stream) + `,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"Write a long story."}]}`)
			upstream := &continuationBudgetUpstream{responses: []*http.Response{geminiOutputLimitResponse(t, stream, text, 10000, "STOP")}}
			svc := &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: upstream}
			c, recorder := newGeminiOutputLimitContext(body)
			result, err := svc.ForwardAsChatCompletions(context.Background(), c, geminiOutputLimitAccount(), body)
			require.NoError(t, err)
			require.Len(t, upstream.bodies, 1)
			require.False(t, gjson.GetBytes(upstream.bodies[0], "generationConfig.maxOutputTokens").Exists())
			require.Equal(t, 10000, result.Usage.OutputTokens)
			require.Equal(t, 200, result.Usage.InputTokens)
			if !stream {
				require.Equal(t, text, gjson.GetBytes(recorder.Body.Bytes(), "choices.0.message.content").String())
				require.Equal(t, "stop", gjson.GetBytes(recorder.Body.Bytes(), "choices.0.finish_reason").String())
				return
			}
			var delivered strings.Builder
			var completionTokens int64
			var finish string
			for _, line := range strings.Split(recorder.Body.String(), "\n") {
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				chunk := strings.TrimPrefix(line, "data: ")
				delivered.WriteString(gjson.Get(chunk, "choices.0.delta.content").String())
				if value := gjson.Get(chunk, "choices.0.finish_reason").String(); value != "" {
					finish = value
				}
				if usage := gjson.Get(chunk, "usage.completion_tokens"); usage.Exists() {
					completionTokens = usage.Int()
				}
			}
			require.Equal(t, text, delivered.String())
			require.Equal(t, "stop", finish)
			require.Equal(t, int64(10000), completionTokens)
		})
	}
}

func geminiOutputLimitAccount() *Account {
	return &Account{ID: 775, Platform: PlatformGemini, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "fake-local-only", "model_mapping": map[string]any{"paygo-vet-4a": "gemini-4-argon"}}}
}

func newGeminiOutputLimitContext(body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
	return c, recorder
}

func geminiOutputLimitResponse(t *testing.T, stream bool, text string, tokens int, finish string) *http.Response {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"candidates":    []any{map[string]any{"content": map[string]any{"parts": []any{map[string]string{"text": text}}}, "finishReason": finish}},
		"usageMetadata": map[string]int{"promptTokenCount": 200, "candidatesTokenCount": tokens, "totalTokenCount": 200 + tokens},
	})
	require.NoError(t, err)
	contentType := "application/json"
	if stream {
		contentType = "text/event-stream"
		payload = []byte("data: " + string(payload) + "\n\n")
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(string(payload)))}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
