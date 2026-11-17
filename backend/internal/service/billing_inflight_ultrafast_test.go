//go:build unit

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

// Deliberately use an operator price above the fallback catalog. Mapping tests
// must retain this requested-model price, not accidentally price the target.
// Keep all tier comparisons below the separate 1-credit estimated-hold ceiling.
const inflightUltrafastStandardCost = 0.03 // 1000 input * .00001 + 1000 output * .00002

type inflightUltrafastEstimator interface {
	EstimateInflightReservation(context.Context, *APIKey, InflightEstimateRequest) (float64, bool)
}

func newInflightUltrafastGroup(model string) *Group {
	input, output := .00001, .00002
	return &Group{
		ID: 7101, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true,
		SubscriptionType: SubscriptionTypeStandard, RateMultiplier: 1,
		ModelPricing: []ChannelModelPricing{{
			Models: []string{model}, BillingMode: BillingModeToken,
			InputPrice: &input, OutputPrice: &output,
		}},
	}
}

func inflightUltrafastRequest(model, tier string) InflightEstimateRequest {
	// Tokenization is covered separately. No input field means the existing
	// BodyBytes fallback supplies exactly 1000 tokens; Body carries tier metadata.
	body := []byte(fmt.Sprintf(`{"model":%q}`, model))
	if tier != "" {
		body = []byte(fmt.Sprintf(`{"model":%q,"service_tier":%q}`, model, tier))
	}
	return InflightEstimateRequest{Model: model, BodyBytes: 4000, Body: body, MaxTokens: 1000, Kind: InflightEstimateToken}
}

func inflightUltrafastContext(key *APIKey) context.Context {
	ctx := context.WithValue(context.Background(), ctxkey.APIKey, key)
	ctx = context.WithValue(ctx, ctxkey.UserID, key.UserID)
	return context.WithValue(ctx, ctxkey.Group, key.Group)
}

func newInflightUltrafastEstimators(t *testing.T, group *Group, channel *Channel, policy *OpenAIFastPolicySettings, accounts []Account) map[string]inflightUltrafastEstimator {
	t.Helper()
	var channels *ChannelService
	if channel != nil {
		channels = &ChannelService{}
		channels.cache.Store(populateChannelCache([]Channel{*channel}, map[int64]string{group.ID: group.Platform}))
	}
	gateway := newInflightEstimateGateway(t, channels)
	openAI := newOpenAIGatewayServiceWithSettings(t, policy)
	gateway.settingService = openAI.settingService
	openAI.cfg = gateway.cfg
	openAI.billingService = gateway.billingService
	openAI.resolver = gateway.resolver
	openAI.channelService = channels
	if accounts == nil {
		accounts = []Account{
			{ID: 7111, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true},
			{ID: 7112, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true},
		}
	}
	snapshot := &inflightSnapshotCacheStub{byBucket: map[string][]Account{
		inflightBucketKey(group.ID, PlatformOpenAI): accounts,
	}}
	attachInflightSnapshot(gateway, snapshot)
	openAI.schedulerSnapshot = gateway.schedulerSnapshot
	return map[string]inflightUltrafastEstimator{"gateway": gateway, "openai": openAI}
}

func newInflightUltrafastKey(group *Group) *APIKey {
	return &APIKey{ID: 7102, UserID: 42, User: &User{ID: 42, Status: StatusActive}, GroupID: &group.ID, Group: group}
}

func TestInflightUltrafast_ForcedSupportedModelsReserveSixTimes(t *testing.T) {
	for _, model := range []string{"gpt-6.1-sol", "gpt-6-astra"} {
		for _, tier := range []string{"", "default", "flex", "priority", "ultrafast"} {
			t.Run(model+"/client="+tier, func(t *testing.T) {
				group := newInflightUltrafastGroup(model)
				group.ForceOpenAIUltrafast, group.ForceOpenAIFast = true, true
				group.RateMultiplier = 1.25
				key := newInflightUltrafastKey(group)
				for name, estimator := range newInflightUltrafastEstimators(t, group, nil, nil, nil) {
					t.Run(name, func(t *testing.T) {
						estimate, priced := estimator.EstimateInflightReservation(inflightUltrafastContext(key), key, inflightUltrafastRequest(model, tier))
						require.True(t, priced)
						require.InDelta(t, inflightUltrafastStandardCost*6*1.25, estimate, 1e-10)
					})
				}
			})
		}
	}
}

func TestInflightUltrafast_FreeFastWaivesOnlyPriority(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		tier                  string
		ultrafast, fast, free bool
		want                  float64
	}{
		{name: "forced_ultrafast", ultrafast: true, free: true, want: 6},
		{name: "client_ultrafast", tier: "ultrafast", free: true, want: 6},
		{name: "forced_priority_free", fast: true, free: true, want: 1},
		{name: "client_priority_free", tier: "priority", free: true, want: 1},
		{name: "client_fast_alias_free", tier: "fast", free: true, want: 1},
		{name: "forced_priority_operator_multiplier", fast: true, want: 7},
		{name: "ultrafast_ignores_operator_fast_multiplier", ultrafast: true, want: 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := newInflightUltrafastGroup("gpt-6.1-sol")
			group.ForceOpenAIUltrafast, group.ForceOpenAIFast, group.FreeOpenAIFast = tc.ultrafast, tc.fast, tc.free
			fast := 7.0
			group.ModelPricing[0].FastMultiplier = &fast
			key := newInflightUltrafastKey(group)
			for name, estimator := range newInflightUltrafastEstimators(t, group, nil, nil, nil) {
				t.Run(name, func(t *testing.T) {
					estimate, priced := estimator.EstimateInflightReservation(inflightUltrafastContext(key), key, inflightUltrafastRequest("gpt-6.1-sol", tc.tier))
					require.True(t, priced)
					require.InDelta(t, inflightUltrafastStandardCost*tc.want, estimate, 1e-10)
				})
			}
		})
	}
}

func TestInflightUltrafast_UnsupportedModelKeepsFastFallback(t *testing.T) {
	for _, tc := range []struct {
		name string
		fast bool
		tier string
		want float64
	}{
		{name: "ultrafast_only_preserves_standard", want: 1},
		{name: "both_flags_fall_back_to_priority", fast: true, want: 2},
		{name: "explicit_ultrafast_retains_generic_multiplier", tier: "ultrafast", want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := newInflightUltrafastGroup("gpt-6.1-terra")
			group.ForceOpenAIUltrafast, group.ForceOpenAIFast = true, tc.fast
			key := newInflightUltrafastKey(group)
			for name, estimator := range newInflightUltrafastEstimators(t, group, nil, nil, nil) {
				t.Run(name, func(t *testing.T) {
					estimate, priced := estimator.EstimateInflightReservation(inflightUltrafastContext(key), key, inflightUltrafastRequest("gpt-6.1-terra", tc.tier))
					require.True(t, priced)
					require.InDelta(t, inflightUltrafastStandardCost*tc.want, estimate, 1e-10)
				})
			}
		})
	}
}

func TestInflightUltrafast_RequiresTrustedGroupContext(t *testing.T) {
	group := newInflightUltrafastGroup("gpt-6.1-sol")
	group.ForceOpenAIUltrafast = true
	key := newInflightUltrafastKey(group)
	unhydrated := *group
	unhydrated.Hydrated = false
	for _, tc := range []struct {
		name string
		ctx  context.Context
		want float64
	}{
		{name: "missing_group", ctx: context.Background(), want: 1},
		{name: "unhydrated_group", ctx: context.WithValue(context.Background(), ctxkey.Group, &unhydrated), want: 1},
		{name: "authenticated_group", ctx: inflightUltrafastContext(key), want: 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for name, estimator := range newInflightUltrafastEstimators(t, group, nil, nil, nil) {
				t.Run(name, func(t *testing.T) {
					estimate, priced := estimator.EstimateInflightReservation(tc.ctx, key, inflightUltrafastRequest("gpt-6.1-sol", ""))
					require.True(t, priced)
					require.InDelta(t, inflightUltrafastStandardCost*tc.want, estimate, 1e-10)
				})
			}
		})
	}
}

func TestInflightUltrafast_CategoryDefaultUsesResolvedSource(t *testing.T) {
	for _, sourceForces := range []bool{false, true} {
		t.Run(fmt.Sprintf("source_forces=%t", sourceForces), func(t *testing.T) {
			source := newInflightUltrafastGroup("gpt-6.1-sol")
			source.ForceOpenAIUltrafast = sourceForces
			source.RateMultiplier = 1.5
			entryID := int64(7200)
			entry := newInflightUltrafastKey(&Group{ID: entryID, Platform: PlatformComposite,
				Status: StatusActive, Hydrated: true, IsCategoryDefault: true,
				ForceOpenAIUltrafast: !sourceForces, RateMultiplier: 99})
			keys := &APIKeyService{
				userRepo:              categoryAPIKeyUserRepo{user: entry.User},
				categoryDefaultGroups: &categoryAPIKeyResolverFake{source: source},
			}
			resolution, err := keys.ResolveCategoryDefaultGroupModel(inflightUltrafastContext(entry), entry, "gpt-6.1-sol")
			require.NoError(t, err)
			require.NotNil(t, resolution)
			key := resolution.APIKey
			require.Equal(t, entryID, *key.CategoryDefaultGroupID)
			want := inflightUltrafastStandardCost * source.RateMultiplier
			if sourceForces {
				want *= 6
			}
			for name, estimator := range newInflightUltrafastEstimators(t, key.Group, nil, nil, nil) {
				t.Run(name, func(t *testing.T) {
					estimate, priced := estimator.EstimateInflightReservation(inflightUltrafastContext(key), key, inflightUltrafastRequest(resolution.SourceModel, ""))
					require.True(t, priced)
					require.InDelta(t, want, estimate, 1e-10, "source flags/rate, not the default entry's flags/rate, determine the hold")
				})
			}
		})
	}
}

func TestInflightUltrafast_RequestedPriceAliasUsesChannelTarget(t *testing.T) {
	group := newInflightUltrafastGroup("sol-public")
	group.ForceOpenAIUltrafast = true
	key := newInflightUltrafastKey(group)
	channel := &Channel{ID: 7103, Status: StatusActive, GroupIDs: []int64{group.ID},
		BillingModelSource: BillingModelSourceRequested,
		ModelMapping:       map[string]map[string]string{PlatformOpenAI: {"sol-public": "gpt-6.1-sol"}},
	}
	for name, estimator := range newInflightUltrafastEstimators(t, group, channel, nil, nil) {
		t.Run(name, func(t *testing.T) {
			estimate, priced := estimator.EstimateInflightReservation(inflightUltrafastContext(key), key, inflightUltrafastRequest("sol-public", ""))
			require.True(t, priced)
			require.InDelta(t, inflightUltrafastStandardCost*6, estimate, 1e-10, "keep the requested price while deriving the tier from the sent model")
		})
	}
}

func TestInflightUltrafast_GroupAliasUsesTargetTier(t *testing.T) {
	group := newInflightUltrafastGroup("sol-public")
	group.ForceOpenAIUltrafast = true
	group.ModelAliases = []domain.ModelAlias{{Alias: "sol-public", UpstreamModel: "gpt-6.1-sol"}}
	key := newInflightUltrafastKey(group)
	for name, estimator := range newInflightUltrafastEstimators(t, group, nil, nil, nil) {
		t.Run(name, func(t *testing.T) {
			estimate, priced := estimator.EstimateInflightReservation(inflightUltrafastContext(key), key, inflightUltrafastRequest("sol-public", ""))
			require.True(t, priced)
			require.InDelta(t, inflightUltrafastStandardCost*6, estimate, 1e-10)
		})
	}
}

func TestInflightUltrafast_PricedAliasChecksEveryAccountMapping(t *testing.T) {
	for _, targets := range [][]string{{"gpt-6.1-terra", "gpt-6.1-sol"}, {"gpt-6.1-sol", "gpt-6.1-terra"}} {
		t.Run("first="+targets[0], func(t *testing.T) {
			group := newInflightUltrafastGroup("sol-public")
			group.ForceOpenAIUltrafast = true
			key := newInflightUltrafastKey(group)
			channel := &Channel{ID: 7103, Status: StatusActive, GroupIDs: []int64{group.ID},
				BillingModelSource: BillingModelSourceRequested,
				ModelMapping:       map[string]map[string]string{PlatformOpenAI: {"sol-public": "provider-alias"}},
			}
			accounts := make([]Account, 0, len(targets))
			for i, target := range targets {
				accounts = append(accounts, Account{ID: int64(7111 + i), Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
					Status: StatusActive, Schedulable: true,
					Credentials: map[string]any{"model_mapping": map[string]any{"provider-alias": target}},
				})
			}
			for name, estimator := range newInflightUltrafastEstimators(t, group, channel, nil, accounts) {
				t.Run(name, func(t *testing.T) {
					estimate, priced := estimator.EstimateInflightReservation(inflightUltrafastContext(key), key, inflightUltrafastRequest("sol-public", ""))
					require.True(t, priced)
					require.InDelta(t, inflightUltrafastStandardCost*6, estimate, 1e-10,
						"even an already-priced alias must cover the Sol account before account selection/failover")
				})
			}
		})
	}
}

func TestInflightUltrafast_GlobalPolicyCoversAccountScopes(t *testing.T) {
	for _, tc := range []struct {
		name, tier, matchTier, scope, action string
		forced, free                         bool
		want                                 float64
	}{
		{name: "filter_forced_ultrafast", forced: true, matchTier: "ultrafast", scope: BetaPolicyScopeAll, action: BetaPolicyActionFilter, want: 1},
		{name: "force_priority_after_forced_ultrafast", forced: true, matchTier: "ultrafast", scope: BetaPolicyScopeAll, action: OpenAIFastPolicyActionForcePriority, want: 2},
		{name: "downgraded_priority_is_free", forced: true, free: true, matchTier: "ultrafast", scope: BetaPolicyScopeAll, action: OpenAIFastPolicyActionForcePriority, want: 1},
		{name: "oauth_filter_leaves_apikey_ultrafast", forced: true, matchTier: "ultrafast", scope: BetaPolicyScopeOAuth, action: BetaPolicyActionFilter, want: 6},
		{name: "apikey_filter_leaves_oauth_ultrafast", forced: true, matchTier: "ultrafast", scope: BetaPolicyScopeAPIKey, action: BetaPolicyActionFilter, want: 6},
		{name: "oauth_block_leaves_apikey_ultrafast", forced: true, matchTier: "ultrafast", scope: BetaPolicyScopeOAuth, action: BetaPolicyActionBlock, want: 6},
		{name: "apikey_block_leaves_oauth_ultrafast", forced: true, matchTier: "ultrafast", scope: BetaPolicyScopeAPIKey, action: BetaPolicyActionBlock, want: 6},
		{name: "missing_tier_upgrades_to_priority", matchTier: OpenAIFastTierMissing, scope: BetaPolicyScopeAll, action: OpenAIFastPolicyActionForcePriority, want: 2},
		{name: "oauth_priority_upgrade_beats_apikey_flex", tier: "flex", matchTier: "flex", scope: BetaPolicyScopeOAuth, action: OpenAIFastPolicyActionForcePriority, want: 2},
		{name: "filtered_flex_is_standard", tier: "flex", matchTier: "flex", scope: BetaPolicyScopeAll, action: BetaPolicyActionFilter, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := newInflightUltrafastGroup("gpt-6.1-sol")
			group.ForceOpenAIUltrafast, group.FreeOpenAIFast = tc.forced, tc.free
			key := newInflightUltrafastKey(group)
			policy := &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{
				ServiceTier: tc.matchTier, Scope: tc.scope, Action: tc.action,
			}}}
			for name, estimator := range newInflightUltrafastEstimators(t, group, nil, policy, nil) {
				for _, prefetched := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/prefetched=%t", name, prefetched), func(t *testing.T) {
						ctx := inflightUltrafastContext(key)
						if prefetched {
							ctx = withOpenAIFastPolicyContext(ctx, policy)
						}
						estimate, priced := estimator.EstimateInflightReservation(ctx, key, inflightUltrafastRequest("gpt-6.1-sol", tc.tier))
						require.True(t, priced)
						require.InDelta(t, inflightUltrafastStandardCost*tc.want, estimate, 1e-10,
							"reserve the most expensive allowed scope, not an arbitrarily assumed account type")
					})
				}
			}
		})
	}
}

func TestInflightUltrafast_PerRequestPriceIsUnchanged(t *testing.T) {
	group := newInflightUltrafastGroup("gpt-6.1-sol")
	group.ForceOpenAIUltrafast = true
	group.RateMultiplier = 1.5
	price := .2 // Keep this pricing assertion below the estimated-hold ceiling.
	group.ModelPricing = []ChannelModelPricing{{Models: []string{"gpt-6.1-sol"}, BillingMode: BillingModePerRequest, PerRequestPrice: &price}}
	key := newInflightUltrafastKey(group)
	request := inflightUltrafastRequest("gpt-6.1-sol", "ultrafast")
	request.Units = 2
	for name, estimator := range newInflightUltrafastEstimators(t, group, nil, nil, nil) {
		t.Run(name, func(t *testing.T) {
			estimate, priced := estimator.EstimateInflightReservation(inflightUltrafastContext(key), key, request)
			require.True(t, priced)
			require.InDelta(t, price*2*1.5, estimate, 1e-10, "tier premiums must not multiply fixed per-request prices")
		})
	}
}

func TestInflightUltrafast_InsufficientBalanceRejectedBeforeReservation(t *testing.T) {
	group := newInflightUltrafastGroup("gpt-6.1-sol")
	group.ForceOpenAIUltrafast = true
	key := newInflightUltrafastKey(group)
	ctx := inflightUltrafastContext(key)
	for name, estimator := range newInflightUltrafastEstimators(t, group, nil, nil, nil) {
		t.Run(name, func(t *testing.T) {
			balance := inflightUltrafastStandardCost + .1
			cache := newMemInflightCache(balance)
			billing := newInflightSvc(t, cache, 60)
			billing.userRepo = &inflightRecheckUserRepo{user: &User{ID: key.UserID, Status: StatusActive, Balance: balance}}
			estimate, priced := estimator.EstimateInflightReservation(ctx, key, inflightUltrafastRequest("gpt-6.1-sol", ""))
			require.True(t, priced)
			reservation, err := billing.ReserveInflight(ctx, key.User, group, nil, estimate)
			if reservation != nil {
				t.Cleanup(reservation.HandlerDone)
			}
			require.ErrorIs(t, err, ErrInsufficientBalance, "enough for Standard, insufficient for forced Ultrafast")
			require.Nil(t, reservation)
			require.Zero(t, cache.count(), "a rejected precharge must not leave a hold")
		})
	}
}

func TestInflightUltrafast_FlexReservationHasStandardFloor(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		name := "ordinary_flex"
		policy := DefaultOpenAIFastPolicySettings()
		if blocked {
			name = "all_scopes_block_original_flex"
			policy.Rules = []OpenAIFastPolicyRule{{
				ServiceTier: OpenAIFastTierFlex, Scope: BetaPolicyScopeAll, Action: BetaPolicyActionBlock,
			}}
		}
		t.Run(name, func(t *testing.T) {
			group := newInflightUltrafastGroup("gpt-6.1-sol")
			key := newInflightUltrafastKey(group)
			ctx := withOpenAIFastPolicyContext(inflightUltrafastContext(key), policy)
			// The protocol is not fixed during precharge. Messages conversion can
			// discard the original Flex tier, leaving an allowed Standard request
			// even when every account scope blocks the original Flex request.
			request := inflightUltrafastRequest("gpt-6.1-sol", OpenAIFastTierFlex)
			for estimatorName, estimator := range newInflightUltrafastEstimators(t, group, nil, policy, nil) {
				t.Run(estimatorName, func(t *testing.T) {
					estimate, priced := estimator.EstimateInflightReservation(ctx, key, request)
					require.True(t, priced)
					require.InDelta(t, inflightUltrafastStandardCost, estimate, 1e-10,
						"before the actual protocol is known, neither Flex discounts nor blocked tier options may reduce the Standard hold")
				})
			}
		})
	}
}

func TestInflightUltrafast_MissingTierPolicyCoversProtocolConversion(t *testing.T) {
	for _, action := range []string{BetaPolicyActionPass, BetaPolicyActionBlock} {
		t.Run("original_flex="+action, func(t *testing.T) {
			group := newInflightUltrafastGroup("gpt-6.1-sol")
			key := newInflightUltrafastKey(group)
			policy := &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{
				{ServiceTier: OpenAIFastTierFlex, Scope: BetaPolicyScopeAll, Action: action},
				{ServiceTier: OpenAIFastTierMissing, Scope: BetaPolicyScopeAll, Action: OpenAIFastPolicyActionForcePriority},
			}}
			ctx := withOpenAIFastPolicyContext(inflightUltrafastContext(key), policy)
			request := inflightUltrafastRequest("gpt-6.1-sol", OpenAIFastTierFlex)
			for name, estimator := range newInflightUltrafastEstimators(t, group, nil, policy, nil) {
				t.Run(name, func(t *testing.T) {
					estimate, priced := estimator.EstimateInflightReservation(ctx, key, request)
					require.True(t, priced)
					require.InDelta(t, inflightUltrafastStandardCost*2, estimate, 1e-10,
						"conversion may drop Flex and activate missing-tier Priority; a Standard floor alone is insufficient")
				})
			}
		})
	}
}
