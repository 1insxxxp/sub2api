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
	mu                   sync.Mutex
	model                string
	pending              string
	text                 strings.Builder
	hasDeliveredText     bool
	hasDeliveredTerminal bool
	frozen               bool
	cancelDone           <-chan struct{}
	upstreamCancel       context.CancelFunc
	upstreamCancelOnce   sync.Once
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
	// Buffered protocol responses are written as one JSON document rather than
	// SSE frames. Wait for the full document before counting its semantic body.
	if strings.HasPrefix(strings.TrimSpace(c.pending), "{") {
		if gjson.Valid(c.pending) {
			c.appendJSONResponse(gjson.Parse(c.pending))
			c.pending = ""
		}
		return
	}
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

// HasDeliveredText reports nonempty answer text parsed from successful writes.
// Reasoning, tool arguments and transport metadata do not prove an answer was
// delivered. The token counter deliberately continues to include those tokens.
func (c *DownstreamOutputTokenCollector) HasDeliveredText() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.freezeIfCanceledLocked()
	return c.hasDeliveredText
}

// HasDeliveredTerminal is set only after a complete terminal protocol frame
// was parsed from successfully written bytes, not merely observed upstream.
func (c *DownstreamOutputTokenCollector) HasDeliveredTerminal() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.freezeIfCanceledLocked()
	return c.hasDeliveredTerminal
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
	if payload == "[DONE]" {
		// DONE closes the transport but cannot prove a successful finish reason.
		return
	}
	if payload == "" || !gjson.Valid(payload) {
		return
	}

	data := gjson.Parse(payload)
	payloadType := strings.TrimSpace(data.Get("type").String())
	if payloadType == "" {
		payloadType = eventType
	}
	if payloadType == "message_stop" {
		c.hasDeliveredTerminal = true
	}
	c.observeDeliveredTerminal(data)
	switch payloadType {
	case "content_block_start":
		block := data.Get("content_block")
		if block.Get("type").String() == "text" {
			c.appendAnswerText(block.Get("text").String())
		}
		return
	case "content_block_delta":
		delta := data.Get("delta")
		switch delta.Get("type").String() {
		case "text_delta":
			c.appendAnswerText(delta.Get("text").String())
		case "thinking_delta":
			c.appendText(delta.Get("thinking").String())
		case "input_json_delta":
			c.appendText(delta.Get("partial_json").String())
		}
		return
	case "response.output_text.delta", "response.refusal.delta":
		c.appendAnswerText(data.Get("delta").String())
		return
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta", "response.reasoning.delta",
		"response.function_call_arguments.delta", "response.custom_tool_call_input.delta":
		c.appendText(data.Get("delta").String())
		return
	}

	data.Get("choices").ForEach(func(_, choice gjson.Result) bool {
		delta := choice.Get("delta")
		c.appendAnswerText(delta.Get("content").String())
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
			if part.Get("thought").Bool() {
				c.appendText(part.Get("text").String())
			} else {
				c.appendAnswerText(part.Get("text").String())
			}
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

func (c *DownstreamOutputTokenCollector) appendAnswerText(text string) {
	if strings.TrimSpace(text) != "" {
		c.hasDeliveredText = true
	}
	c.appendText(text)
}

func (c *DownstreamOutputTokenCollector) appendJSONResponse(data gjson.Result) {
	responseError := data.Get("error")
	if (responseError.Exists() && responseError.Type != gjson.Null) || data.Get("type").String() == "error" || data.Get("status").String() == "failed" {
		return
	}
	c.observeDeliveredTerminal(data)
	data.Get("content").ForEach(func(_, block gjson.Result) bool {
		switch block.Get("type").String() {
		case "text":
			c.appendAnswerText(block.Get("text").String())
		case "thinking":
			c.appendText(block.Get("thinking").String())
		case "tool_use", "server_tool_use":
			if input := block.Get("input"); input.Exists() && input.Type != gjson.Null {
				c.appendText(input.Raw)
			}
		}
		return true
	})
	data.Get("choices").ForEach(func(_, choice gjson.Result) bool {
		message := choice.Get("message")
		c.appendJSONMessageContent(message.Get("content"), true)
		c.appendText(message.Get("reasoning_content").String())
		if reasoning := message.Get("reasoning"); reasoning.Type == gjson.String {
			c.appendText(reasoning.String())
		}
		message.Get("tool_calls").ForEach(func(_, call gjson.Result) bool {
			c.appendText(call.Get("function.arguments").String())
			return true
		})
		c.appendText(message.Get("function_call.arguments").String())
		return true
	})
	data.Get("output").ForEach(func(_, item gjson.Result) bool {
		switch item.Get("type").String() {
		case "message":
			c.appendJSONMessageContent(item.Get("content"), true)
		case "reasoning":
			c.appendJSONMessageContent(item.Get("summary"), false)
			c.appendJSONMessageContent(item.Get("content"), false)
		case "function_call":
			c.appendText(item.Get("arguments").String())
		case "custom_tool_call":
			c.appendText(item.Get("input").String())
		}
		return true
	})
	data.Get("candidates").ForEach(func(_, candidate gjson.Result) bool {
		candidate.Get("content.parts").ForEach(func(_, part gjson.Result) bool {
			if part.Get("thought").Bool() {
				c.appendText(part.Get("text").String())
			} else {
				c.appendAnswerText(part.Get("text").String())
			}
			return true
		})
		return true
	})
}

func (c *DownstreamOutputTokenCollector) observeDeliveredTerminal(data gjson.Result) {
	data.Get("choices").ForEach(func(_, choice gjson.Result) bool {
		if strings.TrimSpace(choice.Get("finish_reason").String()) != "" {
			c.hasDeliveredTerminal = true
		}
		return true
	})
	data.Get("candidates").ForEach(func(_, candidate gjson.Result) bool {
		if strings.TrimSpace(candidate.Get("finishReason").String()) != "" {
			c.hasDeliveredTerminal = true
		}
		return true
	})
	if strings.TrimSpace(data.Get("stop_reason").String()) != "" {
		c.hasDeliveredTerminal = true
	}
}

func (c *DownstreamOutputTokenCollector) appendJSONMessageContent(content gjson.Result, answer bool) {
	appendContent := c.appendText
	if answer {
		appendContent = c.appendAnswerText
	}
	if content.Type == gjson.String {
		appendContent(content.String())
		return
	}
	content.ForEach(func(_, part gjson.Result) bool {
		switch part.Get("type").String() {
		case "text", "output_text":
			appendContent(part.Get("text").String())
		case "summary_text", "reasoning_text":
			c.appendText(part.Get("text").String())
		case "refusal":
			appendContent(part.Get("refusal").String())
		}
		return true
	})
}

type downstreamOutputTokenResponseWriter struct {
	gin.ResponseWriter
	collector *DownstreamOutputTokenCollector
}

func (w *downstreamOutputTokenResponseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	if n > 0 && w.Status() >= 200 && w.Status() < 300 {
		w.collector.ObserveWritten(data[:n])
	}
	if err != nil {
		w.collector.Freeze()
	}
	return n, err
}

func (w *downstreamOutputTokenResponseWriter) WriteString(data string) (int, error) {
	n, err := w.ResponseWriter.WriteString(data)
	if n > 0 && w.Status() >= 200 && w.Status() < 300 {
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

// ApplyDeliveredOutputTokens snapshots output that reached the client. It is
// used as a billing fallback by protocols that allow estimates. Gemini measured
// mode keeps it as delivery evidence; its own fallback counts received upstream
// semantics, which may include output whose downstream write failed.
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
	if result.RequireMeasuredUsage {
		return result.Usage.OutputTokens
	}
	if result.Usage.OutputTokens > 0 && (upstreamUsageComplete(result.Outcome) || (!result.Stream && !result.ClientDisconnect)) {
		return result.Usage.OutputTokens
	}
	if result.Outcome != nil && result.DeliveredOutputTokens != nil {
		return deliveredOutputTokens(result.DeliveredOutputTokens)
	}
	return customerBillableOutputTokens(result.ClientDisconnect, result.DeliveredOutputTokens, result.Usage.OutputTokens)
}

func recordedOpenAIOutputTokens(result *OpenAIForwardResult) int {
	if result == nil {
		return 0
	}
	if upstreamUsageComplete(result.Outcome) && result.Usage.OutputTokens > 0 {
		return result.Usage.OutputTokens
	}
	if result.Outcome != nil && result.DeliveredOutputTokens != nil {
		return deliveredOutputTokens(result.DeliveredOutputTokens)
	}
	return customerBillableOutputTokens(result.ClientDisconnect, result.DeliveredOutputTokens, result.Usage.OutputTokens)
}

func upstreamUsageComplete(outcome *ResponseOutcome) bool {
	return outcome != nil && outcome.StreamCompleted
}

func customerBillableOutputTokens(clientDisconnect bool, delivered *int, provider int) int {
	if delivered == nil || (!clientDisconnect && provider > 0) {
		return provider
	}
	return deliveredOutputTokens(delivered)
}

func deliveredOutputTokens(delivered *int) int {
	if *delivered < 0 {
		return 0
	}
	return *delivered
}

func customerBillingForwardResult(result *ForwardResult) *ForwardResult {
	if result == nil {
		return nil
	}
	outputTokens := recordedOutputTokens(result)
	if outputTokens == result.Usage.OutputTokens {
		return result
	}
	clone := *result
	clone.Usage = result.Usage
	clone.Usage.OutputTokens = outputTokens
	return &clone
}
