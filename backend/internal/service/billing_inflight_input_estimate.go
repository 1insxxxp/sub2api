package service

import (
	"encoding/json"
	"strings"
)

// estimateInflightInputTokens estimates only textual input from a JSON request.
// Raw multimedia payloads (for example base64 image data) are deliberately
// excluded: counting the serialized request bytes as text tokens can inflate a
// single precharge by two orders of magnitude.
func estimateInflightInputTokens(model string, body []byte) (int, bool) {
	if len(body) == 0 {
		return 0, false
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return 0, false
	}
	text := collectInflightText(value, "")
	if text == "" {
		// Valid multimodal JSON with no inline text still has no text token
		// component. Output reservation remains in force.
		return 0, true
	}
	return (len([]rune(text)) + 3) / 4, true
}

func collectInflightText(value any, parent string) string {
	switch v := value.(type) {
	case map[string]any:
		var b strings.Builder
		for key, child := range v {
			if key == "url" || key == "data" || key == "image_url" || key == "input_image" || key == "file" || key == "inline_data" || key == "inlineData" {
				continue
			}
			part := collectInflightText(child, key)
			if part != "" {
				b.WriteString(part)
				b.WriteByte('\n')
			}
		}
		return b.String()
	case []any:
		var b strings.Builder
		for _, child := range v {
			if part := collectInflightText(child, parent); part != "" {
				b.WriteString(part)
				b.WriteByte('\n')
			}
		}
		return b.String()
	case string:
		if parent == "url" || parent == "data" || strings.Contains(strings.ToLower(parent), "image") || strings.Contains(strings.ToLower(parent), "audio") || strings.Contains(strings.ToLower(parent), "video") {
			return ""
		}
		return v
	default:
		return ""
	}
}
