//go:build unit

package service

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestPublicGroupSyncOrdinaryDisplayPricing(t *testing.T) {
	for _, rate := range []float64{1.5, 0.63, 0} {
		t.Run(strconv.FormatFloat(rate, 'g', -1, 64), func(t *testing.T) {
			group := Group{ID: 9, Name: "public", Platform: PlatformOpenAI, Status: StatusActive,
				RateMultiplier: rate, ImageRateIndependent: true, ImageRateMultiplier: 1,
				ModelPricing: []ChannelModelPricing{
					{Platform: PlatformOpenAI, Models: []string{"codex-auto-review"}, BillingMode: BillingModeToken,
						InputPrice: ptrFloat64(2e-7), OutputPrice: ptrFloat64(1.2e-6), CacheReadPrice: ptrFloat64(2e-8), CacheWritePrice: ptrFloat64(2.5e-7)},
					{Platform: PlatformOpenAI, Models: []string{"gpt-image-*"}, BillingMode: BillingModeImage, PerRequestPrice: ptrFloat64(.375)},
				},
			}
			channel := Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{9}, ModelPricing: []ChannelModelPricing{
				{Platform: PlatformOpenAI, Models: []string{"codex-auto-review"}, BillingMode: BillingModeToken, InputPrice: ptrFloat64(5e-6), OutputPrice: ptrFloat64(30e-6)},
				{Platform: PlatformOpenAI, Models: []string{"gpt-image-2.5"}, BillingMode: BillingModeToken, InputPrice: ptrFloat64(1e-6), OutputPrice: ptrFloat64(3e-6)},
			}}
			svc := NewPublicGroupSyncService(&stubGroupRepoForAvailable{activeGroups: []Group{group}}, &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) { return []Channel{channel}, nil }}, nil, nil)
			svc.SetCategoryDefaults(nil, NewModelPricingResolver(nil, NewBillingService(&config.Config{}, &PricingService{})))
			snapshots, err := svc.Snapshot(context.Background())
			require.NoError(t, err)
			require.Len(t, snapshots, 1)
			// Legacy relay catalog is unchanged: the new prices are display-only.
			require.Equal(t, 5e-6, *snapshots[0].ModelPricing["codex-auto-review"].InputPrice)
			require.Equal(t, "token", snapshots[0].ModelPricing["gpt-image-2.5"].BillingMode)
			require.Equal(t, rate, snapshots[0].GroupRatio)
			payload, err := json.Marshal(snapshots[0])
			require.NoError(t, err)
			var decoded struct {
				DisplayPricing map[string]PublicGroupSyncModel `json:"display_pricing"`
			}
			require.NoError(t, json.Unmarshal(payload, &decoded))
			require.Len(t, decoded.DisplayPricing, 2)
			price := decoded.DisplayPricing["codex-auto-review"]
			require.Equal(t, int64(9), price.SourceGroupID)
			require.Equal(t, "codex-auto-review", price.SourceModel)
			require.Equal(t, rate, *price.SourceGroupRatio)
			require.Equal(t, 2e-7, *price.InputPrice)
			require.Equal(t, 1.2e-6, *price.OutputPrice)
			require.Equal(t, 2e-8, *price.CacheReadPrice)
			require.Equal(t, 2.5e-7, *price.CacheWritePrice)
			image := decoded.DisplayPricing["gpt-image-2.5"]
			require.Equal(t, "image", image.BillingMode)
			require.Equal(t, .375, *image.PerRequestPrice)
			require.Equal(t, 1.0, *image.SourceGroupRatio)
			require.Nil(t, image.InputPrice)
		})
	}
}

func TestPublicGroupSyncDisplayPricingKeepsModesAndImageRatesPerGroup(t *testing.T) {
	groups := []Group{
		{ID: 1, Name: "token", Platform: PlatformOpenAI, RateMultiplier: 2},
		{ID: 2, Name: "image", Platform: PlatformOpenAI, RateMultiplier: 3, ImageRateIndependent: true, ImageRateMultiplier: 0,
			ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"shared-model"}, BillingMode: BillingModeImage, PerRequestPrice: ptrFloat64(.375)}}},
		{ID: 3, Name: "channel image", Platform: PlatformOpenAI, RateMultiplier: 4, ImageRateIndependent: true, ImageRateMultiplier: 1, ImagePrice1K: ptrFloat64(.375)},
	}
	channels := []Channel{
		{ID: 1, Status: StatusActive, GroupIDs: []int64{1, 2}, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"shared-model"}, BillingMode: BillingModeToken, InputPrice: ptrFloat64(2e-6), OutputPrice: ptrFloat64(3e-6), ImageInputPrice: ptrFloat64(4e-6), ImageOutputPrice: ptrFloat64(0)}}},
		{ID: 2, Status: StatusActive, GroupIDs: []int64{3}, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"shared-model"}, BillingMode: BillingModeImage, PerRequestPrice: ptrFloat64(1), Intervals: []PricingInterval{{TierLabel: "1K", PerRequestPrice: ptrFloat64(.5)}}}}},
	}
	svc := NewPublicGroupSyncService(&stubGroupRepoForAvailable{activeGroups: groups}, &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) { return channels, nil }}, nil, nil)
	svc.SetCategoryDefaults(nil, NewModelPricingResolver(nil, nil))
	all, err := svc.Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, all, 3)
	token := all[0].DisplayPricing["shared-model"]
	require.Equal(t, "token", token.BillingMode)
	require.Equal(t, 4e-6, *token.ImageInputPrice)
	require.NotNil(t, token.ImageOutputPrice)
	require.Zero(t, *token.ImageOutputPrice)
	image := all[1].DisplayPricing["shared-model"]
	require.Equal(t, "image", image.BillingMode)
	require.Zero(t, *image.SourceGroupRatio)
	require.Equal(t, .375, *image.PerRequestPrice)
	channelImage := all[2].DisplayPricing["shared-model"]
	require.Equal(t, .375, *channelImage.PerRequestPrice)
	require.Equal(t, 1.0, *channelImage.SourceGroupRatio)
	require.Equal(t, 1.0, *all[2].ModelPricing["shared-model"].PerRequestPrice, "display overrides must not change legacy relay catalog")

	groups[2].ImagePrice1K = nil
	all, err = svc.Snapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, .5, *all[2].DisplayPricing["shared-model"].PerRequestPrice)

	// Settlement uses the configured default when a size tier is zero.
	channels[1].ModelPricing[0].Intervals[0].PerRequestPrice = ptrFloat64(0)
	all, err = svc.Snapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1.0, *all[2].DisplayPricing["shared-model"].PerRequestPrice)
}

func TestPublicGroupSyncDisplayPricingDoesNotGuessCompositeRoutes(t *testing.T) {
	group := Group{ID: 1, Platform: PlatformComposite, RateMultiplier: 1}
	svc := NewPublicGroupSyncService(nil, nil, nil, nil)
	svc.SetCategoryDefaults(nil, NewModelPricingResolver(nil, NewBillingService(&config.Config{}, &PricingService{})))
	quotes, err := svc.ordinaryDisplayPricing(context.Background(), &group, []string{"gpt-5.4"}, populateChannelCache(nil, nil))
	require.NoError(t, err)
	require.Empty(t, quotes, "ordinary composite quotes require explicit route declarations; do not guess a fallback model price")
}
