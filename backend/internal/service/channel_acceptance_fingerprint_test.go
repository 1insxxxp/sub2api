package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAcceptanceFingerprintSignaturesArePresenceOnly(t *testing.T) {
	for _, tc := range []struct{ name, wire, signal string }{
		{"messages", `{"type":"message","content":[{"type":"thinking","signature":"private-canary"}]}`, "messages_signature"},
		{"stream", "event: content_block_delta\ndata: {\"delta\":{\"type\":\"signature_delta\",\"signature\":\"private-canary\"}}\n\n", "messages_signature"},
		{"gemini wrapper", `{"response":{"candidates":[{"content":{"parts":[{"thoughtSignature":"private-canary"}]}}]}}`, "gemini_thought_signature"},
		{"tool arguments", `{"type":"message","content":[{"type":"tool_use","input":{"signature":"private-canary","thoughtSignature":"private-canary","assistantResponseEvent":{"content":"x"}}}]}`, ""},
		{"plain text", `{"choices":[{"message":{"content":"private-canary signature_delta thoughtSignature claude_max KIRO"}}]}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newAcceptanceEvidenceParser()
			p.fingerprint = newAcceptanceFingerprint()
			for _, b := range []byte(tc.wire) {
				p.Feed([]byte{b})
			}
			p.Finish()
			result := &AcceptanceResult{EffectiveParameters: map[string]any{}}
			p.snapshot().fingerprint.report("unknown", result)
			wire, err := json.Marshal(result)
			require.NoError(t, err)
			require.NotContains(t, string(wire), "private-canary")
			if tc.signal != "" {
				require.Contains(t, string(wire), tc.signal)
				require.Contains(t, string(wire), `"signature_present":true`)
			} else {
				require.Contains(t, string(wire), `"signature_present":false`)
				require.NotContains(t, string(wire), "kiro_assistant_event")
			}
			require.Contains(t, string(wire), `"signature_verified":false`)
		})
	}
}

func TestAcceptanceFingerprintSourceComparison(t *testing.T) {
	for _, tc := range []struct{ declaration, url, ua, auth, code string }{
		{"vertex", "https://us-central1-aiplatform.googleapis.com/v1/models/m:generateContent", "", "", "source_endpoint_match"},
		{"google_developer_api", "https://generativelanguage.googleapis.com/v1beta/models/m:countTokens", "", "", "source_endpoint_match"},
		{"vertex", "https://generativelanguage.googleapis.com/v1beta/models/m:generateContent", "", "", "source_mismatch"},
		{"google_code_assist", "https://cloudcode-pa.googleapis.com/v1internal:generateContent", "", "", "source_endpoint_match"},
		{"antigravity", "https://cloudcode-pa.googleapis.com/v1internal:generateContent", "antigravity/hub/1", "", "source_compatible"},
		{"antigravity", "https://cloudcode-pa.googleapis.com/v1internal:generateContent", "", "", "source_inconclusive"},
		{"anthropic", "https://api.anthropic.com/v1/messages", "", "", "source_endpoint_match"},
		{"claude_max", "https://api.anthropic.com/v1/messages", "", "Bearer private-canary", "source_inconclusive"},
		{"kiro", "https://q.us-east-1.amazonaws.com/generateAssistantResponse", "", "", "source_compatible"},
		{"bedrock", "https://bedrock-runtime.us-east-1.amazonaws.com/model/m/invoke", "", "", "source_endpoint_match"},
		{"vertex", "https://generativelanguage.googleapis.com.evil.test/v1beta/models/m:generateContent", "", "", "source_inconclusive"},
		{"unknown", "https://api.anthropic.com/v1/messages", "", "", "source_inconclusive"},
	} {
		t.Run(tc.declaration+"/"+tc.url, func(t *testing.T) {
			r := httptest.NewRequest("POST", tc.url+"?key=private-canary", nil)
			r.Header.Set("User-Agent", tc.ua)
			r.Header.Set("Authorization", tc.auth)
			f := newAcceptanceFingerprint()
			f.observeHTTP(r, acceptanceSourceFromRequest(r, "", "", ""), http.Header{"Set-Cookie": {"private-canary"}})
			result := &AcceptanceResult{EffectiveParameters: map[string]any{}, Status: "passed"}
			f.report(tc.declaration, result)
			require.Equal(t, tc.code, result.Findings[0].Code)
			if tc.code == "source_mismatch" {
				require.Equal(t, "anomaly", result.Status)
			} else {
				require.Equal(t, "passed", result.Status)
			}
			raw, err := json.Marshal(result)
			require.NoError(t, err)
			require.NotContains(t, string(raw), "private-canary")
		})
	}
}

func TestAcceptanceFingerprintExecutorUsesUpstreamOnly(t *testing.T) {
	for _, declared := range []string{"", "vertex"} {
		account := &Account{ID: 7, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "fixture"}}
		up := &acceptanceExecutorHTTP{status: 200, contentType: "application/json", body: `{"candidates":[{"content":{"parts":[{"text":"acceptance-ok","thoughtSignature":"private-canary"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":1,"totalTokenCount":4}}`}
		exec := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: account}, nil, &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: up}, nil, nil, nil)
		sample := acceptanceExecutorSample(account, "chat", false)
		sample.Snapshot.DeclaredSource = declared
		result, err := exec.Execute(context.Background(), sample)
		require.NoError(t, err)
		require.Equal(t, 1, up.calls)
		data, err := json.Marshal(result)
		require.NoError(t, err)
		require.NotContains(t, string(data), "private-canary")
		require.Equal(t, declared != "", strings.Contains(string(data), "source_fingerprint"))
		require.Equal(t, declared != "", strings.Contains(string(data), "gemini_thought_signature"))
	}
}

func TestAcceptanceFingerprintInfersKiroIndependentlyOfDeclaration(t *testing.T) {
	events := []string{
		`{"assistantResponseEvent":{"content":"private-canary"}}`,
		`{"type":"message_stop"}`,
		`{"contextUsageEvent":{"contextUsagePercentage":12.5}}`,
	}
	for _, tc := range []struct{ declared, code, level, status string }{
		{"claude_max", "source_mismatch", "warning", "anomaly"},
		{"unknown", "source_candidate_inferred", "info", "passed"},
		{"kiro", "source_compatible", "info", "passed"},
	} {
		t.Run(tc.declared, func(t *testing.T) {
			p := newAcceptanceEvidenceParser()
			p.fingerprint = newAcceptanceFingerprint()
			for _, event := range events {
				p.Feed([]byte("data: " + event + "\n\n"))
			}
			p.Finish()
			r := &AcceptanceResult{Status: "passed", EffectiveParameters: map[string]any{}}
			p.snapshot().fingerprint.report(tc.declared, r)
			require.Equal(t, "kiro", r.EffectiveParameters["source_fingerprint"].(map[string]any)["inferred_candidate"])
			require.Equal(t, tc.code, r.Findings[0].Code)
			require.Equal(t, tc.level, r.Findings[0].Level)
			require.Contains(t, r.Findings[0].Message, "Kiro/Amazon Q")
			require.Equal(t, tc.status, r.Status)
		})
	}

	p := newAcceptanceEvidenceParser()
	p.fingerprint = newAcceptanceFingerprint()
	for range 3 {
		p.Feed([]byte("data: " + events[0] + "\n\n"))
	}
	p.Finish()
	// AWS transport and repeated copies of the same event contribute no independent Kiro signal.
	req := httptest.NewRequest("POST", "https://bedrock-runtime.us-east-1.amazonaws.com/model/m/invoke", nil)
	p.fingerprint.observeHTTP(req, acceptanceSourceFromRequest(req, "", "", ""),
		http.Header{"X-Amzn-Bedrock-Invocation-Latency": {"1"}, "Content-Type": {"application/vnd.amazon.eventstream"}})
	r := &AcceptanceResult{Status: "passed", EffectiveParameters: map[string]any{}}
	p.fingerprint.report("unknown", r)
	require.NotContains(t, r.EffectiveParameters["source_fingerprint"], "inferred_candidate")
	require.NotContains(t, r.Findings[0].Message, "Kiro/Amazon Q")
}

func TestAcceptanceFingerprintPlaceholdersAndUntrustedDeclaration(t *testing.T) {
	p := newAcceptanceEvidenceParser()
	p.fingerprint = newAcceptanceFingerprint()
	p.Feed([]byte(`{"candidates":[{"content":{"parts":[{"thoughtSignature":"skip_thought_signature_validator"},{"thoughtSignature":" "}]}}]}`))
	p.Finish()
	r := &AcceptanceResult{EffectiveParameters: map[string]any{}}
	p.fingerprint.report("private-canary", r)
	require.Empty(t, r.Findings)
	require.Empty(t, r.EffectiveParameters)
	p.fingerprint.report("unknown", r)
	fp := r.EffectiveParameters["source_fingerprint"].(map[string]any)
	require.Equal(t, false, fp["signature_present"])
	require.Contains(t, fp["signals"], "signature_placeholder")
}

func TestAcceptanceFingerprintReportAfterFinishIgnoresDetachedChunks(t *testing.T) {
	p := newAcceptanceEvidenceParser()
	p.fingerprint = newAcceptanceFingerprint()
	// The report must finish the pending JSON document, including on an early return.
	p.Feed([]byte(`{"type":"message","content":[{"type":"thinking","signature":"private"}]}`))
	report := func() map[string]any {
		r := &AcceptanceResult{EffectiveParameters: map[string]any{}}
		p.reportFingerprint("unknown", r)
		return r.EffectiveParameters["source_fingerprint"].(map[string]any)
	}
	first := report()
	require.Equal(t, true, first["signature_present"])
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.Feed([]byte(`{"contextUsageEvent":{"contextUsagePercentage":20}}`))
		p.Finish()
		req := httptest.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
		p.observeFingerprintResponse(req, acceptanceSourceFromRequest(req, "", "", ""), &http.Response{Header: http.Header{}}, false)
	}()
	for range 10 {
		require.Equal(t, first, report())
	}
	<-done
	require.Equal(t, first, report())
	require.Empty(t, first["endpoint"])
	require.NotContains(t, first["signals"], "kiro_context_usage_event")
}

func TestAcceptanceFingerprintAndReportedSourceAgreeOnHost(t *testing.T) {
	const official = "api.anthropic.com"
	for _, tc := range []struct {
		name, dial, host, responseHost string
		direct                         bool
	}{
		{"normal", official, "", "", true},
		{"authority overridden by transport", official, official, "relay.example", false},
		{"case insensitive", official, "API.ANTHROPIC.COM", "", true},
		{"overridden authority", official, "relay.example", "", false},
		{"official Host on relay", "relay.example", official, "", false},
		{"different port", official, official + ":8443", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "https://"+tc.dial+"/v1/messages", nil)
			r.Host = tc.host
			p := newAcceptanceEvidenceParser()
			p.fingerprint = newAcceptanceFingerprint()
			result := &AcceptanceResult{EffectiveParameters: map[string]any{}, Status: "passed"}
			base := &acceptanceExecutorHTTP{status: 200, contentType: "application/json", body: "{}"}
			if tc.responseHost != "" {
				base.fn = func(sent *http.Request) { sent.Host = tc.responseHost }
			}
			u := &acceptanceHTTPUpstream{accountID: 7, base: base, parser: p, result: result}
			resp, err := u.Do(r, "", 7, 1)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			p.reportFingerprint("anthropic", result)
			fp := result.EffectiveParameters["source_fingerprint"].(map[string]any)
			require.Equal(t, tc.direct, result.Source.Kind == "direct")
			require.Equal(t, tc.direct, fp["endpoint"] == "anthropic")
			require.Equal(t, tc.direct, fp["verdict"] == "source_endpoint_match")
		})
	}
}
