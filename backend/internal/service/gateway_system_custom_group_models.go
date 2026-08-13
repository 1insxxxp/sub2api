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
		for _, sourceModel := range source.Models {
			sourceModel = strings.TrimSpace(sourceModel)
			if sourceModel == "" || available[sourceModel] {
				continue
			}
			// A system route is already an explicit administrator-owned model
			// enumeration. The source custom list remains an additional display
			// whitelist when enabled, but account mapping catalogs must not replace
			// the actual scheduler candidate set.
			if group.CustomModelsListEnabled() &&
				!CustomModelsListAllowsModel(group.ModelsListConfig.Models, sourceModel) {
				continue
			}
			for j := range groupAccounts {
				account := &groupAccounts[j]
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
