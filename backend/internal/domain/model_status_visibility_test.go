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
