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
	sanitizedValue, changed := sanitizeUpstreamErrorValue(value)
	if !changed {
		return body
	}
	sanitized, err := json.Marshal(sanitizedValue)
	if err != nil {
		return body
	}
	return sanitized
}

func sanitizeUpstreamErrorValue(value any) (any, bool) {
	switch current := value.(type) {
	case string:
		sanitized := sanitizeUpstreamErrorMessage(current)
		return sanitized, sanitized != current
	case map[string]any:
		changed := false
		for key, item := range current {
			sanitized, itemChanged := sanitizeUpstreamErrorValue(item)
			current[key] = sanitized
			changed = changed || itemChanged
		}
		return current, changed
	case []any:
		changed := false
		for i, item := range current {
			sanitized, itemChanged := sanitizeUpstreamErrorValue(item)
			current[i] = sanitized
			changed = changed || itemChanged
		}
		return current, changed
	default:
		return value, false
	}
}
