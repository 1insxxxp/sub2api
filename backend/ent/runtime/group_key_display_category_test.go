//go:build unit

package runtime_test

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/ent/group"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/stretchr/testify/require"
)

func TestGroupKeyDisplayCategoryGeneratedValidator(t *testing.T) {
	require.NotNil(t, group.KeyDisplayCategoryValidator)
	for _, category := range []string{"", "anthropic", "openai", "gemini", "grok", "domestic", "image", "other"} {
		t.Run(category, func(t *testing.T) {
			require.NoError(t, group.KeyDisplayCategoryValidator(category))
		})
	}
	for _, category := range []string{"unknown", "Gemini", "images", " image "} {
		require.Error(t, group.KeyDisplayCategoryValidator(category))
	}
}
