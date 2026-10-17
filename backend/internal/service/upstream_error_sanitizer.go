package service

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	// Remove the entire URL: paths and query strings can contain credentials or
	// customer identifiers even after the hostname has been hidden.
	upstreamURLRegex              = regexp.MustCompile(`(?i)\bhttps?://[^\s"'<>]+`)
	upstreamBracketedAddressRegex = regexp.MustCompile(
		`\[(?:(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}|(?:\d{1,3}\.){3}\d{1,3})(?::[0-9]{1,5})?\]`,
	)
	upstreamEmailRegex           = regexp.MustCompile(`(?i)[a-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?\.[a-z]{2,}`)
	upstreamAuthValueRegex       = regexp.MustCompile(`(?i)\b((?:bearer|basic)\s+)[a-z0-9._~+/=-]+`)
	upstreamAPIKeyRegex          = regexp.MustCompile(`\b(?:sk-[a-zA-Z0-9_-]{6,}|admin-[a-fA-F0-9]{64}|xai-[a-zA-Z0-9_-]{12,}|AIza[a-zA-Z0-9_-]{20,}|ya29\.[a-zA-Z0-9_-]{12,})\b`)
	upstreamJWTRegex             = regexp.MustCompile(`\beyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\b`)
	upstreamPrivateHeaderRegex   = regexp.MustCompile(`(?im)\b((?:cookie|set-cookie|authorization|proxy-authorization)[ \t]*:[ \t]*)[^\r\n]+`)
	upstreamNamedCredentialRegex = regexp.MustCompile(`(?i)\b(?:api[_-]?key|x-api-key|access[_-]?token|refresh[_-]?token|id[_-]?token|session[_-]?token|token|client[_-]?secret|authorization|proxy-authorization|password|passwd|secret|cookie|set-cookie)["']?\s*[:=]\s*`)
	upstreamPrivatePayloadRegex  = regexp.MustCompile(`(?i)\b(?:request[_-]?(?:body|payload|headers)|headers|prompt|messages|contents|input)["']?\s*[:=]\s*`)
)

func sanitizeUpstreamErrorBody(body []byte) []byte {
	if len(body) == 0 {
		return body
	}

	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		if sanitized, ok := sanitizeUpstreamErrorSSE(body); ok {
			return sanitized
		}
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
			if isPrivateUpstreamErrorField(key) {
				if item != "[REDACTED]" {
					current[key] = "[REDACTED]"
					changed = true
				}
				continue
			}
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

// Retain diagnostic envelopes (type/code/param/message and token counters), but
// never return upstream credentials or an echoed request payload to a client.
func isPrivateUpstreamErrorField(key string) bool {
	if isSensitiveKey(key) {
		return true
	}
	normalized := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(strings.TrimSpace(key)))
	switch normalized {
	case "key", "cookie", "setcookie", "headers", "request", "requestbody", "requestpayload",
		"requestheaders", "prompt", "messages", "contents", "input":
		return true
	default:
		return false
	}
}

// Providers sometimes stringify their request diagnostics inside message. Decode
// valid JSON fragments so arbitrary prompt and header values cannot bypass the
// field-based redaction used for ordinary error bodies.
func sanitizeEmbeddedUpstreamErrorJSON(message string) string {
	var out strings.Builder
	for message != "" {
		start := strings.IndexAny(message, "{[")
		if start < 0 {
			break
		}
		out.WriteString(message[:start])
		message = message[start:]
		sanitized, consumed := sanitizeUpstreamErrorJSONFragment(message)
		if consumed == 0 {
			out.WriteByte(message[0])
			message = message[1:]
			continue
		}
		out.WriteString(sanitized)
		message = message[consumed:]
	}
	out.WriteString(message)
	return out.String()
}

func sanitizeUpstreamErrorJSONFragment(message string) (string, int) {
	decoder := json.NewDecoder(strings.NewReader(message))
	var value any
	if decoder.Decode(&value) != nil {
		return "", 0
	}
	consumed := int(decoder.InputOffset())
	sanitized, changed := sanitizeUpstreamErrorValue(value)
	if !changed {
		return message[:consumed], consumed
	}
	encoded, err := json.Marshal(sanitized)
	if err != nil {
		return "[REDACTED]", consumed
	}
	return string(encoded), consumed
}

// A truncated request echo cannot be decoded safely. Only recognize an
// explicit private payload label and JSON-like value; parameter diagnostics
// such as input[8].tools or input: expected a string must remain readable.
func sanitizeUpstreamErrorPayloadText(message string) string {
	var out strings.Builder
	for message != "" {
		label := upstreamPrivatePayloadRegex.FindStringIndex(message)
		if label == nil {
			break
		}
		out.WriteString(message[:label[1]])
		message = message[label[1]:]
		if message == "" || !strings.ContainsRune(`{["'`, rune(message[0])) {
			continue
		}
		consumed := upstreamPayloadValueLength(message)
		out.WriteString(`"[REDACTED]"`)
		message = message[consumed:]
	}
	out.WriteString(message)
	return out.String()
}

func upstreamPayloadValueLength(value string) int {
	if value[0] == '\'' {
		return upstreamQuotedValueLength(value)
	}
	decoder := json.NewDecoder(strings.NewReader(value))
	var payload any
	if decoder.Decode(&payload) != nil {
		return len(value)
	}
	return int(decoder.InputOffset())
}

// Scan quoted values instead of matching their first quote: escaped quotes and
// truncated strings must never leave a credential suffix in an error message.
func sanitizeNamedUpstreamCredentials(message string) string {
	var out strings.Builder
	for message != "" {
		label := upstreamNamedCredentialRegex.FindStringIndex(message)
		if label == nil {
			break
		}
		out.WriteString(message[:label[1]])
		message = message[label[1]:]
		consumed := upstreamCredentialValueLength(message)
		if consumed == 0 {
			continue
		}
		out.WriteString(redactUpstreamCredentialValue(message[:consumed]))
		message = message[consumed:]
	}
	out.WriteString(message)
	return out.String()
}

func upstreamCredentialValueLength(value string) int {
	if value == "" {
		return 0
	}
	if value[0] == '"' || value[0] == '\'' {
		return upstreamQuotedValueLength(value)
	}
	if strings.HasPrefix(value, "[REDACTED]") {
		return len("[REDACTED]")
	}
	if auth := upstreamAuthValueRegex.FindStringIndex(value); auth != nil && auth[0] == 0 {
		return auth[1]
	}
	if end := strings.IndexAny(value, " \t\r\n,;}]"); end >= 0 {
		return end
	}
	return len(value)
}

func upstreamQuotedValueLength(value string) int {
	escaped := false
	for i := 1; i < len(value); i++ {
		if escaped {
			escaped = false
			continue
		}
		if value[i] == '\\' {
			escaped = true
			continue
		}
		if value[i] == value[0] {
			return i + 1
		}
	}
	return len(value)
}

func redactUpstreamCredentialValue(value string) string {
	if value[0] == '"' || value[0] == '\'' {
		quote := value[:1]
		return quote + "[REDACTED]" + quote
	}
	return "[REDACTED]"
}

// Historical error bodies may contain SSE terminal frames rather than one JSON
// object. Redact their data payloads with the same rules, including multiline
// data, before returning them through the user error-detail endpoint.
func sanitizeUpstreamErrorSSE(body []byte) ([]byte, bool) {
	raw := strings.ReplaceAll(strings.ReplaceAll(string(body), "\r\n", "\n"), "\r", "\n")
	frames := strings.Split(raw, "\n\n")
	found := false
	for i, frame := range frames {
		sanitized, hasData := sanitizeUpstreamErrorSSEFrame(frame)
		frames[i] = sanitized
		found = found || hasData
	}
	sanitized := strings.Join(frames, "\n\n")
	if !found || sanitized == raw {
		return body, found
	}
	return []byte(sanitized), true
}

func sanitizeUpstreamErrorSSEFrame(frame string) (string, bool) {
	lines := strings.Split(frame, "\n")
	var data []string
	for _, line := range lines {
		if payload, ok := strings.CutPrefix(line, "data:"); ok {
			data = append(data, strings.TrimPrefix(payload, " "))
		}
	}
	if len(data) == 0 {
		return sanitizeUpstreamErrorMessage(frame), false
	}
	payload := strings.Join(data, "\n")
	sanitized := sanitizeUpstreamErrorMessage(payload)
	if json.Valid([]byte(payload)) {
		// Only recurse for JSON; malformed nested data: text is not an SSE body.
		sanitized = string(sanitizeUpstreamErrorBody([]byte(payload)))
	}
	return sanitizeUpstreamErrorSSELines(lines, sanitized), true
}

func sanitizeUpstreamErrorSSELines(lines []string, sanitized string) string {
	out := make([]string, 0, len(lines))
	wroteData := false
	for _, line := range lines {
		if !strings.HasPrefix(line, "data:") {
			out = append(out, sanitizeUpstreamErrorMessage(line))
			continue
		}
		if wroteData {
			continue
		}
		for _, payload := range strings.Split(sanitized, "\n") {
			out = append(out, "data: "+payload)
		}
		wroteData = true
	}
	return strings.Join(out, "\n")
}
