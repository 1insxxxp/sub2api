package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type SystemCustomGroupRepository interface {
	Create(ctx context.Context, group *Group, sourceGroupIDs []int64, models []SystemCustomGroupModelInput) error
	Update(ctx context.Context, group *Group, sourceGroupIDs []int64, models []SystemCustomGroupModelInput) error
	Get(ctx context.Context, groupID int64) (*SystemCustomGroup, error)
	ListModels(ctx context.Context, groupID int64, enabledOnly bool) ([]SystemCustomGroupModel, error)
	ResolveModel(ctx context.Context, groupID int64, publicModel string) (*SystemCustomGroupModel, error)
	Delete(ctx context.Context, groupID int64) error
}

// SystemCustomGroupDeleteImpactRepository extends the legacy repository
// without breaking implementations that only provide Delete.
type SystemCustomGroupDeleteImpactRepository interface {
	DeleteWithImpact(ctx context.Context, groupID int64) (*SystemCustomGroupDeleteImpact, error)
}

// systemCustomGroupAuthCacheInvalidator is the optional cache capability used
// after a committed custom-group update. Keeping it narrow preserves legacy
// constructor and test-double compatibility.
type systemCustomGroupAuthCacheInvalidator interface {
	InvalidateAuthCacheByGroupID(ctx context.Context, groupID int64)
}

// SystemCustomGroupModelCatalog is deliberately the small subset of
// GatewayService needed by administration. Keeping this seam narrow makes the
// validation rules testable while production still uses the gateway's exact
// schedulable-account model calculation.
type SystemCustomGroupModelCatalog interface {
	GetAvailableModels(ctx context.Context, groupID *int64, platform string) []string
	HasSchedulableAccountsForGroupPlatform(ctx context.Context, groupID int64, platform string) bool
}

// SystemCustomGroupModelListSource is one live source-group snapshot together
// with the concrete source models whose public aliases may be advertised.
// Runtime list generation deliberately receives the group snapshot loaded with
// the routes so it does not re-query each source independently.
type SystemCustomGroupModelListSource struct {
	Group  Group
	Models []string
}

// SystemCustomGroupModelAvailability is keyed by source group and then by the
// exact source-model spelling supplied in SystemCustomGroupModelListSource.
type SystemCustomGroupModelAvailability map[int64]map[string]bool

// SystemCustomGroupModelListCatalog evaluates one complete route snapshot
// against the same account support rules used by gateway scheduling.
type SystemCustomGroupModelListCatalog interface {
	ListSystemCustomGroupModelAvailability(ctx context.Context, sources []SystemCustomGroupModelListSource) (SystemCustomGroupModelAvailability, error)
}

// SystemCustomGroupRuntimeModelCatalog derives a request-time model catalog
// from live source references. The retained static route repository remains a
// separate rollback-only API.
type SystemCustomGroupRuntimeModelCatalog interface {
	BuildSystemCustomGroupModelCatalog(ctx context.Context, sources []SystemCustomGroupSource, platform string) (*SystemCustomGroupRuntimeCatalog, error)
}

// SystemCustomGroupSchedulableAccount preserves the source-group association
// while loading all candidate accounts in one bounded repository operation.
type SystemCustomGroupSchedulableAccount struct {
	GroupID int64
	Account Account
}

type systemCustomGroupSchedulableAccountRepository interface {
	ListSchedulableByGroupIDs(ctx context.Context, groupIDs []int64) ([]SystemCustomGroupSchedulableAccount, error)
}

type SystemCustomGroupService struct {
	repo                 SystemCustomGroupRepository
	groupRepo            GroupRepository
	modelIndex           SystemCustomGroupModelCatalog
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCacheService  *BillingCacheService
}

// NewSystemCustomGroupServiceWithCacheInvalidation is the production Wire
// constructor. NewSystemCustomGroupService remains source-compatible for tests
// and integrations that do not own cache invalidation dependencies.
func NewSystemCustomGroupServiceWithCacheInvalidation(
	repo SystemCustomGroupRepository,
	groupRepo GroupRepository,
	modelIndex SystemCustomGroupModelCatalog,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	billingCacheService *BillingCacheService,
) *SystemCustomGroupService {
	service := NewSystemCustomGroupService(repo, groupRepo, modelIndex)
	service.authCacheInvalidator = authCacheInvalidator
	service.billingCacheService = billingCacheService
	return service
}

func NewSystemCustomGroupService(repo SystemCustomGroupRepository, groupRepo GroupRepository, modelIndex SystemCustomGroupModelCatalog) *SystemCustomGroupService {
	return &SystemCustomGroupService{repo: repo, groupRepo: groupRepo, modelIndex: modelIndex}
}

func (s *SystemCustomGroupService) Create(ctx context.Context, req CreateSystemCustomGroupRequest) (*SystemCustomGroup, error) {
	name, description, days, daily, weekly, monthly, err := normalizeSystemCustomGroupFields(
		req.Name, req.Description, req.DefaultValidityDays, req.DailyLimitUSD, req.WeeklyLimitUSD, req.MonthlyLimitUSD,
	)
	if err != nil {
		return nil, err
	}
	if s == nil || s.repo == nil || s.groupRepo == nil {
		return nil, fmt.Errorf("system custom group service is not configured")
	}
	exists, err := s.groupRepo.ExistsByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("check group exists: %w", err)
	}
	if exists {
		return nil, ErrGroupExists
	}
	sourceGroupIDs, legacyRequest, err := normalizeSystemCustomGroupSourceSelection(req.SourceGroupIDs, req.Models)
	if err != nil {
		return nil, err
	}
	models := req.Models
	if legacyRequest {
		if err := s.ValidateRoutes(ctx, 0, models); err != nil {
			return nil, err
		}
	} else {
		models = nil
		if err := s.validateSourceGroups(ctx, 0, sourceGroupIDs); err != nil {
			return nil, err
		}
	}

	group := &Group{
		Name:                       name,
		Description:                description,
		Platform:                   PlatformComposite,
		RateMultiplier:             1,
		IsExclusive:                true,
		Status:                     StatusActive,
		SubscriptionType:           SubscriptionTypeSubscription,
		SystemCustomRoutingEnabled: true,
		DailyLimitUSD:              daily,
		WeeklyLimitUSD:             weekly,
		MonthlyLimitUSD:            monthly,
		DefaultValidityDays:        days,
	}
	if err := s.repo.Create(ctx, group, sourceGroupIDs, models); err != nil {
		return nil, fmt.Errorf("create system custom group: %w", err)
	}
	created, err := s.repo.Get(ctx, group.ID)
	if err != nil {
		return nil, fmt.Errorf("load created system custom group: %w", err)
	}
	return created, nil
}

func (s *SystemCustomGroupService) Update(ctx context.Context, groupID int64, req UpdateSystemCustomGroupRequest) (*SystemCustomGroup, error) {
	if s == nil || s.repo == nil || s.groupRepo == nil {
		return nil, fmt.Errorf("system custom group service is not configured")
	}
	existing, err := s.repo.Get(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if !existing.Group.IsSystemCustomRouteGroup() {
		return nil, ErrSystemCustomGroupNotFound
	}
	name, description, days, daily, weekly, monthly, err := normalizeSystemCustomGroupFields(
		req.Name, req.Description, req.DefaultValidityDays, req.DailyLimitUSD, req.WeeklyLimitUSD, req.MonthlyLimitUSD,
	)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(name, existing.Group.Name) {
		exists, checkErr := s.groupRepo.ExistsByName(ctx, name)
		if checkErr != nil {
			return nil, fmt.Errorf("check group exists: %w", checkErr)
		}
		if exists {
			return nil, ErrGroupExists
		}
	}
	sourceGroupIDs, legacyRequest, err := normalizeSystemCustomGroupSourceSelection(req.SourceGroupIDs, req.Models)
	if err != nil {
		return nil, err
	}
	models := req.Models
	if legacyRequest {
		preservedMissing := make(map[string]struct{}, len(existing.Models))
		for _, model := range existing.Models {
			preservedMissing[systemCustomSourceKey(model.SourceGroupID, model.SourceModel)] = struct{}{}
		}
		if err := s.validateRoutes(ctx, groupID, models, preservedMissing); err != nil {
			return nil, err
		}
	} else {
		models = nil
		if err := s.validateSourceGroups(ctx, groupID, sourceGroupIDs); err != nil {
			return nil, err
		}
	}

	group := existing.Group
	group.Name = name
	group.Description = description
	group.DailyLimitUSD = daily
	group.WeeklyLimitUSD = weekly
	group.MonthlyLimitUSD = monthly
	group.DefaultValidityDays = days
	normalizeSystemCustomContainer(&group)
	if err := s.repo.Update(ctx, &group, sourceGroupIDs, models); err != nil {
		return nil, fmt.Errorf("update system custom group: %w", err)
	}
	if invalidator, ok := any(s.authCacheInvalidator).(systemCustomGroupAuthCacheInvalidator); ok && invalidator != nil {
		invalidator.InvalidateAuthCacheByGroupID(ctx, groupID)
	}
	updated, err := s.repo.Get(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("load updated system custom group: %w", err)
	}
	return updated, nil
}

func normalizeSystemCustomGroupSourceSelection(sourceGroupIDs []int64, models []SystemCustomGroupModelInput) ([]int64, bool, error) {
	if len(sourceGroupIDs) > 0 {
		if len(sourceGroupIDs) > MaxSystemCustomGroupSources {
			return nil, false, ErrSystemCustomGroupInvalidRoute
		}
		return append([]int64(nil), sourceGroupIDs...), false, nil
	}
	if len(models) == 0 {
		return nil, false, ErrSystemCustomGroupInvalidRoute
	}
	seen := make(map[int64]struct{}, len(models))
	derived := make([]int64, 0, len(models))
	for _, model := range models {
		if _, ok := seen[model.SourceGroupID]; ok {
			continue
		}
		seen[model.SourceGroupID] = struct{}{}
		derived = append(derived, model.SourceGroupID)
	}
	if len(derived) == 0 || len(derived) > MaxSystemCustomGroupSources {
		return nil, true, ErrSystemCustomGroupInvalidRoute
	}
	return derived, true, nil
}

func (s *SystemCustomGroupService) validateSourceGroups(ctx context.Context, containerGroupID int64, sourceGroupIDs []int64) error {
	if s == nil || s.groupRepo == nil {
		return fmt.Errorf("system custom group service is not configured")
	}
	if len(sourceGroupIDs) == 0 || len(sourceGroupIDs) > MaxSystemCustomGroupSources {
		return ErrSystemCustomGroupInvalidRoute
	}
	seen := make(map[int64]struct{}, len(sourceGroupIDs))
	for _, sourceGroupID := range sourceGroupIDs {
		if sourceGroupID <= 0 {
			return ErrSystemCustomGroupInvalidSourceGroup
		}
		if containerGroupID > 0 && sourceGroupID == containerGroupID {
			return ErrSystemCustomGroupSelfReference
		}
		if _, exists := seen[sourceGroupID]; exists {
			return ErrSystemCustomGroupInvalidSourceGroup
		}
		seen[sourceGroupID] = struct{}{}
	}
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("list system custom source groups: %w", err)
	}
	valid := make(map[int64]struct{}, len(groups))
	for i := range groups {
		if isDirectSystemCustomSource(&groups[i]) {
			valid[groups[i].ID] = struct{}{}
		}
	}
	for _, sourceGroupID := range sourceGroupIDs {
		if _, exists := valid[sourceGroupID]; !exists {
			return ErrSystemCustomGroupInvalidSourceGroup
		}
	}
	return nil
}

func (s *SystemCustomGroupService) Get(ctx context.Context, groupID int64) (*SystemCustomGroup, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("system custom group service is not configured")
	}
	return s.repo.Get(ctx, groupID)
}

func (s *SystemCustomGroupService) Delete(ctx context.Context, groupID int64) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("system custom group service is not configured")
	}
	impactRepo, ok := s.repo.(SystemCustomGroupDeleteImpactRepository)
	if !ok {
		return s.repo.Delete(ctx, groupID)
	}
	impact, err := impactRepo.DeleteWithImpact(ctx, groupID)
	if err != nil {
		return err
	}
	if impact == nil {
		return nil
	}
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if s.authCacheInvalidator != nil {
		for _, key := range impact.APIKeyValues {
			if key != "" {
				s.authCacheInvalidator.InvalidateAuthCacheByKey(cacheCtx, key)
			}
		}
	}
	if s.billingCacheService != nil {
		for _, userID := range impact.UserIDs {
			if userID <= 0 {
				continue
			}
			if err := s.billingCacheService.InvalidateSubscription(cacheCtx, userID, groupID); err != nil {
				logger.LegacyPrintf("service.system_custom_group", "invalidate subscription cache failed: user_id=%d group_id=%d err=%v", userID, groupID, err)
			}
		}
	}
	return nil
}

// ValidateRoutes normalizes route names in place and verifies the full route
// snapshot. Source-group exclusivity is intentionally ignored: these routes
// are an administrator-granted indirect capability, not a direct user bind.
func (s *SystemCustomGroupService) ValidateRoutes(ctx context.Context, containerGroupID int64, models []SystemCustomGroupModelInput) error {
	return s.validateRoutes(ctx, containerGroupID, models, nil)
}

func (s *SystemCustomGroupService) validateRoutes(ctx context.Context, containerGroupID int64, models []SystemCustomGroupModelInput, preservedMissing map[string]struct{}) error {
	if s == nil || s.groupRepo == nil {
		return fmt.Errorf("system custom group service is not configured")
	}
	if len(models) == 0 || len(models) > MaxSystemCustomGroupModels {
		return ErrSystemCustomGroupInvalidRoute
	}
	seenPublic := make(map[string]struct{}, len(models))
	seenSources := make(map[string]struct{}, len(models))
	groups := make(map[int64]*Group)
	available := make(map[int64]map[string]struct{})

	for i := range models {
		model := &models[i]
		model.PublicModel = strings.TrimSpace(model.PublicModel)
		model.SourceModel = strings.TrimSpace(model.SourceModel)
		if model.PublicModel == "" || model.SourceModel == "" || len(model.PublicModel) > 200 || len(model.SourceModel) > 200 || model.SourceGroupID <= 0 {
			return newSystemCustomRouteError(ErrSystemCustomGroupInvalidRoute, *model)
		}
		publicKey := strings.ToLower(model.PublicModel)
		if _, ok := seenPublic[publicKey]; ok {
			return newSystemCustomRouteError(ErrSystemCustomGroupDuplicatePublicModel, *model)
		}
		seenPublic[publicKey] = struct{}{}
		sourceKey := systemCustomSourceKey(model.SourceGroupID, model.SourceModel)
		if _, ok := seenSources[sourceKey]; ok {
			return newSystemCustomRouteError(ErrSystemCustomGroupDuplicateSourceModel, *model)
		}
		seenSources[sourceKey] = struct{}{}
		if containerGroupID > 0 && model.SourceGroupID == containerGroupID {
			return newSystemCustomRouteError(ErrSystemCustomGroupSelfReference, *model)
		}

		source, ok := groups[model.SourceGroupID]
		if !ok {
			var err error
			source, err = s.groupRepo.GetByIDLite(ctx, model.SourceGroupID)
			if err != nil {
				if !errors.Is(err, ErrGroupNotFound) {
					return fmt.Errorf("load system custom source group %d: %w", model.SourceGroupID, err)
				}
				return newSystemCustomRouteError(ErrSystemCustomGroupInvalidSourceGroup, *model)
			}
			if !isDirectSystemCustomSource(source) {
				return newSystemCustomRouteError(ErrSystemCustomGroupInvalidSourceGroup, *model)
			}
			groups[model.SourceGroupID] = source
		}
		modelSet, ok := available[model.SourceGroupID]
		if !ok {
			modelSet = make(map[string]struct{})
			for _, candidate := range s.availableModelsForSource(ctx, source) {
				modelSet[strings.ToLower(candidate)] = struct{}{}
			}
			available[model.SourceGroupID] = modelSet
		}
		if _, ok := modelSet[strings.ToLower(model.SourceModel)]; !ok {
			if _, preserved := preservedMissing[sourceKey]; !preserved {
				return newSystemCustomRouteError(ErrSystemCustomGroupMissingSourceModel, *model)
			}
		}
	}
	return nil
}

func (s *SystemCustomGroupService) Candidates(ctx context.Context) ([]SystemCustomGroupCandidate, error) {
	if s == nil || s.groupRepo == nil {
		return nil, fmt.Errorf("system custom group service is not configured")
	}
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list system custom source groups: %w", err)
	}
	candidates := make([]SystemCustomGroupCandidate, 0, len(groups))
	for i := range groups {
		if !isDirectSystemCustomSource(&groups[i]) {
			continue
		}
		models := s.availableModelsForSource(ctx, &groups[i])
		if len(models) == 0 {
			continue
		}
		candidates = append(candidates, SystemCustomGroupCandidate{Group: groups[i], Models: models})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Group.SortOrder != candidates[j].Group.SortOrder {
			return candidates[i].Group.SortOrder < candidates[j].Group.SortOrder
		}
		if candidates[i].Group.Name != candidates[j].Group.Name {
			return candidates[i].Group.Name < candidates[j].Group.Name
		}
		return candidates[i].Group.ID < candidates[j].Group.ID
	})
	return candidates, nil
}

func (s *SystemCustomGroupService) SyncPreview(ctx context.Context, groupID int64) (*SystemCustomGroupSyncPreview, error) {
	stored, err := s.Get(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if !stored.Group.IsSystemCustomRouteGroup() {
		return nil, ErrSystemCustomGroupNotFound
	}
	candidates, err := s.Candidates(ctx)
	if err != nil {
		return nil, err
	}
	availableByGroup := make(map[int64]map[string]string, len(candidates))
	for _, candidate := range candidates {
		models := make(map[string]string, len(candidate.Models))
		for _, model := range candidate.Models {
			models[strings.ToLower(model)] = model
		}
		availableByGroup[candidate.Group.ID] = models
	}

	preview := &SystemCustomGroupSyncPreview{
		Added:       []SystemCustomGroupSyncAdded{},
		Missing:     []SystemCustomGroupModel{},
		Conflicting: []SystemCustomGroupSyncConflict{},
	}
	existingSources := make(map[string]struct{}, len(stored.Models))
	existingPublic := make(map[string]struct{}, len(stored.Models))
	for _, route := range stored.Models {
		existingSources[systemCustomSourceKey(route.SourceGroupID, route.SourceModel)] = struct{}{}
		existingPublic[strings.ToLower(route.PublicModel)] = struct{}{}
		models, groupAvailable := availableByGroup[route.SourceGroupID]
		if _, modelAvailable := models[strings.ToLower(route.SourceModel)]; !groupAvailable || !modelAvailable {
			preview.Missing = append(preview.Missing, route)
		}
	}

	type proposedRoute struct {
		groupID int64
		model   string
	}
	proposals := make(map[string][]proposedRoute)
	proposalNames := make([]string, 0)
	for _, candidate := range candidates {
		for _, model := range candidate.Models {
			if _, exists := existingSources[systemCustomSourceKey(candidate.Group.ID, model)]; exists {
				continue
			}
			key := strings.ToLower(model)
			if _, exists := proposals[key]; !exists {
				proposalNames = append(proposalNames, key)
			}
			proposals[key] = append(proposals[key], proposedRoute{groupID: candidate.Group.ID, model: model})
		}
	}
	sort.Strings(proposalNames)
	for _, key := range proposalNames {
		entries := proposals[key]
		_, collidesWithStored := existingPublic[key]
		if collidesWithStored || len(entries) > 1 {
			for _, entry := range entries {
				route := SystemCustomGroupModelInput{PublicModel: entry.model, SourceGroupID: entry.groupID, SourceModel: entry.model, Enabled: true}
				routeErr := newSystemCustomRouteError(ErrSystemCustomGroupDuplicatePublicModel, route)
				preview.Conflicting = append(preview.Conflicting, SystemCustomGroupSyncConflict{
					PublicModel: entry.model, SourceGroupID: entry.groupID, SourceModel: entry.model,
					Reason: "public model alias is already used; choose an explicit alias", Err: routeErr,
				})
			}
			continue
		}
		entry := entries[0]
		preview.Added = append(preview.Added, SystemCustomGroupSyncAdded{
			PublicModel: entry.model, SourceGroupID: entry.groupID, SourceModel: entry.model, Selected: false,
		})
	}
	sort.SliceStable(preview.Missing, func(i, j int) bool {
		return strings.ToLower(preview.Missing[i].PublicModel) < strings.ToLower(preview.Missing[j].PublicModel)
	})
	return preview, nil
}

func (s *SystemCustomGroupService) availableModelsForSource(ctx context.Context, group *Group) []string {
	if s == nil || s.modelIndex == nil || group == nil {
		return nil
	}
	available := s.modelIndex.GetAvailableModels(ctx, &group.ID, group.Platform)
	if len(available) == 0 && !s.modelIndex.HasSchedulableAccountsForGroupPlatform(ctx, group.ID, group.Platform) {
		return nil
	}
	if group.CustomModelsListEnabled() {
		return ResolveCustomModelsList(group.Platform, available, group.ModelsListConfig.Models)
	}
	if len(available) == 0 {
		return DefaultModelIDsForPlatform(group.Platform)
	}
	return append([]string(nil), available...)
}

func isDirectSystemCustomSource(group *Group) bool {
	return group != nil && group.ID > 0 && group.Status == StatusActive &&
		isConcreteRequestPlatform(group.Platform) && !group.SystemCustomRoutingEnabled
}

// IsEligibleSystemCustomSource exposes the source eligibility rule to API
// mappers that report unavailable persisted references without querying again.
func IsEligibleSystemCustomSource(group *Group) bool {
	return isDirectSystemCustomSource(group)
}

func normalizeSystemCustomContainer(group *Group) {
	group.Platform = PlatformComposite
	group.SubscriptionType = SubscriptionTypeSubscription
	group.IsExclusive = true
	group.RateMultiplier = 1
	group.SystemCustomRoutingEnabled = true
}

func normalizeSystemCustomGroupFields(name string, description *string, days int, limits ...*float64) (string, string, int, *float64, *float64, *float64, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 {
		return "", "", 0, nil, nil, nil, ErrSystemCustomGroupInvalidInput
	}
	descriptionValue := ""
	if description != nil {
		descriptionValue = strings.TrimSpace(*description)
	}
	if days == 0 {
		days = 30
	}
	if days < 1 || days > 3650 {
		return "", "", 0, nil, nil, nil, ErrSystemCustomGroupInvalidInput
	}
	normalized := make([]*float64, 3)
	for i := 0; i < len(normalized) && i < len(limits); i++ {
		if limits[i] == nil || *limits[i] < 0 {
			continue
		}
		if math.IsNaN(*limits[i]) || math.IsInf(*limits[i], 0) {
			return "", "", 0, nil, nil, nil, ErrSystemCustomGroupInvalidInput
		}
		value := *limits[i]
		normalized[i] = &value
	}
	return name, descriptionValue, days, normalized[0], normalized[1], normalized[2], nil
}

func systemCustomSourceKey(groupID int64, model string) string {
	return fmt.Sprintf("%d:%s", groupID, strings.ToLower(strings.TrimSpace(model)))
}

func newSystemCustomRouteError(kind error, model SystemCustomGroupModelInput) error {
	return &SystemCustomGroupRouteError{
		Kind: kind, PublicModel: model.PublicModel, SourceGroupID: model.SourceGroupID, SourceModel: model.SourceModel,
	}
}
