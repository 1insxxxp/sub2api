package service

import (
	"encoding/json"
	"html"
	"net"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	userSupportPrivateReason          = "Request failed. Additional details were hidden to protect sensitive information."
	userSupportHiddenGroup            = "[REDACTED]"
	userSupportGeminiProhibitedReason = "Gemini rejected the prompt (PROHIBITED_CONTENT). Review the text and attached media, then try again."
)

// Only static reviewed explanations may leave this boundary. Never append
// original text, regex captures, parameter values or diagnostic suffixes.
var supportOriginalTemplates = []struct {
	code    *regexp.Regexp
	message string
}{
	{regexp.MustCompile(`(?i)\bPROHIBITED_CONTENT\b`), userSupportGeminiProhibitedReason},
	{regexp.MustCompile(`(?i)\b(?:SAFETY|BLOCKED_REASON_SAFETY|CONTENT_FILTERED|CONTENT_POLICY_VIOLATION)\b`), "The request was blocked by a content safety check. Review the text and attached media, then try again."},
	{regexp.MustCompile(`(?i)\b(?:RESOURCE_EXHAUSTED|QUOTA_EXCEEDED)\b`), "Request quota is temporarily unavailable. Please try again later or contact support."},
	{regexp.MustCompile(`(?i)\b(?:RATE_LIMIT_EXCEEDED|RATE_LIMIT_ERROR|TOO_MANY_REQUESTS)\b`), "Too many requests. Please try again later."},
	{regexp.MustCompile(`(?i)\b(?:UNAUTHENTICATED|INVALID_API_KEY|AUTHENTICATION_ERROR)\b`), "Request authentication failed. Please contact support."},
	{regexp.MustCompile(`(?i)\bPERMISSION_DENIED\b`), "Request permission was denied. Please contact support."},
	{regexp.MustCompile(`(?i)\b(?:INVALID_ARGUMENT|INVALID_REQUEST_ERROR|CONTEXT_LENGTH_EXCEEDED)\b`), "Invalid request parameters. Review the request and try again."},
	{regexp.MustCompile(`(?i)\b(?:DEADLINE_EXCEEDED|REQUEST_TIMEOUT|TIMEOUT_ERROR)\b`), "The request timed out. Please try again later."},
	{regexp.MustCompile(`(?i)\b(?:UNAVAILABLE|SERVICE_UNAVAILABLE|OVERLOADED_ERROR)\b`), "The service is temporarily unavailable. Please try again later."},
}

var (
	supportOriginalGeminiPromptRejection = regexp.MustCompile(`(?i)^Gemini rejected (?:the )?prompt:[ \t]+PROHIBITED_CONTENT(?:$|[.;, \t])`)
	supportPrivateIdentity               = regexp.MustCompile(`(?i)\b(?:project(?:[_ -]?(?:id|number|name))?|org(?:anization)?(?:[_ -]?(?:id|name))?|account(?:[_ -]?(?:id|name))?|tenant|workspace|subscription|consumer|channel|provider|upstream|proxy|request[_ -]?id|trace[_ -]?id|resource|key)\b(?:\s*[:=#/_-]\s*|\s+)`)
	supportPrivateResource               = regexp.MustCompile(`(?i)\b(?:projects|organizations|folders|subscriptions|tenants|workspaces|consumers|accounts|channels|proxies)/`)
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

// PublicUserSupportOriginalMessage projects recorded reasons to static public
// explanations. An unknown primary reason is never replaced by an earlier
// upstream attempt; fallback is only for an empty or unusable primary reason.
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
	message = strings.TrimSpace(html.UnescapeString(message))
	if message == "" {
		return ""
	}
	for _, pattern := range []*regexp.Regexp{supportOriginalHTML, supportOriginalStack} {
		if location := pattern.FindStringIndex(message); location != nil && location[0] == 0 {
			return ""
		}
	}
	// Repository and service both project; every permitted result must be stable.
	if message == userSupportPrivateReason {
		return message
	}
	for _, template := range supportOriginalTemplates {
		if message == template.message {
			return template.message
		}
	}
	// This recorded natural sentence owns its prompt label; it is not a prompt
	// payload. Return only its static explanation before private-tail removal.
	if supportOriginalGeminiPromptRejection.MatchString(message) {
		return userSupportGeminiProhibitedReason
	}
	end := len(message)
	for _, pattern := range []*regexp.Regexp{supportOriginalPrivateTail, supportOriginalHeader, supportOriginalStack, supportOriginalHTML, supportPrivateIdentity, supportPrivateResource} {
		if location := pattern.FindStringIndex(message); location != nil && location[0] < end {
			end = location[0]
		}
	}
	for _, marker := range []string{"{", "```", "-----BEGIN"} {
		if index := strings.Index(message, marker); index >= 0 && index < end {
			end = index
		}
	}
	for index := 0; index < end; index++ {
		if message[index] != '[' || (index > 0 && !strings.ContainsRune(" \t\r\n:=;", rune(message[index-1]))) {
			continue
		}
		rest := strings.TrimSpace(message[index+1:])
		if rest != "" && strings.ContainsRune(`"{[]`, rune(rest[0])) {
			end = index
			break
		}
	}
	message = strings.TrimSpace(message[:end])
	if message == "" {
		// A private-only primary still belongs to the final request; do not
		// replace it with an earlier upstream attempt after hiding its details.
		return userSupportPrivateReason
	}
	for _, template := range supportOriginalTemplates {
		if template.code.MatchString(message) {
			return template.message
		}
	}
	return userSupportPrivateReason
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
	if projected := supportOriginalPublicReason(reason); projected != "" && projected != userSupportPrivateReason {
		return projected
	}
	// Accept only exact reviewed symbolic codes from the same error object.
	// Never search request/debug trees or return arbitrary code/status strings.
	for _, key := range []string{"status", "code"} {
		if code, ok := object[key].(string); ok {
			code = strings.TrimSpace(code)
			for _, template := range supportOriginalTemplates {
				if location := template.code.FindStringIndex(code); location != nil && location[0] == 0 && location[1] == len(code) {
					return template.message
				}
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
