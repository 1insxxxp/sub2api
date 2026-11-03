package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func categoryDefaultPublishSource(t *testing.T, source CategoryDefaultSourceSnapshot, category string) (*CategoryDefaultGroupService, *categoryDefaultStoreFake) {
	t.Helper()
	store := &categoryDefaultStoreFake{sources: []CategoryDefaultSourceSnapshot{source}}
	svc := NewCategoryDefaultGroupService(store, nil)
	preview, err := svc.Preview(context.Background(), category, []int64{source.Group.ID})
	require.NoError(t, err)
	_, err = svc.Apply(context.Background(), category, CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{source.Group.ID}, PreviewHash: preview.PreviewHash})
	require.NoError(t, err)
	return svc, store
}

func categoryDefaultChannelSource() CategoryDefaultSourceSnapshot {
	source := categoryDefaultSource(1, "openai", map[string]any{"public-model": "gpt-5", "channel-target": "gpt-5-mini"})
	source.Group.Platform = PlatformOpenAI
	source.Group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"public-model"}}
	source.Accounts[0].Platform = PlatformOpenAI
	source.Channel = &Channel{ID: 2, Status: StatusActive, GroupIDs: []int64{1}, ModelMapping: map[string]map[string]string{PlatformOpenAI: {"public-model": "channel-target", "unrelated": "anything"}}}
	return source
}

func TestCategoryDefaultChannelEffectiveDeclarationRequiresRepublish(t *testing.T) {
	ctx := context.Background()
	for _, change := range []string{"channel", "account", "compact"} {
		t.Run(change, func(t *testing.T) {
			source := categoryDefaultChannelSource()
			source.Accounts[0].Credentials["compact_model_mapping"] = map[string]any{"channel-target": "gpt-5-nano"}
			svc, store := categoryDefaultPublishSource(t, source, "openai")
			user := &User{ID: 2, Status: StatusActive}
			// Prices and unrelated public names must stay live without syncing.
			store.sources[0].Channel.ModelMapping[PlatformOpenAI]["unrelated"] = "changed"
			store.sources[0].Channel.ModelPricing = []ChannelModelPricing{{InputPrice: ptrFloat64(3)}}
			_, err := svc.ResolveForUser(ctx, 100, user, "public-model")
			require.NoError(t, err)
			switch change {
			case "channel":
				store.sources[0].Channel.ModelMapping[PlatformOpenAI]["public-model"] = "public-model"
			case "account":
				mapping, ok := store.sources[0].Accounts[0].Credentials["model_mapping"].(map[string]any)
				require.True(t, ok)
				mapping["channel-target"] = "gpt-5-nano"
			case "compact":
				mapping, ok := store.sources[0].Accounts[0].Credentials["compact_model_mapping"].(map[string]any)
				require.True(t, ok)
				mapping["channel-target"] = "gpt-5-mini"
			}
			_, err = svc.ResolveForUser(ctx, 100, user, "public-model")
			require.ErrorIs(t, err, ErrCategoryDefaultPendingSync)
		})
	}
}

func TestCategoryDefaultCompositeChannelUsesResolvedPlatformAndRouteModel(t *testing.T) {
	source := categoryDefaultChannelSource()
	source.Group.Platform = PlatformComposite
	source.Group.KeyDisplayCategory = "openai"
	source.Routes = []CompositeModelRoute{{PublicModel: "public-model", MatchType: CompositeRouteMatchExact, TargetPlatform: PlatformOpenAI, UpstreamModel: "route-target", Endpoint: CompositeRouteEndpointResponses, Enabled: true}}
	source.Channel.ModelMapping = map[string]map[string]string{PlatformOpenAI: {"route-target": "channel-target"}, PlatformGemini: {"route-target": "unrelated-gemini-target"}}
	svc, store := categoryDefaultPublishSource(t, source, "openai")
	user := &User{ID: 2, Status: StatusActive}
	store.sources[0].Channel.ModelMapping[PlatformGemini]["route-target"] = "changed-unrelated-gemini"
	_, err := svc.ResolveForUser(context.Background(), 100, user, "public-model")
	require.NoError(t, err)
	mapping, ok := store.sources[0].Accounts[0].Credentials["model_mapping"].(map[string]any)
	require.True(t, ok)
	mapping["channel-target"] = "gpt-5-nano"
	_, err = svc.ResolveForUser(context.Background(), 100, user, "public-model")
	require.ErrorIs(t, err, ErrCategoryDefaultPendingSync)
}

func TestCategoryDefaultChannelNativeCapabilityUsesMappedTarget(t *testing.T) {
	source := categoryDefaultSource(1, "ag", map[string]any{"public-model": "claude-sonnet-4-5", "channel-target": "gemini-2.5-flash"})
	source.Group.Platform = PlatformAntigravity
	source.Group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"public-model"}}
	source.Accounts[0].Platform = PlatformAntigravity
	source.Channel = &Channel{ID: 2, Status: StatusActive, GroupIDs: []int64{1}, ModelMapping: map[string]map[string]string{PlatformAntigravity: {"public-model": "channel-target"}}}
	svc, _ := categoryDefaultPublishSource(t, source, "other")
	models, err := svc.ModelsForUser(context.Background(), 100, &User{ID: 2, Status: StatusActive}, PlatformGemini)
	require.NoError(t, err)
	require.Equal(t, []string{"public-model"}, models)
}

func TestCategoryDefaultCompositeNativeCatalogUsesActualEndpointAndCase(t *testing.T) {
	for _, routes := range [][]CompositeModelRoute{
		{{PublicModel: "Foo", MatchType: CompositeRouteMatchExact, TargetPlatform: PlatformGemini, UpstreamModel: "gemini-2.5-flash", Endpoint: CompositeRouteEndpointResponses, Enabled: true}},
		{
			{PublicModel: "Foo", MatchType: CompositeRouteMatchExact, TargetPlatform: PlatformAntigravity, UpstreamModel: "claude-sonnet-4-5", Endpoint: CompositeRouteEndpointGemini, Enabled: true},
			{PublicModel: "foo", MatchType: CompositeRouteMatchExact, TargetPlatform: PlatformGemini, UpstreamModel: "gemini-2.5-flash", Endpoint: CompositeRouteEndpointGemini, Enabled: true},
		},
	} {
		source := CategoryDefaultSourceSnapshot{Group: &Group{ID: 1, Platform: PlatformComposite, KeyDisplayCategory: "gemini", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}, Routes: routes}
		models, err := categoryDefaultSourceModels(source)
		require.NoError(t, err)
		require.Len(t, models, 1)
		require.False(t, models[0].GeminiNative, "only the exact source call name and a native endpoint route can declare native support")
	}
}

func TestCategoryDefaultMessagesDispatchTracksOnlyEffectiveFallback(t *testing.T) {
	for _, accountMapping := range []map[string]any{{}, {"claude-sonnet-4-5": "gpt-5"}} {
		source := categoryDefaultSource(1, "openai", accountMapping)
		source.Group.Platform = PlatformOpenAI
		source.Group.AllowMessagesDispatch = true
		source.Group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"claude-sonnet-4-5"}}
		source.Group.MessagesDispatchModelConfig.SonnetMappedModel = "gpt-5-mini"
		source.Accounts[0].Platform = PlatformOpenAI
		svc, store := categoryDefaultPublishSource(t, source, "openai")
		user := &User{ID: 2, Status: StatusActive}
		store.sources[0].Group.MessagesDispatchModelConfig.HaikuMappedModel = "unrelated"
		_, err := svc.ResolveForUser(context.Background(), 100, user, "claude-sonnet-4-5")
		require.NoError(t, err)
		store.sources[0].Group.MessagesDispatchModelConfig.SonnetMappedModel = "gpt-5-nano"
		_, err = svc.ResolveForUser(context.Background(), 100, user, "claude-sonnet-4-5")
		if len(accountMapping) == 0 {
			require.ErrorIs(t, err, ErrCategoryDefaultPendingSync)
		} else {
			require.NoError(t, err, "the exact account mapping takes precedence over the unused dispatch fallback")
		}
	}
}
