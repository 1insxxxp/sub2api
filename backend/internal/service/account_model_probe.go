package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

const modelProbeEventLimit = 64 << 10

// runStrictModelProbe consumes only control events from the existing native
// account probe. Response text, credentials and generated media never leave it.
func runStrictModelProbe(ctx context.Context, run func(*gin.Context) error) (ModelProbeExecution, error) {
	if result, done := modelProbeContextResult(ctx); done {
		return result, nil
	}
	w := &modelProbeCapture{header: make(http.Header)}
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequestWithContext(ctx, http.MethodPost, "/", nil)
	c.Set(accountTestBackgroundKey, true)
	err := run(c)
	if result, done := modelProbeContextResult(ctx); done {
		return result, nil
	}
	if err != nil || w.failed || w.status >= 400 {
		return ModelProbeExecution{Status: "failed", ErrorCode: "probe_failed"}, nil
	}
	if !w.complete {
		return ModelProbeExecution{Status: "failed", ErrorCode: "probe_incomplete"}, nil
	}
	return ModelProbeExecution{Status: "success"}, nil
}

func modelProbeContextResult(ctx context.Context) (ModelProbeExecution, bool) {
	switch ctx.Err() {
	case context.DeadlineExceeded:
		return ModelProbeExecution{Status: "timeout", ErrorCode: "probe_timeout"}, true
	case context.Canceled:
		return ModelProbeExecution{Status: "cancelled", ErrorCode: "probe_cancelled"}, true
	default:
		return ModelProbeExecution{}, false
	}
}

// The native emitter may write multi-megabyte media events. Keep only a bounded
// prefix of each line/event and discard those payloads rather than recording them.
type modelProbeCapture struct {
	header                    http.Header
	status                    int
	line, data                []byte
	lineOverflow, overflow    bool
	hasData, complete, failed bool
}

func (w *modelProbeCapture) Header() http.Header { return w.header }
func (w *modelProbeCapture) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *modelProbeCapture) Flush() {}
func (w *modelProbeCapture) Write(p []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	n := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			w.appendLine(p)
			break
		}
		w.appendLine(p[:i])
		w.consumeLine()
		p = p[i+1:]
	}
	return n, nil
}

func (w *modelProbeCapture) appendLine(p []byte) {
	space := modelProbeEventLimit - len(w.line)
	if len(p) > space {
		w.lineOverflow = true
		p = p[:space]
	}
	w.line = append(w.line, p...)
}

func (w *modelProbeCapture) consumeLine() {
	line := bytes.TrimSuffix(w.line, []byte{'\r'})
	switch {
	case len(line) == 0:
		w.consumeEvent()
	case bytes.HasPrefix(line, []byte("data:")):
		data := bytes.TrimPrefix(line, []byte("data:"))
		data = bytes.TrimPrefix(data, []byte{' '})
		if w.hasData && len(w.data) < modelProbeEventLimit {
			w.data = append(w.data, '\n')
		}
		space := modelProbeEventLimit - len(w.data)
		w.overflow = w.overflow || w.lineOverflow || len(data) > space
		if len(data) > space {
			data = data[:space]
		}
		w.data = append(w.data, data...)
		w.hasData = true
	}
	w.line = w.line[:0]
	w.lineOverflow = false
}

func (w *modelProbeCapture) consumeEvent() {
	if w.hasData {
		w.inspectEvent()
	}
	w.data = w.data[:0]
	w.hasData = false
	w.overflow = false
}

func (w *modelProbeCapture) inspectEvent() {
	if w.overflow {
		// TestEvent is serialized by sendEvent with type first. Only its known
		// nonterminal payloads may be discarded; oversized control/unknown
		// events fail closed and can never synthesize a successful completion.
		if !modelProbeDiscardablePayload(w.data) {
			w.failed = true
		}
		return
	}
	var event struct {
		Type    string `json:"type"`
		Success bool   `json:"success"`
	}
	if json.Unmarshal(w.data, &event) != nil {
		w.failed = true
		return
	}
	switch event.Type {
	case "error":
		w.failed = true
	case "test_complete":
		w.complete = true
		w.failed = w.failed || !event.Success
	}
}

func modelProbeDiscardablePayload(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return false
	}
	key, err := decoder.Token()
	if err != nil || key != "type" {
		return false
	}
	var kind string
	if decoder.Decode(&kind) != nil {
		return false
	}
	switch kind {
	case "content", "image", "audio", "video":
		return true
	default:
		return false
	}
}
