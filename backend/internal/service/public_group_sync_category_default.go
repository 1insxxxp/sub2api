package service

import (
	"context"
	"fmt"
	"reflect"
	"sort"
)

// Public snapshots have no user identity. Only currently valid, unique
// routes to public sources may be advertised. Do not synthesize an admin user
// or query schedulable accounts: temporary cooling is not publication removal.
type publicGroupSyncCategoryRoute struct {
	CategoryDefaultModelRoute
	Source CategoryDefaultSourceSnapshot
}

func (s *CategoryDefaultGroupService) publicSyncRoutes(ctx context.Context, entryID int64) ([]publicGroupSyncCategoryRoute, error) {
	runtime, err := s.runtime(ctx, entryID)
	if err != nil {
		return nil, err
	}
	routes := make([]publicGroupSyncCategoryRoute, 0, len(runtime.config.Models))
	for _, current := range runtime.config.Models {
		group := runtime.groups[current.SourceGroupID]
		if group == nil || group.IsExclusive {
			continue
		}
		routes = append(routes, publicGroupSyncCategoryRoute{CategoryDefaultModelRoute: CategoryDefaultModelRoute{EntryGroupID: entryID, SourceGroup: group, PublicModel: current.PublicModel, SourceModel: current.SourceModel, GeminiNative: current.GeminiNative}, Source: runtime.sources[current.SourceGroupID]})
	}
	return routes, nil
}

func (s *PublicGroupSyncService) categoryDefaultSnapshot(ctx context.Context, group Group, catalog *channelCache) (PublicGroupSyncRequest, error) {
	if s.categoryDefaults == nil || s.pricingResolver == nil {
		return PublicGroupSyncRequest{}, fmt.Errorf("category default sync dependencies unavailable for group %d", group.ID)
	}
	routes, err := s.categoryDefaults.publicSyncRoutes(ctx, group.ID)
	if err != nil {
		// Fail the whole snapshot on a read error, preserving New's last good
		// catalog instead of publishing an accidental empty list.
		return PublicGroupSyncRequest{}, fmt.Errorf("category default group %d: %w", group.ID, err)
	}
	snapshot := PublicGroupSyncRequest{
		Version: PublicGroupSyncSnapshotVersion, Revision: group.UpdatedAt.UnixNano(), GroupID: group.ID,
		SortOrder: group.SortOrder, GroupName: group.Name, PublicEnabled: true, GroupRatio: 1,
		Models: []string{}, ModelMapping: map[string]string{}, ModelPricing: map[string]PublicGroupSyncModel{},
	}
	for _, route := range routes {
		source := route.SourceGroup
		// Publication validation intentionally loads only channel declarations.
		// Attach prices from this snapshot without mixing a changed mapping or
		// channel binding into the already validated publication.
		fullChannel := catalog.channelByGroupID[source.ID]
		declaredChannel := route.Source.Channel
		if (fullChannel == nil) != (declaredChannel == nil) ||
			(fullChannel != nil && (fullChannel.ID != declaredChannel.ID ||
				!publicGroupSyncMappingsEqual(fullChannel.ModelMapping, declaredChannel.ModelMapping))) {
			return PublicGroupSyncRequest{}, fmt.Errorf("category default source %d changed during snapshot; retry", source.ID)
		}
		route.Source.Channel = fullChannel
		resolved, platform, err := resolvePublicGroupSyncSourcePricing(ctx, s.pricingResolver, route.Source, route.SourceModel)
		if err != nil {
			return PublicGroupSyncRequest{}, fmt.Errorf("category default group %d source %d pricing: %w", group.ID, source.ID, err)
		}
		if resolved == nil || resolved.Mode == BillingModeVideo || !evaluateEffectiveResolvedPricing(resolved).usable {
			continue
		}
		price := publicGroupSyncResolvedModel(platform, route.PublicModel, resolved)
		rate := source.RateMultiplier
		if resolved.Mode == BillingModeImage {
			rate = resolveImageRateMultiplier(&APIKey{Group: source}, rate)
		}
		price.SourceGroupID = source.ID
		price.SourceModel = route.SourceModel
		price.SourceGroupRatio = &rate
		snapshot.Models = append(snapshot.Models, route.PublicModel)
		snapshot.ModelPricing[route.PublicModel] = price
	}
	sort.Strings(snapshot.Models)
	return snapshot, nil
}

func publicGroupSyncMappingsEqual(a, b map[string]map[string]string) bool {
	return (len(a) == 0 && len(b) == 0) || reflect.DeepEqual(a, b)
}

// Export the source's resolved base/first-tier price. The mirror applies the
// source rate per model; an entry's own price and rate never affect this data.
func publicGroupSyncResolvedModel(platform, name string, resolved *ResolvedPricing) PublicGroupSyncModel {
	price := PublicGroupSyncModel{Platform: platform, DisplayName: name, BillingMode: string(resolved.Mode)}
	if resolved.channelPricing != nil && resolved.channelPricing.Platform != "" {
		price.Platform = resolved.channelPricing.Platform
	}
	if resolved.Mode == BillingModePerRequest || resolved.Mode == BillingModeImage {
		unit := resolved.DefaultPerRequestPrice
		if tier := FindMatchingInterval(resolved.RequestTiers, 1); tier != nil && tier.PerRequestPrice != nil {
			unit = *tier.PerRequestPrice
		}
		price.PerRequestPrice = &unit
		return price
	}
	if base := (*ModelPricingResolver)(nil).GetIntervalPricing(resolved, 1); base != nil {
		input, output, cacheRead, cacheWrite := base.InputPricePerToken, base.OutputPricePerToken, base.CacheReadPricePerToken, base.CacheCreationPricePerToken
		price.InputPrice, price.OutputPrice = &input, &output
		price.CacheReadPrice, price.CacheWritePrice = &cacheRead, &cacheWrite
		if base.ImageInputPricePerToken != 0 {
			imageInput := base.ImageInputPricePerToken
			price.ImageInputPrice = &imageInput
		}
		if base.ImageOutputPriceExplicit || base.ImageOutputPricePerToken != 0 {
			imageOutput := base.ImageOutputPricePerToken
			price.ImageOutputPrice = &imageOutput
		}
	}
	return price
}
