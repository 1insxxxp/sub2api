package service

import (
	"context"
	"fmt"
)

type categoryDefaultGroupResolver interface {
	ResolveForUser(context.Context, int64, *User, string) (*CategoryDefaultModelRoute, error)
	ModelsForUser(context.Context, int64, *User, string) ([]string, error)
	CanBind(context.Context, int64, *User) (bool, error)
}

func (s *APIKeyService) SetCategoryDefaultGroupService(groups *CategoryDefaultGroupService) {
	s.categoryDefaultGroups = groups
}

type CategoryDefaultGroupModelResolution struct {
	APIKey       *APIKey
	EntryGroupID int64
	PublicModel  string
	SourceModel  string
	GeminiNative bool
}

// categoryDefaultUser always reads current source authorization. An entry may
// remain in the auth cache while any of its source groups change independently.
func (s *APIKeyService) categoryDefaultUser(ctx context.Context, key *APIKey) (*User, error) {
	if key == nil || key.UserID <= 0 || s == nil || s.userRepo == nil {
		return nil, ErrGroupNotAllowed
	}
	user, err := s.userRepo.GetByID(ctx, key.UserID)
	if err != nil || user == nil || user.ID != key.UserID || user.Status != StatusActive {
		return nil, ErrGroupNotAllowed
	}
	return user, nil
}

func (s *APIKeyService) ResolveCategoryDefaultGroupModel(ctx context.Context, key *APIKey, model string) (*CategoryDefaultGroupModelResolution, error) {
	if key == nil || key.Group == nil || !key.Group.IsCategoryDefault {
		return nil, nil
	}
	if s == nil || s.categoryDefaultGroups == nil || key.GroupID == nil || *key.GroupID != key.Group.ID {
		return nil, ErrCategoryDefaultSourceUnavailable
	}
	user, err := s.categoryDefaultUser(ctx, key)
	if err != nil {
		return nil, err
	}
	// The key repository loads the user's current group-account exclusions, which
	// GetByID(User) intentionally does not hydrate. Never clear these on a clone.
	if s.apiKeyRepo != nil {
		fresh, loadErr := s.apiKeyRepo.GetByID(ctx, key.ID)
		if loadErr != nil || fresh == nil || fresh.User == nil || fresh.UserID != key.UserID || fresh.GroupID == nil || *fresh.GroupID != key.Group.ID {
			return nil, ErrCategoryDefaultSourceUnavailable
		}
		user.GroupAccountExclusions = cloneGroupAccountExclusions(fresh.User.GroupAccountExclusions)
	} else if key.User != nil {
		user.GroupAccountExclusions = cloneGroupAccountExclusions(key.User.GroupAccountExclusions)
	}
	route, err := s.categoryDefaultGroups.ResolveForUser(ctx, key.Group.ID, user, model)
	if err != nil {
		return nil, err
	}
	if route == nil || route.EntryGroupID != key.Group.ID || route.SourceGroup == nil || route.SourceGroup.ID <= 0 || route.SourceGroup.IsCategoryDefault || route.SourceGroup.IsSubscriptionType() || route.SourceGroup.SystemCustomRoutingEnabled {
		return nil, ErrCategoryDefaultSourceUnavailable
	}
	source := *route.SourceGroup
	source.FallbackGroupID = nil
	source.FallbackGroupIDOnInvalidRequest = nil
	userClone := *user
	userClone.UserGroupRPMOverride = nil
	if s.userGroupRateRepo != nil {
		override, loadErr := s.userGroupRateRepo.GetRPMOverrideByUserAndGroup(ctx, user.ID, source.ID)
		if loadErr != nil {
			return nil, fmt.Errorf("load category source rpm: %w", loadErr)
		}
		userClone.UserGroupRPMOverride = override
	}
	clone := *key
	clone.User = &userClone
	clone.Group = &source
	sourceID, entryID := source.ID, key.Group.ID
	clone.GroupID = &sourceID
	clone.CustomGroupID = nil
	clone.CustomGroup = nil
	clone.CategoryDefaultGroupID = &entryID
	return &CategoryDefaultGroupModelResolution{APIKey: &clone, EntryGroupID: entryID, PublicModel: route.PublicModel, SourceModel: route.SourceModel, GeminiNative: route.GeminiNative}, nil
}

func (s *APIKeyService) ListCategoryDefaultGroupModels(ctx context.Context, key *APIKey, platform string) ([]string, error) {
	if key == nil || key.Group == nil || !key.Group.IsCategoryDefault || key.GroupID == nil || *key.GroupID != key.Group.ID || s == nil || s.categoryDefaultGroups == nil {
		return nil, ErrCategoryDefaultSourceUnavailable
	}
	user, err := s.categoryDefaultUser(ctx, key)
	if err != nil {
		return nil, err
	}
	return s.categoryDefaultGroups.ModelsForUser(ctx, key.Group.ID, user, platform)
}
