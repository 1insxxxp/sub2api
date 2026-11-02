//go:build unit

package service

import (
	"strings"
	"testing"
)

func TestPublicUserSupportGroupNameKeepsPublicLabels(t *testing.T) {
	for _, name := range []string{"", "客户分组", "【酒馆】量Gemini-反重力", "Gemini 2.5 Flash", "[REDACTED]"} {
		if got := PublicUserSupportGroupName(name); got != name {
			t.Errorf("public group label changed: %q -> %q", name, got)
		}
	}
}

func TestPublicUserSupportGroupNameHidesSensitiveWholeLabels(t *testing.T) {
	for _, name := range []string{
		"Gemini https://private.example/v1?key=opaqueXYZ", "Gemini node.private.invalid",
		"Gemini 10.4.3.2:8080", "Gemini [2001:db8::1]", "Gemini owner@example.test",
		"Gemini key=opaqueXYZ", "Gemini api_key: opaqueXYZ", "Gemini Bearer opaqueXYZ",
		"Gemini account ent-eu-prod exhausted", "Gemini project=billing-prod-42",
		"Gemini projects/billing-prod-42/locations/us-central1", "Gemini organization org-private",
		"Gemini channel=ent-relay-a", "Gemini proxy relay-private", "Gemini trace_id=private-request",
		"Gemini provider: private-supplier", "Gemini upstream=private-supplier", "Gemini request_id=private-request",
		"Gemini\nPRIVATE_LABEL", "Gemini\u202ePRIVATE_LABEL",
	} {
		got := PublicUserSupportGroupName(name)
		if got != "[REDACTED]" {
			t.Errorf("sensitive group name must be hidden as a whole: %q -> %q", name, got)
		}
		if again := PublicUserSupportGroupName(got); again != got {
			t.Errorf("group projection must be idempotent: %q -> %q", got, again)
		}
	}
	if got := PublicUserSupportGroupName(strings.Repeat("x", 3000)); got != "[REDACTED]" {
		t.Error("oversized labels must not bypass privacy checks")
	}
}
