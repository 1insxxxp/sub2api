package service

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInputProtectionOriginalBudget(t *testing.T) {
	p := &InputTokenProtection{Limit: 19}
	ctx := WithInputTokenProtection(context.Background(), p)
	p.Limit = 5000
	r, i, s, l := InputTokenProtectionFromContext(ctx).Cap(3295, 2061, math.MaxInt, math.MaxInt)
	require.Equal(t, [4]int{0, 0, 19, 0}, [4]int{r, i, s, l})
	r, i, s, l = InputTokenProtectionFromContext(ctx).Cap(2, 3, 4, 20)
	require.Equal(t, [4]int{0, 0, 4, 15}, [4]int{r, i, s, l})
}

func TestInputProtectionDivergenceBoundary(t *testing.T) {
	p := &InputTokenProtection{Limit: 20000}
	r, i, s, l := p.Cap(24950, 25, 15, 10)
	require.Equal(t, [4]int{24950, 25, 15, 10}, [4]int{r, i, s, l})
	r, i, s, l = p.Cap(24951, 25, 15, 10)
	require.Equal(t, [4]int{19950, 25, 15, 10}, [4]int{r, i, s, l})
	r, i, s, l = (&InputTokenProtection{Limit: 19999}).Cap(19950, 25, 15, 10)
	require.Equal(t, [4]int{19949, 25, 15, 10}, [4]int{r, i, s, l})
	r, i, s, l = (&InputTokenProtection{Limit: 10000}).Cap(3, 2, 1, 0)
	require.Equal(t, [4]int{3, 2, 1, 0}, [4]int{r, i, s, l})
	r, i, s, l = (&InputTokenProtection{Pending: true}).Cap(5051, 25, 15, 10)
	require.Equal(t, [4]int{5051, 25, 15, 10}, [4]int{r, i, s, l})
}

func TestInputProtectionBillingMode(t *testing.T) {
	id := int64(1700)
	s := &OpenAIGatewayService{resolver: newOpenAITextChannelPricingResolverForTest(t, id, "tavern-roleplay-pro", .05)}
	key := &APIKey{GroupID: &id, Group: &Group{ID: id}}
	require.False(t, s.InputTokenProtectionBillsByTokens(context.Background(), key, "tavern-roleplay-pro"))
	require.True(t, s.InputTokenProtectionBillsByTokens(context.Background(), key, "another-model"))
}

func TestInputProtectionWireTariff(t *testing.T) {
	const flat, metered = "public-flat", "account-metered"
	id := int64(23)
	r := newOpenAITextChannelPricingResolverForTest(t, id, flat, .05)
	cache, ok := r.channelService.cache.Load().(*channelCache)
	require.True(t, ok)
	price := 0.000001
	tokenPrice := ChannelModelPricing{Models: []string{metered}, BillingMode: BillingModeToken, InputPrice: &price, OutputPrice: &price}
	cache.channelByGroupID[id].ModelPricing = []ChannelModelPricing{tokenPrice}
	cache.pricingByGroupModel[channelModelKey{groupID: id, model: metered}] = &tokenPrice
	s := &OpenAIGatewayService{resolver: r, channelService: r.channelService}
	key := &APIKey{GroupID: &id, Group: &Group{ID: id}}
	ctx := context.Background()
	check := func(source, requested, actual string, image, want bool) {
		t.Helper()
		cache.channelByGroupID[id].BillingModelSource = source
		require.Equal(t, want, s.InputTokenProtectionWireBillsByTokens(ctx, key, requested, actual, image), "%s requested=%s actual=%s image=%v", source, requested, actual, image)
	}
	check(BillingModelSourceUpstream, flat, "", false, false)
	check(BillingModelSourceUpstream, flat, metered, false, true)
	check(BillingModelSourceUpstream, metered, flat, false, false)
	check(BillingModelSourceRequested, flat, metered, false, false)
	check(BillingModelSourceRequested, metered, flat, false, true)
	check(BillingModelSourceResponse, flat, flat, false, true)
	check(BillingModelSourceResponse, flat, flat, true, false)
	check(BillingModelSourceUpstream, flat, metered, true, false)
	check(BillingModelSourceUpstream, metered, flat, true, true)
	check(BillingModelSourceUpstream, "unconfigured", metered, true, true)
	check(BillingModelSourceUpstream, "unconfigured", "", true, false)
	cache.channelByGroupID[id].BillingModelSource = BillingModelSourceResponse
	for _, tc := range []struct {
		model          string
		conflict, want bool
	}{
		{flat, false, false}, {metered, false, true}, {metered, true, false}, {"unpriced", false, false},
	} {
		require.Equal(t, tc.want, s.InputTokenProtectionProjectsUsage(ctx, key, flat, InputTokenProtectionAttempt{UpstreamModel: flat, ResponseModel: tc.model, ResponseConflict: tc.conflict}, false))
	}
	cache.channelByGroupID[id].ModelPricing = nil
	delete(cache.pricingByGroupModel, channelModelKey{groupID: id, model: metered})
	check(BillingModelSourceResponse, flat, flat, false, false)
	key.Group.ModelPricing = []ChannelModelPricing{tokenPrice}
	check(BillingModelSourceResponse, flat, flat, false, true)
	require.False(t, s.InputTokenProtectionProjectsUsage(ctx, key, flat, InputTokenProtectionAttempt{UpstreamModel: metered, Passthrough: true, ResponseModel: flat}, false))
	key.Group.ImageCountBilling = true
	check(BillingModelSourceUpstream, flat, metered, true, false)
	check(BillingModelSourceUpstream, flat, metered, false, true)
}

func TestInputProtectionSystemCustomPricingSource(t *testing.T) {
	const source, alias = "source-flat", "display-alias"
	id := int64(72)
	r := newOpenAITextChannelPricingResolverForTest(t, id, source, .05)
	cache, ok := r.channelService.cache.Load().(*channelCache)
	require.True(t, ok)
	channel := cache.channelByGroupID[id]
	channel.BillingModelSource = BillingModelSourceRequested
	s := &OpenAIGatewayService{resolver: r, channelService: r.channelService, billingService: r.billingService}
	tokenPrice := 0.00001
	key := &APIKey{GroupID: &id, Group: &Group{ID: id, ModelPricing: []ChannelModelPricing{
		{Models: []string{alias, "account-metered"}, BillingMode: BillingModeToken, InputPrice: &tokenPrice, OutputPrice: &tokenPrice},
	}}}
	ctx := WithSystemCustomGroupResolution(context.Background(), SystemCustomGroupResolution{
		BillingGroupID: 10, SourceGroupID: id, PublicModel: alias, SourceModel: source, SourcePlatform: PlatformOpenAI,
	})
	require.False(t, s.InputTokenProtectionWireBillsByTokens(ctx, key, alias, "account-metered", false))
	sharedCtx, _ := WithGatewayTokenRequestPricing(ctx)
	require.False(t, s.InputTokenProtectionWireBillsByTokens(sharedCtx, key, alias, "account-metered", false))
	channel.BillingModelSource = BillingModelSourceUpstream
	require.True(t, s.InputTokenProtectionWireBillsByTokens(ctx, key, alias, "account-metered", false))
}
