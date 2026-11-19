package service

import (
	"math"
	"strings"
	"testing"
)

func TestAcceptanceManifestFixedSizeAndCachePrefix(t *testing.T) {
	snapshot := AcceptanceSnapshot{Platform: PlatformGemini, AccountType: AccountTypeAPIKey, AccountID: 7, Model: "gemini-flash", UpstreamModel: "gemini-flash", Price: AcceptancePrice{InputPerMillion: 1, OutputPerMillion: 5}}
	samples := acceptanceManifest(snapshot, nil, "run-nonce")
	if len(samples) != 20 {
		t.Fatalf("want 17 generation + 3 count calls, got %d", len(samples))
	}
	counts := map[string]int{}
	var cache string
	for _, s := range samples {
		counts[s.Category]++
		if s.Snapshot.AccountID != 7 || s.TimeoutSeconds != 180 || s.MaxOutputTokens != 2048 {
			t.Fatalf("lost fixed target/limits: %+v", s)
		}
		if s.Category == "cache" {
			if cache == "" {
				cache = s.Prompt
			}
			if cache != s.Prompt {
				t.Fatal("cache prefix changed")
			}
		}
		if s.EstimatedCostUSD < 0 || math.IsNaN(s.EstimatedCostUSD) {
			t.Fatal("invalid reservation")
		}
	}
	if counts["protocol"] != 4 || counts["input"] != 6 || counts["repeat"] != 3 || counts["cache"] != 4 || counts["count"] != 3 {
		t.Fatal(counts)
	}
	other := acceptanceManifest(snapshot, nil, "another-run")
	if other[13].Prompt == samples[13].Prompt {
		t.Fatal("fresh run must have a fresh cache prefix")
	}
}

func TestAcceptanceManifestExtrasAndNonceChallenges(t *testing.T) {
	samples := acceptanceManifest(AcceptanceSnapshot{Platform: PlatformGemini, AccountType: AccountTypeAPIKey, AccountID: 1, Model: "gemini", UpstreamModel: "gemini"}, []string{"long_context", "image", "audio", "tools"}, "seed")
	if len(samples) != 24 {
		t.Fatal(len(samples))
	}
	seen := map[string]bool{}
	for _, s := range samples {
		if s.Category == "repeat" {
			if s.Expected == "" || seen[s.Expected] || !strings.Contains(s.Prompt, s.Expected) {
				t.Fatal("invalid challenge")
			}
			seen[s.Expected] = true
		}
		if s.Ordinal >= 20 && s.TimeoutSeconds != 360 {
			t.Fatal("deep timeout")
		}
	}
}

func TestAcceptancePriceRejectsNonFiniteOrMissing(t *testing.T) {
	for _, p := range []*AcceptancePrice{nil, {}, {InputPerMillion: -1, Source: "manual"}, {InputPerMillion: math.NaN(), Source: "manual"}, {OutputPerMillion: math.Inf(1), Source: "manual"}} {
		if acceptanceValidPrice(p) {
			t.Fatalf("accepted invalid price %+v", p)
		}
	}
	if !acceptanceValidPrice(&AcceptancePrice{Source: "manual"}) {
		t.Fatal("explicitly free price should be valid")
	}
}

func TestAcceptanceManifestUsesAccountProtocolsAndRealCountEndpoint(t *testing.T) {
	for _, tc := range []struct {
		platform    string
		accountType string
		protocols   []string
		count       int
	}{
		{PlatformAnthropic, AccountTypeAPIKey, []string{"messages", "chat"}, 3},
		{PlatformOpenAI, AccountTypeAPIKey, []string{"chat", "responses"}, 0},
		{PlatformDeepseek, AccountTypeAPIKey, []string{"chat"}, 0},
	} {
		target := AcceptanceSnapshot{Platform: tc.platform, AccountType: tc.accountType, Protocols: tc.protocols, AccountID: 6178}
		got := acceptanceManifest(target, nil, "nonce")
		seen := make(map[string]int)
		for _, sample := range got {
			switch sample.Category {
			case "protocol":
				seen[sample.Protocol]++
			case "count":
				seen["count"]++
			default:
				if sample.Protocol != tc.protocols[0] {
					t.Fatalf("%s: unexpected protocol %s", tc.platform, sample.Protocol)
				}
			}
		}
		if seen["count"] != tc.count || seen["gemini"] != 0 {
			t.Fatalf("%s: wrong protocol plan %v", tc.platform, seen)
		}
		for _, protocol := range tc.protocols {
			if seen[protocol] != 2 {
				t.Fatalf("%s: missing stream/nonstream samples: %v", tc.platform, seen)
			}
		}
	}
}
