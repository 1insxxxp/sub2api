package service

import (
	"context"
	"fmt"
)

// ordinaryDisplayPricing projects the group's resolved base prices independently
// of the historical relay catalog. A group override must apply even when the
// model was already enumerated from channel pricing. Reuse the settlement
// projection so aliases and channel/account billing names follow the same rules
// as category-default quotes, without scheduling requests or changing billing.
func (s *PublicGroupSyncService) ordinaryDisplayPricing(ctx context.Context, group *Group, models []string, catalog *channelCache) (map[string]PublicGroupSyncModel, error) {
	if s.pricingResolver == nil || len(models) == 0 {
		return nil, nil
	}
	// Ordinary composite groups need their explicit route declarations to quote
	// safely. Category defaults already supply them through their separate path.
	if group.Platform == PlatformComposite {
		return nil, nil
	}
	source := CategoryDefaultSourceSnapshot{Group: group, Channel: catalog.channelByGroupID[group.ID]}
	if s.accounts != nil {
		accounts, err := s.accounts.ListSchedulableByGroupID(ctx, group.ID)
		if err != nil {
			return nil, fmt.Errorf("public group %d display accounts: %w", group.ID, err)
		}
		source.Accounts = accounts
	}
	quotes := make(map[string]PublicGroupSyncModel, len(models))
	for _, name := range models {
		resolved, platform, err := resolvePublicGroupSyncSourcePricing(ctx, s.pricingResolver, source, name)
		if err != nil {
			return nil, fmt.Errorf("public group %d display pricing: %w", group.ID, err)
		}
		if resolved == nil || resolved.Mode == BillingModeVideo || !evaluateEffectiveResolvedPricing(resolved).usable {
			continue
		}
		quote := publicGroupSyncResolvedModel(platform, name, resolved)
		rate := group.RateMultiplier
		if resolved.Mode == BillingModeImage {
			rate = resolveImageRateMultiplier(&APIKey{Group: group}, rate)
			// The compact quote advertises the 1K image tier. A model-level
			// group fixed price wins; otherwise group image prices override
			// channel defaults, matching the existing image settlement path.
			// Use the same pure calculation as settlement: a zero size tier
			// falls through the context tier and then the configured default.
			cost, err := (&BillingService{}).calculatePerRequestCost(resolved, CostInput{
				Resolver: s.pricingResolver, RequestCount: 1, SizeTier: "1K", RateMultiplier: 1,
			})
			if err != nil {
				return nil, fmt.Errorf("public group %d image display pricing: %w", group.ID, err)
			}
			unit := cost.TotalCost
			if resolved.Source != PricingSourceGroup && group.ImagePrice1K != nil {
				unit = *group.ImagePrice1K
			}
			quote.PerRequestPrice = &unit
		}
		quote.SourceGroupID, quote.SourceModel, quote.SourceGroupRatio = group.ID, name, &rate
		quotes[name] = quote
	}
	return quotes, nil
}
