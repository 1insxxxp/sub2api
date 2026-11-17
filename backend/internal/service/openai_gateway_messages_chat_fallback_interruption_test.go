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
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStreamChatCompletionsAsAnthropic_ContextInterruptionPreservesPartialUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stream := "data: " + `{"choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}` + "\n\n" +
		"data: " + `{"choices":[{"index":0,"delta":{},"finish_reason":"length"}]}` + "\n\n" +
		"data: " + `{"choices":[],"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18,"prompt_tokens_details":{"cached_tokens":3}}}` + "\n\n"

	for _, interruption := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(interruption.Error(), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"X-Request-Id": []string{"partial-request"}},
				Body:       io.NopCloser(&errAfterReader{data: strings.NewReader(stream), err: fmt.Errorf("upstream read: %w", interruption)}),
			}
			continuationCalls := 0
			policy := domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1, MaxDurationSeconds: 30}
			result, err := (&OpenAIGatewayService{}).streamChatCompletionsAsAnthropic(c, resp, "alias", "billing", "upstream", nil, nil, time.Now(), policy, func(string, int) (*http.Response, error) {
				continuationCalls++
				return nil, nil
			})

			require.ErrorIs(t, err, interruption)
			require.NotNil(t, result)
			require.Equal(t, 11, result.Usage.InputTokens)
			require.Equal(t, 7, result.Usage.OutputTokens)
			require.Equal(t, 3, result.Usage.CacheReadInputTokens)
			require.Equal(t, "partial-request", result.RequestID)
			require.NotNil(t, result.FirstTokenMs)
			require.Zero(t, continuationCalls)
			require.Contains(t, recorder.Body.String(), "partial")
			require.NotContains(t, recorder.Body.String(), "event: message_delta")
			require.NotContains(t, recorder.Body.String(), "event: message_stop")
		})
	}
}
