package service

import (
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// DownstreamOutputTokenCollector tracks only semantic text that was written to
// the downstream client. It intentionally excludes reasoning and tool payloads.
type DownstreamOutputTokenCollector struct {
	mu      sync.Mutex
	model   string
	pending string
	text    strings.Builder
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

	c.pending += string(written)
	for {
		frameEnd := strings.Index(c.pending, "\n\n")
		if frameEnd < 0 {
			return
		}
		frame := c.pending[:frameEnd]
		c.pending = c.pending[frameEnd+2:]
		c.appendSSEFrame(frame)
	}
}

func (c *DownstreamOutputTokenCollector) TokenCount() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
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
		payloadType = strings.TrimSpace(eventType)
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
		if delta.Get("type").String() == "text_delta" {
			c.appendText(delta.Get("text").String())
		}
		return
	case "response.output_text.delta":
		c.appendText(data.Get("delta").String())
		return
	}

	data.Get("choices").ForEach(func(_, choice gjson.Result) bool {
		c.appendText(choice.Get("delta.content").String())
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
		c.text.WriteString(text)
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
	return n, err
}

func (w *downstreamOutputTokenResponseWriter) WriteString(data string) (int, error) {
	n, err := w.ResponseWriter.WriteString(data)
	if n > 0 {
		w.collector.ObserveWritten([]byte(data[:n]))
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
	return collector, func() {
		if c.Writer == wrapped {
			c.Writer = original
		}
	}
}

// ApplyDeliveredOutputTokenBilling swaps token-billed streaming output to the
// text actually sent to the client. The upstream final amount remains on the
// result for administrator cost diagnostics.
func ApplyDeliveredOutputTokenBilling(result *ForwardResult, collector *DownstreamOutputTokenCollector) {
	if result == nil || result.DownstreamOutputTokenBilling || collector == nil || result.ImageCount > 0 || result.Usage.ImageOutputTokens > 0 {
		return
	}
	result.UpstreamOutputTokens = result.Usage.OutputTokens
	result.Usage.OutputTokens = collector.TokenCount()
	result.DownstreamOutputTokenBilling = true
}

// ApplyDeliveredOpenAIOutputTokenBilling is the OpenAI-compatible equivalent
// of ApplyDeliveredOutputTokenBilling.
func ApplyDeliveredOpenAIOutputTokenBilling(result *OpenAIForwardResult, collector *DownstreamOutputTokenCollector) {
	if result == nil || result.DownstreamOutputTokenBilling || collector == nil || result.ImageCount > 0 || result.Usage.ImageOutputTokens > 0 {
		return
	}
	result.UpstreamOutputTokens = result.Usage.OutputTokens
	result.Usage.OutputTokens = collector.TokenCount()
	result.DownstreamOutputTokenBilling = true
}

func recordedUpstreamOutputTokens(result *ForwardResult) int {
	if result == nil {
		return 0
	}
	if result.DownstreamOutputTokenBilling {
		return result.UpstreamOutputTokens
	}
	return result.Usage.OutputTokens
}

func recordedOpenAIUpstreamOutputTokens(result *OpenAIForwardResult) int {
	if result == nil {
		return 0
	}
	if result.DownstreamOutputTokenBilling {
		return result.UpstreamOutputTokens
	}
	return result.Usage.OutputTokens
}
