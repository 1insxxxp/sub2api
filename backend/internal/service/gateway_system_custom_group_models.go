package service

import (
	"context"
	"fmt"
	"strings"
)

// ListSystemCustomGroupModelAvailability evaluates a complete system-custom
// route snapshot against one batched schedulable-account snapshot. Model
// support is delegated to the gateway's real scheduler predicates so wildcard,
// provider normalization, casing, Bedrock, and passthrough behavior cannot
// drift from request routing.
func (s *GatewayService) ListSystemCustomGroupModelAvailability(
	ctx context.Context,
	sources []SystemCustomGroupModelListSource,
) (SystemCustomGroupModelAvailability, error) {
	availability := make(SystemCustomGroupModelAvailability, len(sources))
	if len(sources) == 0 {
		return availability, nil
	}
	if s == nil || s.accountRepo == nil {
		return nil, fmt.Errorf("system custom group model catalog is not configured")
	}
	batchRepo, ok := s.accountRepo.(systemCustomGroupSchedulableAccountRepository)
	if !ok {
		return nil, fmt.Errorf("account repository does not support system custom group snapshots")
	}

	groupIDs := make([]int64, 0, len(sources))
	seenGroupIDs := make(map[int64]struct{}, len(sources))
	for i := range sources {
		groupID := sources[i].Group.ID
		if groupID <= 0 {
			continue
		}
		if _, exists := seenGroupIDs[groupID]; exists {
			continue
		}
		seenGroupIDs[groupID] = struct{}{}
		groupIDs = append(groupIDs, groupID)
	}
	accounts, err := batchRepo.ListSchedulableByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("list system custom group schedulable accounts: %w", err)
	}
	accountsByGroup := make(map[int64][]Account, len(groupIDs))
	for i := range accounts {
		entry := accounts[i]
		accountsByGroup[entry.GroupID] = append(accountsByGroup[entry.GroupID], entry.Account)
	}

	for i := range sources {
		source := &sources[i]
		group := &source.Group
		if !isDirectSystemCustomSource(group) || !IsGroupContextValid(group) {
			continue
		}
		groupAccounts := systemCustomAccountsForPlatform(accountsByGroup[group.ID], group.Platform)
		if len(groupAccounts) == 0 {
			continue
		}
		available := make(map[string]bool, len(source.Models))
		availability[group.ID] = available
		visibleMappings, mappedAccounts := systemCustomVisibleMappings(groupAccounts)
		for _, sourceModel := range source.Models {
			sourceModel = strings.TrimSpace(sourceModel)
			if sourceModel == "" || available[sourceModel] {
				continue
			}
			candidates := groupAccounts
			if group.CustomModelsListEnabled() {
				visible := ResolveCustomModelsList(group.Platform, visibleMappings, group.ModelsListConfig.Models)
				if !containsModelFold(visible, sourceModel) {
					continue
				}
			} else if len(visibleMappings) == 0 {
				if !containsModelFold(DefaultModelIDsForPlatform(group.Platform), sourceModel) {
					continue
				}
			} else {
				// GetAvailableModels exposes mapping-backed catalogs whenever at
				// least one visible mapping exists. Empty-mapping accounts must
				// therefore not broaden that catalog, even though they remain
				// valid request candidates for a custom-list/default model.
				candidates = mappedAccounts
			}
			for j := range candidates {
				account := &candidates[j]
				if shouldHideUnavailableProviderModel(account, sourceModel) ||
					!account.IsSchedulableForModelWithContext(ctx, sourceModel) ||
					!s.isModelSupportedByAccountWithContext(ctx, account, sourceModel) {
					continue
				}
				available[sourceModel] = true
				break
			}
		}
	}
	return availability, nil
}

func systemCustomAccountsForPlatform(accounts []Account, platform string) []Account {
	filtered := make([]Account, 0, len(accounts))
	for i := range accounts {
		if accounts[i].Platform == platform {
			filtered = append(filtered, accounts[i])
		}
	}
	return filtered
}

func systemCustomVisibleMappings(accounts []Account) ([]string, []Account) {
	seen := make(map[string]struct{})
	models := make([]string, 0)
	mappedAccounts := make([]Account, 0, len(accounts))
	for i := range accounts {
		account := &accounts[i]
		mapping := account.GetModelMapping()
		if len(mapping) == 0 {
			continue
		}
		mappedAccounts = append(mappedAccounts, *account)
		for model := range mapping {
			model = strings.TrimSpace(model)
			if model == "" || shouldHideUnavailableProviderModel(account, model) {
				continue
			}
			if _, exists := seen[model]; exists {
				continue
			}
			seen[model] = struct{}{}
			models = append(models, model)
		}
	}
	return models, mappedAccounts
}

func containsModelFold(models []string, target string) bool {
	for _, model := range models {
		if strings.EqualFold(strings.TrimSpace(model), target) {
			return true
		}
	}
	return false
}
