package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCompatBufferedFailurePreservesUsageEvidence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GatewayService{}
	for _, protocol := range []string{"chat", "responses"} {
		t.Run(protocol, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+protocol, nil)
			// The provider accepted and charged the prompt, then truncated its
			// stream. Retain that evidence without marking the turn complete.
			body := "event: message_start\ndata: " +
				`{"type":"message_start","message":{"id":"msg_usage","model":"claude-test","usage":{"input_tokens":11,"cache_creation_input_tokens":5,"cache_read_input_tokens":7}}}` + "\n\n"
			resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
			var result *ForwardResult
			var err error
			if protocol == "chat" {
				result, err = svc.handleCCBufferedFromAnthropic(resp, c, "alias", "claude-test", "1h", nil, time.Now())
			} else {
				result, err = svc.handleResponsesBufferedStreamingResponse(resp, c, "alias", "claude-test", "1h", nil, time.Now(), apicompat.ResponsesClientToolMapping{})
			}
			require.Error(t, err)
			require.Equal(t, http.StatusBadGateway, recorder.Code)
			require.NotNil(t, result)
			require.NotNil(t, result.Outcome, "failed streams still need evidence for settlement")
			require.False(t, result.Outcome.StreamCompleted)
			require.Equal(t, "1h", result.CacheCreationTTLTarget)
			require.Equal(t, 11, result.Usage.InputTokens)
			require.Equal(t, 5, result.Usage.CacheCreationInputTokens)
			require.Equal(t, 7, result.Usage.CacheReadInputTokens)
		})
	}
}
