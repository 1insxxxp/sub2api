//go:build unit

package service

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPublicUserSupportOriginalMessagePreservesRecordedReason(t *testing.T) {
	const reason = "Gemini rejected the prompt (PROHIBITED_CONTENT). Review the text and attached media, then try again."
	for _, tc := range []struct{ name, primary, upstream, want string }{
		{"request scoped wins", reason, "Earlier provider rate limit", reason},
		{"short reason projects safely", "Gemini rejected the request: PROHIBITED_CONTENT", "", reason},
		{"exact customer example", "Gemini rejected the prompt (PROHIBITED_CONTENT). Review the text and attached media, then try again.", "Earlier provider failure", "Gemini rejected the prompt (PROHIBITED_CONTENT). Review the text and attached media, then try again."},
		{"upstream fallback", "  ", reason, reason},
		{"unknown diagnostic is private", "Invalid input: expected a string; max_tokens must exceed thinking.budget_tokens", "", userSupportPrivateReason},
		{"unknown schema diagnostic is private", "Invalid field contents.parts: expected an array", "", userSupportPrivateReason},
		{"error message only", `{"error":{"message":"` + reason + `","code":"PROHIBITED_CONTENT","request_body":"PRIVATE_REQUEST"},"debug":"PRIVATE_DEBUG"}`, "", reason},
		{"wrapped error message", `{"response":{"error":{"message":"` + reason + `"}},"request":{"messages":["PRIVATE_REQUEST"]}}`, "", reason},
		{"top level message", `{"message":"` + reason + `","stack":"PRIVATE_STACK"}`, "", reason},
		{"string error", `{"error":"` + reason + `","headers":{"Authorization":"PRIVATE_HEADER"}}`, "", reason},
		{"encoded message", `{"error":{"message":"{\"message\":\"` + reason + `\",\"request_body\":\"PRIVATE_REQUEST\"}"}}`, "", reason},
		{"unknown json falls back", `{"request":{"error":{"message":"PRIVATE_REQUEST"}},"debug":"PRIVATE_DEBUG"}`, reason, reason},
		{"nothing recorded", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := PublicUserSupportOriginalMessage(tc.primary, tc.upstream); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPublicUserSupportOriginalMessageDropsPrivateDiagnostics(t *testing.T) {
	const reason = "Gemini rejected the prompt (PROHIBITED_CONTENT). Review the text and attached media, then try again."
	for _, tc := range []struct {
		name, raw string
		private   []string
	}{
		{"request echo", reason + `; request_body={"messages":[{"content":"PRIVATE_REQUEST"}],"headers":{"Authorization":"PRIVATE_HEADER"}}`, []string{"PRIVATE_REQUEST", "PRIVATE_HEADER", "request_body", "messages"}},
		{"plain request echo", reason + `; request_body=PRIVATE_REQUEST`, []string{"PRIVATE_REQUEST", "request_body"}},
		{"plain prompt echo", reason + `; prompt=PRIVATE_PROMPT`, []string{"PRIVATE_PROMPT", "prompt="}},
		{"plain input echo", reason + `; input=PRIVATE_INPUT`, []string{"PRIVATE_INPUT", "input="}},
		{"embedded debug json", reason + `; debug={"arbitrary":"PRIVATE_DEBUG","request":{"input":"PRIVATE_REQUEST"}}`, []string{"PRIVATE_DEBUG", "PRIVATE_REQUEST", "arbitrary", "debug="}},
		{"message carries private fields", `{"error":{"message":"` + reason + `; request_body=PRIVATE_REQUEST; Authorization: Bearer PRIVATE_HEADER"}}`, []string{"PRIVATE_REQUEST", "PRIVATE_HEADER", "Authorization"}},
		{"credential header", reason + "\nAuthorization: AWS4-HMAC-SHA256 Credential=PRIVATE_AWS, Signature=PRIVATE_SIGNATURE", []string{"PRIVATE_AWS", "PRIVATE_SIGNATURE", "Authorization"}},
		{"credential metadata", reason + `; Credential=PRIVATE_AWS; Signature=PRIVATE_SIGNATURE`, []string{"PRIVATE_AWS", "PRIVATE_SIGNATURE"}},
		{"credentials", reason + `; Bearer opaqueCredential; sk-ant-api03-privateCredential123 AQ.Ab` + strings.Repeat("x", 48) + ` GOCSPX-` + strings.Repeat("y", 30), []string{"opaqueCredential", "privateCredential123", strings.Repeat("x", 48), strings.Repeat("y", 30)}},
		{"locations and account identity", reason + `; endpoint=https://private.example/customer/PRIVATE_PATH?key=PRIVATE_KEY host=node.private.invalid:9443 ip=10.12.13.14 [2001:db8::1]:8080 account_id=9123 account_name="PRIVATE_POOL"`, []string{"private.example", "PRIVATE_PATH", "PRIVATE_KEY", "node.private.invalid", "10.12.13.14", "2001:db8::1", "9123", "PRIVATE_POOL"}},
		{"internal endpoint", reason + `; host=localhost:8443`, []string{"localhost", "8443", "host="}},
		{"bare addresses", reason + `; node.private.invalid:9443 10.12.13.14 [2001:db8::1]:8080 account_id=9123 account_name="PRIVATE_POOL"`, []string{"node.private.invalid", "10.12.13.14", "2001:db8::1", "9123", "PRIVATE_POOL"}},
		{"stack trace", reason + "\ngoroutine 12 [running]:\nservice.privateHandler /PRIVATE_SOURCE.go:12", []string{"goroutine", "privateHandler", "PRIVATE_SOURCE"}},
		{"python trace", reason + "\nTraceback (most recent call last):\nFile \"PRIVATE_SOURCE.py\", line 4", []string{"Traceback", "PRIVATE_SOURCE"}},
		{"html suffix", reason + ` <html><body>PRIVATE_HTML<script>PRIVATE_SCRIPT</script></body></html>`, []string{"<html", "PRIVATE_HTML", "PRIVATE_SCRIPT"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := PublicUserSupportOriginalMessage(tc.raw, "")
			if got != reason {
				t.Fatalf("recorded reason lost: %q", got)
			}
			for _, private := range tc.private {
				if strings.Contains(got, private) {
					t.Errorf("private fragment %q remains in %q", private, got)
				}
			}
			if strings.ContainsAny(got, "{}") {
				t.Errorf("JSON/debug envelope remains in %q", got)
			}
			if again := PublicUserSupportOriginalMessage(got, ""); again != got {
				t.Errorf("sanitization is not idempotent: %q then %q", got, again)
			}
		})
	}
}

func TestPublicUserSupportOriginalMessageRejectsPayloadsAndBoundsRunes(t *testing.T) {
	for _, raw := range []string{
		`{"request_body":"PRIVATE_REQUEST","headers":{"Authorization":"PRIVATE_HEADER"}}`,
		`[{"message":"PRIVATE_ARRAY_DUMP"}]`,
		`<html><body>PRIVATE_HTML</body></html>`,
		"Traceback (most recent call last):\nPRIVATE_STACK",
		`{"error":{"message":"unfinished PRIVATE_REQUEST`,
	} {
		if got := PublicUserSupportOriginalMessage(raw, ""); got != "" {
			t.Errorf("payload without an extractable reason should be empty: %q", got)
		}
	}
	got := PublicUserSupportOriginalMessage("PROHIBITED_CONTENT "+strings.Repeat("原因", 3000), "")
	if got != "Gemini rejected the prompt (PROHIBITED_CONTENT). Review the text and attached media, then try again." || !utf8.ValidString(got) || utf8.RuneCountInString(got) > 2000 {
		t.Fatalf("reason should remain valid UTF-8 and bounded to 2000 runes: length=%d", utf8.RuneCountInString(got))
	}
}
