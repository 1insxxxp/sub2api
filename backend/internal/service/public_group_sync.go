package service

import (
	"context"
	"sort"
	"strings"
	"time"
)

type PublicGroupSyncService struct {
	groups   GroupRepository
	channels ChannelRepository
	accounts AccountRepository
	pricing  *PricingService
}

func NewPublicGroupSyncService(groups GroupRepository, channels ChannelRepository, accounts AccountRepository, pricing *PricingService) *PublicGroupSyncService {
	return &PublicGroupSyncService{groups: groups, channels: channels, accounts: accounts, pricing: pricing}
}

func (s *PublicGroupSyncService) Snapshot(ctx context.Context) ([]PublicGroupSyncRequest, error) {
	groups, err := s.groups.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := s.channels.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	byGroup := make(map[int64][]Channel)
	groupPlatforms := make(map[int64]string, len(groups))
	for _, group := range groups {
		groupPlatforms[group.ID] = group.Platform
	}
	activeChannels := make([]Channel, 0, len(channels))
	for _, ch := range channels {
		if ch.Status != StatusActive {
			continue
		}
		activeChannels = append(activeChannels, ch)
		for _, id := range ch.GroupIDs {
			byGroup[id] = append(byGroup[id], ch)
		}
	}
	channelCatalog := populateChannelCache(activeChannels, groupPlatforms)
	out := make([]PublicGroupSyncRequest, 0, len(groups))
	for _, g := range groups {
		if g.IsExclusive {
			continue
		}
		// Channel pricing is shared by every group attached to that channel. Use
		// the group's schedulable account mappings to avoid publishing pricing
		// aliases that the group cannot actually route.
		availableModels, err := s.groupAvailableModels(ctx, g, byGroup[g.ID], channelCatalog)
		if err != nil {
			return nil, err
		}
		models := map[string]PublicGroupSyncModel{}
		for _, ch := range byGroup[g.ID] {
			for _, p := range ch.ModelPricing {
				if strings.TrimSpace(g.Platform) != "" && !isPlatformPricingMatch(g.Platform, p.Platform) {
					continue
				}
				if p.BillingMode == BillingModeVideo {
					continue
				}
				for _, name := range p.Models {
					if strings.ContainsAny(name, "*?") {
						continue
					}
					if availableModels != nil {
						if _, ok := availableModels[p.Platform][name]; !ok {
							continue
						}
					}
					key := p.Platform + "\x00" + name
					m := PublicGroupSyncModel{
						Platform:         p.Platform,
						DisplayName:      name,
						BillingMode:      string(p.BillingMode),
						InputPrice:       p.InputPrice,
						OutputPrice:      p.OutputPrice,
						ImageInputPrice:  p.ImageInputPrice,
						ImageOutputPrice: p.ImageOutputPrice,
						PerRequestPrice:  p.PerRequestPrice,
						CacheWritePrice:  p.CacheWritePrice,
						CacheReadPrice:   p.CacheReadPrice,
					}
					if mapped := ch.ModelMapping[p.Platform][name]; mapped != "" {
						m.UpstreamModel = mapped
					}
					models[key] = m
				}
			}
			// A channel may expose a model through mapping without an explicit
			// pricing row. Preserve that model in the mirror; New can then use
			// its own fallback catalog while keeping the exact display alias.
			for platform, mapping := range ch.ModelMapping {
				if strings.TrimSpace(g.Platform) != "" && !isPlatformPricingMatch(g.Platform, platform) {
					continue
				}
				for name, upstream := range mapping {
					if strings.ContainsAny(name, "*?") {
						continue
					}
					if availableModels != nil {
						if _, ok := availableModels[platform][name]; !ok {
							continue
						}
					}
					key := platform + "\x00" + name
					if _, exists := models[key]; exists {
						continue
					}
					m := PublicGroupSyncModel{Platform: platform, DisplayName: name, UpstreamModel: upstream, BillingMode: string(BillingModeToken)}
					models[key] = m
				}
			}
		}
		if g.AllowImageGeneration && s.accounts != nil {
			accounts, err := s.accounts.ListByGroup(ctx, g.ID)
			if err != nil {
				return nil, err
			}
			for _, account := range accounts {
				if strings.TrimSpace(g.Platform) != "" && !isPlatformPricingMatch(g.Platform, account.Platform) {
					continue
				}
				for name, upstream := range account.GetModelMapping() {
					if !isImageModelName(name) {
						continue
					}
					key := account.Platform + "\x00" + name
					if _, exists := models[key]; exists {
						continue
					}
					models[key] = PublicGroupSyncModel{
						Platform:        account.Platform,
						DisplayName:     name,
						UpstreamModel:   upstream,
						BillingMode:     string(BillingModeImage),
						PerRequestPrice: g.ImagePrice1K,
					}
				}
			}
		}
		// Channels provide prices, not the complete catalog: unbound groups and
		// account models absent from channel rows still belong in the snapshot.
		for platform, available := range availableModels {
			for name := range available {
				key := platform + "\x00" + name
				if _, exists := models[key]; exists {
					continue
				}
				pricing := matchGroupModelPricing(&g, name)
				if pricing == nil {
					pricing = lookupPricingAcrossPlatforms(channelCatalog, g.ID, platform, name)
				}
				// Bound channels define the published price catalog. Do not turn
				// an unpriced account model into a token model in a fixed-price group.
				if pricing == nil && len(byGroup[g.ID]) > 0 {
					continue
				}
				if pricing != nil && pricing.BillingMode == BillingModeVideo {
					continue
				}
				entry := []SupportedModel{{Platform: platform, Name: name, Pricing: pricing}}
				fillGlobalPricingFallback(s.pricing, entry)
				models[key] = publicGroupSyncModel(platform, name, entry[0].Pricing)
			}
		}
		list := make([]PublicGroupSyncModel, 0, len(models))
		for _, m := range models {
			list = append(list, m)
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].Platform != list[j].Platform {
				return list[i].Platform < list[j].Platform
			}
			return list[i].DisplayName < list[j].DisplayName
		})
		rev := g.UpdatedAt.UnixNano()
		if rev == 0 {
			rev = time.Unix(1, 0).UnixNano()
		}
		pricing := make(map[string]PublicGroupSyncModel, len(list))
		names := make([]string, 0, len(list))
		mappings := make(map[string]string)
		for _, m := range list {
			pricing[m.DisplayName] = m
			names = append(names, m.DisplayName)
			if m.UpstreamModel != "" {
				mappings[m.DisplayName] = m.UpstreamModel
			}
		}
		out = append(out, PublicGroupSyncRequest{Version: PublicGroupSyncSnapshotVersion, Revision: rev, GroupID: g.ID, SortOrder: g.SortOrder, GroupName: g.Name, PublicEnabled: true, GroupRatio: g.RateMultiplier, Models: names, ModelMapping: mappings, ModelPricing: pricing})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GroupID < out[j].GroupID })
	return out, nil
}

func (s *PublicGroupSyncService) groupAvailableModels(ctx context.Context, group Group, channels []Channel, catalog *channelCache) (map[string]map[string]struct{}, error) {
	if s.accounts == nil {
		return nil, nil
	}
	accounts, err := s.accounts.ListSchedulableByGroupID(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	byPlatform := make(map[string][]Account)
	for _, account := range accounts {
		if group.Platform == "" || isPlatformPricingMatch(group.Platform, account.Platform) {
			byPlatform[account.Platform] = append(byPlatform[account.Platform], account)
		}
	}
	// A non-nil empty catalog means no schedulable accounts, never unrestricted.
	available := make(map[string]map[string]struct{})
	for platform, platformAccounts := range byPlatform {
		models := availableModelsForAccounts(platformAccounts, platform)
		if len(models) == 0 && !IsMultiProtocolAPIKeyProvider(platform) {
			models = DefaultModelIDsForPlatform(platform)
		}
		// Concrete channel entries expand wildcard account mappings without
		// treating an empty group as unrestricted.
		for _, channel := range channels {
			for _, pricing := range channel.ModelPricing {
				if pricing.Platform == platform {
					models = append(models, pricing.Models...)
				}
			}
			for name := range channel.ModelMapping[platform] {
				models = append(models, name)
			}
		}
		models = group.EffectiveModelAllowlist().FilterForListing(models)
		available[platform] = make(map[string]struct{})
		for _, name := range models {
			if strings.TrimSpace(name) != "" && !strings.ContainsAny(name, "*?") && publicGroupSyncModelRoutable(catalog, group.ID, platform, name, platformAccounts) {
				available[platform][name] = struct{}{}
			}
		}
	}
	return available, nil
}

func publicGroupSyncModelRoutable(catalog *channelCache, groupID int64, platform, name string, accounts []Account) bool {
	mapped := lookupMappingAcrossPlatforms(catalog, groupID, platform, strings.ToLower(name))
	if mapped == "" {
		mapped = name
	}
	channel := catalog.channelByGroupID[groupID]
	accountModel := name
	if platform == PlatformOpenAI || platform == PlatformGrok || IsMultiProtocolAPIKeyProvider(platform) {
		accountModel = mapped
	}
	for i := range accounts {
		account := &accounts[i]
		if shouldHideUnavailableProviderModel(account, name) || !isModelSupportedByAccount(account, accountModel) {
			continue
		}
		if channel == nil || !channel.RestrictModels {
			return true
		}
		billingModel := billingModelForRestriction(channel.BillingModelSource, name, mapped)
		if billingModel == "" {
			billingModel = resolveAccountUpstreamModel(account, accountModel)
		}
		if lookupPricingAcrossPlatforms(catalog, groupID, platform, billingModel) != nil {
			return true
		}
	}
	return false
}

func publicGroupSyncModel(platform, name string, pricing *ChannelModelPricing) PublicGroupSyncModel {
	model := PublicGroupSyncModel{Platform: platform, DisplayName: name, BillingMode: string(BillingModeToken)}
	if pricing == nil {
		return model
	}
	if pricing.BillingMode != "" {
		model.BillingMode = string(pricing.BillingMode)
	}
	model.InputPrice = pricing.InputPrice
	model.OutputPrice = pricing.OutputPrice
	model.CacheWritePrice = pricing.CacheWritePrice
	model.CacheReadPrice = pricing.CacheReadPrice
	model.PerRequestPrice = pricing.PerRequestPrice
	model.ImageInputPrice = pricing.ImageInputPrice
	model.ImageOutputPrice = pricing.ImageOutputPrice
	return model
}

func isImageModelName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(name, "image") || strings.Contains(name, "imagen")
}
