package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestGroupAliasBillingUsesTargetUnlessExplicitlyPriced(t *testing.T) {
	group := &Group{ModelAliases: []domain.ModelAlias{{Alias: "claude-sonnet-long", UpstreamModel: "claude-opus-4-6"}}}
	require.Equal(t, "claude-opus-4-6", groupAliasBillingModel(group, "claude-sonnet-long", nil))
	require.Equal(t, "claude-sonnet-long", groupAliasBillingModel(group, "claude-sonnet-long", func(string) bool { return true }))
	require.Equal(t, "original-model", groupAliasBillingModel(group, "original-model", nil))
}

func TestGroupAliasPricingResolverUsesTargetAndAllowsExplicitAliasPrice(t *testing.T) {
	inputPrice := 15e-6
	aliasPrice := 20e-6
	group := &Group{
		ModelAliases: []domain.ModelAlias{{Alias: "claude-sonnet-long", UpstreamModel: "claude-opus-4-6"}},
		ModelPricing: []ChannelModelPricing{{Models: []string{"claude-opus-4-6"}, InputPrice: &inputPrice}},
	}
	resolver := NewModelPricingResolver(nil, nil)
	resolved := resolver.Resolve(context.Background(), PricingInput{Model: "claude-sonnet-long", Group: group})
	require.NotNil(t, resolved.BasePricing)
	require.Equal(t, inputPrice, resolved.BasePricing.InputPricePerToken)
	group.ModelPricing = append(group.ModelPricing, ChannelModelPricing{Models: []string{"claude-sonnet-long"}, InputPrice: &aliasPrice})
	resolved = resolver.Resolve(context.Background(), PricingInput{Model: "claude-sonnet-long", Group: group})
	require.NotNil(t, resolved.BasePricing)
	require.Equal(t, aliasPrice, resolved.BasePricing.InputPricePerToken)
}

func TestGroupAliasPricingResolverChannelOverrideKeepsTargetBasePrice(t *testing.T) {
	groupID := int64(77)
	aliasInput := 20e-6
	group := &Group{ID: groupID, Platform: PlatformAnthropic,
		ModelAliases: []domain.ModelAlias{{Alias: "claude-sonnet-long", UpstreamModel: "target-model"}}}
	channel := Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{groupID},
		ModelPricing: []ChannelModelPricing{{Platform: PlatformAnthropic, Models: []string{"claude-sonnet-long"}, InputPrice: &aliasInput}}}
	channels := &ChannelService{}
	channels.cache.Store(populateChannelCache([]Channel{channel}, map[int64]string{groupID: PlatformAnthropic}))
	billing := NewBillingService(&config.Config{}, &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"target-model": {InputCostPerToken: 15e-6, OutputCostPerToken: 75e-6},
	}})
	resolver := NewModelPricingResolver(channels, billing)
	resolved := resolver.Resolve(context.Background(), PricingInput{Model: "claude-sonnet-long", Group: group, GroupID: &groupID})
	require.Equal(t, PricingSourceChannel, resolved.Source)
	require.Equal(t, aliasInput, resolved.BasePricing.InputPricePerToken)
	require.Equal(t, 75e-6, resolved.BasePricing.OutputPricePerToken)
}
