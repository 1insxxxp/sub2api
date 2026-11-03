package domain

import (
	"encoding/json"
	"strings"
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
	in := []ModelAlias{{Alias: "fast", UpstreamModel: "claude-sonnet"}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out []ModelAlias
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0] != in[0] {
		t.Fatalf("round trip failed: %#v", out)
	}
	resolved, ok := ResolveModelAlias(out, "FAST")
	if !ok || resolved != "claude-sonnet" {
		t.Fatalf("resolve = %q, %v", resolved, ok)
	}
}

func TestModelAliasesRejectUnsafeNames(t *testing.T) {
	for _, alias := range []ModelAlias{
		{Alias: "same", UpstreamModel: "SAME"},
		{Alias: "with space", UpstreamModel: "model"},
		{Alias: "public", UpstreamModel: "bad\nmodel"},
	} {
		if _, err := NormalizeModelAliases([]ModelAlias{alias}); err == nil {
			t.Fatalf("expected invalid alias: %#v", alias)
		}
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

func TestModelAliasesIgnoreStoredGroupContinuationPolicy(t *testing.T) {
	var aliases []ModelAlias
	err := json.Unmarshal([]byte(`[{"alias":" Fast ","upstream_model":" claude-sonnet ","anti_truncation":{"enabled":true,"max_rounds":4}}]`), &aliases)
	if err != nil {
		t.Fatal(err)
	}
	aliases, err = NormalizeModelAliases(aliases)
	if err != nil {
		t.Fatalf("retired continuation policy must not invalidate alias: %v", err)
	}
	resolved, ok := ResolveModelAlias(aliases, "FAST")
	if !ok || resolved != "claude-sonnet" {
		t.Fatalf("resolve = %q, %v", resolved, ok)
	}
	raw, err := json.Marshal(aliases)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "anti_truncation") {
		t.Fatalf("retired continuation policy must not be re-encoded: %s", raw)
	}
}
