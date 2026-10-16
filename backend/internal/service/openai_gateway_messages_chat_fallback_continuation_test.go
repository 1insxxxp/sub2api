//go:build unit

package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStreamChatCompletionsAsAnthropicContinuesWithoutRepeatedTerminalEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	calls := 0
	var continuationPrompt string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", fmt.Sprintf("round-%d", calls))
		if calls == 1 {
			_, _ = w.Write([]byte("data: {\"id\":\"resp-1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"},\"finish_reason\":null}]}\n\n"))
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
	initial, err := http.Get(upstream.URL)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	svc := &OpenAIGatewayService{}
	policy := domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1, MaxOutputTokens: 20, MaxDurationSeconds: 30}
	result, err := svc.streamChatCompletionsAsAnthropic(c, initial, "alias", "alias", "upstream", nil, nil, time.Now(), policy, func(assistant string, remaining int) (*http.Response, error) {
		continuationPrompt = assistant
		return http.Get(upstream.URL)
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.AntiTruncationRounds)
	require.Equal(t, 7, result.Usage.OutputTokens)
	require.Equal(t, "partial", continuationPrompt)
	body := recorder.Body.String()
	require.Equal(t, 1, strings.Count(body, "event: message_start"))
	require.Equal(t, 1, strings.Count(body, "event: message_stop"))
	require.Contains(t, body, "partial")
	require.Contains(t, body, " continued")
	require.Equal(t, 2, calls)
}

func TestStreamChatCompletionsAsAnthropicDoesNotContinueToolCalls(t *testing.T) {
	gin.SetMode(gin.TestMode)
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\",\"tool_calls\":[{\"index\":0,\"id\":\"t1\",\"type\":\"function\",\"function\":{\"name\":\"lookup\",\"arguments\":\"{}\"}}]},\"finish_reason\":\"length\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()
	initial, err := http.Get(upstream.URL)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	_, err = (&OpenAIGatewayService{}).streamChatCompletionsAsAnthropic(c, initial, "alias", "alias", "upstream", nil, nil, time.Now(), domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 2}, func(string, int) (*http.Response, error) {
		return http.Get(upstream.URL)
	})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
}
