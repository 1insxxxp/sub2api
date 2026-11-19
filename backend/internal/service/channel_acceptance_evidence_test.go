package service

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestAcceptanceEvidenceCumulativeUsageAndPresence(t *testing.T) {
	p := newAcceptanceEvidenceParser()
	raw := "data: {\"usageMetadata\":{\"promptTokenCount\":100,\"candidatesTokenCount\":2,\"thoughtsTokenCount\":3,\"cachedContentTokenCount\":0}}\r\n\r\n" +
		"data: {\"candidates\":[{\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":100,\"candidatesTokenCount\":5,\"thoughtsTokenCount\":3}}\n\n"
	for _, b := range []byte(raw) {
		p.Feed([]byte{b})
	}
	p.Finish()
	if p.usage.Input == nil || *p.usage.Input != 100 || p.usage.Output == nil || *p.usage.Output != 5 || *p.usage.Thoughts != 3 || p.usage.CacheRead == nil || *p.usage.CacheRead != 0 || p.usage.Total != nil || !p.completed {
		t.Fatalf("bad evidence: %+v", p)
	}
}
func TestAcceptanceEvidenceTailAndReasoning(t *testing.T) {
	p := newAcceptanceEvidenceParser()
	p.Feed([]byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"thinking\"},\"finish_reason\":null}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"answer\"},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":8,\"completion_tokens_details\":{\"reasoning_tokens\":6}}}\n\ndata: [DONE]\n\n"))
	p.Finish()
	if !p.hasText || !p.hasReasoning || p.excerpt != "answer" || *p.usage.Output != 2 || *p.usage.Thoughts != 6 {
		t.Fatalf("bad evidence: %+v", p)
	}
}
func TestAcceptanceEvidenceOnlyKnownNumericFieldsPersist(t *testing.T) {
	p := newAcceptanceEvidenceParser()
	p.Feed([]byte(`{"usageMetadata":{"promptTokenCount":-1,"candidatesTokenCount":0,"secret":"sk-private"},"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	p.Finish()
	if p.usage.Input != nil || p.usage.Output == nil || *p.usage.Output != 0 || strings.Contains(string(p.frames[0]), "secret") {
		t.Fatalf("unsafe evidence: %+v", p)
	}
}
func TestAcceptanceSourceSanitizesCredentialsAndDoesNotTrustSuffix(t *testing.T) {
	u, _ := url.Parse("https://secret:password@generativelanguage.googleapis.com.evil.test/secret-token/v1beta/models/gemini-test:generateContent?key=private")
	r := &http.Request{URL: u, Header: http.Header{"X-Goog-Api-Key": []string{"private"}}}
	s := acceptanceSourceFromRequest(r, "gemini", "gemini-test", "gemini-test")
	if s.Kind != "unverified" || strings.Contains(s.Path, "secret-token") || strings.Contains(s.Host, "password") || s.AuthKind != "api_key" {
		t.Fatalf("unsafe source %+v", s)
	}
}
func TestAcceptanceEvidenceBoundedLargeFrame(t *testing.T) {
	p := newAcceptanceEvidenceParser()
	p.Feed([]byte("data: " + strings.Repeat("x", acceptanceFrameLimit+1) + "\n\ndata: {\"usage\":{\"completion_tokens\":7}}\n\n"))
	p.Finish()
	if !p.truncated || p.usage.Output == nil || *p.usage.Output != 7 {
		t.Fatalf("failed to recover after large event: %+v", p)
	}
}

func TestAcceptanceEvidenceChatUnknownThoughtBreakdown(t *testing.T) {
	p := newAcceptanceEvidenceParser()
	p.Feed([]byte(`{"usage":{"prompt_tokens":10,"completion_tokens":20}}`))
	p.Finish()
	if p.usage.Thoughts != nil || p.usage.Output == nil || *p.usage.Output != 20 || !p.usage.OutputIncludesThoughts || p.usage.Source != "upstream_chat_reported" {
		t.Fatalf("unknown thoughts falsely normalized: %+v", p.usage)
	}
}
func TestAcceptanceEvidenceToolsFragments(t *testing.T) {
	p := newAcceptanceEvidenceParser()
	p.Feed([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"name\":\"acceptance_echo\",\"arguments\":\"{\\\"value\\\":\"}}]}}]}\n\ndata: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"\\\"nonce\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n"))
	p.Finish()
	if !p.toolMatches("nonce") || p.toolMatches("old-nonce") || p.hasText {
		t.Fatal("tool arguments should be checked separately from visible text")
	}
}

func TestAcceptanceEvidenceFinalUsageRequiresTerminalEvidence(t *testing.T) {
	geminiUsage := `"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":5,"thoughtsTokenCount":0,"totalTokenCount":105}`
	chatUsage := `"usage":{"prompt_tokens":100,"completion_tokens":5,"total_tokens":105}`
	for _, tc := range []struct {
		name, body      string
		final, complete bool
	}{
		{"Gemini partial consistent stats", "data: {" + geminiUsage + "}\n\n", false, false},
		{"Gemini terminal without new usage", "data: {" + geminiUsage + "}\n\ndata: {\"candidates\":[{\"finishReason\":\"STOP\"}]}\n\n", false, true},
		{"Gemini terminal prompt-only usage", "data: {" + geminiUsage + "}\n\ndata: {\"candidates\":[{\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":100}}\n\n", false, true},
		{"Gemini terminal with usage", "data: {\"candidates\":[{\"finishReason\":\"STOP\"}]," + geminiUsage + "}\n\n", true, true},
		{"Gemini independent tail", "data: {\"candidates\":[{\"finishReason\":\"STOP\"}]}\n\ndata: {" + geminiUsage + "}\n\n", true, true},
		{"Chat partial then DONE", "data: {" + chatUsage + "}\n\ndata: [DONE]\n\n", false, false},
		{"Chat terminal without new usage", "data: {" + chatUsage + "}\n\ndata: {\"choices\":[{\"finish_reason\":\"stop\"}]}\n\n", false, true},
		{"Chat terminal with usage", "data: {\"choices\":[{\"finish_reason\":\"stop\"}]," + chatUsage + "}\n\n", true, true},
		{"Chat independent tail", "data: {\"choices\":[{\"finish_reason\":\"stop\"}]}\n\ndata: {" + chatUsage + "}\n\n", true, true},
		{"Responses completed nested usage", `{"type":"response.completed","response":{"usage":{"input_tokens":100,"output_tokens":5}}}`, true, true},
		{"Responses interrupted nested usage", `{"type":"response.incomplete","response":{"usage":{"input_tokens":100,"output_tokens":5}}}`, false, false},
		{"Nonstream final usage", `{"candidates":[{"finishReason":"STOP"}],` + geminiUsage + `}`, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newAcceptanceEvidenceParser()
			p.Feed([]byte(tc.body))
			p.Finish()
			snapshot := p.snapshot()
			if snapshot.usageFinal != tc.final || snapshot.completed != tc.complete {
				t.Fatalf("final=%v complete=%v, want %v/%v", snapshot.usageFinal, snapshot.completed, tc.final, tc.complete)
			}
			if snapshot.usage.Input == nil || *snapshot.usage.Input != 100 || snapshot.usage.Output == nil || *snapshot.usage.Output != 5 {
				t.Fatalf("partial evidence must remain visible: %+v", snapshot.usage)
			}
		})
	}
}

func TestAcceptanceEvidenceTerminalUsageCannotPromotePartialComponents(t *testing.T) {
	for _, tc := range []struct {
		name, partial, terminal string
		final                   bool
	}{
		{"Gemini prompt only", `{"promptTokenCount":100,"candidatesTokenCount":1,"thoughtsTokenCount":0,"totalTokenCount":101}`, `{"promptTokenCount":100}`, false},
		{"Gemini cache only", `{"promptTokenCount":100,"candidatesTokenCount":1,"cachedContentTokenCount":10}`, `{"cachedContentTokenCount":10}`, false},
		{"Gemini missing cache", `{"promptTokenCount":100,"candidatesTokenCount":1,"cachedContentTokenCount":10}`, `{"promptTokenCount":100,"candidatesTokenCount":5}`, false},
		{"Gemini missing thoughts", `{"promptTokenCount":100,"candidatesTokenCount":1,"thoughtsTokenCount":0}`, `{"promptTokenCount":100,"candidatesTokenCount":5}`, false},
		{"Gemini missing total", `{"promptTokenCount":100,"candidatesTokenCount":1,"totalTokenCount":101}`, `{"promptTokenCount":100,"candidatesTokenCount":5}`, false},
		{"Gemini full final", `{"promptTokenCount":100,"candidatesTokenCount":1,"thoughtsTokenCount":0,"cachedContentTokenCount":10,"totalTokenCount":101}`, `{"promptTokenCount":100,"candidatesTokenCount":5,"thoughtsTokenCount":0,"cachedContentTokenCount":10,"totalTokenCount":105}`, true},
		{"Chat missing cache", `{"prompt_tokens":100,"completion_tokens":1,"prompt_tokens_details":{"cached_tokens":10}}`, `{"prompt_tokens":100,"completion_tokens":5}`, false},
		{"Chat missing thoughts", `{"prompt_tokens":100,"completion_tokens":1,"completion_tokens_details":{"reasoning_tokens":0}}`, `{"prompt_tokens":100,"completion_tokens":5}`, false},
		{"Chat full final", `{"prompt_tokens":100,"completion_tokens":1,"completion_tokens_details":{"reasoning_tokens":0},"prompt_tokens_details":{"cached_tokens":10},"total_tokens":101}`, `{"prompt_tokens":100,"completion_tokens":5,"completion_tokens_details":{"reasoning_tokens":0},"prompt_tokens_details":{"cached_tokens":10},"total_tokens":105}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			field, finish := "usageMetadata", `"candidates":[{"finishReason":"STOP"}]`
			if strings.HasPrefix(tc.name, "Chat") {
				field, finish = "usage", `"choices":[{"finish_reason":"stop"}]`
			}
			p := newAcceptanceEvidenceParser()
			p.Feed([]byte("data: {\"" + field + "\":" + tc.partial + "}\n\ndata: {" + finish + ",\"" + field + "\":" + tc.terminal + "}\n\n"))
			p.Finish()
			if p.usageFinal != tc.final || !p.completed || p.usage.Input == nil || *p.usage.Input != 100 || p.usage.Output == nil {
				t.Fatalf("partial components promoted or lost: final=%v usage=%+v", p.usageFinal, p.usage)
			}
		})
	}
}
