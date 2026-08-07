package service

import (
	"errors"
	"strings"

	"github.com/tidwall/gjson"
)

type ResponseOutcomeProtocol string

const (
	ResponseOutcomeProtocolAnthropic ResponseOutcomeProtocol = "anthropic"
	ResponseOutcomeProtocolOpenAI    ResponseOutcomeProtocol = "openai"
	ResponseOutcomeProtocolGemini    ResponseOutcomeProtocol = "gemini"
)

func (c *ResponseOutcomeCollector) ObserveJSONPayload(protocol ResponseOutcomeProtocol, payload []byte) error {
	if c == nil {
		return nil
	}
	if !gjson.ValidBytes(payload) {
		return errors.New("invalid response JSON")
	}
	root := gjson.ParseBytes(payload)
	switch protocol {
	case ResponseOutcomeProtocolAnthropic:
		c.observeAnthropicJSON(root)
	case ResponseOutcomeProtocolOpenAI:
		c.observeOpenAIJSON(root)
	case ResponseOutcomeProtocolGemini:
		c.observeGeminiJSON(root)
	default:
		return errors.New("unsupported response outcome protocol")
	}
	return nil
}

func (c *ResponseOutcomeCollector) observeAnthropicJSON(root gjson.Result) {
	root.Get("content").ForEach(func(_, block gjson.Result) bool {
		switch strings.ToLower(block.Get("type").String()) {
		case "text":
			c.ObserveText(block.Get("text").String())
		case "tool_use", "server_tool_use":
			c.ObserveToolCall()
		case "thinking", "redacted_thinking":
			c.ObserveReasoning(block.Get("thinking").String())
		case "image", "document":
			c.ObserveMedia()
		}
		return true
	})
	if reason := root.Get("stop_reason").String(); reason != "" {
		c.MarkCompleted(reason)
	}
}

func (c *ResponseOutcomeCollector) ObserveAnthropicSSEData(data string) {
	if c == nil {
		return
	}
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return
	}
	c.ObserveEvent(len(data))
	if trimmed == "[DONE]" {
		c.MarkCompleted("done")
		return
	}
	if !gjson.Valid(trimmed) {
		return
	}
	root := gjson.Parse(trimmed)
	switch root.Get("type").String() {
	case "message_start":
		c.observeAnthropicJSON(root.Get("message"))
	case "content_block_start":
		switch root.Get("content_block.type").String() {
		case "text":
			c.ObserveText(root.Get("content_block.text").String())
		case "thinking", "redacted_thinking":
			c.ObserveReasoning(root.Get("content_block.thinking").String())
		case "tool_use", "server_tool_use":
			c.ObserveToolCall()
		case "image", "document":
			c.ObserveMedia()
		}
	case "content_block_delta":
		switch root.Get("delta.type").String() {
		case "text_delta":
			c.ObserveText(root.Get("delta.text").String())
		case "thinking_delta", "signature_delta":
			c.ObserveReasoning(root.Get("delta.thinking").String())
		case "input_json_delta":
			c.ObserveToolCall()
		}
	case "message_delta":
		if reason := root.Get("delta.stop_reason").String(); reason != "" {
			c.ObserveFinishReason(reason)
		}
	case "message_stop":
		c.MarkCompleted("message_stop")
	}
}

func (c *ResponseOutcomeCollector) observeOpenAIJSON(root gjson.Result) {
	if response := root.Get("response"); response.Exists() {
		c.observeOpenAIJSON(response)
	}
	root.Get("choices").ForEach(func(_, choice gjson.Result) bool {
		message := choice.Get("message")
		c.ObserveText(message.Get("content").String())
		c.ObserveReasoning(message.Get("reasoning_content").String())
		delta := choice.Get("delta")
		c.ObserveText(delta.Get("content").String())
		c.ObserveReasoning(delta.Get("reasoning_content").String())
		if message.Get("tool_calls.#").Int() > 0 || message.Get("function_call").Exists() {
			c.ObserveToolCall()
		}
		if delta.Get("tool_calls.#").Int() > 0 || delta.Get("function_call").Exists() {
			c.ObserveToolCall()
		}
		if reason := choice.Get("finish_reason").String(); reason != "" {
			c.ObserveFinishReason(reason)
		}
		return true
	})

	root.Get("output").ForEach(func(_, item gjson.Result) bool {
		c.observeOpenAIOutputItem(item)
		return true
	})
	if item := root.Get("item"); item.Exists() {
		c.observeOpenAIOutputItem(item)
	}

	if root.Get("data.#").Int() > 0 {
		root.Get("data").ForEach(func(_, item gjson.Result) bool {
			if item.Get("url").Exists() || item.Get("b64_json").Exists() {
				c.ObserveMedia()
			}
			return true
		})
	}
	if status := strings.ToLower(root.Get("status").String()); status == "completed" {
		c.MarkCompleted(status)
	}
	if typ := strings.ToLower(root.Get("type").String()); strings.Contains(typ, "image_generation") {
		if root.Get("b64_json").Exists() || root.Get("url").Exists() || root.Get("result").Exists() || root.Get("partial_image_b64").Exists() {
			c.ObserveMedia()
		}
		if strings.HasSuffix(typ, ".completed") || strings.HasSuffix(typ, ".done") {
			c.MarkCompleted(typ)
		}
	}
}

func (c *ResponseOutcomeCollector) observeOpenAIOutputItem(item gjson.Result) {
	typ := strings.ToLower(item.Get("type").String())
	switch typ {
	case "function_call", "computer_call", "web_search_call", "file_search_call", "mcp_call", "tool_search_call", "custom_tool_call":
		c.ObserveToolCall()
	case "reasoning":
		c.ObserveReasoning(item.Get("summary.0.text").String())
	case "image", "output_image", "input_image", "image_generation_call":
		if item.Get("result").Exists() || item.Get("url").Exists() || item.Get("b64_json").Exists() || typ != "image_generation_call" {
			c.ObserveMedia()
		}
	}
	item.Get("content").ForEach(func(_, part gjson.Result) bool {
		switch strings.ToLower(part.Get("type").String()) {
		case "output_text", "text":
			c.ObserveText(part.Get("text").String())
		case "image", "output_image", "input_image":
			c.ObserveMedia()
		}
		return true
	})
}

func (c *ResponseOutcomeCollector) ObserveOpenAISSEData(data string) {
	if c == nil {
		return
	}
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return
	}
	c.ObserveEvent(len(data))
	if trimmed == "[DONE]" {
		c.MarkCompleted("done")
		return
	}
	if !gjson.Valid(trimmed) {
		return
	}
	root := gjson.Parse(trimmed)
	typ := strings.ToLower(root.Get("type").String())
	switch typ {
	case "response.output_text.delta", "response.output_text.done", "response.refusal.delta":
		c.ObserveText(root.Get("delta").String())
		c.ObserveText(root.Get("text").String())
	case "response.reasoning_summary_text.delta", "response.reasoning_summary_text.done", "response.reasoning_text.delta", "response.reasoning.delta":
		c.ObserveReasoning(root.Get("delta").String())
		c.ObserveReasoning(root.Get("text").String())
	case "response.function_call_arguments.delta", "response.function_call_arguments.done", "response.custom_tool_call_input.delta", "response.custom_tool_call_input.done":
		c.ObserveToolCall()
	case "response.completed", "response.done":
		c.observeOpenAIJSON(root)
		c.MarkCompleted(typ)
	case "response.failed", "response.incomplete", "response.cancelled", "response.canceled":
		c.observeOpenAIJSON(root)
		c.MarkCompleted(typ)
		c.MarkStreamError(errors.New(typ), false)
	default:
		c.observeOpenAIJSON(root)
	}
}

func (c *ResponseOutcomeCollector) observeGeminiJSON(root gjson.Result) {
	root.Get("candidates").ForEach(func(_, candidate gjson.Result) bool {
		candidate.Get("content.parts").ForEach(func(_, part gjson.Result) bool {
			c.ObserveText(part.Get("text").String())
			if part.Get("thought").Bool() {
				c.ObserveReasoning(part.Get("text").String())
			}
			if part.Get("functionCall").Exists() {
				c.ObserveToolCall()
			}
			if part.Get("inlineData").Exists() || part.Get("fileData").Exists() {
				c.ObserveMedia()
			}
			return true
		})
		if reason := candidate.Get("finishReason").String(); reason != "" {
			c.MarkCompleted(reason)
		}
		return true
	})
}

func (c *ResponseOutcomeCollector) ObserveGeminiSSEData(data string) {
	if c == nil {
		return
	}
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return
	}
	c.ObserveEvent(len(data))
	if trimmed == "[DONE]" {
		c.MarkCompleted("done")
		return
	}
	if !gjson.Valid(trimmed) {
		return
	}
	c.observeGeminiJSON(gjson.Parse(trimmed))
}
