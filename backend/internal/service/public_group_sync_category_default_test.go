//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func publicSyncDefaultFixture(t *testing.T) (*PublicGroupSyncService, *categoryDefaultStoreFake) {
	t.Helper()
	a := categoryDefaultSource(1, "token source", map[string]any{"token-model": "upstream-a"})
	a.Group.RateMultiplier = 2
	a.Group.ModelPricing = []ChannelModelPricing{{Platform: PlatformGemini, Models: []string{"token-model"}, BillingMode: BillingModeToken, InputPrice: ptrFloat64(1e-6), OutputPrice: ptrFloat64(3e-6), CacheReadPrice: ptrFloat64(.1e-6), CacheWritePrice: ptrFloat64(1.25e-6)}}
	b := categoryDefaultSource(2, "request source", map[string]any{"request-model": "upstream-b"})
	b.Group.RateMultiplier = 3
	b.Group.ModelPricing = []ChannelModelPricing{{Platform: PlatformGemini, Models: []string{"request-model"}, BillingMode: BillingModePerRequest, PerRequestPrice: ptrFloat64(.25)}}
	store := &categoryDefaultStoreFake{sources: []CategoryDefaultSourceSnapshot{a, b}}
	defaults := NewCategoryDefaultGroupService(store, nil)
	preview, err := defaults.Preview(context.Background(), "gemini", []int64{1, 2})
	require.NoError(t, err)
	_, err = defaults.Apply(context.Background(), "gemini", CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{1, 2}, PreviewHash: preview.PreviewHash})
	require.NoError(t, err)
	groups := &stubGroupRepoForAvailable{activeGroups: []Group{
		*a.Group, *b.Group,
		{ID: 100, Name: "Gemini default", Status: StatusActive, Platform: PlatformComposite, IsCategoryDefault: true, RateMultiplier: 99, ModelPricing: []ChannelModelPricing{{Models: []string{"token-model"}, InputPrice: ptrFloat64(99)}}},
	}}
	channels := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) { return nil, nil }}
	// No accounts are attached to the entry. Source declarations remain valid
	// even while all accounts are temporarily absent from the scheduler.
	accounts := &stubAccountRepositoryForPublicGroupSync{schedulableByGroup: map[int64][]Account{}}
	svc := NewPublicGroupSyncService(groups, channels, accounts, nil)
	svc.SetCategoryDefaults(defaults, NewModelPricingResolver(nil, nil))
	return svc, store
}

func defaultSyncSnapshot(t *testing.T, svc *PublicGroupSyncService) PublicGroupSyncRequest {
	t.Helper()
	all, err := svc.Snapshot(context.Background())
	require.NoError(t, err)
	for _, snapshot := range all {
		if snapshot.GroupID == 100 {
			return snapshot
		}
	}
	t.Fatal("category default entry missing")
	return PublicGroupSyncRequest{}
}

func TestPublicGroupSyncCategoryDefaultPublishesSourceModelsAndPrices(t *testing.T) {
	svc, _ := publicSyncDefaultFixture(t)
	snapshot := defaultSyncSnapshot(t, svc)
	require.ElementsMatch(t, []string{"token-model", "request-model"}, snapshot.Models)
	require.Equal(t, 1.0, snapshot.GroupRatio)
	model := snapshot.ModelPricing["token-model"]
	require.Equal(t, int64(1), model.SourceGroupID)
	require.Equal(t, "token-model", model.SourceModel)
	require.Equal(t, 2.0, *model.SourceGroupRatio)
	require.Equal(t, 1e-6, *model.InputPrice)
	require.Equal(t, 3e-6, *model.OutputPrice)
	require.Equal(t, .1e-6, *model.CacheReadPrice)
	require.Equal(t, "token", model.BillingMode)
	fixed := snapshot.ModelPricing["request-model"]
	require.Equal(t, int64(2), fixed.SourceGroupID)
	require.Equal(t, 3.0, *fixed.SourceGroupRatio)
	require.Equal(t, .25, *fixed.PerRequestPrice)
	require.Equal(t, "per_request", fixed.BillingMode)
	require.Empty(t, snapshot.ModelMapping, "entry requests must keep their published call name")
}

func TestPublicGroupSyncCategoryDefaultFollowsDeclarationsAndFiltersInvalidSources(t *testing.T) {
	for _, change := range []string{"exclusive", "disabled", "mapping", "invalidated", "conflict", "unpublished"} {
		t.Run(change, func(t *testing.T) {
			svc, store := publicSyncDefaultFixture(t)
			expected := []string{"request-model"}
			switch change {
			case "exclusive":
				store.sources[0].Group.IsExclusive = true
			case "disabled":
				store.sources[0].Group.Status = StatusDisabled
			case "mapping":
				store.sources[0].Accounts[0].Credentials["model_mapping"] = map[string]any{"token-model": "changed"}
				expected = append(expected, "token-model")
			case "invalidated":
				store.config.InvalidatedModels = []string{"token-model"}
				expected = append(expected, "token-model")
			case "conflict":
				store.sources[1].Accounts[0].Credentials["model_mapping"] = map[string]any{"request-model": "upstream-b", "token-model": "other"}
			case "unpublished":
				store.sources[0].Accounts[0].Credentials["model_mapping"] = map[string]any{"token-model": "upstream-a", "new-model": "new-upstream"}
				store.sources[0].Group.ModelPricing = append(store.sources[0].Group.ModelPricing, ChannelModelPricing{Platform: PlatformGemini, Models: []string{"new-model"}, BillingMode: BillingModeToken, InputPrice: ptrFloat64(2e-6), OutputPrice: ptrFloat64(4e-6)})
				expected = append(expected, "token-model", "new-model")
			}
			require.ElementsMatch(t, expected, defaultSyncSnapshot(t, svc).Models)
		})
	}
}

func TestPublicGroupSyncCategoryDefaultUsesLiveSourcePriceAndZeroRate(t *testing.T) {
	svc, store := publicSyncDefaultFixture(t)
	store.sources[0].Group.RateMultiplier = 0
	store.sources[0].Group.ModelPricing[0].InputPrice = ptrFloat64(5e-6)
	model := defaultSyncSnapshot(t, svc).ModelPricing["token-model"]
	require.NotNil(t, model.SourceGroupRatio)
	require.NotNil(t, model.InputPrice)
	require.Equal(t, 0.0, *model.SourceGroupRatio)
	require.Equal(t, 5e-6, *model.InputPrice)
}

func TestPublicGroupSyncCategoryDefaultAliasInheritsTargetPrice(t *testing.T) {
	svc, store := publicSyncDefaultFixture(t)
	store.sources[0].Group.ModelAliases = []domain.ModelAlias{{Alias: "public-alias", UpstreamModel: "token-model"}}
	model := defaultSyncSnapshot(t, svc).ModelPricing["public-alias"]
	require.Equal(t, "public-alias", model.SourceModel)
	require.NotNil(t, model.InputPrice)
	require.Equal(t, 1e-6, *model.InputPrice)
}

type failedCategoryDefaultSyncRepo struct{ *categoryDefaultStoreFake }

func (f failedCategoryDefaultSyncRepo) Snapshot(context.Context, CategoryDefaultSnapshotRequest) (*CategoryDefaultCatalogSnapshot, error) {
	return nil, errors.New("database unavailable")
}

func TestPublicGroupSyncCategoryDefaultReadFailureAbortsSnapshot(t *testing.T) {
	svc, store := publicSyncDefaultFixture(t)
	svc.SetCategoryDefaults(NewCategoryDefaultGroupService(failedCategoryDefaultSyncRepo{store}, nil), NewModelPricingResolver(nil, nil))
	_, err := svc.Snapshot(context.Background())
	require.ErrorContains(t, err, "database unavailable")
}

func TestPublicGroupSyncResolvedModelUsesMatchingBaseTier(t *testing.T) {
	resolved := &ResolvedPricing{Mode: BillingModeToken, BasePricing: &ModelPricing{InputPricePerToken: 2e-6, OutputPricePerToken: 4e-6},
		Intervals: []PricingInterval{{MinTokens: 100000, InputPrice: ptrFloat64(9e-6)}, {MinTokens: 0, InputMultiplier: ptrFloat64(2), OutputMultiplier: ptrFloat64(3)}},
	}
	model := publicGroupSyncResolvedModel(PlatformGemini, "token-model", resolved)
	require.Equal(t, 4e-6, *model.InputPrice)
	require.Equal(t, 12e-6, *model.OutputPrice)
	fixed := publicGroupSyncResolvedModel(PlatformGemini, "request-model", &ResolvedPricing{Mode: BillingModePerRequest, DefaultPerRequestPrice: .2,
		RequestTiers: []PricingInterval{{MinTokens: 100000, PerRequestPrice: ptrFloat64(9)}, {MinTokens: 0, PerRequestPrice: ptrFloat64(.3)}},
	})
	require.Equal(t, .3, *fixed.PerRequestPrice)
}

func TestPublicGroupSyncResolvedModelPreservesImagePrices(t *testing.T) {
	model := publicGroupSyncResolvedModel(PlatformOpenAI, "multimodal", &ResolvedPricing{Mode: BillingModeToken, BasePricing: &ModelPricing{
		InputPricePerToken: 2e-6, OutputPricePerToken: 4e-6, ImageInputPricePerToken: 8e-6, ImageOutputPriceExplicit: true,
	}})
	require.NotNil(t, model.ImageInputPrice)
	require.Equal(t, 8e-6, *model.ImageInputPrice)
	require.NotNil(t, model.ImageOutputPrice)
	require.Zero(t, *model.ImageOutputPrice)
	fallback := publicGroupSyncResolvedModel(PlatformOpenAI, "ordinary", &ResolvedPricing{Mode: BillingModeToken, BasePricing: &ModelPricing{InputPricePerToken: 2e-6, OutputPricePerToken: 4e-6}})
	require.Nil(t, fallback.ImageInputPrice)
	require.Nil(t, fallback.ImageOutputPrice)
}

func TestPublicGroupSyncCategoryDefaultLoadsFullSourceChannelPricing(t *testing.T) {
	svc, store := publicSyncDefaultFixture(t)
	store.sources[0].Group.ModelPricing = nil
	store.sources[0].Channel = &Channel{ID: 42, Status: StatusActive, GroupIDs: []int64{1}}
	full := *store.sources[0].Channel
	full.ModelPricing = []ChannelModelPricing{{Platform: PlatformGemini, Models: []string{"token-model"}, BillingMode: BillingModeToken, InputPrice: ptrFloat64(9e-6), OutputPrice: ptrFloat64(18e-6)}}
	svc.channels = &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) { return []Channel{full}, nil }}
	price := defaultSyncSnapshot(t, svc).ModelPricing["token-model"]
	require.NotNil(t, price.InputPrice)
	require.Equal(t, 9e-6, *price.InputPrice)
	// Changing channel identity between enumeration and publication validation
	// must preserve the previous mirror rather than mixing two configurations.
	full.ID++
	_, err := svc.Snapshot(context.Background())
	require.ErrorContains(t, err, "changed during snapshot")
}
