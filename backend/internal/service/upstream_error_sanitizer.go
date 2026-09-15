package service

import (
	"encoding/json"
	"regexp"
)

var (
	upstreamURLRegex              = regexp.MustCompile(`(?i)\bhttps?://[^\s/?#"'<>]+`)
	upstreamBracketedAddressRegex = regexp.MustCompile(
		`\[(?:(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}|(?:\d{1,3}\.){3}\d{1,3})(?::[0-9]{1,5})?\]`,
	)
)

func sanitizeUpstreamErrorBody(body []byte) []byte {
	if len(body) == 0 {
		return body
	}

	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return []byte(sanitizeUpstreamErrorMessage(string(body)))
	}
	sanitized, err := json.Marshal(sanitizeUpstreamErrorValue(value))
	if err != nil {
		return body
	}
	return sanitized
}

func sanitizeUpstreamErrorValue(value any) any {
	switch current := value.(type) {
	case string:
		return sanitizeUpstreamErrorMessage(current)
	case map[string]any:
		for key, item := range current {
			current[key] = sanitizeUpstreamErrorValue(item)
		}
		return current
	case []any:
		for i, item := range current {
			current[i] = sanitizeUpstreamErrorValue(item)
		}
		return current
	default:
		return value
	}
}
