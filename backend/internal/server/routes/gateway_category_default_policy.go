package routes

import (
	"net/http"
	"strings"
)

// The first version has no persistent session/task-to-source association.
// An explicit allowlist prevents an unfamiliar endpoint from escaping the
// published model catalogue or being dispatched as an ordinary composite key.
func categoryDefaultEndpointSupported(method, path string) bool {
	path = strings.TrimRight(path, "/")
	if strings.HasPrefix(path, "/antigravity/") {
		return false
	}
	if method == http.MethodGet {
		for _, prefix := range []string{"/v1/models", "/models", "/v1beta/models", "/backend-api/codex/models", "/relay-stream/v1/models", "/relay-nonstream/v1/models"} {
			if path == prefix || strings.HasPrefix(path, prefix+"/") {
				return true
			}
		}
		return path == "/v1/sub2api/billing" || path == "/v1/usage"
	}
	if method != http.MethodPost {
		return false
	}
	if strings.HasPrefix(path, "/v1beta/models/") {
		switch path[strings.LastIndex(path, ":")+1:] {
		case "generateContent", "streamGenerateContent", "countTokens":
			return strings.Contains(path, ":")
		}
		return false
	}
	for _, prefix := range []string{"/relay-stream/v1", "/relay-nonstream/v1", "/backend-api/codex", "/v1", ""} {
		switch strings.TrimPrefix(path, prefix) {
		case "/messages", "/messages/count_tokens", "/chat/completions", "/responses", "/responses/compact", "/responses/input_tokens":
			return true
		}
	}
	return false
}

func uniqueCategoryDefaultRequestModel(candidates []string) (string, bool) {
	if len(candidates) == 0 {
		return "", false
	}
	model := strings.TrimSpace(candidates[0])
	if model == "" {
		return "", false
	}
	for _, other := range candidates[1:] {
		if !strings.EqualFold(model, strings.TrimSpace(other)) {
			return "", false
		}
	}
	return model, true
}
