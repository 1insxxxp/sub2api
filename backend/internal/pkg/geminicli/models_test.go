package geminicli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultModels_ContainsImageModels(t *testing.T) {
	t.Parallel()

	byID := make(map[string]Model, len(DefaultModels))
	for _, model := range DefaultModels {
		byID[model.ID] = model
	}

	required := []string{
		"gemini-2.5-flash-image",
		"gemini-3.1-flash-image",
	}

	for _, id := range required {
		if _, ok := byID[id]; !ok {
			t.Fatalf("expected curated Gemini model %q to exist", id)
		}
	}
}

func TestDefaultModels_ExcludeShutdownModels(t *testing.T) {
	t.Parallel()

	ids := make(map[string]struct{}, len(DefaultModels))
	for _, model := range DefaultModels {
		ids[model.ID] = struct{}{}
	}

	require.NotContains(t, ids, "gemini-2.0-flash")
	require.NotEqual(t, "gemini-2.0-flash", DefaultTestModel)
}
