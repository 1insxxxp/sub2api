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
}

func NewPublicGroupSyncService(groups GroupRepository, channels ChannelRepository) *PublicGroupSyncService {
	return &PublicGroupSyncService{groups: groups, channels: channels}
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
		models := map[string]PublicGroupSyncModel{}
		for _, ch := range byGroup[g.ID] {
			for _, p := range ch.ModelPricing {
				if p.BillingMode == BillingModeImage || p.BillingMode == BillingModeVideo {
					continue
				}
				for _, name := range p.Models {
					if strings.ContainsAny(name, "*?") {
						continue
					}
					key := p.Platform + "\x00" + name
					m := PublicGroupSyncModel{Platform: p.Platform, DisplayName: name, BillingMode: string(p.BillingMode), InputPrice: p.InputPrice, OutputPrice: p.OutputPrice, PerRequestPrice: p.PerRequestPrice, CacheWritePrice: p.CacheWritePrice, CacheReadPrice: p.CacheReadPrice}
					applyPublicGroupRate(&m, g.RateMultiplier)
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
				for name, upstream := range mapping {
					if strings.ContainsAny(name, "*?") {
						continue
					}
					key := platform + "\x00" + name
					if _, exists := models[key]; exists {
						continue
					}
					m := PublicGroupSyncModel{Platform: platform, DisplayName: name, UpstreamModel: upstream, BillingMode: string(BillingModeToken)}
					applyPublicGroupRate(&m, g.RateMultiplier)
					models[key] = m
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
		out = append(out, PublicGroupSyncRequest{Version: PublicGroupSyncSnapshotVersion, Revision: rev, GroupID: g.ID, GroupName: g.Name, PublicEnabled: true, GroupRatio: g.RateMultiplier, Models: names, ModelMapping: mappings, ModelPricing: pricing})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GroupID < out[j].GroupID })
	return out, nil
}

func applyPublicGroupRate(m *PublicGroupSyncModel, multiplier float64) {
	if m == nil || multiplier <= 0 || multiplier == 1 {
		return
	}
	scale := func(value *float64) *float64 {
		if value == nil {
			return nil
		}
		v := *value * multiplier
		return &v
	}
	m.InputPrice = scale(m.InputPrice)
	m.OutputPrice = scale(m.OutputPrice)
	m.PerRequestPrice = scale(m.PerRequestPrice)
	m.CacheWritePrice = scale(m.CacheWritePrice)
	m.CacheReadPrice = scale(m.CacheReadPrice)
}
