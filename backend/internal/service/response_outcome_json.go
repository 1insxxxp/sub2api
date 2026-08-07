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

func (c *ResponseOutcomeCollector) observeOpenAIJSON(root gjson.Result) {
	root.Get("choices").ForEach(func(_, choice gjson.Result) bool {
		message := choice.Get("message")
		c.ObserveText(message.Get("content").String())
		c.ObserveReasoning(message.Get("reasoning_content").String())
		if message.Get("tool_calls.#").Int() > 0 || message.Get("function_call").Exists() {
			c.ObserveToolCall()
		}
		if reason := choice.Get("finish_reason").String(); reason != "" {
			c.MarkCompleted(reason)
		}
		return true
	})

	root.Get("output").ForEach(func(_, item gjson.Result) bool {
		typ := strings.ToLower(item.Get("type").String())
		switch typ {
		case "function_call", "computer_call", "web_search_call", "file_search_call", "mcp_call":
			c.ObserveToolCall()
		case "reasoning":
			c.ObserveReasoning(item.Get("summary.0.text").String())
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
		return true
	})

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
