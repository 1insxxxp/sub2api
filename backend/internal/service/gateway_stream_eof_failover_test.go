//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicStreamEOFReplayBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name             string
		body             string
		wantRetry        bool
		passthroughRetry bool
		cancel           bool
		writeFails       bool
	}{
		{name: "empty", wantRetry: true, passthroughRetry: true},
		{name: "upstream ping", body: "event: ping\ndata: {\"type\":\"ping\"}\n\n", wantRetry: true},
		{name: "ping with unknown fields", body: "event: ping\ndata: {\"type\":\"ping\",\"unknown\":\"opaque\"}\n\n"},
		{name: "upstream comment", body: ": keepalive\n\n", wantRetry: true},
		{name: "metadata already committed", body: "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"content\":[],\"usage\":{\"input_tokens\":11}}}\n\n"},
		{name: "text", body: "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n"},
		{name: "tool", body: "data: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"tool_use\",\"id\":\"t1\",\"name\":\"Read\",\"input\":{}}}\n\n"},
		{name: "reasoning", body: "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"thinking\"}}\n\n"},
		{name: "signature", body: "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"signature_delta\",\"signature\":\"sig\"}}\n\n"},
		{name: "redacted thinking", body: "data: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"redacted_thinking\",\"data\":\"opaque\"}}\n\n"},
		{name: "unknown event", body: "event: future_event\ndata: {\"type\":\"future_event\"}\n\n"},
		{name: "malformed event", body: "data: not-json\n\n"},
		{name: "partial upstream event", body: "event: ping\n", wantRetry: true},
		{name: "canceled", cancel: true},
		{name: "downstream write failure", body: "data: {\"type\":\"message_start\",\"message\":{\"content\":[]}}\n\n", writeFails: true},
	}
	for _, passthrough := range []bool{false, true} {
		mode := "normal"
		if passthrough {
			mode = "passthrough"
		}
		for _, tc := range cases {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
				if tc.cancel {
					cancel()
				}
				if tc.writeFails {
					c.Writer = &failWriteResponseWriter{ResponseWriter: c.Writer}
				}
				resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"X-Request-Id": {"rid-eof"}}, Body: io.NopCloser(strings.NewReader(tc.body))}
				svc := newMinimalGatewayService()
				account := &Account{ID: 4400, Name: "eof-account", Platform: PlatformAnthropic}
				var result *streamingResult
				var err error
				if passthrough {
					result, err = svc.handleStreamingResponseAnthropicAPIKeyPassthrough(ctx, resp, c, account, time.Now(), "claude-model")
				} else {
					result, err = svc.handleStreamingResponse(ctx, resp, c, account, time.Now(), "claude-model", "claude-model", false)
				}
				require.Error(t, err)
				wantRetry := tc.wantRetry
				if passthrough {
					wantRetry = tc.passthroughRetry
				}
				var failover *UpstreamFailoverError
				require.Equal(t, wantRetry, errors.As(err, &failover))
				if !wantRetry {
					return
				}
				require.Nil(t, result, "failed retryable attempts must not contribute a usage result")
				require.Equal(t, http.StatusBadGateway, failover.StatusCode)
				require.True(t, failover.SafeToFailoverAfterWrite)
				require.True(t, failover.ShouldRetryNextAccount())
				require.False(t, failover.RetryableOnSameAccount)
				require.False(t, failover.PreferNextAccount, "preserve ordinary switch limits")
				require.Equal(t, "rid-eof", failover.ResponseHeaders.Get("x-request-id"))
				require.NotContains(t, rec.Body.String(), "event: error")
				value, ok := c.Get(OpsUpstreamErrorsKey)
				require.True(t, ok)
				events := value.([]*OpsUpstreamErrorEvent)
				require.NotEmpty(t, events)
				require.Equal(t, account.ID, events[len(events)-1].AccountID)
				require.Contains(t, events[len(events)-1].Message, "missing terminal event")
			})
		}
	}
}

func TestAnthropicStreamEOFWithBufferedMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	svc := newMinimalGatewayService()
	svc.settingService = &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":30,"switch_seconds":30,"hard_cap_seconds":60}}`}}
	ctx, guard, _ := newModelFirstOutputGuard(c.Request.Context(), c, svc.settingService, PlatformAnthropic, "model")
	require.NotNil(t, guard)
	defer guard.Close()
	resp := &http.Response{
		StatusCode: http.StatusOK, Header: make(http.Header),
		Body: io.NopCloser(strings.NewReader("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"content\":[],\"usage\":{\"input_tokens\":11}}}\n\n")),
	}
	result, err := svc.handleStreamingResponse(ctx, resp, c, &Account{ID: 1, Platform: PlatformAnthropic}, time.Now(), "model", "model", false)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Nil(t, result)
	require.Empty(t, rec.Body.String(), "buffered metadata must not leak into the next attempt")
}

type anthropicEOFHeartbeatWriter struct {
	gin.ResponseWriter
	written chan struct{}
	once    sync.Once
}

func (w *anthropicEOFHeartbeatWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if strings.Contains(string(p), "event: ping") {
		w.once.Do(func() { close(w.written) })
	}
	return n, err
}

func TestAnthropicStreamEOFAfterLocalHeartbeat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		mode := "normal"
		if passthrough {
			mode = "passthrough"
		}
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
			writer := &anthropicEOFHeartbeatWriter{ResponseWriter: c.Writer, written: make(chan struct{})}
			c.Writer = writer
			pr, pw := io.Pipe()
			defer pr.Close()
			go func() {
				select {
				case <-writer.written:
					_ = pw.Close()
				case <-ctx.Done():
					_ = pw.CloseWithError(ctx.Err())
				}
			}()
			svc := newMinimalGatewayService()
			svc.cfg.Gateway.StreamKeepaliveInterval = 1
			resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: pr}
			var result *streamingResult
			var err error
			if passthrough {
				result, err = svc.handleStreamingResponseAnthropicAPIKeyPassthrough(ctx, resp, c, &Account{ID: 1}, time.Now(), "model")
			} else {
				result, err = svc.handleStreamingResponse(ctx, resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
			}
			var failover *UpstreamFailoverError
			require.ErrorAs(t, err, &failover)
			require.Nil(t, result)
			require.True(t, failover.SafeToFailoverAfterWrite)
			require.Contains(t, rec.Body.String(), "event: ping")
			require.NotContains(t, rec.Body.String(), "event: error")
		})
	}
}
