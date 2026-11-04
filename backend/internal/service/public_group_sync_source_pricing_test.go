//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func publicSyncSourcePricingFixture(platform string) (CategoryDefaultSourceSnapshot, *ModelPricingResolver) {
	source := categoryDefaultSource(71, "source", map[string]any{"public-model": "target-a", "channel-model": "target-b"})
	source.Group.Platform = platform
	source.Accounts[0].Platform = platform
	source.Accounts[0].Type = AccountTypeAPIKey
	billing := NewBillingService(&config.Config{}, &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"target-a": {InputCostPerToken: 2e-6, OutputCostPerToken: 6e-6},
		"target-b": {InputCostPerToken: 4e-6, OutputCostPerToken: 12e-6},
	}})
	return source, NewModelPricingResolver(nil, billing)
}

func publicSyncSourcePrice(platform, model string, price float64) ChannelModelPricing {
	return ChannelModelPricing{Platform: platform, Models: []string{model}, BillingMode: BillingModeToken, InputPrice: ptrFloat64(price), OutputPrice: ptrFloat64(price * 3)}
}

func TestPublicGroupSyncSourcePricingAccountMapping(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformGemini, PlatformAnthropic} {
		t.Run(platform, func(t *testing.T) {
			source, resolver := publicSyncSourcePricingFixture(platform)
			resolved, gotPlatform, err := resolvePublicGroupSyncSourcePricing(context.Background(), resolver, source, "public-model")
			require.NoError(t, err)
			require.NotNil(t, resolved)
			require.NotNil(t, resolved.BasePricing)
			require.Equal(t, 2e-6, resolved.BasePricing.InputPricePerToken)
			require.Equal(t, platform, gotPlatform)
		})
	}
}

func TestPublicGroupSyncSourcePricingBillingSource(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformGemini} {
		for _, tc := range []struct {
			basis string
			want  float64
		}{
			{BillingModelSourceRequested, 7e-6}, {BillingModelSourceChannelMapped, 8e-6}, {"", 8e-6}, {BillingModelSourceUpstream, 9e-6},
		} {
			t.Run(platform+"/"+tc.basis, func(t *testing.T) {
				source, resolver := publicSyncSourcePricingFixture(platform)
				source.Channel = &Channel{ID: 3, Status: StatusActive, BillingModelSource: tc.basis,
					ModelMapping: map[string]map[string]string{platform: {"public-model": "channel-model"}},
					ModelPricing: []ChannelModelPricing{publicSyncSourcePrice(platform, "public-model", 7e-6), publicSyncSourcePrice(platform, "channel-model", 8e-6), publicSyncSourcePrice(platform, "target-b", 9e-6)},
				}
				resolved, _, err := resolvePublicGroupSyncSourcePricing(context.Background(), resolver, source, "public-model")
				require.NoError(t, err)
				require.NotNil(t, resolved)
				require.NotNil(t, resolved.BasePricing)
				want := tc.want
				if platform == PlatformGemini && tc.basis == BillingModelSourceUpstream {
					// Generic ForwardResult.Model retains the channel-mapped
					// request name; settlement falls back upstream only if unpriced.
					want = 8e-6
				}
				require.Equal(t, want, resolved.BasePricing.InputPricePerToken)
			})
		}
	}
}

func TestPublicGroupSyncSourcePricingOpenAIConfiguredRequestedBeforeUpstreamFallback(t *testing.T) {
	source, resolver := publicSyncSourcePricingFixture(PlatformOpenAI)
	source.Channel = &Channel{ID: 3, Status: StatusActive, BillingModelSource: BillingModelSourceUpstream}
	source.Group.ModelPricing = []ChannelModelPricing{publicSyncSourcePrice(PlatformOpenAI, "public-model", 7e-6)}
	resolved, _, err := resolvePublicGroupSyncSourcePricing(context.Background(), resolver, source, "public-model")
	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.Equal(t, 7e-6, resolved.BasePricing.InputPricePerToken)
}

func TestPublicGroupSyncSourcePricingGroupAlias(t *testing.T) {
	source, resolver := publicSyncSourcePricingFixture(PlatformGemini)
	source.Group.ModelAliases = []domain.ModelAlias{{Alias: "alias", UpstreamModel: "public-model"}}
	source.Group.ModelPricing = []ChannelModelPricing{publicSyncSourcePrice(PlatformGemini, "public-model", 7e-6)}
	source.Channel = &Channel{ID: 3, Status: StatusActive, BillingModelSource: BillingModelSourceRequested}
	for _, explicit := range []bool{false, true} {
		if explicit {
			source.Group.ModelPricing = append(source.Group.ModelPricing, publicSyncSourcePrice(PlatformGemini, "alias", 11e-6))
		}
		resolved, _, err := resolvePublicGroupSyncSourcePricing(context.Background(), resolver, source, "alias")
		require.NoError(t, err)
		require.NotNil(t, resolved)
		want := 7e-6
		if explicit {
			want = 11e-6
		}
		require.Equal(t, want, resolved.BasePricing.InputPricePerToken)
	}
}

func TestPublicGroupSyncSourcePricingAccountAmbiguity(t *testing.T) {
	for _, samePrice := range []bool{false, true} {
		source, resolver := publicSyncSourcePricingFixture(PlatformOpenAI)
		other := source.Accounts[0]
		other.ID++
		other.Credentials = map[string]any{"model_mapping": map[string]any{"public-model": "target-b"}}
		source.Accounts = append(source.Accounts, other)
		if samePrice {
			source.Group.ModelPricing = []ChannelModelPricing{publicSyncSourcePrice(PlatformOpenAI, "target-a", 5e-6), publicSyncSourcePrice(PlatformOpenAI, "target-b", 5e-6)}
		}
		resolved, _, err := resolvePublicGroupSyncSourcePricing(context.Background(), resolver, source, "public-model")
		require.NoError(t, err)
		if samePrice {
			require.NotNil(t, resolved)
			require.Equal(t, 5e-6, resolved.BasePricing.InputPricePerToken)
		} else {
			require.Nil(t, resolved)
		}
	}
}

func TestPublicGroupSyncSourcePricingComposite(t *testing.T) {
	source, resolver := publicSyncSourcePricingFixture(PlatformOpenAI)
	source.Group.Platform = PlatformComposite
	source.Routes = []CompositeModelRoute{{PublicModel: "entry", UpstreamModel: "public-model", TargetPlatform: PlatformOpenAI, MatchType: CompositeRouteMatchExact, Endpoint: CompositeRouteEndpointAny, Enabled: true}}
	resolved, platform, err := resolvePublicGroupSyncSourcePricing(context.Background(), resolver, source, "entry")
	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.Equal(t, PlatformOpenAI, platform)
	require.Equal(t, 2e-6, resolved.BasePricing.InputPricePerToken)
	source.Routes = append(source.Routes, CompositeModelRoute{PublicModel: "entry", UpstreamModel: "channel-model", TargetPlatform: PlatformOpenAI, MatchType: CompositeRouteMatchExact, Endpoint: CompositeRouteEndpointChatCompletions, Enabled: true})
	resolved, _, err = resolvePublicGroupSyncSourcePricing(context.Background(), resolver, source, "entry")
	require.NoError(t, err)
	require.Nil(t, resolved, "endpoint-dependent prices must not choose an arbitrary route")
}

func TestPublicGroupSyncSourcePricingUnknownResponseAndCancelled(t *testing.T) {
	source, resolver := publicSyncSourcePricingFixture(PlatformOpenAI)
	resolved, _, err := resolvePublicGroupSyncSourcePricing(context.Background(), resolver, source, "unknown-model")
	require.NoError(t, err)
	require.Nil(t, resolved)
	source.Channel = &Channel{ID: 3, Status: StatusActive, BillingModelSource: BillingModelSourceResponse}
	resolved, _, err = resolvePublicGroupSyncSourcePricing(context.Background(), resolver, source, "public-model")
	require.NoError(t, err)
	require.Nil(t, resolved, "response-dependent pricing cannot be promised by a static snapshot")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = resolvePublicGroupSyncSourcePricing(ctx, resolver, source, "public-model")
	require.ErrorIs(t, err, context.Canceled)
}
