package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUsageOutcomePolicyFailureIsNeverSuccessful(t *testing.T) {
	for _, reason := range []string{"prohibited_content", "SAFETY", "content_filter", "image_prohibited_content"} {
		for _, disconnect := range []DisconnectSource{DisconnectSourceNone, DisconnectSourceUpstream} {
			t.Run(reason+"/"+string(disconnect), func(t *testing.T) {
				o := &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, HasReasoning: true,
					StreamCompleted: true, FinishReason: reason, DisconnectSource: disconnect,
					UpstreamErrorKind: UpstreamErrorOther, CollectorVersion: ResponseOutcomeCollectorVersion}
				require.Equal(t, UsageRequestFailed, ClassifyUsageRequestStatus(o))
				require.Equal(t, UsageOutcomeFailure, ClassifyUsageOutcome(o))
			})
		}
	}
}

func TestGeminiChatPolicyFailureCapturesFinalHTTPStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := "data: " + `{"candidates":[{"content":{"parts":[{"text":"reasoning","thought":true}]}}]}` + "\n\n" +
		"data: " + `{"candidates":[{"finishReason":"PROHIBITED_CONTENT"}],"usageMetadata":{"promptTokenCount":33782,"candidatesTokenCount":0,"thoughtsTokenCount":5748,"totalTokenCount":39530}}` + "\n\n"
	resp := &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
	svc := &GeminiMessagesCompatService{}
	result, err := svc.handleChatCompletionsStreamingResponseFromGemini(c.Request.Context(), c, resp, time.Now(), "test-model", &Account{}, false, true, nil, ModelFirstOutputTimeoutPolicy{})
	require.Error(t, err)
	require.Equal(t, 400, w.Code)
	require.NotNil(t, result)
	require.Equal(t, 400, result.outcome.HTTPStatus, "audit must keep the actual downstream status, not the upstream's initial 200")
	require.Equal(t, 200, result.outcome.UpstreamStatus)
	require.Equal(t, UsageRequestFailed, ClassifyUsageRequestStatus(&result.outcome))
}

func TestUsageRequestStatusSuccessfulRetryMatchesMonitor(t *testing.T) {
	o := &ResponseOutcome{CollectorVersion: 1, HTTPStatus: 200, UpstreamStatus: 200,
		HasText: true, StreamCompleted: true, UpstreamErrorKind: UpstreamErrorOther}
	require.Equal(t, UsageOutcomeSuccess, ClassifyUsageOutcome(o))
	require.Equal(t, UsageRequestCompleted, ClassifyUsageRequestStatus(o))
}
