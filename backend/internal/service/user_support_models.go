package service

import (
	"context"
	"sort"
	"strings"
	"time"
)

type userSupportModelSources interface {
	Sources(context.Context, []int64) ([]CategoryDefaultSourceSnapshot, error)
}

func ProvideUserSupportService(repo UserSupportRepository, billingCache *BillingCacheService, apiKeys *APIKeyService, sources CategoryDefaultGroupRepository) *UserSupportService {
	s := NewUserSupportService(repo, billingCache, apiKeys)
	s.modelSources = sources
	return s
}

func safeUserSupportModel(value string) bool {
	return value != "" && len(value) <= 256 && !strings.ContainsAny(value, "*?\\%#<>&=\"':") &&
		PublicUserSupportGroupName(value) == value
}

type userSupportModelCatalog map[int64]map[string]bool

func (s *UserSupportService) recordModelCatalog(ctx context.Context, usage []UserSupportUsageRecord, errors []UserSupportErrorRecord) userSupportModelCatalog {
	catalog := userSupportModelCatalog{}
	if s.modelSources == nil {
		return catalog
	}
	ids := map[int64]bool{}
	add := func(c *UserSupportModelContext) {
		if c != nil && c.GroupID > 0 && safeUserSupportModel(c.RequestedModel) && PublicUserSupportModel(c.RequestedModel) == "unknown" {
			ids[c.GroupID] = true
		}
	}
	for i := 0; i < len(usage) && i < 10; i++ {
		add(usage[i].ModelContext)
	}
	for i := 0; i < len(errors) && i < 10; i++ {
		add(errors[i].ModelContext)
	}
	if len(ids) == 0 {
		return catalog
	}
	groupIDs := make([]int64, 0, len(ids))
	for id := range ids {
		groupIDs = append(groupIDs, id)
	}
	sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })
	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	sources, err := s.modelSources.Sources(readCtx, groupIDs)
	if err != nil {
		return catalog
	}
	for _, source := range sources {
		if source.Group == nil || !ids[source.Group.ID] {
			continue
		}
		models, err := CategoryDefaultSourceDeclarations(source)
		if err != nil {
			continue
		}
		allowed := map[string]bool{}
		for _, model := range models {
			if safeUserSupportModel(model.PublicModel) {
				allowed[categoryDefaultModelKey(model.PublicModel)] = true
			}
		}
		catalog[source.Group.ID] = allowed
	}
	return catalog
}

func projectUserSupportModel(fallback string, evidence *UserSupportModelContext, catalog userSupportModelCatalog) string {
	if evidence == nil {
		return PublicUserSupportModel(fallback)
	}
	name := strings.TrimSpace(evidence.RequestedModel)
	if safeUserSupportModel(name) && catalog[evidence.GroupID][categoryDefaultModelKey(name)] {
		return name
	}
	// Missing requested_model never falls back to an internal or previously projected field.
	return PublicUserSupportModel(name)
}
