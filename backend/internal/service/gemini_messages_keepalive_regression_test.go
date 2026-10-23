//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Each read notification means the preceding event has been processed and the
// converter is waiting for its next upstream chunk. This keeps heartbeat and
// buffering assertions independent of scheduler timing.
type geminiMessagesKeepaliveBody struct {
	*io.PipeReader
	reads chan struct{}
}

func (b *geminiMessagesKeepaliveBody) Read(p []byte) (int, error) {
	select {
	case b.reads <- struct{}{}:
	default:
	}
	return b.PipeReader.Read(p)
}

type geminiMessagesKeepaliveResult struct {
	result *geminiStreamResult
	err    error
}

type geminiMessagesKeepaliveStream struct {
	c       *gin.Context
	rec     *httptest.ResponseRecorder
	body    *geminiMessagesKeepaliveBody
	writer  *io.PipeWriter
	guard   *modelFirstOutputGuard
	attempt *modelFirstOutputKeepaliveAttempt
	cancel  context.CancelFunc
	done    chan geminiMessagesKeepaliveResult
}

func newGeminiMessagesKeepaliveStream(t *testing.T, guarded bool, interval time.Duration) *geminiMessagesKeepaliveStream {
	t.Helper()
	c, rec := newModelKeepaliveTestContext("/v1/messages")
	ctx, cancel := context.WithCancel(c.Request.Context())
	c.Request = c.Request.WithContext(ctx)
	reader, writer := io.Pipe()
	h := &geminiMessagesKeepaliveStream{
		c: c, rec: rec, body: &geminiMessagesKeepaliveBody{PipeReader: reader, reads: make(chan struct{}, 1)},
		writer: writer, cancel: cancel, done: make(chan geminiMessagesKeepaliveResult, 1),
	}
	policy := ModelFirstOutputTimeoutPolicy{Enabled: true, SwitchSeconds: 300, HardCapSeconds: 300}
	if guarded {
		stop := startModelFirstOutputKeepalive(c, interval)
		h.attempt = modelFirstOutputKeepaliveForTest(t, c)
		h.guard = &modelFirstOutputGuard{ctx: ctx, stopKeepalive: stop}
	}
	t.Cleanup(func() {
		cancel()
		_ = reader.CloseWithError(context.Canceled)
		_ = writer.Close()
		select {
		case <-h.done:
		case <-time.After(2 * time.Second):
			t.Error("messages converter did not stop after closing its upstream")
		}
		if h.guard != nil {
			h.guard.Close()
		}
	})
	go func() {
		result, err := (&GeminiMessagesCompatService{}).handleStreamingResponseWithContext(ctx, c, &http.Response{
			StatusCode: http.StatusOK, Header: make(http.Header), Body: h.body,
		}, time.Now(), "gemini-3.1-pro-preview", nil, h.guard, policy)
		h.done <- geminiMessagesKeepaliveResult{result: result, err: err}
	}()
	h.waitForRead(t)
	return h
}

func (h *geminiMessagesKeepaliveStream) waitForRead(t *testing.T) {
	t.Helper()
	select {
	case <-h.body.reads:
	case <-time.After(2 * time.Second):
		t.Fatal("messages converter did not request the next upstream chunk")
	}
}

func (h *geminiMessagesKeepaliveStream) send(t *testing.T, data string) {
	t.Helper()
	written := make(chan error, 1)
	go func() {
		_, err := io.WriteString(h.writer, "data: "+data+"\n\n")
		written <- err
	}()
	select {
	case err := <-written:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("messages converter did not read the upstream event")
	}
	h.waitForRead(t)
}

func (h *geminiMessagesKeepaliveStream) finish(t *testing.T) geminiMessagesKeepaliveResult {
	t.Helper()
	_ = h.writer.Close()
	select {
	case result := <-h.done:
		// Cleanup also waits for completion, so retain the result for it.
		h.done <- result
		return result
	case <-time.After(2 * time.Second):
		t.Fatal("messages converter did not finish")
		return geminiMessagesKeepaliveResult{}
	}
}

func TestGeminiMessagesKeepalive_WaitingEventsKeepHeartbeatUntilVisibleText(t *testing.T) {
	h := newGeminiMessagesKeepaliveStream(t, true, 5*time.Millisecond)
	for _, phase := range []struct{ name, data string }{
		{"message_start", ""},
		{"metadata", `{"usageMetadata":{"promptTokenCount":100}}`},
		{"thought", `{"candidates":[{"content":{"parts":[{"text":"Plan the reply","thought":true}]}}]}`},
		{"whitespace", `{"candidates":[{"content":{"parts":[{"text":"   "}]}}]}`},
	} {
		if phase.data != "" {
			h.send(t, phase.data)
		}
		h.attempt.state.mu.Lock()
		before := h.attempt.state.bytes
		h.attempt.state.mu.Unlock()
		require.Eventually(t, func() bool {
			h.attempt.state.mu.Lock()
			defer h.attempt.state.mu.Unlock()
			return h.attempt.state.bytes >= before+2*len(": keepalive\n\n")
		}, time.Second, time.Millisecond, "%s must retain recurring heartbeats", phase.name)
		h.attempt.state.mu.Lock()
		body := h.rec.Body.String()
		h.attempt.state.mu.Unlock()
		require.Empty(t, strings.ReplaceAll(body, ": keepalive\n\n", ""), "%s must remain staged for safe account failover", phase.name)
		require.Equal(t, -1, ModelFirstOutputKeepaliveAdjustedWrittenSize(h.c))
	}
	h.send(t, `{"candidates":[{"content":{"parts":[{"text":"Hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":7,"thoughtsTokenCount":4}}`)
	result := h.finish(t)
	require.NoError(t, result.err)
	require.True(t, result.result.finished)
	require.Equal(t, 100, result.result.usage.InputTokens)
	require.False(t, h.attempt.beat(), "visible text must stop pre-output heartbeats")
	out := h.rec.Body.String()
	require.Less(t, strings.Index(out, "event: message_start"), strings.Index(out, `"thinking":"Plan the reply"`))
	require.Less(t, strings.Index(out, `"thinking":"Plan the reply"`), strings.Index(out, `"text":"Hello"`))
	require.Contains(t, out, "event: message_stop")
	var started []string
	for _, event := range parseAnthropicContentBlockEvents(t, out) {
		if event.event == "content_block_start" {
			started = append(started, event.blockType)
		}
	}
	require.Equal(t, []string{"thinking", "text"}, started)
}

func TestGeminiMessagesKeepalive_NoBodyBeforeTimeoutOrClientCancel(t *testing.T) {
	for _, cause := range []string{"timeout", "client_cancel"} {
		t.Run(cause, func(t *testing.T) {
			h := newGeminiMessagesKeepaliveStream(t, true, time.Hour)
			h.send(t, `{"usageMetadata":{"promptTokenCount":100}}`)
			h.send(t, `{"candidates":[{"content":{"parts":[{"text":"Still thinking","thought":true},{"text":"   "}]}}]}`)
			require.True(t, h.attempt.beat(), "waiting events must not stop keepalive or commit a response")
			require.True(t, h.attempt.beat())
			require.Equal(t, -1, ModelFirstOutputKeepaliveAdjustedWrittenSize(h.c), "comments alone must preserve failover eligibility")
			if cause == "timeout" {
				h.guard.timedOut.Store(true)
			} else {
				h.cancel()
			}
			_ = h.body.CloseWithError(context.Canceled)
			result := h.finish(t)
			if cause == "timeout" {
				var failover *UpstreamFailoverError
				require.ErrorAs(t, result.err, &failover)
				require.True(t, failover.PreferNextAccount)
				require.Nil(t, result.result, "a thought-only failed attempt must not become final usage")
			} else {
				require.NotNil(t, result.result)
				require.True(t, result.result.clientDisconnect)
			}
			require.Equal(t, ": keepalive\n\n: keepalive\n\n", h.rec.Body.String())
		})
	}
}

func TestGeminiMessagesKeepalive_UnguardedThoughtsStillStreamImmediately(t *testing.T) {
	h := newGeminiMessagesKeepaliveStream(t, false, 0)
	require.Contains(t, h.rec.Body.String(), "event: message_start")
	h.send(t, `{"candidates":[{"content":{"parts":[{"text":"Plan the reply","thought":true}]}}]}`)
	require.Contains(t, h.rec.Body.String(), `"thinking":"Plan the reply"`)
	h.send(t, `{"candidates":[{"content":{"parts":[{"text":"Hello"}]},"finishReason":"STOP"}]}`)
	result := h.finish(t)
	require.NoError(t, result.err)
	require.Contains(t, h.rec.Body.String(), `"text":"Hello"`)
	require.Contains(t, h.rec.Body.String(), "event: message_stop")
}
