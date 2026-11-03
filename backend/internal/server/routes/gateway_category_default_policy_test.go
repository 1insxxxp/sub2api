package routes

import (
	"net/http"
	"testing"
)

func TestCategoryDefaultEndpointPolicy(t *testing.T) {
	for _, tt := range []struct {
		method, path string
		want         bool
	}{
		{http.MethodPost, "/v1/messages", true}, {http.MethodPost, "/v1/chat/completions", true},
		{http.MethodPost, "/relay-stream/v1/responses", true}, {http.MethodPost, "/backend-api/codex/responses", true},
		{http.MethodPost, "/v1beta/models/gemini-test:streamGenerateContent", true},
		{http.MethodPost, "/v1beta/models/gemini-test:countTokens", true},
		{http.MethodGet, "/v1/models/test", true}, {http.MethodGet, "/backend-api/codex/models", true},
		{http.MethodGet, "/v1/responses", false}, {http.MethodPost, "/v1/live", false},
		{http.MethodPost, "/v1/images/batches", false}, {http.MethodGet, "/v1beta/cachedContents", false},
		{http.MethodPost, "/antigravity/v1/messages", false}, {http.MethodPost, "/v1beta/models/test:predictLongRunning", false},
	} {
		if got := categoryDefaultEndpointSupported(tt.method, tt.path); got != tt.want {
			t.Errorf("%s %s = %v", tt.method, tt.path, got)
		}
	}
}

func TestCategoryDefaultRejectsAmbiguousModelCandidates(t *testing.T) {
	for _, tt := range []struct {
		values []string
		want   string
		ok     bool
	}{
		{[]string{" model-a ", "MODEL-A"}, "model-a", true}, {[]string{"model-a", "model-b"}, "", false},
		{[]string{"model-a", ""}, "", false}, {nil, "", false},
	} {
		got, ok := uniqueCategoryDefaultRequestModel(tt.values)
		if got != tt.want || ok != tt.ok {
			t.Errorf("%v = %q,%v", tt.values, got, ok)
		}
	}
}
