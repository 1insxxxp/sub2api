//go:build unit

package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupKeyDisplayCategoryVisibleInAllResponses(t *testing.T) {
	for _, category := range []string{"", "domestic", "openai", "anthropic", "other"} {
		group := &service.Group{Platform: service.PlatformOpenAI, KeyDisplayCategory: category}
		for _, mapped := range []any{GroupFromService(group), GroupFromServiceAdmin(group), GroupFromServiceShallow(group)} {
			raw, err := json.Marshal(mapped)
			require.NoError(t, err)
			var body map[string]any
			require.NoError(t, json.Unmarshal(raw, &body))
			require.Equal(t, category, body["key_display_category"])
			require.Equal(t, service.PlatformOpenAI, body["platform"])
		}
	}
}
