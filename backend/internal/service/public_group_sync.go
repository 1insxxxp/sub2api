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
}

func NewPublicGroupSyncService(groups GroupRepository, channels ChannelRepository, accounts ...AccountRepository) *PublicGroupSyncService {
	var accountRepo AccountRepository
	if len(accounts) > 0 {
		accountRepo = accounts[0]
	}
	return &PublicGroupSyncService{groups: groups, channels: channels, accounts: accountRepo}
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
	for _, ch := range channels {
		if ch.Status != StatusActive {
			continue
		}
		for _, id := range ch.GroupIDs {
			byGroup[id] = append(byGroup[id], ch)
		}
	}
	out := make([]PublicGroupSyncRequest, 0, len(groups))
	for _, g := range groups {
		if g.IsExclusive {
			continue
		}
		// Channel pricing is shared by every group attached to that channel. Use
		// the group's schedulable account mappings to avoid publishing pricing
		// aliases that the group cannot actually route.
		availableModels := map[string]map[string]struct{}{}
		if s.accounts != nil {
			accounts, err := s.accounts.ListSchedulableByGroupID(ctx, g.ID)
			if err != nil {
				return nil, err
			}
			for _, account := range accounts {
				if strings.TrimSpace(g.Platform) != "" && !isPlatformPricingMatch(g.Platform, account.Platform) {
					continue
				}
				mapping := account.GetModelMapping()
				if len(mapping) == 0 {
					continue
				}
				if availableModels[account.Platform] == nil {
					availableModels[account.Platform] = map[string]struct{}{}
				}
				for name := range mapping {
					if !strings.ContainsAny(name, "*?") {
						availableModels[account.Platform][name] = struct{}{}
					}
				}
			}
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
					if allowed := availableModels[p.Platform]; len(allowed) > 0 {
						if _, ok := allowed[name]; !ok {
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
					if allowed := availableModels[platform]; len(allowed) > 0 {
						if _, ok := allowed[name]; !ok {
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

func isImageModelName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(name, "image") || strings.Contains(name, "imagen")
}
