//go:build unit

package domain

import "testing"

func TestGroupKeyDisplayCategoryAllowedValues(t *testing.T) {
	for _, category := range []string{"", "anthropic", "openai", "domestic", "other"} {
		if err := ValidateGroupKeyDisplayCategory(category); err != nil {
			t.Errorf("allowed category %q: %v", category, err)
		}
	}
	for _, category := range []string{"unknown", "DOMESTIC", " domestic ", "gemini", "\x00"} {
		if err := ValidateGroupKeyDisplayCategory(category); err == nil {
			t.Errorf("unsupported category %q must be rejected", category)
		}
	}
}
