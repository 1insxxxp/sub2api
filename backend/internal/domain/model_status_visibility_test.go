package domain

import "testing"

func TestGroupModelStatusVisibilityNormalize(t *testing.T) {
	cfg := GroupModelStatusVisibility{Enabled: true, Models: []string{" gpt-image-1 ", "gpt-image-1", " gemini-* "}}
	got := cfg.Normalize()
	if len(got.Models) != 2 || got.Models[0] != "gpt-image-1" || got.Models[1] != "gemini-*" {
		t.Fatalf("Normalize() = %#v", got)
	}
	if !got.Allows("gpt-image-1") || !got.Allows("gemini-2.5-flash") || got.Allows("gpt-5.4") {
		t.Fatalf("unexpected visibility matching: %#v", got)
	}
}

func TestGroupModelStatusVisibilityDefaultsToAll(t *testing.T) {
	var cfg GroupModelStatusVisibility
	if !cfg.Allows("any-model") {
		t.Fatal("disabled or empty visibility config must allow all models")
	}
}

func TestGroupModelStatusVisibilityFilterPreservesSourceOrderAndMatchingRules(t *testing.T) {
	cfg := GroupModelStatusVisibility{Enabled: true, Models: []string{" GPT-IMAGE-1 ", "gemini-*"}}.Normalize()
	source := []string{"gpt-5.4", "gemini-2.5-flash", "gpt-image-1", "claude-sonnet"}
	got := cfg.Filter(source)
	want := []string{"gemini-2.5-flash", "gpt-image-1"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Filter() = %#v, want %#v", got, want)
	}
}

func TestGroupModelStatusVisibilityEmptyEnabledConfigAllowsAll(t *testing.T) {
	cfg := GroupModelStatusVisibility{Enabled: true}.Normalize()
	if !cfg.Allows("gpt-5.4") || len(cfg.Filter([]string{"gpt-5.4"})) != 1 {
		t.Fatal("empty enabled visibility config must keep all models visible")
	}
}
