//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicGroupSyncCategoryDefaultAutoFollowNewAndRestoredPrices(t *testing.T) {
	svc, store := publicSyncDefaultFixture(t)
	store.sources[0].Accounts[0].Credentials["model_mapping"] = map[string]any{"new-model": "upstream-new"}
	store.sources[0].Group.ModelPricing = []ChannelModelPricing{{Platform: PlatformGemini, Models: []string{"new-model"}, BillingMode: BillingModeToken, InputPrice: ptrFloat64(4e-6), OutputPrice: ptrFloat64(8e-6)}}
	store.config.InvalidatedModels = []string{"token-model", "new-model"}
	snapshot := defaultSyncSnapshot(t, svc)
	require.ElementsMatch(t, []string{"new-model", "request-model"}, snapshot.Models)
	price := snapshot.ModelPricing["new-model"]
	require.Equal(t, int64(1), price.SourceGroupID)
	require.Equal(t, "new-model", price.SourceModel)
	require.NotNil(t, price.InputPrice)
	require.Equal(t, 4e-6, *price.InputPrice)
	require.Equal(t, 2.0, *price.SourceGroupRatio)
	store.sources[0].Group.Status = StatusDisabled
	require.Equal(t, []string{"request-model"}, defaultSyncSnapshot(t, svc).Models)
	store.sources[0].Group.Status = StatusActive
	require.ElementsMatch(t, []string{"new-model", "request-model"}, defaultSyncSnapshot(t, svc).Models)
}
