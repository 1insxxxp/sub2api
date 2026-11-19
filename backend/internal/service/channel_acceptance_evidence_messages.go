package service

import (
	"encoding/json"
	"math"
	"strconv"
)

type acceptanceMessagesState struct {
	started, terminalOutput, invalidCache bool
	uncachedInput, promptTotal            *int64
}

func (p *acceptanceEvidenceParser) consumeMessages(m map[string]any) bool {
	switch acceptanceString(m["type"]) {
	case "message":
		for i, block := range acceptanceArray(m["content"]) {
			p.messagesBlock(strconv.Itoa(i), acceptanceObject(block), false)
		}
		p.responseModel = acceptanceString(m["model"])
		p.finishReason = acceptanceString(m["stop_reason"])
		p.captureMessagesUsage(acceptanceObject(m["usage"]))
		// message_start carries a nested message too. Only a standalone JSON
		// message can prove completion without a message_stop event.
		p.completed = p.mode == 1 && p.finishReason != ""
		p.usageFinal = p.completed && p.usage.Input != nil && p.usage.Output != nil
	case "message_start":
		msg := acceptanceObject(m["message"])
		p.messages.started = msg != nil
		p.responseModel = acceptanceString(msg["model"])
		p.captureMessagesUsage(acceptanceObject(msg["usage"]))
	case "content_block_start":
		p.messagesBlock(acceptanceIndex(m["index"]), acceptanceObject(m["content_block"]), true)
	case "content_block_delta":
		d := acceptanceObject(m["delta"])
		switch acceptanceString(d["type"]) {
		case "text_delta":
			p.text(acceptanceString(d["text"]), false)
		case "thinking_delta":
			p.text(acceptanceString(d["thinking"]), true)
		case "input_json_delta":
			p.hasTool = true
			p.toolFragment(acceptanceIndex(m["index"]), "", acceptanceString(d["partial_json"]))
		}
	case "message_delta":
		u := acceptanceObject(m["usage"])
		p.captureMessagesUsage(u)
		if reason := acceptanceString(acceptanceObject(m["delta"])["stop_reason"]); reason != "" {
			p.finishReason = reason
			p.messages.terminalOutput = acceptanceNumber(u["output_tokens"]) != nil
		}
	case "message_stop":
		p.protocolEnded = true
		p.completed = p.messages.started && p.finishReason != ""
		p.usageFinal = p.completed && p.usage.Input != nil && p.messages.terminalOutput
	case "error":
		p.protocolEnded = true
		p.completed, p.usageFinal = false, false
	default:
		return false
	}
	return true
}

func acceptanceArray(value any) []any {
	a, _ := value.([]any)
	return a
}

func acceptanceIndex(value any) string {
	if n := acceptanceNumber(value); n != nil {
		return strconv.FormatInt(*n, 10)
	}
	return "0"
}

func (p *acceptanceEvidenceParser) messagesBlock(key string, block map[string]any, start bool) {
	switch acceptanceString(block["type"]) {
	case "text":
		p.text(acceptanceString(block["text"]), false)
	case "thinking":
		p.text(acceptanceString(block["thinking"]), true)
	case "redacted_thinking":
		p.hasReasoning = true
	case "tool_use", "server_tool_use":
		p.hasTool = true
		args := ""
		if input := acceptanceObject(block["input"]); input != nil && (!start || len(input) != 0) {
			b, _ := json.Marshal(input)
			args = string(b)
		}
		p.toolFragment(key, acceptanceString(block["name"]), args)
	}
}

func (p *acceptanceEvidenceParser) captureMessagesUsage(u map[string]any) {
	frame := make(acceptanceUsageFrame)
	record := func(dst **int64, paths ...string) {
		if n := frame.capture(u, paths...); n != nil {
			*dst = n
		}
	}
	// Native Messages input buckets are mutually exclusive. Some compatible
	// providers additionally report prompt_tokens inclusive of all cache usage.
	record(&p.messages.uncachedInput, "prompt_cache_miss_tokens", "input_tokens")
	record(&p.messages.promptTotal, "prompt_tokens")
	record(&p.usage.CacheRead, "cache_read_input_tokens", "cached_tokens", "prompt_tokens_details.cached_tokens", "prompt_cache_hit_tokens")
	record(&p.usage.Output, "output_tokens")
	if n := frame.capture(u, "cache_creation_input_tokens"); n != nil {
		p.usage.CacheCreate = n
	} else {
		short := frame.capture(u, "cache_creation.ephemeral_5m_input_tokens")
		long := frame.capture(u, "cache_creation.ephemeral_1h_input_tokens")
		if short != nil || long != nil {
			p.usage.CacheCreate = acceptanceInputSum(short, long)
			if p.usage.CacheCreate == nil {
				p.recordUsageFrame(frame, "upstream_anthropic_reported")
				p.usage.Input = nil
				p.messages.invalidCache = true
				return
			}
		}
	}
	if len(frame) == 0 {
		return
	}
	p.recordUsageFrame(frame, "upstream_anthropic_reported")
	p.usage.OutputIncludesThoughts = p.usage.Output != nil
	if p.messages.invalidCache {
		return
	}
	if p.messages.promptTotal != nil {
		p.usage.Input = p.messages.promptTotal
		return
	}
	if base := p.messages.uncachedInput; base != nil {
		p.usage.Input = acceptanceInputSum(base, p.usage.CacheRead, p.usage.CacheCreate)
	}
}

func acceptanceInputSum(parts ...*int64) *int64 {
	var sum int64
	for _, n := range parts {
		if n != nil {
			if *n > math.MaxInt64-sum {
				return nil
			}
			sum += *n
		}
	}
	return &sum
}
