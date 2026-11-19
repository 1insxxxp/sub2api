package service

import (
	"sort"
	"strconv"
	"strings"
)

// Several Responses events repeat a complete snapshot of the same content part
// or tool call after streaming its deltas. Keep each indexed part once.
type acceptanceResponsePart struct{ output, content int64 }
type acceptanceResponsesState struct {
	parts   map[acceptanceResponsePart]string
	toolIDs map[int64]string
}

func (p *acceptanceEvidenceParser) consumeResponses(m map[string]any) bool {
	typ := acceptanceString(m["type"])
	if !strings.HasPrefix(typ, "response.") {
		if typ != "" || (acceptanceString(m["object"]) != "response" && m["output"] == nil) {
			return false
		}
		status := acceptanceString(m["status"])
		p.completed = p.mode == 1 && status == "completed"
		if status != "" {
			p.finishReason = status
		}
		p.responseObject(m)
		return true
	}
	r := acceptanceObject(m["response"])
	switch typ {
	case "response.completed", "response.done", "response.incomplete", "response.failed", "response.cancelled":
		status := acceptanceString(r["status"])
		p.completed = (typ == "response.completed" && (status == "" || status == "completed")) ||
			(typ == "response.done" && status == "completed")
		p.finishReason = strings.TrimPrefix(typ, "response.")
		if p.completed {
			p.finishReason = "completed"
		} else if detail := acceptanceString(acceptanceObject(r["incomplete_details"])["reason"]); detail != "" {
			p.finishReason = detail
		} else if status != "" && status != "completed" {
			p.finishReason = status
		}
		p.responseObject(r)
		p.captureUsage(acceptanceObject(m["usage"]), "upstream_responses_reported")
		p.protocolEnded = true
	case "response.created", "response.in_progress", "response.queued":
		p.responseObject(r)
	case "response.output_item.added", "response.output_item.done":
		p.responseItem(acceptanceResponseIndex(m["output_index"]), acceptanceObject(m["item"]))
	case "response.content_part.added", "response.content_part.done", "response.reasoning_summary_part.added", "response.reasoning_summary_part.done":
		p.responsePart(acceptanceResponseIndex(m["output_index"]), acceptanceResponseIndex(m["content_index"]), acceptanceObject(m["part"]))
	case "response.output_text.delta", "response.output_text.done":
		delta := typ == "response.output_text.delta"
		s := acceptanceString(m["text"])
		if delta {
			s = acceptanceString(m["delta"])
		}
		p.responseText(acceptanceResponseIndex(m["output_index"]), acceptanceResponseIndex(m["content_index"]), s, delta)
	case "response.reasoning_text.delta", "response.reasoning_summary_text.delta":
		p.text(acceptanceString(m["delta"]), true)
	case "response.reasoning_text.done", "response.reasoning_summary_text.done":
		p.text(acceptanceString(m["text"]), true)
	case "response.function_call_arguments.delta", "response.function_call_arguments.done":
		key := p.responseToolKey(acceptanceResponseIndex(m["output_index"]), acceptanceString(m["item_id"]))
		p.hasTool = true
		if typ == "response.function_call_arguments.delta" {
			p.toolFragment(key, acceptanceString(m["name"]), acceptanceString(m["delta"]))
		} else {
			p.responseTool(key, acceptanceString(m["name"]), m["arguments"])
		}
	}
	return true
}

func acceptanceResponseIndex(v any) int64 {
	if n := acceptanceNumber(v); n != nil {
		return *n
	}
	return 0
}

func (p *acceptanceEvidenceParser) responseObject(r map[string]any) {
	if model := acceptanceString(r["model"]); model != "" {
		p.responseModel = model
	}
	for i, item := range acceptanceArray(r["output"]) {
		p.responseItem(int64(i), acceptanceObject(item))
	}
	p.captureUsage(acceptanceObject(r["usage"]), "upstream_responses_reported")
}

func (p *acceptanceEvidenceParser) responseItem(index int64, item map[string]any) {
	switch acceptanceString(item["type"]) {
	case "message":
		for i, part := range acceptanceArray(item["content"]) {
			p.responsePart(index, int64(i), acceptanceObject(part))
		}
	case "reasoning":
		// An encrypted/redacted reasoning item still proves there was a
		// reasoning block, even if the provider withholds its summary.
		p.hasReasoning = true
	case "function_call":
		p.hasTool = true
		id := acceptanceString(item["id"])
		key := p.responseToolKey(index, id)
		if id != "" && len(id) <= 128 && len(p.responses.toolIDs) < 16 {
			if p.responses.toolIDs == nil {
				p.responses.toolIDs = make(map[int64]string)
			}
			p.responses.toolIDs[index] = id
		}
		p.responseTool(key, acceptanceString(item["name"]), item["arguments"])
	}
}

func (p *acceptanceEvidenceParser) responseToolKey(index int64, id string) string {
	if id == "" {
		id = p.responses.toolIDs[index]
	}
	if id != "" && len(id) <= 128 {
		return "item:" + id
	}
	return strconv.FormatInt(index, 10)
}

func (p *acceptanceEvidenceParser) responseTool(key, name string, arguments any) {
	// A nonempty snapshot supersedes deltas for this call. The skeleton item
	// announced at the start (or echoed by some bridges at the end) is empty.
	if args := acceptanceString(arguments); args != "" {
		if t := p.tools[key]; t != nil {
			t.args, t.truncated = "", false
		}
		p.toolFragment(key, name, args)
	} else {
		p.toolFragment(key, name, "")
	}
}

func (p *acceptanceEvidenceParser) responsePart(output, content int64, part map[string]any) {
	switch acceptanceString(part["type"]) {
	case "output_text":
		p.responseText(output, content, acceptanceString(part["text"]), false)
	case "reasoning_text", "summary_text":
		p.text(acceptanceString(part["text"]), true)
	}
}

func (p *acceptanceEvidenceParser) responseText(output, content int64, text string, delta bool) {
	if text == "" {
		return
	}
	p.hasText = true
	if p.responses.parts == nil {
		p.responses.parts = make(map[acceptanceResponsePart]string)
	}
	k := acceptanceResponsePart{output, content}
	previous, ok := p.responses.parts[k]
	if !ok && len(p.responses.parts) >= 32 {
		p.truncated = true
		return
	}
	if delta {
		text = previous + acceptanceBoundedText(text, acceptanceExcerptLimit-len(previous))
	}
	p.responses.parts[k] = acceptanceBoundedText(text, acceptanceExcerptLimit)
}

func (p *acceptanceEvidenceParser) finishResponseText() {
	if len(p.responses.parts) == 0 {
		return
	}
	keys := make([]acceptanceResponsePart, 0, len(p.responses.parts))
	for k := range p.responses.parts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].output == keys[j].output {
			return keys[i].content < keys[j].content
		}
		return keys[i].output < keys[j].output
	})
	p.excerpt = ""
	for _, k := range keys {
		p.text(p.responses.parts[k], false)
	}
}
