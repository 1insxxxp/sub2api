package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAcceptanceNativeFixtures(t *testing.T) {
	target := AcceptanceSnapshot{Platform: PlatformAnthropic, Model: "kiro/claude-opus", UpstreamModel: "claude-opus"}
	for _, tc := range []struct{ protocol, category, input, cap string }{
		{"messages", "cache", "messages", "max_tokens"},
		{"count_tokens", "count", "messages", ""},
		{"responses", "tools", "input", "max_output_tokens"},
	} {
		payload, err := acceptanceRequestBody(AcceptanceSample{Protocol: tc.protocol, Category: tc.category, Prompt: "verify", MaxOutputTokens: 9999, Snapshot: target})
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]any
		if err = json.Unmarshal(payload, &body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != target.Model || body[tc.input] == nil || body["contents"] != nil {
			t.Fatalf("incorrect %s body %s", tc.protocol, payload)
		}
		if tc.cap != "" && body[tc.cap] != float64(2048) {
			t.Fatalf("missing output cap %s", payload)
		}
		if tc.category == "cache" && !strings.Contains(string(payload), "cache_control") {
			t.Fatal("cache sample missing cache control")
		}
		if tc.category == "count" && (body["max_tokens"] != nil || body["stream"] != nil) {
			t.Fatal("count used generator parameters")
		}
		if tc.category == "tools" && body["tools"] == nil {
			t.Fatal("tool fixture missing")
		}
	}
}

func TestAcceptanceStandardSuiteCoversThreeProtocolsWithinBudget(t *testing.T) {
	target := AcceptanceSnapshot{Platform: PlatformAnthropic, AccountType: AccountTypeAPIKey, Model: "claude-opus", UpstreamModel: "claude-opus", Protocols: []string{"messages", "chat", "responses"}}
	modes := map[string]map[bool]bool{}
	calls := 0
	for _, sample := range acceptanceManifest(target, nil, "run") {
		if sample.Category == "count" {
			continue
		}
		calls++
		if modes[sample.Protocol] == nil {
			modes[sample.Protocol] = map[bool]bool{}
		}
		modes[sample.Protocol][sample.Stream] = true
	}
	if calls != 17 {
		t.Fatalf("expected 17 generated responses, got %d", calls)
	}
	for _, protocol := range target.Protocols {
		if len(modes[protocol]) != 2 {
			t.Fatalf("missing protocol/mode %v", modes)
		}
	}
}
func TestAcceptanceMessagesCacheWriteCost(t *testing.T) {
	input, output, read, write := int64(120), int64(5), int64(50), int64(20)
	u := AcceptanceUsage{Source: "upstream_anthropic_reported", Input: &input, Output: &output, CacheRead: &read, CacheCreate: &write, OutputIncludesThoughts: true}
	p := AcceptancePrice{Source: "catalog", InputPerMillion: 2, OutputPerMillion: 10, CachePerMillion: 0.2, CacheWritePerMillion: 2.5}
	cost := AcceptanceCalculatedCost(p, u)
	if cost == nil || *cost != 0.00021 {
		t.Fatalf("incorrect native price %v", cost)
	}
	p.CacheWritePerMillion = 0
	if AcceptanceCalculatedCost(p, u) != nil {
		t.Fatal("missing cache write price was treated as free")
	}
}
