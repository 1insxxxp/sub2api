package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResponseOutcomeCollectorTracksEffectiveOutput(t *testing.T) {
	tests := []struct {
		name    string
		observe func(*ResponseOutcomeCollector)
	}{
		{name: "text", observe: func(c *ResponseOutcomeCollector) { c.ObserveText(" hello ") }},
		{name: "tool call", observe: func(c *ResponseOutcomeCollector) { c.ObserveToolCall() }},
		{name: "reasoning", observe: func(c *ResponseOutcomeCollector) { c.ObserveReasoning("thinking") }},
		{name: "media", observe: func(c *ResponseOutcomeCollector) { c.ObserveMedia() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewResponseOutcomeCollector(200, 200)
			tt.observe(collector)
			collector.MarkCompleted("stop")

			outcome := collector.Snapshot()
			require.True(t, outcome.HasEffectiveOutput())
			require.True(t, outcome.StreamCompleted)
			require.Equal(t, "stop", outcome.FinishReason)
		})
	}
}

func TestResponseOutcomeCollectorIgnoresWhitespaceOnlyContent(t *testing.T) {
	collector := NewResponseOutcomeCollector(200, 200)
	collector.ObserveText(" \n\t ")
	collector.ObserveReasoning("  ")
	collector.ObserveEvent(12)
	collector.MarkCompleted("stop")

	outcome := collector.Snapshot()
	require.False(t, outcome.HasEffectiveOutput())
	require.Equal(t, int64(12), outcome.OutputBytes)
	require.Equal(t, 1, outcome.EventCount)
}

func TestResponseOutcomeCollectorClassifiesDisconnectSource(t *testing.T) {
	t.Run("client canceled", func(t *testing.T) {
		collector := NewResponseOutcomeCollector(200, 200)
		collector.MarkStreamError(context.Canceled, true)
		outcome := collector.Snapshot()
		require.Equal(t, DisconnectSourceClient, outcome.DisconnectSource)
		require.Equal(t, UpstreamErrorNone, outcome.UpstreamErrorKind)
	})

	t.Run("upstream eof", func(t *testing.T) {
		collector := NewResponseOutcomeCollector(200, 200)
		collector.MarkStreamError(errors.New("unexpected EOF"), false)
		outcome := collector.Snapshot()
		require.Equal(t, DisconnectSourceUpstream, outcome.DisconnectSource)
		require.Equal(t, UpstreamErrorProtocol, outcome.UpstreamErrorKind)
	})

	t.Run("upstream timeout", func(t *testing.T) {
		collector := NewResponseOutcomeCollector(504, 504)
		collector.MarkStreamError(context.DeadlineExceeded, false)
		outcome := collector.Snapshot()
		require.Equal(t, DisconnectSourceUpstream, outcome.DisconnectSource)
		require.Equal(t, UpstreamErrorTimeout, outcome.UpstreamErrorKind)
	})
}

func TestResponseOutcomeSnapshotDoesNotRetainObservedContent(t *testing.T) {
	collector := NewResponseOutcomeCollector(200, 200)
	collector.ObserveText("private-response-marker")
	collector.ObserveReasoning("private-reasoning-marker")
	collector.ObserveToolCall()
	collector.MarkCompleted("stop")

	raw, err := json.Marshal(collector.Snapshot())
	require.NoError(t, err)
	require.NotContains(t, string(raw), "private-response-marker")
	require.NotContains(t, string(raw), "private-reasoning-marker")
	require.False(t, strings.Contains(strings.ToLower(string(raw)), "content"))
}

func TestEnsureResponseOutcomeCollectorAcceptsNilContextWithRequestCollector(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	requestContext, collector := WithResponseOutcomeCollector(context.Background(), http.StatusOK, http.StatusOK)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil).WithContext(requestContext)

	//nolint:staticcheck // Exercise compatibility with callers that pass a nil context.
	ctx, ensured := EnsureResponseOutcomeCollector(nil, c, http.StatusOK, http.StatusOK)

	require.NotNil(t, ctx)
	require.Same(t, collector, ensured)
}
