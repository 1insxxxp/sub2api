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
		{"short reason preserved", "Gemini rejected the request: PROHIBITED_CONTENT", "", "Gemini rejected the request: PROHIBITED_CONTENT"},
		{"exact customer example", "Gemini rejected the prompt (PROHIBITED_CONTENT). Review the text and attached media, then try again.", "Earlier provider failure", "Gemini rejected the prompt (PROHIBITED_CONTENT). Review the text and attached media, then try again."},
		{"upstream fallback", "  ", reason, reason},
		{"parameter diagnostic preserved", "Invalid input: expected a string; max_tokens must exceed thinking.budget_tokens", "", "Invalid input: expected a string; max_tokens must exceed thinking.budget_tokens"},
		{"schema diagnostic preserved", "Invalid field contents.parts: expected an array", "", "Invalid field contents.parts: expected an array"},
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
		{"internal endpoint", reason + `; host=localhost:8443`, []string{"localhost", "8443"}},
		{"bare addresses", reason + `; node.private.invalid:9443 10.12.13.14 [2001:db8::1]:8080 account_id=9123 account_name="PRIVATE_POOL"`, []string{"node.private.invalid", "10.12.13.14", "2001:db8::1", "9123", "PRIVATE_POOL"}},
		{"stack trace", reason + "\ngoroutine 12 [running]:\nservice.privateHandler /PRIVATE_SOURCE.go:12", []string{"goroutine", "privateHandler", "PRIVATE_SOURCE"}},
		{"python trace", reason + "\nTraceback (most recent call last):\nFile \"PRIVATE_SOURCE.py\", line 4", []string{"Traceback", "PRIVATE_SOURCE"}},
		{"html suffix", reason + ` <html><body>PRIVATE_HTML<script>PRIVATE_SCRIPT</script></body></html>`, []string{"<html", "PRIVATE_HTML", "PRIVATE_SCRIPT"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := PublicUserSupportOriginalMessage(tc.raw, "")
			if !strings.HasPrefix(got, reason) {
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
	if !strings.HasPrefix(got, "PROHIBITED_CONTENT ") || !utf8.ValidString(got) || utf8.RuneCountInString(got) > 2000 {
		t.Fatalf("reason should remain valid UTF-8 and bounded to 2000 runes: length=%d", utf8.RuneCountInString(got))
	}
}

func TestPublicUserSupportOriginalMessageMasksFragmentsWithoutReplacingReason(t *testing.T) {
	for _, tc := range []struct{ name, raw, want string }{
		{"ordinary provider reason", "Unable to submit request because max_output_tokens must be greater than thinking_budget", "Unable to submit request because max_output_tokens must be greater than thinking_budget"},
		{"unrecognized reason", "The request contains an unsupported parameter: temperature", "The request contains an unsupported parameter: temperature"},
		{"known code retains details", "INVALID_ARGUMENT: max_tokens must exceed thinking.budget_tokens", "INVALID_ARGUMENT: max_tokens must exceed thinking.budget_tokens"},
		{"URL and credential", "Request to https://private.example/v1?key=private failed; api_key=private-secret; retry later", "Request to * failed; api_key=*; retry later"},
		{"identity suffix keeps advice", "Quota exceeded for project=private-project; retry after 30 seconds", "Quota exceeded for project=*; retry after 30 seconds"},
		{"unlabelled credential", "Rejected opaqueCredentialCanary9f4A7b2Z5c8N6m3Q; try a shorter request", "Rejected *; try a shorter request"},
		{"private payload suffix", `Invalid request; request_body={"input":"private content"}; retry later`, "Invalid request; *; retry later"},
		{"legacy placeholders", "Connect [upstream-url] failed for [email]: [REDACTED]", "Connect * failed for *: *"},
		{"auth-like payload tail", "Rejected; prompt=Bearer FIRST; PRIVATE_SECOND", "Rejected; *"},
		{"unstructured payload tail", "Rejected; prompt=PRIVATE_FIRST; PRIVATE_SECOND\nPRIVATE_THIRD", "Rejected; *"},
		{"quoted payload label", `Rejected; request_body"="PRIVATE_REQUEST"; retry later`, `Rejected; *; retry later`},
		{"numeric payload", `Rejected; [1234,5678]; retry later`, `Rejected; *; retry later`},
		{"request field index", `Invalid input[8].tools: expected an array`, `Invalid input[8].tools: expected an array`},
		{"quoted identity", `Rejected; "project":"shortProject"; retry later`, `Rejected; "project":*; retry later`},
		{"quoted Google key", `Rejected; x-goog-api-key"="shortGoogleSecret"; retry later`, `Rejected; x-goog-api-key"=*; retry later`},
		{"workspace identifier", `Rejected; workspace_id=shortWorkspace; retry later`, `Rejected; workspace_id=*; retry later`},
		{"IPv4 embedded IPv6", `Connection to 2001:db8:1:2:3:4:192.0.2.128 failed`, `Connection to * failed`},
		{"IPv6 socket", `dial tcp [2001:db8:1234:5678::1]:8080: connection refused`, `dial tcp *: connection refused`},
		{"bare IPv6", `Connection to 2001:db8:1234:5678::1 failed`, `Connection to * failed`},
		{"internal service DNS", `dial tcp upstream-prod:8080: connection refused`, `dial tcp *: connection refused`},
		{"unlabelled internal socket", `Connection to localhost:18093 failed; try again`, `Connection to * failed; try again`},
		{"unlabelled request ID", `Failed 2e506d17-354f-4e97-ab9e-b1d1e112d774; retry later`, `Failed *; retry later`},
		{"only code", `{"error":{"code":"INVALID_ARGUMENT"},"request":{"input":"private"}}`, "INVALID_ARGUMENT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := PublicUserSupportOriginalMessage(tc.raw, ""); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPublicUserSupportOriginalMessageMasksProviderIdentityLabels(t *testing.T) {
	for _, label := range []string{
		"upstream_name", "provider_id", "proxy_name",
		"upstream_account_id", "provider_account_name", "proxy_channel_id",
	} {
		t.Run(label, func(t *testing.T) {
			for _, assignment := range []string{
				label + "=privatePool",
				`"` + label + `":"privatePool"`,
				"'" + label + "'='privatePool'",
			} {
				raw := "Request failed; " + assignment + "; retry later"
				got := PublicUserSupportOriginalMessage(raw, "")
				if strings.Contains(got, "privatePool") || !strings.Contains(got, "*") || !strings.HasPrefix(got, "Request failed;") || !strings.HasSuffix(got, "; retry later") {
					t.Errorf("private identity must be masked without replacing the reason: %q -> %q", raw, got)
				}
				if again := PublicUserSupportOriginalMessage(got, ""); again != got {
					t.Errorf("masking must be stable: %q -> %q", got, again)
				}
			}
		})
	}
	for _, reason := range []string{
		"Unknown upstream failure; retry later",
		"Provider failed to process the request",
		"Proxy connection failed",
		"INVALID_ARGUMENT: max_tokens must exceed thinking.budget_tokens",
	} {
		if got := PublicUserSupportOriginalMessage(reason, ""); got != reason {
			t.Errorf("ordinary reason changed: got %q, want %q", got, reason)
		}
	}
}

func TestPublicUserSupportOriginalMessageMasksMultiwordIdentityValues(t *testing.T) {
	for _, label := range []string{"account_name", "upstream_name", "provider_name", "proxy_name"} {
		for _, value := range []string{"Secret Company", "内部 渠道一", "Secret, Company"} {
			for _, boundary := range []string{"; ", "\n"} {
				raw := "Request failed; " + label + "=" + value + boundary + "retry later"
				want := "Request failed; " + label + "=*" + boundary + "retry later"
				got := PublicUserSupportOriginalMessage(raw, "")
				if got != want {
					t.Errorf("unquoted identity value must be fully masked: got %q, want %q", got, want)
				}
				if again := PublicUserSupportOriginalMessage(got, ""); again != got {
					t.Errorf("masking must be stable: %q -> %q", got, again)
				}
			}
		}
		// Quoted values can contain a semicolon without ending the secret.
		raw := `Request failed; ` + label + `="Secret; Company"; retry later`
		want := "Request failed; " + label + "=*; retry later"
		if got := PublicUserSupportOriginalMessage(raw, ""); got != want {
			t.Errorf("quoted identity must use the quoted boundary: got %q, want %q", got, want)
		}
	}
}
