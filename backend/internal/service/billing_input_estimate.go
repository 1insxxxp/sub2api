package service

import (
	"encoding/json"
	"errors"
	"unicode/utf8"

	"github.com/tidwall/gjson"
	"github.com/tiktoken-go/tokenizer"
)

var errInflightOpaqueInput = errors.New("input requires media or cached-content token accounting")

// Decoding model inputs before tokenization excludes JSON transport escapes and
// client metadata. Opaque media/cached inputs retain the conservative fallback.
func estimateInflightInputTokens(model string, body []byte) (int, bool) {
	if len(body) == 0 {
		return 0, false
	}
	var request map[string]json.RawMessage
	if json.Unmarshal(body, &request) != nil {
		return 0, false
	}
	if len(request["cachedContent"]) > 0 || len(request["cached_content"]) > 0 {
		return 0, false
	}
	codec, err := openAIInputTokensCodecForModel(model)
	if err != nil {
		return 0, false
	}
	total := 0
	found := false
	for _, field := range []string{"messages", "system", "input", "instructions", "tools", "tool_choice", "functions", "function_call", "contents", "systemInstruction", "system_instruction", "response_format"} {
		raw, exists := request[field]
		if !exists {
			continue
		}
		var value any
		if json.Unmarshal(raw, &value) != nil {
			return 0, false
		}
		var count int
		switch field {
		case "tools", "tool_choice", "functions", "function_call", "response_format":
			count, err = countInflightJSON(codec, projectInflightTool(value))
		case "input":
			var tokenIDs bool
			count, tokenIDs = countInflightTokenIDs(value)
			if !tokenIDs {
				count, err = countInflightInputValue(codec, value)
			}
		default:
			count, err = countInflightInputValue(codec, value)
		}
		if err != nil {
			return 0, false
		}
		total += count
		found = true
	}
	// Output schemas are visible to the model, unlike generation limits/samplers.
	for _, path := range []string{"text.format", "generationConfig.responseSchema", "generationConfig.responseJsonSchema", "generation_config.response_schema", "generation_config.response_json_schema"} {
		raw := gjson.GetBytes(body, path)
		if !raw.Exists() {
			continue
		}
		var schema any
		if json.Unmarshal([]byte(raw.Raw), &schema) != nil {
			return 0, false
		}
		count, err := countInflightJSON(codec, schema)
		if err != nil {
			return 0, false
		}
		total += count
		found = true
	}
	return total, found
}

func countInflightJSON(codec tokenizer.Codec, value any) (int, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	return countInflightText(codec, string(raw))
}

// A bounded BPE input prevents quadratic work on long unbroken strings. Cutting
// only at rune boundaries keeps Unicode valid with small boundary differences.
func countInflightText(codec tokenizer.Codec, text string) (int, error) {
	const chunkBytes = 2048
	total := 0
	for len(text) > chunkBytes {
		end := chunkBytes
		for !utf8.RuneStart(text[end]) {
			end--
		}
		count, err := codec.Count(text[:end])
		if err != nil {
			return 0, err
		}
		total += count
		text = text[end:]
	}
	count, err := codec.Count(text)
	return total + count, err
}

// Embeddings can supply token IDs directly, including batches. Each supplied ID
// represents one token, so its decimal representation must not be tokenized.
func countInflightTokenIDs(value any) (int, bool) {
	items, ok := value.([]any)
	if !ok {
		return 0, false
	}
	total := 0
	for _, item := range items {
		if _, ok := item.(float64); ok {
			total++
			continue
		}
		count, ok := countInflightTokenIDs(item)
		if !ok {
			return 0, false
		}
		total += count
	}
	return total, true
}

func countInflightInputValue(codec tokenizer.Codec, value any) (int, error) {
	switch item := value.(type) {
	case nil:
		return 0, nil
	case string:
		return countInflightText(codec, item)
	case []any:
		total := 0
		for _, child := range item {
			count, err := countInflightInputValue(codec, child)
			if err != nil {
				return 0, err
			}
			total += count
		}
		return total, nil
	case map[string]any:
		switch item["type"] {
		case "image", "input_image", "image_url", "input_audio", "audio", "video", "input_file", "file", "document", "redacted_thinking":
			return 0, errInflightOpaqueInput
		}
		for _, key := range []string{"inlineData", "inline_data", "fileData", "file_data", "encrypted_content", "audio", "image_url"} {
			if item[key] != nil {
				return 0, errInflightOpaqueInput
			}
		}
		total := openAIResponsesInputItemTokenOverhead
		// Protocol fields only. In particular, metadata, cache_control and signatures
		// are not model text. Structured tool arguments/results include all JSON keys.
		for _, key := range []string{"role", "type", "content", "text", "parts", "name", "arguments", "output", "tool_calls", "function", "function_call", "functionCall", "functionResponse", "tool_use_id", "tool_call_id", "call_id", "id", "input", "response", "args", "thinking", "reasoning_content", "summary", "refusal"} {
			child, exists := item[key]
			if !exists {
				continue
			}
			var count int
			var err error
			switch key {
			case "input", "response", "args", "output":
				if text, ok := child.(string); ok {
					count, err = countInflightText(codec, text)
				} else {
					count, err = countInflightJSON(codec, child)
				}
			default:
				count, err = countInflightInputValue(codec, child)
			}
			if err != nil {
				return 0, err
			}
			total += count
		}
		return total, nil
	default:
		return countInflightJSON(codec, item)
	}
}

// Schema objects are preserved verbatim. Only protocol wrapper fields are
// projected, excluding extensions discarded by the gateway converters.
func projectInflightTool(value any) any {
	switch item := value.(type) {
	case []any:
		result := make([]any, 0, len(item))
		for _, child := range item {
			result = append(result, projectInflightTool(child))
		}
		return result
	case map[string]any:
		result := make(map[string]any)
		for _, key := range []string{"type", "name", "description", "strict", "parameters", "input_schema", "schema", "format", "json_schema", "function", "tools", "children", "functionDeclarations", "function_declarations", "googleSearch", "google_search", "codeExecution", "code_execution", "urlContext", "url_context", "allowed_x_handles", "excluded_x_handles", "from_date", "to_date", "enable_image_understanding", "enable_video_understanding"} {
			child, exists := item[key]
			if !exists {
				continue
			}
			switch key {
			case "parameters", "input_schema", "schema":
				result[key] = child
			default:
				result[key] = projectInflightTool(child)
			}
		}
		return result
	default:
		return value
	}
}
