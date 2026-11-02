//go:build unit

package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func supportPrivacyJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func assertSupportPrivacyOutput(t *testing.T, got string, private ...string) {
	t.Helper()
	if strings.TrimSpace(got) == "" {
		t.Fatal("a recorded reason must yield a nonempty static safe explanation")
	}
	for _, canary := range private {
		if strings.Contains(got, canary) {
			t.Errorf("private synthetic canary %q reflected in %q", canary, got)
		}
	}
	if again := PublicUserSupportOriginalMessage(got, ""); again != got {
		t.Errorf("safe projection must be stable on repeated application: %q -> %q", got, again)
	}
}

func TestUserSupportPrivacyPreservesExactApprovedGeminiReason(t *testing.T) {
	const approved = "Gemini rejected the prompt (PROHIBITED_CONTENT). Review the text and attached media, then try again."
	for _, tc := range []struct{ name, primary, upstream string }{
		{"plain", approved, ""},
		{"reviewed error message", supportPrivacyJSON(t, map[string]any{"error": map[string]any{"message": approved}, "project": "metadataProjectCanary6c2"}), ""},
		{"missing primary fallback", "", approved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := PublicUserSupportOriginalMessage(tc.primary, tc.upstream)
			if got != approved {
				t.Errorf("approved customer reason changed: got %q, want %q", got, approved)
			}
			assertSupportPrivacyOutput(t, got, "metadataProjectCanary6c2")
		})
	}
}

func TestUserSupportPrivacyPreservesReviewedGeminiPromptRejection(t *testing.T) {
	const approved = "Gemini rejected the prompt (PROHIBITED_CONTENT). Review the text and attached media, then try again."
	const private = "; project=naturalPromptProjectCanary7f2; key=naturalPromptKeyCanary4d8; request_body={\"input\":\"naturalPromptInputCanary9c3\"}"
	for _, reason := range []string{
		"Gemini rejected prompt: PROHIBITED_CONTENT",
		"Gemini rejected the prompt: PROHIBITED_CONTENT",
	} {
		for _, raw := range []string{
			reason,
			reason + private,
			supportPrivacyJSON(t, map[string]any{"error": map[string]any{"message": reason + private}}),
			supportPrivacyJSON(t, map[string]any{"response": map[string]any{"error": map[string]any{"message": reason + private}}}),
		} {
			got := PublicUserSupportOriginalMessage(raw, "RESOURCE_EXHAUSTED")
			if got != approved {
				t.Errorf("reviewed request-scoped natural reason must use its static template: got %q, want %q", got, approved)
			}
			assertSupportPrivacyOutput(t, got, "naturalPromptProjectCanary7f2", "naturalPromptKeyCanary4d8", "naturalPromptInputCanary9c3")
		}
	}
}

func TestUserSupportPrivacyGeminiPromptRejectionDoesNotRecognizePayloadEchoes(t *testing.T) {
	want := PublicUserSupportOriginalMessage("Unrecognized final failure", "")
	for _, raw := range []string{
		`prompt: PROHIBITED_CONTENT`,
		`prompt: "Gemini rejected prompt: PROHIBITED_CONTENT"`,
		`Final failure; prompt: "PROHIBITED_CONTENT"`,
		`Final failure; prompt: Gemini rejected prompt: PROHIBITED_CONTENT`,
		`Final failure; debug="Gemini rejected prompt: PROHIBITED_CONTENT"`,
		`Final failure; ["Gemini rejected prompt: PROHIBITED_CONTENT"]`,
		`Final failure; Gemini rejected prompt: PROHIBITED_CONTENT`,
		`Gemini rejected prompt: ["PROHIBITED_CONTENT"]`,
		`Gemini rejected prompt: "PROHIBITED_CONTENT"`,
		`Gemini rejected prompt: PROHIBITED_CONTENT_SUFFIX`,
		`{"message":"Final failure","request":{"message":"Gemini rejected prompt: PROHIBITED_CONTENT"}}`,
		`{"error":{"message":"Final failure"},"debug":{"message":"Gemini rejected prompt: PROHIBITED_CONTENT"}}`,
	} {
		if got := PublicUserSupportOriginalMessage(raw, ""); got != want {
			t.Errorf("unknown payload text must not supply an approved Gemini reason: %q -> %q", raw, got)
		}
	}
}

func TestUserSupportPrivacyUnknownTextNeverReflectsPrivateValues(t *testing.T) {
	canonical := PublicUserSupportOriginalMessage("Unrecognized upstream failure", "")
	assertSupportPrivacyOutput(t, canonical)
	for _, tc := range []struct{ name, raw, canary string }{
		{"project", "Rejected; project=privateProjectCanary9b7", "privateProjectCanary9b7"},
		{"organization", "Rejected; organization=privateOrgCanary3f2", "privateOrgCanary3f2"},
		{"channel", "Rejected; channel=privateChannelCanary8d1", "privateChannelCanary8d1"},
		{"proxy pool", "Rejected; proxy=privateProxyCanary5c4", "privateProxyCanary5c4"},
		{"request identity", "Rejected; request_id=privateRequestCanary7e6", "privateRequestCanary7e6"},
		{"trace identity", "Rejected; trace_id=privateTraceCanary2a8", "privateTraceCanary2a8"},
		{"cloud resource", "Rejected; resource=projects/privateResourceCanary4e9/locations/global", "privateResourceCanary4e9"},
		{"bare key", "Rejected; key=privateBareKeyCanary6a2", "privateBareKeyCanary6a2"},
		{"unlabelled opaque credential", "Rejected opaqueCredentialCanary9f4A7b2Z5c8N6m3Q", "opaqueCredentialCanary9f4A7b2Z5c8N6m3Q"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, raw := range []string{
				tc.raw,
				supportPrivacyJSON(t, map[string]any{"error": map[string]any{"message": tc.raw}}),
				supportPrivacyJSON(t, map[string]any{"response": map[string]any{"error": map[string]any{"message": tc.raw}}}),
			} {
				got := PublicUserSupportOriginalMessage(raw, "")
				assertSupportPrivacyOutput(t, got, tc.canary)
				if got != canonical {
					t.Errorf("unknown free text must use one static safe explanation: got %q, want %q", got, canonical)
				}
			}
		})
	}
}

func TestUserSupportPrivacyRecognizedCodesUseOnlyStaticTemplates(t *testing.T) {
	const private = "project=privateCodeProjectCanary4d8; org=privateCodeOrgCanary2a5; channel=privateCodeChannelCanary1c9; proxy=privateCodeProxyCanary3f7; request_id=privateCodeRequestCanary6b2; trace_id=privateCodeTraceCanary8e1; resource=projects/privateCodeResourceCanary7d3; key=privateCodeBareKeyCanary5a6; opaqueCodeCredentialCanary8q2M5v7P1k9"
	canaries := []string{"privateCodeProjectCanary4d8", "privateCodeOrgCanary2a5", "privateCodeChannelCanary1c9", "privateCodeProxyCanary3f7", "privateCodeRequestCanary6b2", "privateCodeTraceCanary8e1", "privateCodeResourceCanary7d3", "privateCodeBareKeyCanary5a6", "opaqueCodeCredentialCanary8q2M5v7P1k9"}
	for _, code := range []string{"RESOURCE_EXHAUSTED", "INVALID_ARGUMENT", "UNAUTHENTICATED", "PROHIBITED_CONTENT"} {
		t.Run(code, func(t *testing.T) {
			canonical := PublicUserSupportOriginalMessage(code, "")
			assertSupportPrivacyOutput(t, canonical, canaries...)
			for _, raw := range []string{
				code + "; " + private,
				supportPrivacyJSON(t, map[string]any{"error": map[string]any{"message": code + "; " + private}}),
				supportPrivacyJSON(t, map[string]any{"error": map[string]any{"status": code, "message": private}}),
			} {
				got := PublicUserSupportOriginalMessage(raw, "")
				assertSupportPrivacyOutput(t, got, canaries...)
				if got != canonical {
					t.Errorf("recognized code must project its static template: got %q, want %q", got, canonical)
				}
			}
		})
	}
}

func TestUserSupportPrivacyUnknownPrimaryIsNotReplacedByEarlierAttempt(t *testing.T) {
	primary := "Unrecognized final failure; project=finalPrimaryCanary4b9; opaqueFinalCredentialCanary7m2V5q8"
	want := PublicUserSupportOriginalMessage(primary, "")
	for _, upstream := range []string{
		"Gemini rejected the prompt (PROHIBITED_CONTENT). Review the text and attached media, then try again.",
		"RESOURCE_EXHAUSTED; project=earlierAttemptCanary1d7",
	} {
		got := PublicUserSupportOriginalMessage(primary, upstream)
		assertSupportPrivacyOutput(t, got, "finalPrimaryCanary4b9", "opaqueFinalCredentialCanary7m2V5q8", "earlierAttemptCanary1d7")
		if got != want || got == PublicUserSupportOriginalMessage(upstream, "") {
			t.Errorf("unknown final reason must retain its own safe projection, not an earlier different attempt: %q", got)
		}
	}
}

func TestUserSupportPrivacyMissingOrUnextractableReasonIsEmpty(t *testing.T) {
	for _, raw := range []string{
		"", " \t\n", `""`, `{"message":""}`, `{"error":{"message":[]}}`,
		`{"request":{"message":"requestOnlyCanary4g8"},"project":"metadataOnlyCanary8v2"}`,
		`[{"message":"arrayDumpCanary3c9"}]`, `{"error":{"message":"unfinishedCanary5d1`,
		`<html><body>htmlOnlyCanary7b4</body></html>`, "Traceback (most recent call last):\nstackOnlyCanary2k6",
	} {
		if got := PublicUserSupportOriginalMessage(raw, ""); got != "" {
			t.Errorf("missing or unextractable reason must stay empty: got %q", got)
		}
	}
}

func TestUserSupportPrivacyIdentityOnlyPrimaryDoesNotReviveEarlierAttempt(t *testing.T) {
	const earlier = "PROHIBITED_CONTENT"
	want := PublicUserSupportOriginalMessage("Unrecognized final failure", "")
	for _, primary := range []string{
		"project=private-final-project; request failed",
		"projects/private-final-project/locations/global: request failed",
		"key=private-final-key",
	} {
		if got := PublicUserSupportOriginalMessage(primary, earlier); got != want {
			t.Errorf("private primary must retain an unknown explanation, not an earlier attempt: %q", got)
		}
	}
}

func TestUserSupportPrivacyDoesNotClassifyRequestOrDebugPayloads(t *testing.T) {
	want := PublicUserSupportOriginalMessage("Unrecognized final failure", "")
	for _, raw := range []string{
		`Final failure; debug="PROHIBITED_CONTENT"`,
		`Final failure; request_body={"message":"PROHIBITED_CONTENT"}`,
		`Final failure; input: ["PROHIBITED_CONTENT"]`,
		`Final failure; messages: ["RESOURCE_EXHAUSTED"]`,
		`Final failure; prompt: "PROHIBITED_CONTENT"`,
		`Final failure; ["PROHIBITED_CONTENT"]`,
		`{"message":"Final failure","request":{"status":"PROHIBITED_CONTENT"}}`,
		`{"message":"Final failure","status":"INVALID_ARGUMENT private-metadata"}`,
	} {
		if got := PublicUserSupportOriginalMessage(raw, ""); got != want {
			t.Errorf("private diagnostic data must not supply a public reason: %q", got)
		}
	}
}
