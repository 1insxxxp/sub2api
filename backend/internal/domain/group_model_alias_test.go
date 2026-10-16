package domain

import (
	"encoding/json"
	"testing"
)

func TestNormalizeModelAliasesRejectsDuplicateAndResolves(t *testing.T) {
	aliases, err := NormalizeModelAliases([]ModelAlias{{Alias: " Fast ", UpstreamModel: "claude-sonnet"}, {Alias: "fast", UpstreamModel: "other"}})
	if err == nil {
		t.Fatal("expected duplicate alias error")
	}
	if aliases != nil {
		t.Fatalf("expected nil aliases on error, got %#v", aliases)
	}

	normalized, err := NormalizeModelAliases([]ModelAlias{{Alias: " Fast ", UpstreamModel: " claude-sonnet "}})
	if err != nil {
		t.Fatal(err)
	}
	if got := normalized[0].Alias; got != "Fast" {
		t.Fatalf("alias = %q", got)
	}
	if got := normalized[0].UpstreamModel; got != "claude-sonnet" {
		t.Fatalf("upstream = %q", got)
	}
}

func TestModelAliasesJSONRoundTripAndResolve(t *testing.T) {
	in := []ModelAlias{{Alias: "fast", UpstreamModel: "claude-sonnet", AntiTruncation: AntiTruncationPolicy{Enabled: true, MaxRounds: 2}}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out []ModelAlias
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out[0].AntiTruncation.MaxRounds != 2 {
		t.Fatalf("round trip failed: %#v", out)
	}
	resolved, ok := ResolveModelAlias(out, "FAST")
	if !ok || resolved != "claude-sonnet" {
		t.Fatalf("resolve = %q, %v", resolved, ok)
	}
}

func TestModelAliasesRejectUnsafeNamesAndInvalidLimits(t *testing.T) {
	for _, alias := range []ModelAlias{
		{Alias: "same", UpstreamModel: "SAME"},
		{Alias: "with space", UpstreamModel: "model"},
		{Alias: "public", UpstreamModel: "bad\nmodel"},
		{Alias: "public", UpstreamModel: "model", AntiTruncation: AntiTruncationPolicy{MaxRounds: 4}},
		{Alias: "public", UpstreamModel: "model", AntiTruncation: AntiTruncationPolicy{MaxDurationSeconds: 301}},
	} {
		if _, err := NormalizeModelAliases([]ModelAlias{alias}); err == nil {
			t.Fatalf("expected invalid alias: %#v", alias)
		}
	}
	aliases, err := NormalizeModelAliases([]ModelAlias{{Alias: "public", UpstreamModel: "model", AntiTruncation: AntiTruncationPolicy{Enabled: true}}})
	if err != nil || aliases[0].AntiTruncation.MaxOutputTokens != 0 {
		t.Fatalf("zero output budget must not impose a cap: %#v, %v", aliases, err)
	}
}

func TestModelAliasesRejectAliasTargetChains(t *testing.T) {
	_, err := NormalizeModelAliases([]ModelAlias{
		{Alias: "long", UpstreamModel: "claude-opus-4-6"},
		{Alias: "CLAUDE-OPUS-4-6", UpstreamModel: "other-model"},
	})
	if err == nil {
		t.Fatal("alias targets must remain real upstream names, not another alias")
	}
}
