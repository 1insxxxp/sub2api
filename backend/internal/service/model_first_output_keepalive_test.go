package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newModelKeepaliveTestContext(path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	return c, rec
}

func TestModelFirstOutputKeepalive_HeaderPreparationDoesNotStopHeartbeat(t *testing.T) {
	c, rec := newModelKeepaliveTestContext("/v1/chat/completions")
	original := c.Writer
	stop := startModelFirstOutputKeepalive(c, time.Hour)
	c.Header("x-request-id", "selected-account")
	c.Header("Content-Type", "text/event-stream")
	c.Status(http.StatusOK)
	require.True(t, modelFirstOutputKeepaliveForTest(t, c).beat())
	require.Equal(t, -1, ModelFirstOutputKeepaliveAdjustedWrittenSize(c))
	require.True(t, StopModelFirstOutputKeepaliveCommitted(c))
	stop()
	require.Same(t, original, c.Writer)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/event-stream", rec.Result().Header.Get("Content-Type"))
	require.Equal(t, ": keepalive\n\n", rec.Body.String())
}

func TestModelFirstOutputKeepalive_PreservesAccountingAcrossAccounts(t *testing.T) {
	c, rec := newModelKeepaliveTestContext("/v1/messages")
	firstStop := startModelFirstOutputKeepalive(c, time.Hour)
	first := modelFirstOutputKeepaliveForTest(t, c)
	require.True(t, first.beat())
	firstStop()
	secondStop := startModelFirstOutputKeepalive(c, time.Hour)
	second := modelFirstOutputKeepaliveForTest(t, c)
	require.True(t, second.beat())
	require.Equal(t, -1, ModelFirstOutputKeepaliveAdjustedWrittenSize(c))
	secondStop()
	require.True(t, StopModelFirstOutputKeepaliveCommitted(c))
	require.False(t, first.beat())
	require.False(t, second.beat())
	_, err := c.Writer.Write([]byte("data: semantic\n\n"))
	require.NoError(t, err)
	require.Equal(t, len("data: semantic\n\n"), ModelFirstOutputKeepaliveAdjustedWrittenSize(c))
	require.Equal(t, 2, strings.Count(rec.Body.String(), ": keepalive"))
}

func TestModelFirstOutputKeepalive_FastFailureKeepsHTTPStatus(t *testing.T) {
	c, rec := newModelKeepaliveTestContext("/v1/messages")
	stop := startModelFirstOutputKeepalive(c, time.Hour)
	require.False(t, StopModelFirstOutputKeepaliveCommitted(c))
	c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
	stop()
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	require.NotContains(t, rec.Body.String(), ": keepalive")
}

func TestModelFirstOutputKeepalive_RealWriteStopsHeartbeat(t *testing.T) {
	c, rec := newModelKeepaliveTestContext("/v1/chat/completions")
	stop := startModelFirstOutputKeepalive(c, time.Hour)
	defer stop()
	attempt := modelFirstOutputKeepaliveForTest(t, c)
	c.Header("Content-Type", "text/event-stream")
	_, err := c.Writer.Write([]byte("data: semantic\n\n"))
	require.NoError(t, err)
	require.False(t, attempt.beat())
	require.Equal(t, len("data: semantic\n\n"), ModelFirstOutputKeepaliveAdjustedWrittenSize(c))
	require.Equal(t, "data: semantic\n\n", rec.Body.String())
}

func TestModelFirstOutputKeepalive_RejectingNativeSDKHasNoComments(t *testing.T) {
	for _, path := range []string{"/v1beta/models/gemini:streamGenerateContent", "/v1/models/gemini:streamGenerateContent"} {
		t.Run(path, func(t *testing.T) {
			c, rec := newModelKeepaliveTestContext(path)
			c.Request.Header.Set("X-Goog-Api-Client", "google-genai-sdk/1.0 gl-python/3.12")
			original := c.Writer
			stop := startModelFirstOutputKeepalive(c, time.Millisecond)
			stop()
			require.Same(t, original, c.Writer)
			require.Zero(t, rec.Body.Len())
			require.False(t, StopModelFirstOutputKeepaliveCommitted(c))
		})
	}
	c, _ := newModelKeepaliveTestContext("/v1/chat/completions")
	c.Request.Header.Set("User-Agent", "google-genai-sdk/1.0 gl-go/go1.25")
	stop := startModelFirstOutputKeepalive(c, time.Hour)
	defer stop()
	require.True(t, modelFirstOutputKeepaliveForTest(t, c).beat(), "compat protocols permit SSE comments")
}

func TestModelFirstOutputKeepalive_CanceledRequestStopsHeartbeat(t *testing.T) {
	c, rec := newModelKeepaliveTestContext("/v1/chat/completions")
	ctx, cancel := context.WithCancel(c.Request.Context())
	c.Request = c.Request.WithContext(ctx)
	stop := startModelFirstOutputKeepalive(c, time.Hour)
	defer stop()
	attempt := modelFirstOutputKeepaliveForTest(t, c)
	cancel()
	require.False(t, attempt.beat())
	require.Zero(t, rec.Body.Len())
}

func modelFirstOutputKeepaliveForTest(t *testing.T, c *gin.Context) *modelFirstOutputKeepaliveAttempt {
	t.Helper()
	value, ok := c.Get(modelFirstOutputKeepaliveKey)
	require.True(t, ok)
	state := value.(*modelFirstOutputKeepaliveState)
	require.NotNil(t, state.active)
	return state.active
}

func TestModelFirstOutputKeepalive_TimerSurvivesHeaderEdits(t *testing.T) {
	c, rec := newModelKeepaliveTestContext("/v1/messages")
	stop := startModelFirstOutputKeepalive(c, time.Millisecond)
	defer stop()
	attempt := modelFirstOutputKeepaliveForTest(t, c)
	require.Eventually(t, func() bool {
		c.Header("x-request-id", "waiting-for-output")
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		attempt.state.mu.Lock()
		defer attempt.state.mu.Unlock()
		return attempt.state.bytes > 0
	}, time.Second, time.Millisecond)
	require.True(t, StopModelFirstOutputKeepaliveCommitted(c))
	stop()
	require.Contains(t, rec.Body.String(), ": keepalive\n\n")
	before := rec.Body.Len()
	require.False(t, attempt.beat())
	require.Equal(t, before, rec.Body.Len())
}
