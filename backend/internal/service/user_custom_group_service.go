package service

import (
	"context"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrUserCustomGroupNotFound     = infraerrors.NotFound("CUSTOM_GROUP_NOT_FOUND", "custom group not found")
	ErrUserCustomGroupExists       = infraerrors.Conflict("CUSTOM_GROUP_EXISTS", "custom group name already exists")
	ErrUserCustomGroupInUse        = infraerrors.Conflict("CUSTOM_GROUP_IN_USE", "custom group is bound to one or more API keys")
	ErrUserCustomGroupInvalidModel = infraerrors.BadRequest("CUSTOM_GROUP_INVALID_MODEL", "custom group model mapping is invalid")
	ErrUserCustomGroupLimit        = infraerrors.BadRequest("CUSTOM_GROUP_LIMIT", "custom group limit exceeded")
)

type UserCustomGroupRepository interface {
	ListByUserID(ctx context.Context, userID int64) ([]UserCustomGroup, error)
	GetOwned(ctx context.Context, id, userID int64) (*UserCustomGroup, error)
	Create(ctx context.Context, group *UserCustomGroup, models []UserCustomGroupModelInput) error
	Update(ctx context.Context, group *UserCustomGroup, models *[]UserCustomGroupModelInput) error
	Delete(ctx context.Context, id, userID int64) error
	CountByUserID(ctx context.Context, userID int64) (int, error)
	CountBoundAPIKeys(ctx context.Context, id int64) (int, error)
	ResolveModel(ctx context.Context, id, userID int64, model string) (*UserCustomGroupModel, error)
}

type UserCustomGroupService struct {
	repo      UserCustomGroupRepository
	userRepo  UserRepository
	groupRepo GroupRepository
	gateway   *GatewayService
}

func NewUserCustomGroupService(repo UserCustomGroupRepository, userRepo UserRepository, groupRepo GroupRepository, gateway *GatewayService) *UserCustomGroupService {
	return &UserCustomGroupService{repo: repo, userRepo: userRepo, groupRepo: groupRepo, gateway: gateway}
}

func (s *UserCustomGroupService) List(ctx context.Context, userID int64) ([]UserCustomGroup, error) {
	groups, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := s.annotateSourceAvailability(ctx, userID, groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *UserCustomGroupService) Get(ctx context.Context, userID, id int64) (*UserCustomGroup, error) {
	group, err := s.repo.GetOwned(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	annotated := []UserCustomGroup{*group}
	if err := s.annotateSourceAvailability(ctx, userID, annotated); err != nil {
		return nil, err
	}
	return &annotated[0], nil
}

func (s *UserCustomGroupService) Create(ctx context.Context, userID int64, req CreateUserCustomGroupRequest) (*UserCustomGroup, error) {
	count, err := s.repo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if count >= MaxUserCustomGroups {
		return nil, ErrUserCustomGroupLimit
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 100 {
		return nil, ErrUserCustomGroupExists
	}
	if err := s.validateModels(ctx, userID, req.Models); err != nil {
		return nil, err
	}
	g := &UserCustomGroup{UserID: userID, Name: name, Status: StatusActive}
	if err := s.repo.Create(ctx, g, req.Models); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID, g.ID)
}

func (s *UserCustomGroupService) Update(ctx context.Context, userID, id int64, req UpdateUserCustomGroupRequest) (*UserCustomGroup, error) {
	g, err := s.repo.GetOwned(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" || len(name) > 100 {
			return nil, ErrUserCustomGroupExists
		}
		g.Name = name
	}
	if req.Status != nil {
		if *req.Status != StatusActive && *req.Status != StatusDisabled {
			return nil, fmt.Errorf("invalid status")
		}
		g.Status = *req.Status
	}
	if req.Models != nil {
		if err := s.validateModels(ctx, userID, *req.Models); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Update(ctx, g, req.Models); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID, id)
}

func (s *UserCustomGroupService) annotateSourceAvailability(ctx context.Context, userID int64, groups []UserCustomGroup) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	for groupIndex := range groups {
		for modelIndex := range groups[groupIndex].Models {
			model := &groups[groupIndex].Models[modelIndex]
			model.SourceAvailable = false
			model.SourceIssue = UserCustomGroupSourceIssueUnavailable
			source := model.SourceGroup
			if source == nil || source.Status != StatusActive || source.Platform == PlatformComposite {
				continue
			}
			if !user.CanBindGroup(source.ID, source.IsExclusive) {
				model.SourceIssue = UserCustomGroupSourceIssueNotAllowed
				continue
			}
			model.SourceAvailable = true
			model.SourceIssue = ""
		}
	}
	return nil
}

func (s *UserCustomGroupService) Delete(ctx context.Context, userID, id int64) error {
	if _, err := s.repo.GetOwned(ctx, id, userID); err != nil {
		return err
	}
	count, err := s.repo.CountBoundAPIKeys(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrUserCustomGroupInUse
	}
	return s.repo.Delete(ctx, id, userID)
}

func (s *UserCustomGroupService) validateModels(ctx context.Context, userID int64, models []UserCustomGroupModelInput) error {
	if len(models) == 0 || len(models) > MaxUserCustomGroupModels {
		return ErrUserCustomGroupInvalidModel
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(models))
	seenSources := make(map[string]struct{}, len(models))
	for i := range models {
		m := &models[i]
		m.PublicModel = strings.TrimSpace(m.PublicModel)
		m.SourceModel = strings.TrimSpace(m.SourceModel)
		if m.PublicModel == "" || m.SourceModel == "" || len(m.PublicModel) > 200 || len(m.SourceModel) > 200 {
			return ErrUserCustomGroupInvalidModel
		}
		key := strings.ToLower(m.PublicModel)
		if _, ok := seen[key]; ok {
			return ErrUserCustomGroupInvalidModel
		}
		seen[key] = struct{}{}
		sourceKey := fmt.Sprintf("%d:%s", m.SourceGroupID, strings.ToLower(m.SourceModel))
		if _, ok := seenSources[sourceKey]; ok {
			return ErrUserCustomGroupInvalidModel
		}
		seenSources[sourceKey] = struct{}{}
		group, err := s.groupRepo.GetByIDLite(ctx, m.SourceGroupID)
		if err != nil || group.Status != StatusActive || group.Platform == PlatformComposite || !user.CanBindGroup(group.ID, group.IsExclusive) {
			return ErrGroupNotAllowed
		}
		if s.gateway != nil {
			available := s.gateway.GetAvailableModels(ctx, &group.ID, group.Platform)
			found := false
			for _, candidate := range available {
				if strings.EqualFold(candidate, m.SourceModel) {
					found = true
					break
				}
			}
			if !found {
				return ErrUserCustomGroupInvalidModel
			}
		}
	}
	return nil
}
