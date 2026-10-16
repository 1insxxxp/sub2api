package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/gin-gonic/gin"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildRawChatContinuationBodyAppendsAssistantAndPrompt(t *testing.T) {
	body := []byte(`{"model":"public","stream":true,"max_tokens":100,"messages":[{"role":"user","content":"question"}]}`)
	got, err := buildRawChatContinuationBody(body, "partial answer", 42)
	require.NoError(t, err)
	require.Equal(t, "public", gjson.GetBytes(got, "model").String())
	require.Equal(t, int64(42), gjson.GetBytes(got, "max_tokens").Int())
	require.Equal(t, 3, len(gjson.GetBytes(got, "messages").Array()))
	require.Equal(t, "assistant", gjson.GetBytes(got, "messages.1.role").String())
	require.Equal(t, "partial answer", gjson.GetBytes(got, "messages.1.content").String())
	require.Equal(t, "Continue the response from where you left off. Do not repeat any earlier text.", gjson.GetBytes(got, "messages.2.content").String())
}

func TestStreamRawChatCompletionsContinuesLengthRoundWithoutIntermediateTerminal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var continuationBody []byte
	round := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if round == 1 {
			continuationBody, _ = io.ReadAll(r.Body)
		}
		round++
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", fmt.Sprintf("round-%d", round))
		if round == 1 {
			_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"},\"finish_reason\":null}]}\n\n"))
			_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"length\"}]}\n\n"))
			_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":3,\"total_tokens\":7}}\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			return
		}
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\" continued\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":4,\"total_tokens\":12}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	initialBody := []byte(`{"model":"public","stream":true,"max_tokens":10,"messages":[{"role":"user","content":"question"}]}`)
	initialReq, err := http.NewRequest(http.MethodPost, upstream.URL, bytes.NewReader(initialBody))
	require.NoError(t, err)
	initialResp, err := http.DefaultClient.Do(initialReq)
	require.NoError(t, err)
	defer initialResp.Body.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(initialBody))
	policy := domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 2, MaxOutputTokens: 10, MaxDurationSeconds: 30}
	service := &OpenAIGatewayService{}
	result, err := service.streamRawChatCompletions(c, initialResp, &Account{ID: 1, Platform: PlatformOpenAI}, "public", "public", "upstream", nil, nil, time.Now(), len(initialBody), policy, func(assistant string, remaining int) (*http.Response, error) {
		body, buildErr := buildRawChatContinuationBody(initialBody, assistant, remaining)
		if buildErr != nil {
			return nil, buildErr
		}
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodPost, upstream.URL, bytes.NewReader(body))
		if reqErr != nil {
			return nil, reqErr
		}
		return http.DefaultClient.Do(req)
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.AntiTruncationRounds)
	require.Equal(t, 7, result.Usage.OutputTokens)
	require.Equal(t, "partial", gjson.GetBytes(continuationBody, "messages.1.content").String())
	require.Equal(t, int64(7), gjson.GetBytes(continuationBody, "max_tokens").Int())
	clientStream := recorder.Body.String()
	require.Contains(t, clientStream, "partial")
	require.Contains(t, clientStream, " continued")
	require.NotContains(t, clientStream, `"finish_reason":"length"`)
	require.Equal(t, 1, strings.Count(clientStream, "data: [DONE]"))
}

func TestAntiTruncationPolicyContextRoundTrip(t *testing.T) {
	policy := domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 2, MaxOutputTokens: 500, MaxDurationSeconds: 20}
	ctx := WithRawChatAntiTruncationPolicy(t.Context(), policy)
	got, ok := RawChatAntiTruncationPolicyFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, policy, got)
}
