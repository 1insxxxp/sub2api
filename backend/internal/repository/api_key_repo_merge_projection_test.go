package repository

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestGroupEntityToServicePreservesImageCountBillingWithCustomPolicy(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		group := &dbent.Group{
			ID:                               17,
			Tag:                              "custom",
			TagColor:                         "#123456",
			KeyDisplayCategory:               "gemini",
			SortOrder:                        27,
			SystemCustomRoutingEnabled:       true,
			EmptyResponseCompensationEnabled: true,
			ImageCountBilling:                enabled,
		}
		got := groupEntityToService(group)
		require.Equal(t, enabled, got.ImageCountBilling)
		require.Equal(t, group.Tag, got.Tag)
		require.Equal(t, group.TagColor, got.TagColor)
		require.Equal(t, group.KeyDisplayCategory, got.KeyDisplayCategory)
		require.Equal(t, group.SortOrder, got.SortOrder)
		require.True(t, got.SystemCustomRoutingEnabled)
		require.True(t, got.EmptyResponseCompensationEnabled)
	}
}
