package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type ImageStudioModelAccounts interface {
	ListModelAvailabilityCandidates(context.Context, *int64, []string, bool) ([]Account, error)
}

func (s *ImageStudioService) SetModelDiscovery(accounts ImageStudioModelAccounts, channels *ChannelService) {
	s.modelAccounts = accounts
	s.modelChannels = channels
}

func (s *ImageStudioService) discoverImageModels(ctx context.Context, group *Group, cfg *ImageStudioSettings) ([]string, error) {
	if s.modelAccounts == nil {
		return imageStudioModelsForGroup(group, cfg), nil
	}
	models := []string{}
	if group == nil || (group.Platform != PlatformOpenAI && group.Platform != PlatformGemini && group.Platform != PlatformGrok) {
		return models, nil
	}
	// Use persistent eligibility so temporary upstream limits do not make the
	// model selector flicker. An empty account pool must stay empty.
	accounts, err := s.modelAccounts.ListModelAvailabilityCandidates(ctx, &group.ID, []string{group.Platform}, false)
	if err != nil {
		return nil, fmt.Errorf("discover image models: %w", err)
	}
	allowlist := group.EffectiveModelAllowlist()
	candidates := append([]string(nil), DefaultModelIDsForPlatform(group.Platform)...)
	candidates = append(candidates, allowlist.Models...)
	if cfg != nil {
		candidates = append(candidates, cfg.AllowedModels...)
	}
	if s.modelChannels != nil && group.Platform != PlatformGemini {
		channel, err := s.modelChannels.GetChannelForGroup(ctx, group.ID)
		if err != nil {
			return nil, fmt.Errorf("discover image channel: %w", err)
		}
		if channel != nil {
			for model := range channel.ModelMapping[group.Platform] {
				candidates = append(candidates, model)
			}
		}
	}
	for i := range accounts {
		if accounts[i].Platform != group.Platform {
			continue
		}
		for model := range accounts[i].GetModelMapping() {
			candidates = append(candidates, model)
		}
	}
	candidates = dedupeImageStudioModelsForGroup(group, candidates)
	for _, model := range candidates {
		if strings.Contains(model, "*") || !allowlist.Allows(model) {
			continue
		}
		if s.modelChannels != nil && s.modelChannels.IsModelRestricted(ctx, group.ID, model) {
			continue
		}
		requestModel := model
		if s.modelChannels != nil && group.Platform != PlatformGemini {
			mapping, _ := s.modelChannels.ResolveChannelMappingAndRestrict(ctx, &group.ID, model)
			requestModel = mapping.MappedModel
		}
		if !imageStudioModelSupportedByGroup(group, requestModel) {
			continue
		}
		for i := range accounts {
			account := &accounts[i]
			if account.Platform != group.Platform || !account.IsModelSupported(model) || shouldHideUnavailableProviderModel(account, model) {
				continue
			}
			// Public image-like aliases mapped to chat models cannot be sent to
			// the existing image executor, even if their names look compatible.
			if !imageStudioModelSupportedByGroup(group, account.GetMappedModel(requestModel)) {
				continue
			}
			models = append(models, model)
			break
		}
	}
	sort.Strings(models)
	return models, nil
}

func (s *ImageStudioService) resolveImageStudioKeyGroup(ctx context.Context, userID int64, keyID, groupID *int64) (*int64, error) {
	provider, ok := s.groupResolver.(interface {
		GetImageStudioAPIKeyByID(context.Context, int64, int64) (*APIKey, error)
	})
	if !ok || keyID == nil {
		return groupID, nil
	}
	key, err := provider.GetImageStudioAPIKeyByID(ctx, userID, *keyID)
	if err != nil {
		return nil, err
	}
	if key == nil || key.GroupID == nil || key.IsExpired() {
		return nil, ErrAPIKeyNotFound
	}
	if groupID != nil && *groupID != *key.GroupID {
		return nil, ErrGroupNotAllowed
	}
	return key.GroupID, nil
}
