package service

import (
	"encoding/json"
	"html"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	userSupportPrivateReason = "Request failed. Additional details were hidden to protect sensitive information."
	userSupportHiddenGroup   = "[REDACTED]"
)

// Metadata remains outside this boundary; only these symbolic codes can be
// used when an error object has no recorded message.
var supportOriginalCode = regexp.MustCompile(`^(?:PROHIBITED_CONTENT|SAFETY|BLOCKED_REASON_SAFETY|CONTENT_FILTERED|CONTENT_POLICY_VIOLATION|RESOURCE_EXHAUSTED|QUOTA_EXCEEDED|RATE_LIMIT_EXCEEDED|RATE_LIMIT_ERROR|TOO_MANY_REQUESTS|UNAUTHENTICATED|INVALID_API_KEY|AUTHENTICATION_ERROR|PERMISSION_DENIED|INVALID_ARGUMENT|INVALID_REQUEST_ERROR|CONTEXT_LENGTH_EXCEEDED|DEADLINE_EXCEEDED|REQUEST_TIMEOUT|TIMEOUT_ERROR|UNAVAILABLE|SERVICE_UNAVAILABLE|OVERLOADED_ERROR|INTERNAL|UNKNOWN)$`)

var (
	supportPrivateIdentity = regexp.MustCompile(`(?i)\b(?:project(?:[_ -]?(?:id|number|name))?|org(?:anization)?(?:[_ -]?(?:id|name))?|account(?:[_ -]?(?:id|name))?|tenant|workspace|subscription|consumer|channel|provider|upstream|proxy|request[_ -]?id|trace[_ -]?id|resource|key)\b(?:\s*[:=#/_-]\s*|\s+)`)
	supportPrivateResource = regexp.MustCompile(`(?i)\b(?:projects|organizations|folders|subscriptions|tenants|workspaces|consumers|accounts|channels|proxies)/`)
)

var (
	supportOriginalPrivateTail = regexp.MustCompile(`(?i)\b(?:(?:request[_ -]?(?:body|payload|headers)|(?:response|upstream)[_ -]?(?:body|headers)|headers|debug(?:[_ -]?(?:data|payload|info))?|stack[_ -]?trace|traceback|stack|credential|signature|private[_ -]?key|endpoint|upstream[_ -]?(?:url|endpoint)|host(?:name)?|proxy[_ -]?url)\s*[:=]|(?:prompt|messages|contents|input)\s*[:=])`)
	supportOriginalHeader      = regexp.MustCompile(`(?i)\b(?:authorization|proxy-authorization|cookie|set-cookie|x-api-key|x-goog-api-key)\s*:`)
	supportOriginalStack       = regexp.MustCompile(`(?im)(?:^|\n)[ \t]*(?:traceback\b|goroutine\s+[0-9]+\b|stack(?:trace| trace)?\s*:|at\s+\S+\(|file\s+["'])`)
	supportOriginalHTML        = regexp.MustCompile(`(?i)<(?:!doctype|/?[a-z][a-z0-9]*\b)`)
	supportOriginalExtraKey    = regexp.MustCompile(`\b(?:AQ\.Ab[a-zA-Z0-9_-]{20,}|GOCSPX-[a-zA-Z0-9_-]{16,})\b`)
	supportOriginalAccount     = regexp.MustCompile(`(?i)\b(?:(?:upstream|provider)[_ -]?)?account(?:[_ -]?(?:id|name))?\s*[:=#]\s*(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^\s,;)\]]+)`)
	supportOriginalAccountID   = regexp.MustCompile(`(?i)\baccount\s+(?:id\s+)?#?[0-9]+\b`)
	supportOriginalIPv4        = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}(?::[0-9]{1,5})?\b`)
	supportOriginalIPv6        = regexp.MustCompile(`(?i)\[[0-9a-f:.%]+\](?::[0-9]+)?|[0-9a-f]*:[0-9a-f:.%]+`)
	supportOriginalDomain      = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,63}(?::[0-9]{1,5})?(?:/[^\s"'<>]*)?`)
)

var (
	supportOriginalUUID          = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	supportOriginalSocket        = regexp.MustCompile(`(?i)\b[a-z][a-z0-9_-]*:[0-9]{1,5}\b`)
	supportOriginalLocalHost     = regexp.MustCompile(`(?i)\blocalhost(?::[0-9]{1,5})?\b`)
	supportOriginalURL           = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s"'<>]+`)
	supportOriginalEscape        = regexp.MustCompile(`%[0-9a-fA-F]{2}`)
	supportOriginalStars         = regexp.MustCompile(`\*{2,}`)
	supportOriginalPEM           = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]+-----.*?(?:-----END [A-Z0-9 ]+-----|$)`)
	supportOriginalIdentityValue = regexp.MustCompile(`(?i)\b(?:(?:project(?:[_ -]?(?:id|number|name))?|org(?:anization)?(?:[_ -]?(?:id|name))?|account(?:[_ -]?(?:id|name))?|(?:tenant|workspace|subscription|consumer|channel)(?:[_ -]?(?:id|name))?|request[_ -]?id|trace[_ -]?id|resource|key)\b["']?(?:\s*[:=#/_-]\s*|\s+)|(?:provider|upstream|proxy)(?:[_ -]?(?:account|channel))?(?:[_ -]?(?:id|name))?["']?\s*[:=#]\s*)`)
	supportOriginalIdentityName  = regexp.MustCompile(`(?i)\b(?:(?:project|org(?:anization)?|account|tenant|workspace|subscription|consumer|channel)[_ -]?name|(?:provider|upstream|proxy)(?:[_ -]?(?:account|channel))?[_ -]?name)["']?\s*[:=#]\s*`)
	supportOriginalMessageDomain = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9_-]*[a-z0-9_])?\.)+[a-z][a-z0-9_-]{1,62}(?::[0-9]{1,5})?(?:/[^\s"'<>]*)?`)
	supportOriginalNaturalPrompt = regexp.MustCompile(`(?i)^Gemini rejected (?:the )?prompt:[ \t]+PROHIBITED_CONTENT(?:$|[.;, \t])`)
	supportOriginalPayload       = regexp.MustCompile(`(?i)\b(?:request[_ -]?(?:body|payload|headers)|(?:response|upstream)[_ -]?(?:body|headers)|headers|debug(?:[_ -]?(?:data|payload|info))?|prompt|messages|contents|input)["']?\s*[:=]\s*`)
	supportOriginalPrivateValue  = regexp.MustCompile(`(?i)\b(?:api[_-]?key|x-api-key|x-goog-api-key|access[_-]?token|refresh[_-]?token|id[_-]?token|session[_-]?token|token|client[_-]?(?:secret|id)|password|passwd|secret|credential|signature|private[_ -]?key|endpoint|upstream[_ -]?(?:url|endpoint)|host(?:name)?|proxy[_ -]?url)["']?\s*[:=]\s*`)
	supportOriginalOpaque        = regexp.MustCompile(`\b[a-zA-Z0-9_+/=-]{24,}\b`)
	supportOriginalResourceValue = regexp.MustCompile(`(?i)\b(?:projects|organizations|folders|subscriptions|tenants|workspaces|consumers|accounts|channels|proxies)/[^\s,;"'<>]+`)
	supportOriginalLocalPath     = regexp.MustCompile(`(?:^|[\s("'])(?:/(?:opt|home|root|usr|var|etc|tmp|Users|srv|app|mnt|Volumes|private)/|[A-Za-z]:\\)[^\s"'<>]+`)
	supportOriginalPlaceholders  = strings.NewReplacer("[REDACTED]", "*", "[upstream-url]", "*", "[upstream]", "*", "[email]", "*", "***", "*")
)

// PublicUserSupportOriginalMessage returns the recorded reason with private
// fragments masked. An earlier attempt must never replace a final reason.
func PublicUserSupportOriginalMessage(primary, upstream string) string {
	for _, raw := range []string{primary, upstream} {
		if message := supportOriginalReason(raw, 0); message != "" {
			if message = supportOriginalPublicReason(message); message != "" {
				return message
			}
		}
	}
	return ""
}

func supportOriginalPublicReason(message string) string {
	// Decode common escaping before masking, so encoding cannot hide a secret.
	for range 3 {
		decoded := html.UnescapeString(message)
		decoded = supportOriginalEscape.ReplaceAllStringFunc(decoded, func(value string) string { unescaped, _ := url.PathUnescape(value); return unescaped })
		if decoded == message {
			break
		}
		message = decoded
	}
	// Deeply nested escaping is unusable as a trustworthy reason. Avoid
	// leaving a value which would reveal more text on the next projection.
	if supportOriginalEscape.MatchString(message) || html.UnescapeString(message) != message {
		return "*"
	}
	message = strings.TrimSpace(strings.Map(func(r rune) rune {
		if (unicode.IsControl(r) && r != '\n' && r != '\t') || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, message))
	if message == "" {
		return ""
	}
	if message == userSupportPrivateReason {
		return "Request failed."
	}
	// Stack traces and HTML are diagnostic payloads, not a recorded explanation.
	for _, pattern := range []*regexp.Regexp{supportOriginalHTML, supportOriginalStack} {
		if location := pattern.FindStringIndex(message); location != nil {
			if location[0] == 0 {
				return ""
			}
			message = strings.TrimSpace(message[:location[0]]) + " *"
		}
	}
	message = supportOriginalPEM.ReplaceAllString(message, "*")
	if start := strings.Index(message, "-----BEGIN"); start >= 0 {
		message = message[:start] + "*"
	}
	message = supportMaskPayloads(message)
	// A header may contain spaces and multiple credentials; mask the whole line.
	message = regexpSupportHeaders.ReplaceAllString(message, "*")
	message = supportMaskValues(message, upstreamNamedCredentialRegex)
	message = supportMaskValues(message, supportOriginalPrivateValue)
	message = supportMaskIdentityValues(message)
	message = supportMaskValues(message, supportOriginalIdentityValue)
	message = supportOriginalResourceValue.ReplaceAllString(message, "*")
	message = supportOriginalURL.ReplaceAllString(message, "*")
	message = upstreamEmailRegex.ReplaceAllString(message, "*")
	message = upstreamAuthValueRegex.ReplaceAllString(message, "*")
	for _, pattern := range []*regexp.Regexp{upstreamAPIKeyRegex, upstreamJWTRegex, supportOriginalExtraKey, supportOriginalAccountID, supportOriginalUUID, supportOriginalLocalHost} {
		message = pattern.ReplaceAllString(message, "*")
	}
	message = supportOriginalIPv6.ReplaceAllStringFunc(message, func(value string) string {
		address := value
		if host, _, err := net.SplitHostPort(value); err == nil {
			address = host
		}
		address = strings.Trim(address, "[]")
		if index := strings.LastIndexByte(address, '%'); index >= 0 {
			address = address[:index]
		}
		if net.ParseIP(address) != nil {
			return "*"
		}
		return value
	})
	message = supportOriginalIPv4.ReplaceAllString(message, "*")
	message = supportOriginalSocket.ReplaceAllString(message, "*")
	message = supportOriginalMessageDomain.ReplaceAllStringFunc(message, func(value string) string {
		// These are request schema paths, not network addresses.
		switch value {
		case "contents.parts", "thinking.budget_tokens", "generationConfig.maxOutputTokens", "generationConfig.thinkingConfig", "input.tools":
			return value
		}
		return "*"
	})
	message = supportOriginalLocalPath.ReplaceAllStringFunc(message, func(value string) string {
		if strings.HasPrefix(value, "/") || (len(value) > 1 && value[1] == ':') {
			return "*"
		}
		return value[:1] + "*"
	})
	message = supportOriginalOpaque.ReplaceAllStringFunc(message, func(value string) string {
		hasLower, hasUpper, hasDigit := false, false, false
		for _, r := range value {
			hasLower = hasLower || (r >= 'a' && r <= 'z')
			hasUpper = hasUpper || (r >= 'A' && r <= 'Z')
			hasDigit = hasDigit || (r >= '0' && r <= '9')
		}
		if hasLower && (hasUpper || hasDigit) {
			return "*"
		}
		return value
	})
	message = supportOriginalStars.ReplaceAllString(supportOriginalPlaceholders.Replace(message), "*")
	runes := []rune(message)
	if len(runes) > 2000 {
		message = string(runes[:1999]) + "…"
	}
	return strings.TrimSpace(message)
}

var regexpSupportHeaders = regexp.MustCompile(`(?im)\b(?:authorization|proxy-authorization|cookie|set-cookie|x-api-key|x-goog-api-key)\s*:[^\r\n]*`)

// Preserve surrounding prose, while dropping entire echoed private values.
func supportMaskPayloads(message string) string {
	var out strings.Builder
	if location := supportOriginalNaturalPrompt.FindStringIndex(message); location != nil {
		_, _ = out.WriteString(message[:location[1]])
		message = message[location[1]:]
	}
	for message != "" {
		location := supportOriginalPayload.FindStringIndex(message)
		if location == nil {
			break
		}
		_, _ = out.WriteString(message[:location[0]])
		label := message[location[0]:location[1]]
		message = message[location[1]:]
		// A type validation diagnostic is not an echoed input value.
		if strings.EqualFold(strings.TrimSpace(label), "input:") && strings.HasPrefix(message, "expected a string") {
			_, _ = out.WriteString(label)
			continue
		}
		consumed := supportPrivateValueLength(message, true)
		_, _ = out.WriteString("*")
		message = message[consumed:]
	}
	_, _ = out.WriteString(message)
	// Inline objects/arrays may be unlabelled request or debug dumps. Never
	// recursively search those payloads for an error reason.
	message = out.String()
	out.Reset()
	for message != "" {
		start := strings.IndexAny(message, "{[")
		if start < 0 {
			break
		}
		_, _ = out.WriteString(message[:start])
		message = message[start:]
		if strings.HasPrefix(message, "[REDACTED]") || strings.HasPrefix(message, "[upstream") || strings.HasPrefix(message, "[email]") {
			end := strings.IndexByte(message, ']') + 1
			if end == 0 {
				end = len(message)
			}
			_, _ = out.WriteString("*")
			message = message[end:]
			continue
		}
		// Bracketed addresses are masked by the address rules below; field indexes
		// such as input[8] are legitimate parameter diagnostics.
		rest := strings.TrimSpace(message[1:])
		var value any
		decoder := json.NewDecoder(strings.NewReader(message))
		isJSON := decoder.Decode(&value) == nil
		// A single numerical index following a parameter path is not a payload.
		prefix := out.String()
		index := message[0] == '[' && isJSON && decoder.InputOffset() > 2 && !strings.ContainsAny(message[1:decoder.InputOffset()-1], ",\"{[ \t\r\n") && len(prefix) > 0 && ((prefix[len(prefix)-1] >= 'a' && prefix[len(prefix)-1] <= 'z') || prefix[len(prefix)-1] == ']')
		if message[0] == '[' && (index || (!isJSON && (rest == "" || !strings.ContainsRune(`{["'`, rune(rest[0]))))) {
			_ = out.WriteByte(message[0])
			message = message[1:]
			continue
		}
		consumed := supportPrivateValueLength(message, true)
		_, _ = out.WriteString("*")
		message = message[consumed:]
	}
	_, _ = out.WriteString(message)
	return out.String()
}

func supportMaskValues(message string, pattern *regexp.Regexp) string {
	return supportMaskValueSpans(message, pattern, func(value string) int {
		return supportPrivateValueLength(value, false)
	})
}

func supportMaskIdentityValues(message string) string {
	return supportMaskValueSpans(message, supportOriginalIdentityName, func(value string) int {
		if value == "" || strings.ContainsRune(`{["'`, rune(value[0])) {
			return supportPrivateValueLength(value, false)
		}
		// Explicit identity names can contain spaces and commas. Only an explicit
		// separator ends an unquoted value; quoted values use their own boundary.
		if end := strings.IndexAny(value, ";\r\n"); end >= 0 {
			return end
		}
		return len(value)
	})
}

func supportMaskValueSpans(message string, pattern *regexp.Regexp, valueLength func(string) int) string {
	var out strings.Builder
	for message != "" {
		location := pattern.FindStringIndex(message)
		if location == nil {
			break
		}
		_, _ = out.WriteString(message[:location[1]])
		message = message[location[1]:]
		consumed := valueLength(message)
		if consumed == 0 {
			continue
		}
		_, _ = out.WriteString("*")
		message = message[consumed:]
	}
	_, _ = out.WriteString(message)
	return out.String()
}

func supportPrivateValueLength(value string, payload bool) int {
	if value == "" {
		return 0
	}
	if auth := upstreamAuthValueRegex.FindStringIndex(value); !payload && auth != nil && auth[0] == 0 {
		return auth[1]
	}
	if strings.ContainsRune(`{["'`, rune(value[0])) {
		if value[0] == '\'' {
			return upstreamQuotedValueLength(value)
		}
		decoder := json.NewDecoder(strings.NewReader(value))
		var decoded any
		if decoder.Decode(&decoded) == nil {
			return int(decoder.InputOffset())
		}
		return len(value) // Incomplete payloads/quoted secrets fail closed.
	}
	// Unstructured request/debug echoes have no trustworthy delimiter: a
	// semicolon or newline can be customer content. Drop their full remainder.
	if payload {
		return len(value)
	}
	delimiters := " \t\r\n,;)}]"
	if end := strings.IndexAny(value, delimiters); end >= 0 {
		return end
	}
	return len(value)
}

// Decode only reviewed reason locations. In particular, never search arbitrary
// request/metadata trees and never return the serialized object on failure.
func supportOriginalReason(raw string, depth int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 64<<10 || depth > 4 {
		return ""
	}
	if raw[0] != '{' && raw[0] != '[' && raw[0] != '"' {
		return raw
	}
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return supportOriginalReason(text, depth+1)
	}
	object, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	if errObject, ok := object["error"].(map[string]any); ok {
		if reason := supportOriginalObjectReason(errObject, depth); reason != "" {
			return reason
		}
	}
	if response, ok := object["response"].(map[string]any); ok {
		if errObject, ok := response["error"].(map[string]any); ok {
			if reason := supportOriginalObjectReason(errObject, depth); reason != "" {
				return reason
			}
		}
	}
	return supportOriginalObjectReason(object, depth)
}

func supportOriginalObjectReason(object map[string]any, depth int) string {
	var reason string
	for _, key := range []string{"message", "error"} {
		if text, ok := object[key].(string); ok {
			if reason = supportOriginalReason(text, depth+1); reason != "" {
				break
			}
		}
	}
	if reason != "" {
		return reason
	}
	for _, key := range []string{"status", "code"} {
		if code, ok := object[key].(string); ok {
			code = strings.TrimSpace(code)
			if supportOriginalCode.MatchString(code) {
				return code
			}
		}
	}

	return reason
}

// Group names are deliberate public business labels. Preserve normal labels,
// but hide the whole value when explicit secrets, addresses or identities occur.
func PublicUserSupportGroupName(value string) string {
	if value == userSupportHiddenGroup {
		return value
	}
	if !utf8.ValidString(value) || len(value) > 512 || strings.IndexFunc(value, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }) >= 0 {
		return userSupportHiddenGroup
	}
	decoded := html.UnescapeString(value)
	if strings.Contains(decoded, "://") || strings.Contains(decoded, "-----BEGIN") {
		return userSupportHiddenGroup
	}
	for _, pattern := range []*regexp.Regexp{supportOriginalExtraKey, supportOriginalIPv4, supportOriginalDomain, supportOriginalAccount, supportOriginalAccountID, supportPrivateIdentity, supportPrivateResource, supportOriginalPrivateTail, supportOriginalHeader, upstreamEmailRegex, upstreamAPIKeyRegex, upstreamJWTRegex, upstreamAuthValueRegex, upstreamNamedCredentialRegex} {
		if pattern.MatchString(decoded) {
			return userSupportHiddenGroup
		}
	}
	for _, value := range supportOriginalIPv6.FindAllString(decoded, -1) {
		address := value
		if host, _, err := net.SplitHostPort(value); err == nil {
			address = host
		}
		address = strings.Trim(address, "[]")
		if index := strings.LastIndexByte(address, '%'); index >= 0 {
			address = address[:index]
		}
		if net.ParseIP(address) != nil {
			return userSupportHiddenGroup
		}
	}
	return value
}
