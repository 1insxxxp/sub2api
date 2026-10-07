package service

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const modelFirstOutputKeepaliveKey = "model_first_output_keepalive"

// The byte count belongs to the downstream request, not an account attempt.
// Otherwise an account switch would turn the preceding heartbeat into output
// and incorrectly disable failover.
type modelFirstOutputKeepaliveState struct {
	mu        sync.Mutex
	writer    gin.ResponseWriter
	active    *modelFirstOutputKeepaliveAttempt
	bytes     int
	committed bool
}

type modelFirstOutputKeepaliveAttempt struct {
	state   *modelFirstOutputKeepaliveState
	writer  gin.ResponseWriter
	header  http.Header
	ctx     context.Context
	stop    chan struct{}
	stopped bool
}

// startModelFirstOutputKeepalive is only called for guarded Gemini text streams.
// Delaying the first beat preserves normal HTTP status codes for fast failures.
// Header preparation is isolated from heartbeat writes: Gemini prepares its SSE
// headers before semantic output, so Header must not stop this timer.
func startModelFirstOutputKeepalive(c *gin.Context, interval time.Duration) func() {
	if c == nil || c.Writer == nil || interval <= 0 {
		return func() {}
	}
	if c.Request != nil {
		path := c.Request.URL.Path
		native := strings.Contains(path, "/v1beta/") || strings.Contains(path, ":streamGenerateContent")
		if native && downstreamRejectsSSEComments(c) {
			return func() {}
		}
	}
	var state *modelFirstOutputKeepaliveState
	if value, ok := c.Get(modelFirstOutputKeepaliveKey); ok {
		state, _ = value.(*modelFirstOutputKeepaliveState)
	}
	if state == nil {
		state = &modelFirstOutputKeepaliveState{}
		c.Set(modelFirstOutputKeepaliveKey, state)
	}
	// The caller closes the prior attempt before starting its successor.
	original := c.Writer
	ctx := context.Background()
	if c.Request != nil {
		ctx = c.Request.Context()
	}
	attempt := &modelFirstOutputKeepaliveAttempt{state: state, writer: original, header: original.Header().Clone(), ctx: ctx, stop: make(chan struct{})}
	wrapped := &modelFirstOutputKeepaliveWriter{ResponseWriter: original, attempt: attempt}
	state.mu.Lock()
	state.writer = original
	state.active = attempt
	state.mu.Unlock()
	c.Writer = wrapped
	go func() {
		timer := time.NewTimer(interval)
		defer timer.Stop()
		for {
			select {
			case <-attempt.stop:
				return
			case <-ctx.Done():
				state.mu.Lock()
				attempt.markStoppedLocked()
				state.mu.Unlock()
				return
			case <-timer.C:
				if !attempt.beat() {
					return
				}
				timer.Reset(interval)
			}
		}
	}()
	return func() {
		attempt.claimWriter()
		if c.Writer == wrapped {
			c.Writer = original
		}
		state.mu.Lock()
		if state.active == attempt {
			state.active = nil
		}
		state.mu.Unlock()
	}
}

func (a *modelFirstOutputKeepaliveAttempt) markStoppedLocked() {
	if a.stopped {
		return
	}
	a.stopped = true
	close(a.stop)
}

// claimWriter is only used by the request goroutine. The timer never reads the
// pending header map, so request-side header edits cannot race with a heartbeat.
func (a *modelFirstOutputKeepaliveAttempt) claimWriter() {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	a.markStoppedLocked()
	header := a.writer.Header()
	for key, values := range a.header {
		header[key] = append([]string(nil), values...)
	}
}

func (a *modelFirstOutputKeepaliveAttempt) beat() bool {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	if a.stopped || a.ctx.Err() != nil {
		a.markStoppedLocked()
		return false
	}
	if !a.state.committed {
		header := a.writer.Header()
		header.Set("Content-Type", "text/event-stream")
		header.Set("Cache-Control", "no-cache")
		header.Set("Connection", "keep-alive")
		header.Set("X-Accel-Buffering", "no")
		a.writer.WriteHeader(http.StatusOK)
		a.state.committed = true
	}
	n, err := a.writer.Write([]byte(": keepalive\n\n"))
	a.state.bytes += n
	if err != nil {
		a.markStoppedLocked()
		return false
	}
	a.writer.Flush()
	return true
}

// StopModelFirstOutputKeepaliveCommitted stops pre-output heartbeats and reports
// whether any attempt committed HTTP 200. Error writers must then use their
// protocol's SSE error envelope, rather than append a JSON HTTP response.
func StopModelFirstOutputKeepaliveCommitted(c *gin.Context) bool {
	state := modelFirstOutputKeepaliveStateFromContext(c)
	if state == nil {
		return false
	}
	state.mu.Lock()
	active := state.active
	state.mu.Unlock()
	if active != nil {
		active.claimWriter()
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.committed
}

// ModelFirstOutputKeepaliveAdjustedWrittenSize excludes all attempts' heartbeat
// bytes. A response containing only heartbeats retains Gin's unwritten sentinel.
func ModelFirstOutputKeepaliveAdjustedWrittenSize(c *gin.Context) int {
	if c == nil || c.Writer == nil {
		return -1
	}
	state := modelFirstOutputKeepaliveStateFromContext(c)
	if state == nil {
		return c.Writer.Size()
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	size := state.writer.Size()
	if size < 0 || state.bytes == 0 {
		return size
	}
	if remaining := size - state.bytes; remaining > 0 {
		return remaining
	}
	return -1
}

func modelFirstOutputKeepaliveStateFromContext(c *gin.Context) *modelFirstOutputKeepaliveState {
	if c == nil {
		return nil
	}
	value, _ := c.Get(modelFirstOutputKeepaliveKey)
	state, _ := value.(*modelFirstOutputKeepaliveState)
	return state
}

type modelFirstOutputKeepaliveWriter struct {
	gin.ResponseWriter
	attempt *modelFirstOutputKeepaliveAttempt
}

func (w *modelFirstOutputKeepaliveWriter) Header() http.Header { return w.attempt.header }
func (w *modelFirstOutputKeepaliveWriter) Write(b []byte) (int, error) {
	w.attempt.claimWriter()
	return w.ResponseWriter.Write(b)
}
func (w *modelFirstOutputKeepaliveWriter) WriteString(s string) (int, error) {
	w.attempt.claimWriter()
	return w.ResponseWriter.WriteString(s)
}

// Gin prepares a success status without committing it. Both Gemini stream
// handlers do this before the first semantic event; it must not stop the beat.
func (w *modelFirstOutputKeepaliveWriter) WriteHeader(code int) {
	if code == http.StatusOK {
		return
	}
	w.attempt.claimWriter()
	w.ResponseWriter.WriteHeader(code)
}
func (w *modelFirstOutputKeepaliveWriter) WriteHeaderNow() {
	w.attempt.claimWriter()
	w.ResponseWriter.WriteHeaderNow()
}
func (w *modelFirstOutputKeepaliveWriter) Flush() { w.attempt.claimWriter(); w.ResponseWriter.Flush() }
func (w *modelFirstOutputKeepaliveWriter) Status() int {
	w.attempt.state.mu.Lock()
	defer w.attempt.state.mu.Unlock()
	return w.ResponseWriter.Status()
}
func (w *modelFirstOutputKeepaliveWriter) Size() int {
	w.attempt.state.mu.Lock()
	defer w.attempt.state.mu.Unlock()
	return w.ResponseWriter.Size()
}
func (w *modelFirstOutputKeepaliveWriter) Written() bool {
	w.attempt.state.mu.Lock()
	defer w.attempt.state.mu.Unlock()
	return w.ResponseWriter.Written()
}
