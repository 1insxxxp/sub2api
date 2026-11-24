package service

import (
	"context"
	"fmt"
	"strconv"
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
	DeleteAndUnbindAPIKeys(ctx context.Context, id, userID int64) (int, error)
	CountByUserID(ctx context.Context, userID int64) (int, error)
	CountBoundAPIKeys(ctx context.Context, id int64) (int, error)
	ResolveModel(ctx context.Context, id, userID int64, model string) (*UserCustomGroupModel, error)
}

type UserCustomGroupService struct {
	repo                 UserCustomGroupRepository
	userRepo             UserRepository
	groupRepo            GroupRepository
	apiKeys              *APIKeyService
	gateway              *GatewayService
	authCacheInvalidator APIKeyAuthCacheInvalidator
}

func NewUserCustomGroupService(repo UserCustomGroupRepository, userRepo UserRepository, groupRepo GroupRepository, gateway *GatewayService, authCacheInvalidator APIKeyAuthCacheInvalidator, apiKeys *APIKeyService) *UserCustomGroupService {
	// Both the API key runtime and the editor use this gateway's source catalog.
	if apiKeys != nil {
		apiKeys.customGroupGateway = gateway
	}
	return &UserCustomGroupService{apiKeys: apiKeys, repo: repo, userRepo: userRepo, groupRepo: groupRepo, gateway: gateway, authCacheInvalidator: authCacheInvalidator}
}

func (s *UserCustomGroupService) List(ctx context.Context, userID int64) ([]UserCustomGroup, error) {
	groups, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := s.selectAvailableSources(ctx, userID, groups); err != nil {
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
	if err := s.selectAvailableSources(ctx, userID, annotated); err != nil {
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
		if len(req.PreserveModelIDs) > MaxUserCustomGroupModels || len(*req.Models) > MaxUserCustomGroupModels {
			return nil, ErrUserCustomGroupLimit
		}
		// A source can go offline while the editor is open. Preserve unchanged
		// suspended rows by ID without revalidating or recreating those routes.
		current := []UserCustomGroup{*g}
		if err := s.selectAvailableSources(ctx, userID, current); err != nil {
			return nil, err
		}
		suspended := make(map[int64]bool, len(current[0].SuspendedModelIDs))
		for _, id := range current[0].SuspendedModelIDs {
			suspended[id] = true
		}
		pausedInputs := make(map[UserCustomGroupModelInput]bool)
		for _, m := range g.Models {
			if suspended[m.ID] {
				pausedInputs[UserCustomGroupModelInput{PublicModel: m.PublicModel, SourceModel: m.SourceModel, SourceGroupID: m.SourceGroupID}] = true
			}
		}
		selected := make([]UserCustomGroupModelInput, 0, len(*req.Models))
		for _, m := range *req.Models {
			if !pausedInputs[m] {
				selected = append(selected, m)
			}
		}
		req.Models = &selected
		if len(selected) > 0 {
			if err := s.validateModels(ctx, userID, selected); err != nil {
				return nil, err
			}
		}
		// Also retain rows suspended in the editor snapshot which have since
		// recovered. Only IDs still owned by this custom group are eligible.
		preserve := append(current[0].SuspendedModelIDs, req.PreserveModelIDs...)
		g.SuspendedModelIDs = retainedCustomGroupModels(g.Models, preserve, selected)
		if len(selected) == 0 && len(g.SuspendedModelIDs) == 0 && len(g.Models) > 0 {
			return nil, ErrUserCustomGroupInvalidModel
		}
		if len(g.SuspendedModelIDs)+len(selected) > MaxUserCustomGroupModels {
			return nil, ErrUserCustomGroupLimit
		}
	}
	if err := s.repo.Update(ctx, g, req.Models); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID, id)
}

func (s *UserCustomGroupService) selectAvailableSources(ctx context.Context, userID int64, groups []UserCustomGroup) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	catalog := newCustomSourceCatalog(s.gateway)
	allowed := make(map[int64]bool)
	for i := range groups {
		group := &groups[i]
		visible := make([]UserCustomGroupModel, 0, len(group.Models))
		group.SuspendedModelIDs = nil
		for _, model := range group.Models {
			source := model.SourceGroup
			if source == nil {
				continue
			}
			canUse, checked := allowed[source.ID]
			if !checked {
				canUse = source.Status == StatusActive && source.Platform != PlatformComposite && s.canUseSource(ctx, user, source)
				allowed[source.ID] = canUse
			}
			if !canUse || !catalog.contains(ctx, source, model.SourceModel) {
				group.SuspendedModelIDs = append(group.SuspendedModelIDs, model.ID)
				continue
			}
			model.SourceAvailable = true
			model.SourceIssue = ""
			visible = append(visible, model)
		}
		group.Models = visible
	}
	return nil
}

type customGroupSourceKey struct {
	groupID int64
	model   string
}

func retainedCustomGroupModels(existing []UserCustomGroupModel, preserve []int64, selected []UserCustomGroupModelInput) []int64 {
	keep := make(map[int64]bool, len(preserve))
	for _, id := range preserve {
		keep[id] = true
	}
	aliases := make(map[string]bool, len(selected))
	sources := make(map[customGroupSourceKey]bool, len(selected))
	for _, m := range selected {
		aliases[strings.ToLower(m.PublicModel)] = true
		sources[customGroupSourceKey{m.SourceGroupID, strings.ToLower(m.SourceModel)}] = true
	}
	var ids []int64
	for _, m := range existing {
		if keep[m.ID] && !aliases[strings.ToLower(m.PublicModel)] && !sources[customGroupSourceKey{m.SourceGroupID, strings.ToLower(m.SourceModel)}] {
			ids = append(ids, m.ID)
		}
	}
	return ids
}

func (s *UserCustomGroupService) canUseSource(ctx context.Context, user *User, group *Group) bool {
	if s.apiKeys != nil {
		return s.apiKeys.canUserBindGroup(ctx, user, group)
	}
	return user.CanBindGroup(group.ID, group.IsExclusive)
}

func (s *UserCustomGroupService) Delete(ctx context.Context, userID, id int64, force bool) (int, error) {
	if force {
		unboundCount, err := s.repo.DeleteAndUnbindAPIKeys(ctx, id, userID)
		if err != nil {
			return 0, err
		}
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
		}
		return unboundCount, nil
	}
	if _, err := s.repo.GetOwned(ctx, id, userID); err != nil {
		return 0, err
	}
	count, err := s.repo.CountBoundAPIKeys(ctx, id)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		return 0, ErrUserCustomGroupInUse.WithMetadata(map[string]string{
			"bound_api_key_count": strconv.Itoa(count),
		})
	}
	return 0, s.repo.Delete(ctx, id, userID)
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
	catalog := newCustomSourceCatalog(s.gateway)
	verified := make(map[int64]*Group)
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
		group := verified[m.SourceGroupID]
		if group == nil {
			group, err = s.groupRepo.GetByIDLite(ctx, m.SourceGroupID)
			if err != nil || group == nil || group.Status != StatusActive || group.Platform == PlatformComposite || !s.canUseSource(ctx, user, group) {
				return ErrGroupNotAllowed
			}
			verified[m.SourceGroupID] = group
		}
		if !catalog.contains(ctx, group, m.SourceModel) {
			return ErrUserCustomGroupInvalidModel
		}
	}
	return nil
}

// CustomGroupSourceModels returns the current catalog for editor and lifecycle
// checks. It deliberately bypasses the listing cache so a restored mapping is
// visible immediately.
func (s *GatewayService) CustomGroupSourceModels(ctx context.Context, group *Group) []string {
	available := s.GetAvailableModelsFresh(ctx, &group.ID, group.Platform)
	return s.customGroupSourceModelsFromAvailable(ctx, group, available)
}

// CustomGroupSourceModelsCached is reserved for hot-path routing checks. The
// short-lived cache avoids an account catalog query for every request.
func (s *GatewayService) CustomGroupSourceModelsCached(ctx context.Context, group *Group) []string {
	return s.customGroupSourceModelsFromAvailable(ctx, group, s.GetAvailableModels(ctx, &group.ID, group.Platform))
}

func (s *GatewayService) customGroupSourceModelsFromAvailable(ctx context.Context, group *Group, available []string) []string {
	if !group.CustomModelsListEnabled() {
		return available
	}
	if len(available) == 0 && (available != nil || !s.HasSchedulableAccountsForGroupPlatform(ctx, group.ID, group.Platform)) {
		return nil
	}
	return ResolveCustomModelsList(group.Platform, available, group.EffectiveModelAllowlist().Models)
}

// One catalog lookup per distinct source group within this request.
type customSourceCatalog struct {
	gateway *GatewayService
	models  map[int64]map[string]bool
}

func newCustomSourceCatalog(gateway *GatewayService) customSourceCatalog {
	return customSourceCatalog{gateway: gateway, models: make(map[int64]map[string]bool)}
}

func (c customSourceCatalog) contains(ctx context.Context, group *Group, model string) bool {
	if c.gateway == nil {
		return true
	}
	names, loaded := c.models[group.ID]
	if !loaded {
		names = make(map[string]bool)
		for _, name := range c.gateway.CustomGroupSourceModelsCached(ctx, group) {
			names[strings.ToLower(name)] = true
		}
		c.models[group.ID] = names
	}
	return names[strings.ToLower(model)]
}
