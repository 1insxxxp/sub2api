package service

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

// resolvePublicGroupSyncSourcePricing projects settlement pricing without choosing
// a schedulable account. source.Channel must be the complete channel from this
// snapshot (including billing source and prices), not the declaration-only row.
// No database/cache refresh or upstream request is performed. A nil price means
// unpriced or ambiguous; operational failures are returned to abort the snapshot.
func resolvePublicGroupSyncSourcePricing(ctx context.Context, resolver *ModelPricingResolver, source CategoryDefaultSourceSnapshot, model string) (*ResolvedPricing, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if resolver == nil || source.Group == nil {
		return nil, "", fmt.Errorf("public group source pricing dependencies unavailable")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, "", nil
	}
	source = categoryDefaultFreezeGrokDeclarations(source)
	// The declaration repository may include disabled routes. Never let one win
	// matchCompositeRoute's priority ordering over an enabled route.
	routes := make([]CompositeModelRoute, 0, len(source.Routes))
	for _, route := range source.Routes {
		if route.Enabled {
			routes = append(routes, route)
		}
	}
	source.Routes = routes
	channels := []Channel{}
	if source.Channel != nil && source.Channel.IsActive() {
		channel := *source.Channel
		if channel.BillingModelSource == BillingModelSourceResponse {
			return nil, "", nil // The upstream response is not a static declaration.
		}
		channel.GroupIDs = []int64{source.Group.ID}
		channels = append(channels, channel)
	}
	cache := populateChannelCache(channels, map[int64]string{source.Group.ID: source.Group.Platform})
	channelService := &ChannelService{}
	channelService.cache.Store(cache)
	localResolver := NewModelPricingResolver(channelService, resolver.billingService)
	key := &APIKey{GroupID: &source.Group.ID, Group: source.Group}
	openAI := &OpenAIGatewayService{resolver: localResolver, billingService: resolver.billingService}
	gateway := &GatewayService{resolver: localResolver, billingService: resolver.billingService}
	target, aliased := source.Group.ResolveModelAlias(model)
	paths := categoryDefaultDeclarationPaths(source, model, target, aliased)
	var selected *ResolvedPricing
	var platform string
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		pathCtx := WithResolvedTargetPlatform(ctx, path.Platform)
		mapping := channelService.ResolveChannelMapping(pathCtx, source.Group.ID, path.Model)
		// These protocol-specific dispatch fallbacks depend on request shape. Do
		// not represent them as a single ordinary-model price without that shape.
		if (source.Group.AllowMessagesDispatch || path.Platform == PlatformGrok) && claudeMessagesDispatchFamily(path.Model) != "" {
			return nil, "", nil
		}
		accounts := []*Account{}
		for i := range source.Accounts {
			account := &source.Accounts[i]
			if !categoryDefaultPathAllowsAccount(source.Group, path, account) {
				continue
			}
			if len(account.GetModelMapping()) > 0 && !account.IsOpenAIPassthroughEnabled() {
				if _, matched := account.ResolveMappedModel(mapping.MappedModel); !matched {
					continue
				}
			}
			accounts = append(accounts, account)
		}
		// Publication survives temporary lack of account capacity. A declaration
		// whose routed name itself is priced can still have an unambiguous price.
		if len(accounts) == 0 {
			accounts = append(accounts, nil)
		}
		for _, account := range accounts {
			billingModel, upstreamModel := mapping.MappedModel, mapping.MappedModel
			usesOpenAI := domain.UsesOpenAIGateway(path.Platform)
			if account != nil {
				usesOpenAI = domain.UsesOpenAIGateway(account.Platform)
				if usesOpenAI {
					billingModel, upstreamModel = resolveOpenAIForwardMappedModels(account, mapping.MappedModel, false)
				} else {
					upstreamModel = resolveAccountUpstreamModel(account, mapping.MappedModel)
				}
			}
			fields := mapping.ToUsageFields(model, upstreamModel)
			var candidates []string
			if usesOpenAI {
				// Reuse settlement's billing-source ordering, explicit configured
				// price precedence, group aliases, and CN-provider candidate guard.
				candidates = openAI.openAIRecordUsageBillingModels(pathCtx, &OpenAIRecordUsageInput{
					APIKey: key, Account: account, ChannelUsageFields: fields,
					Result: &OpenAIForwardResult{Model: mapping.MappedModel, BillingModel: billingModel, UpstreamModel: upstreamModel},
				})
			} else {
				// Generic ForwardResult.Model is the forwarded request name before
				// account mapping. Preserve that existing settlement behavior even
				// for upstream billing; only an unpriced name falls back upstream.
				primary := mapping.MappedModel
				if fields.BillingModelSource == BillingModelSourceRequested {
					primary = model
				}
				if fields.BillingModelSource == BillingModelSourceChannelMapped {
					primary = fields.ChannelMappedModel
				}
				if source.Group.Platform == PlatformComposite {
					primary = gateway.compositeBillableModel(pathCtx, key, primary, mapping.MappedModel)
				}
				primary = groupAliasBillingModel(source.Group, primary, func(alias string) bool {
					return localResolver.hasConfiguredModelPricing(pathCtx, PricingInput{Model: alias, GroupID: key.GroupID, Group: source.Group})
				})
				primary = gateway.billableModelWithFallback(pathCtx, key, primary, upstreamModel, mapping.MappedModel)
				candidates = []string{primary}
			}
			var price *ResolvedPricing
			for _, candidate := range candidates {
				resolved := localResolver.Resolve(pathCtx, PricingInput{Model: candidate, GroupID: key.GroupID, Group: source.Group})
				if evaluateEffectiveResolvedPricing(resolved).usable {
					price = resolved
					break
				}
			}
			if price == nil {
				return nil, "", nil
			}
			if selected != nil && (platform != path.Platform || !publicGroupSyncSourcePricesEqual(selected, price)) {
				return nil, "", nil
			}
			selected, platform = price, path.Platform
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	return selected, platform, nil
}

// Compare billing data, excluding row/name/source metadata. Checking all tiers
// and modifiers is deliberately conservative: equal first-tier prices alone do
// not make account-dependent prices interchangeable.
func publicGroupSyncSourcePricesEqual(a, b *ResolvedPricing) bool {
	normalize := func(in *ResolvedPricing) ResolvedPricing {
		out := *in
		out.Source, out.BaseSource = "", ""
		cleanIntervals := func(in []PricingInterval) []PricingInterval {
			if len(in) == 0 {
				return nil
			}
			out := append([]PricingInterval(nil), in...)
			for i := range out {
				out[i].ID, out[i].PricingID = 0, 0
				out[i].CreatedAt, out[i].UpdatedAt = time.Time{}, time.Time{}
			}
			return out
		}
		out.Intervals = cleanIntervals(in.Intervals)
		out.RequestTiers = cleanIntervals(in.RequestTiers)
		if in.channelPricing != nil {
			pricing := in.channelPricing.Clone()
			pricing.ID, pricing.ChannelID = 0, 0
			pricing.Models, pricing.Platform = nil, ""
			pricing.CreatedAt, pricing.UpdatedAt = time.Time{}, time.Time{}
			pricing.Intervals = cleanIntervals(pricing.Intervals)
			out.channelPricing = &pricing
		}
		return out
	}
	return reflect.DeepEqual(normalize(a), normalize(b))
}
