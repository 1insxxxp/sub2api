package service

import (
	"context"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// DownstreamOutputTokenCollector tracks model-generated content that was
// successfully written to the downstream client. Transport metadata and opaque
// signatures are excluded.
type DownstreamOutputTokenCollector struct {
	mu                 sync.Mutex
	model              string
	pending            string
	text               strings.Builder
	frozen             bool
	cancelDone         <-chan struct{}
	upstreamCancel     context.CancelFunc
	upstreamCancelOnce sync.Once
}

func NewDownstreamOutputTokenCollector(model string) *DownstreamOutputTokenCollector {
	return &DownstreamOutputTokenCollector{model: model}
}

// ObserveWritten accepts bytes only after the wrapped response writer reports
// a successful write, so disconnected clients cannot accrue unseen output.
func (c *DownstreamOutputTokenCollector) ObserveWritten(written []byte) {
	if c == nil || len(written) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.freezeIfCanceledLocked()
	if c.frozen {
		return
	}

	c.pending += string(written)
	for {
		frameEnd, separatorLength := nextSSEFrameBoundary(c.pending)
		if frameEnd < 0 {
			return
		}
		frame := c.pending[:frameEnd]
		c.pending = c.pending[frameEnd+separatorLength:]
		c.appendSSEFrame(frame)
	}
}

func nextSSEFrameBoundary(pending string) (int, int) {
	lf := strings.Index(pending, "\n\n")
	crlf := strings.Index(pending, "\r\n\r\n")
	switch {
	case lf < 0:
		return crlf, 4
	case crlf < 0 || lf < crlf:
		return lf, 2
	default:
		return crlf, 4
	}
}

func (c *DownstreamOutputTokenCollector) bindCancellation(done <-chan struct{}) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.cancelDone = done
	c.freezeIfCanceledLocked()
	c.mu.Unlock()
}

func (c *DownstreamOutputTokenCollector) bindUpstreamCancel(cancel context.CancelFunc) {
	if c == nil || cancel == nil {
		return
	}
	c.mu.Lock()
	c.upstreamCancel = cancel
	frozen := c.frozen
	c.mu.Unlock()
	if frozen {
		c.cancelUpstream()
	}
}

func (c *DownstreamOutputTokenCollector) cancelUpstream() {
	if c == nil {
		return
	}
	c.mu.Lock()
	cancel := c.upstreamCancel
	c.mu.Unlock()
	if cancel == nil {
		return
	}
	c.upstreamCancelOnce.Do(cancel)
}

func (c *DownstreamOutputTokenCollector) freezeIfCanceledLocked() {
	if c.frozen || c.cancelDone == nil {
		return
	}
	select {
	case <-c.cancelDone:
		c.frozen = true
		c.pending = ""
	default:
	}
}

// Freeze prevents bytes written after a client cancellation from affecting
// the user-facing delivered-token snapshot while the upstream may drain.
func (c *DownstreamOutputTokenCollector) Freeze() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.frozen = true
	c.pending = ""
	c.mu.Unlock()
	c.cancelUpstream()
}

func (c *DownstreamOutputTokenCollector) Frozen() bool {
	if c == nil {
		return true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.freezeIfCanceledLocked()
	return c.frozen
}

func (c *DownstreamOutputTokenCollector) VisibleText() string {
	if c == nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.freezeIfCanceledLocked()
	return c.text.String()
}

func (c *DownstreamOutputTokenCollector) TokenCount() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	c.freezeIfCanceledLocked()
	text := c.text.String()
	model := c.model
	c.mu.Unlock()
	if strings.TrimSpace(text) == "" {
		return 0
	}
	codec, err := openAIInputTokensCodecForModel(model)
	if err != nil {
		return 0
	}
	tokens, _, err := codec.Encode(text)
	if err != nil {
		return 0
	}
	return len(tokens)
}

func (c *DownstreamOutputTokenCollector) appendSSEFrame(frame string) {
	eventType := ""
	var dataLines []string
	for _, line := range strings.Split(frame, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if len(dataLines) == 0 {
		return
	}
	c.appendSSEPayload(strings.Join(dataLines, "\n"), eventType)
}

func (c *DownstreamOutputTokenCollector) appendSSEPayload(payload, eventType string) {
	payload = strings.TrimSpace(payload)
	if payload == "" || payload == "[DONE]" || !gjson.Valid(payload) {
		return
	}

	data := gjson.Parse(payload)
	payloadType := strings.TrimSpace(data.Get("type").String())
	if payloadType == "" {
		payloadType = eventType
	}
	switch payloadType {
	case "content_block_start":
		block := data.Get("content_block")
		if block.Get("type").String() == "text" {
			c.appendText(block.Get("text").String())
		}
		return
	case "content_block_delta":
		delta := data.Get("delta")
		switch delta.Get("type").String() {
		case "text_delta":
			c.appendText(delta.Get("text").String())
		case "thinking_delta":
			c.appendText(delta.Get("thinking").String())
		case "input_json_delta":
			c.appendText(delta.Get("partial_json").String())
		}
		return
	case "response.output_text.delta", "response.refusal.delta",
		"response.reasoning_summary_text.delta", "response.reasoning_text.delta", "response.reasoning.delta",
		"response.function_call_arguments.delta", "response.custom_tool_call_input.delta":
		c.appendText(data.Get("delta").String())
		return
	}

	data.Get("choices").ForEach(func(_, choice gjson.Result) bool {
		delta := choice.Get("delta")
		c.appendText(delta.Get("content").String())
		c.appendText(delta.Get("reasoning_content").String())
		delta.Get("tool_calls").ForEach(func(_, toolCall gjson.Result) bool {
			c.appendText(toolCall.Get("function.arguments").String())
			return true
		})
		c.appendText(delta.Get("function_call.arguments").String())
		return true
	})
	data.Get("candidates").ForEach(func(_, candidate gjson.Result) bool {
		candidate.Get("content.parts").ForEach(func(_, part gjson.Result) bool {
			c.appendText(part.Get("text").String())
			return true
		})
		return true
	})
}

func (c *DownstreamOutputTokenCollector) appendText(text string) {
	if text != "" {
		_, _ = c.text.WriteString(text)
	}
}

type downstreamOutputTokenResponseWriter struct {
	gin.ResponseWriter
	collector *DownstreamOutputTokenCollector
}

func (w *downstreamOutputTokenResponseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	if n > 0 {
		w.collector.ObserveWritten(data[:n])
	}
	if err != nil {
		w.collector.Freeze()
	}
	return n, err
}

func (w *downstreamOutputTokenResponseWriter) WriteString(data string) (int, error) {
	n, err := w.ResponseWriter.WriteString(data)
	if n > 0 {
		w.collector.ObserveWritten([]byte(data[:n]))
	}
	if err != nil {
		w.collector.Freeze()
	}
	return n, err
}

// AttachDownstreamOutputTokenCollector wraps the active request writer for the
// lifetime of a gateway forward. The restore callback is idempotent.
func AttachDownstreamOutputTokenCollector(c *gin.Context, model string) (*DownstreamOutputTokenCollector, func()) {
	collector := NewDownstreamOutputTokenCollector(model)
	if c == nil || c.Writer == nil {
		return collector, func() {}
	}
	original := c.Writer
	wrapped := &downstreamOutputTokenResponseWriter{ResponseWriter: original, collector: collector}
	c.Writer = wrapped
	stopWatch := make(chan struct{})
	var cancelForward context.CancelFunc
	if c.Request != nil {
		forwardContext, cancel := context.WithCancel(c.Request.Context())
		cancelForward = cancel
		c.Request = c.Request.WithContext(forwardContext)
		collector.bindCancellation(forwardContext.Done())
		collector.bindUpstreamCancel(cancel)
		go func(ctx context.Context) {
			select {
			case <-ctx.Done():
				collector.Freeze()
			case <-stopWatch:
			}
		}(forwardContext)
	}
	var restoreOnce sync.Once
	return collector, func() {
		restoreOnce.Do(func() {
			close(stopWatch)
			if cancelForward != nil {
				cancelForward()
			}
			if c.Writer == wrapped {
				c.Writer = original
			}
		})
	}
}

// ApplyDeliveredOutputTokens snapshots display-only output without mutating
// provider usage, which remains authoritative for billing.
func ApplyDeliveredOutputTokens(result *ForwardResult, collector *DownstreamOutputTokenCollector) {
	if result == nil || collector == nil {
		return
	}
	if collector.Frozen() {
		result.ClientDisconnect = true
	}
	if result.DeliveredOutputTokens != nil || result.ImageCount > 0 || result.Usage.ImageOutputTokens > 0 {
		return
	}
	delivered := collector.TokenCount()
	result.DeliveredOutputTokens = &delivered
}

func ApplyDeliveredOpenAIOutputTokens(result *OpenAIForwardResult, collector *DownstreamOutputTokenCollector) {
	if result == nil || collector == nil {
		return
	}
	if collector.Frozen() {
		result.ClientDisconnect = true
	}
	if result.DeliveredOutputTokens != nil || result.ImageCount > 0 || result.Usage.ImageOutputTokens > 0 {
		return
	}
	delivered := collector.TokenCount()
	result.DeliveredOutputTokens = &delivered
}

func recordedOutputTokens(result *ForwardResult) int {
	if result == nil {
		return 0
	}
	return customerBillableOutputTokens(result.ClientDisconnect, result.DeliveredOutputTokens, result.Usage.OutputTokens)
}

func recordedOpenAIOutputTokens(result *OpenAIForwardResult) int {
	if result == nil {
		return 0
	}
	return customerBillableOutputTokens(result.ClientDisconnect, result.DeliveredOutputTokens, result.Usage.OutputTokens)
}

func customerBillableOutputTokens(clientDisconnect bool, delivered *int, provider int) int {
	if !clientDisconnect || delivered == nil {
		return provider
	}
	if *delivered < 0 {
		return 0
	}
	return *delivered
}

func customerBillingForwardResult(result *ForwardResult) *ForwardResult {
	if result == nil {
		return nil
	}
	outputTokens := customerBillableOutputTokens(result.ClientDisconnect, result.DeliveredOutputTokens, result.Usage.OutputTokens)
	if outputTokens == result.Usage.OutputTokens {
		return result
	}
	clone := *result
	clone.Usage = result.Usage
	clone.Usage.OutputTokens = outputTokens
	return &clone
}
