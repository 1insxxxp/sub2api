package apicompat

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Content stays raw until role conversion. Validate cache metadata separately so
// a type error cannot make the permissive text extractor drop an entire prompt.
func validateContentCacheControls(raw json.RawMessage) error {
	var parts []map[string]json.RawMessage
	if json.Unmarshal(raw, &parts) != nil {
		return nil // strings and other legacy content keep their existing handling
	}
	for _, part := range parts {
		if value, ok := part["cache_control"]; ok {
			var cc *AnthropicCacheControl
			if err := json.Unmarshal(value, &cc); err != nil {
				return fmt.Errorf("invalid cache_control on content block: %w", err)
			}
		}
	}
	return nil
}

// Only the Anthropic bridge emits structured Chat tool outputs. The standard
// Chat-to-Responses path keeps its existing string output representation.
func marshalAnthropicBridgeInput(items []ResponsesInputItem) (json.RawMessage, error) {
	out := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		var raw []byte
		var err error
		if len(item.outputRaw) > 0 {
			raw, err = json.Marshal(struct {
				ResponsesInputItem
				Output json.RawMessage `json:"output"`
			}{ResponsesInputItem: item, Output: item.outputRaw})
		} else {
			raw, err = json.Marshal(item)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return json.Marshal(out)
}

// Preserve separators on the uncached side of a breakpoint rather than emitting
// standalone whitespace text blocks (which Anthropic rejects). Trailing blank
// suffixes after a marked block are trimmed, never folded into its cache prefix.
func compactMarkedAnthropicTextBlocks(blocks []AnthropicContentBlock) ([]AnthropicContentBlock, error) {
	hasMarker := false
	for _, block := range blocks {
		hasMarker = hasMarker || block.CacheControl != nil
	}
	if !hasMarker {
		return blocks, nil
	}
	out := make([]AnthropicContentBlock, 0, len(blocks))
	pending := ""
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) == "" {
			if block.CacheControl != nil {
				return nil, fmt.Errorf("cache_control cannot mark a whitespace-only text block")
			}
			pending += block.Text
			continue
		}
		if block.Type == "text" {
			block.Text = pending + block.Text
		}
		pending = ""
		out = append(out, block)
	}
	if pending != "" && len(out) > 0 && out[len(out)-1].Type == "text" && out[len(out)-1].CacheControl == nil {
		out[len(out)-1].Text += pending
	}
	return out, nil
}

// Assistant content normally collapses into one output_text part. When a text
// block has an explicit cache marker, retain the original boundaries instead;
// otherwise the uncached suffix becomes part of the marked prefix.
func chatAssistantPartsWithCacheControl(m ChatMessage, fallback []ResponsesContentPart) ([]ResponsesContentPart, error) {
	var rawParts []json.RawMessage
	if json.Unmarshal(m.Content, &rawParts) != nil {
		return fallback, nil
	}
	var parts []ResponsesContentPart
	hasCacheControl := false
	for _, raw := range rawParts {
		var part struct {
			Type         string                 `json:"type"`
			CacheControl *AnthropicCacheControl `json:"cache_control"`
		}
		if err := json.Unmarshal(raw, &part); err != nil {
			return nil, fmt.Errorf("parse assistant cache control: %w", err)
		}
		// Reuse the existing reasoning/unknown-text conversion for each part.
		single, _ := json.Marshal([]json.RawMessage{raw})
		text, err := parseAssistantContent(single)
		if err != nil {
			return nil, err
		}
		if text == "" {
			continue
		}
		converted := ResponsesContentPart{Type: "output_text", Text: text}
		if part.Type == "text" {
			converted.CacheControl = part.CacheControl
			hasCacheControl = hasCacheControl || part.CacheControl != nil
		}
		parts = append(parts, converted)
	}
	if !hasCacheControl {
		return fallback, nil
	}
	if m.ReasoningContent != "" {
		parts[0].Text = "<thinking>" + m.ReasoningContent + "</thinking>\n" + parts[0].Text
	}
	return parts, nil
}

// Keep system block boundaries only when the client explicitly marks a cache
// prefix. Flattening a marked prefix and its changing suffix into one string
// loses the breakpoint; moving it to the end changes what the client caches.
func responsesSystemCacheBlocks(raw json.RawMessage) []AnthropicContentBlock {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		if text == "" {
			return nil
		}
		return []AnthropicContentBlock{{Type: "text", Text: text}}
	}
	var parts []ResponsesContentPart
	if json.Unmarshal(raw, &parts) != nil {
		return nil
	}
	var blocks []AnthropicContentBlock
	for _, part := range parts {
		if (part.Type == "input_text" || part.Type == "output_text" || part.Type == "text") && part.Text != "" {
			blocks = append(blocks, AnthropicContentBlock{Type: "text", Text: part.Text, CacheControl: part.CacheControl})
		}
	}
	return blocks
}

func marshalAnthropicSystemWithCacheControl(texts []string, blocks []AnthropicContentBlock) (json.RawMessage, error) {
	for _, block := range blocks {
		if block.CacheControl != nil {
			compacted, err := compactMarkedAnthropicTextBlocks(blocks)
			if err != nil {
				return nil, err
			}
			return json.Marshal(compacted)
		}
	}
	// Preserve the existing representation and text joining for unmarked input.
	return json.Marshal(strings.Join(texts, "\n\n"))
}
