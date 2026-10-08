//go:build unit

package repository

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/stretchr/testify/require"
)

func TestGroupEntityPreservesKeyDisplayCategory(t *testing.T) {
	for _, category := range []string{"", "anthropic", "openai", "domestic", "other"} {
		entity := &dbent.Group{Platform: "openai", KeyDisplayCategory: category}
		require.NoError(t, group.KeyDisplayCategoryValidator(category))
		mapped := groupEntityToService(entity)
		require.Equal(t, category, mapped.KeyDisplayCategory)
		require.Equal(t, entity.Platform, mapped.Platform)
	}
	for _, category := range []string{"unknown", "DOMESTIC", " domestic ", "gemini"} {
		require.Error(t, group.KeyDisplayCategoryValidator(category))
	}
}
