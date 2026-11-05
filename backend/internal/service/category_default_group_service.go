package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

type CategoryDefaultGroupService struct {
	repo        CategoryDefaultGroupRepository
	invalidator APIKeyAuthCacheInvalidator
	channels    *ChannelService
}

func NewCategoryDefaultGroupService(repo CategoryDefaultGroupRepository, invalidator APIKeyAuthCacheInvalidator) *CategoryDefaultGroupService {
	return &CategoryDefaultGroupService{repo: repo, invalidator: invalidator}
}

func (s *CategoryDefaultGroupService) List(ctx context.Context) ([]CategoryDefaultGroupConfig, error) {
	out := make([]CategoryDefaultGroupConfig, 0, len(CategoryDefaultCategories))
	for _, category := range CategoryDefaultCategories {
		snapshot, err := s.repo.Snapshot(ctx, CategoryDefaultSnapshotRequest{Category: category})
		if err != nil {
			return nil, err
		}
		projected, err := projectCategoryDefaultCatalog(snapshot)
		if err != nil {
			return nil, err
		}
		out = append(out, *projected.config)
	}
	return out, nil
}

func (s *CategoryDefaultGroupService) Candidates(ctx context.Context, category string) ([]CategoryDefaultSourceCandidate, error) {
	if !validCategoryDefaultCategory(category) {
		return nil, ErrCategoryDefaultInvalidCategory
	}
	sources, err := s.repo.CandidateSources(ctx, category)
	if err != nil {
		return nil, err
	}
	out := make([]CategoryDefaultSourceCandidate, 0)
	for _, source := range sources {
		if !categoryDefaultSourceEligible(category, source.Group) {
			continue
		}
		models, err := categoryDefaultSourceModels(source)
		candidate := CategoryDefaultSourceCandidate{Group: source.Group, Models: []string{}, ModelsCount: len(models)}
		if err != nil {
			candidate.InvalidReason = "请先为来源分组配置有限、明确的模型清单"
		}
		for _, model := range models {
			candidate.Models = append(candidate.Models, model.PublicModel)
		}
		out = append(out, candidate)
	}
	return out, nil
}

func normalizeCategoryDefaultSourceIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, ErrCategoryDefaultInvalidSource
	}
	out := append([]int64(nil), ids...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	for i, id := range out {
		if id <= 0 || (i > 0 && out[i-1] == id) {
			return nil, ErrCategoryDefaultInvalidSource
		}
	}
	return out, nil
}

func categoryDefaultInvalidationSet(models []string) map[string]bool {
	out := make(map[string]bool, len(models))
	for _, model := range models {
		out[categoryDefaultModelKey(model)] = true
	}
	return out
}

func categoryDefaultPreviewHash(category string, revision int64, ids []int64, models []CategoryDefaultPublishedModel, conflicts []CategoryDefaultModelConflict, sources []CategoryDefaultSourceSnapshot, invalidated []string) string {
	type sourceEpoch struct{ ID, Epoch int64 }
	epochs := make([]sourceEpoch, 0, len(sources))
	for _, source := range sources {
		if source.Group != nil {
			epochs = append(epochs, sourceEpoch{source.Group.ID, source.DeclarationEpoch})
		}
	}
	sort.Slice(epochs, func(i, j int) bool { return epochs[i].ID < epochs[j].ID })
	invalidatedSet := categoryDefaultInvalidationSet(invalidated)
	invalidatedNames := make([]string, 0, len(invalidatedSet))
	for model := range invalidatedSet {
		invalidatedNames = append(invalidatedNames, model)
	}
	sort.Strings(invalidatedNames)
	data, _ := json.Marshal(struct {
		Category    string
		Revision    int64
		Sources     []int64
		Models      []CategoryDefaultPublishedModel
		Conflicts   []CategoryDefaultModelConflict
		Epochs      []sourceEpoch
		Invalidated []string
	}{category, revision, ids, models, conflicts, epochs, invalidatedNames})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *CategoryDefaultGroupService) Preview(ctx context.Context, category string, sourceIDs []int64) (*CategoryDefaultGroupPreview, error) {
	if !validCategoryDefaultCategory(category) {
		return nil, ErrCategoryDefaultInvalidCategory
	}
	ids, err := normalizeCategoryDefaultSourceIDs(sourceIDs)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.repo.Snapshot(ctx, CategoryDefaultSnapshotRequest{Category: category, AdditionalSourceGroupIDs: ids, IncludeHistory: true})
	if err != nil {
		return nil, err
	}
	previousCatalog, err := projectCategoryDefaultCatalog(snapshot)
	if err != nil {
		return nil, err
	}
	current := snapshot.Config
	sources := categoryDefaultSelectedSources(snapshot.Sources, ids)
	if len(sources) != len(ids) {
		return nil, ErrCategoryDefaultInvalidSource
	}
	models, conflicts, err := buildCategoryDefaultCatalog(category, sources)
	if err != nil {
		return nil, err
	}
	preview := &CategoryDefaultGroupPreview{Category: category, Revision: current.Revision, SourceGroupIDs: ids, PreviewHash: categoryDefaultPreviewHash(category, current.Revision, ids, models, conflicts, sources, current.InvalidatedModels), Added: []CategoryDefaultPublishedModel{}, Modified: []CategoryDefaultPublishedModel{}, Removed: []CategoryDefaultPublishedModel{}, Unchanged: []CategoryDefaultPublishedModel{}, Conflicts: conflicts, ModelsCount: len(models)}
	previous := make(map[string]CategoryDefaultPublishedModel, len(previousCatalog.config.Models))
	for _, model := range previousCatalog.config.Models {
		previous[categoryDefaultModelKey(model.PublicModel)] = model
	}
	for _, model := range models {
		key := categoryDefaultModelKey(model.PublicModel)
		old, exists := previous[key]
		delete(previous, key)
		if !exists {
			preview.Added = append(preview.Added, model)
		} else if old.SourceGroupID != model.SourceGroupID || old.SourceModel != model.SourceModel || old.Fingerprint != model.Fingerprint || old.PublicModel != model.PublicModel {
			preview.Modified = append(preview.Modified, model)
		} else {
			preview.Unchanged = append(preview.Unchanged, model)
		}
	}
	for _, model := range previous {
		preview.Removed = append(preview.Removed, model)
	}
	sort.Slice(preview.Removed, func(i, j int) bool {
		return categoryDefaultModelKey(preview.Removed[i].PublicModel) < categoryDefaultModelKey(preview.Removed[j].PublicModel)
	})
	return preview, nil
}

func (s *CategoryDefaultGroupService) Apply(ctx context.Context, category string, req CategoryDefaultGroupApplyRequest) (*CategoryDefaultGroupConfig, error) {
	if !validCategoryDefaultCategory(category) {
		return nil, ErrCategoryDefaultInvalidCategory
	}
	ids, err := normalizeCategoryDefaultSourceIDs(req.SourceGroupIDs)
	if err != nil {
		return nil, err
	}
	if req.ExpectedRevision < 0 || len(req.PreviewHash) != 64 {
		return nil, ErrCategoryDefaultStalePreview
	}
	result, err := s.repo.Publish(ctx, category, ids, req.ExpectedRevision, func(current *CategoryDefaultGroupConfig, sources []CategoryDefaultSourceSnapshot) ([]CategoryDefaultPublishedModel, error) {
		if current == nil || current.Revision != req.ExpectedRevision || len(sources) != len(ids) {
			return nil, ErrCategoryDefaultStalePreview
		}
		models, conflicts, err := buildCategoryDefaultCatalog(category, sources)
		if err != nil {
			return nil, err
		}
		if len(conflicts) > 0 {
			return nil, ErrCategoryDefaultModelConflict
		}
		if len(models) == 0 {
			return nil, ErrCategoryDefaultInvalidSource
		}
		if categoryDefaultPreviewHash(category, current.Revision, ids, models, conflicts, sources, current.InvalidatedModels) != req.PreviewHash {
			return nil, ErrCategoryDefaultStalePreview
		}
		return models, nil
	})
	if err == nil && s.invalidator != nil {
		s.invalidator.InvalidateAuthCacheByGroupID(ctx, result.GroupID)
	}
	if err != nil {
		return nil, err
	}
	return s.currentConfig(ctx, category)
}

func (s *CategoryDefaultGroupService) SetStatus(ctx context.Context, category string, enabled bool) (*CategoryDefaultGroupConfig, error) {
	if !validCategoryDefaultCategory(category) {
		return nil, ErrCategoryDefaultInvalidCategory
	}
	config, err := s.repo.SetStatus(ctx, category, enabled)
	if err == nil && s.invalidator != nil {
		s.invalidator.InvalidateAuthCacheByGroupID(ctx, config.GroupID)
	}
	if err != nil {
		return nil, err
	}
	return s.currentConfig(ctx, category)
}

func (s *CategoryDefaultGroupService) currentConfig(ctx context.Context, category string) (*CategoryDefaultGroupConfig, error) {
	snapshot, err := s.repo.Snapshot(ctx, CategoryDefaultSnapshotRequest{Category: category})
	if err != nil {
		return nil, err
	}
	projected, err := projectCategoryDefaultCatalog(snapshot)
	if err != nil {
		return nil, err
	}
	return projected.config, nil
}

type categoryDefaultRuntime struct {
	config      *CategoryDefaultGroupConfig
	current     map[string]CategoryDefaultPublishedModel
	conflicts   map[string]bool
	invalidated map[string]bool
	groups      map[int64]*Group
	sources     map[int64]CategoryDefaultSourceSnapshot
}

func (s *CategoryDefaultGroupService) runtime(ctx context.Context, entryID int64) (*categoryDefaultRuntime, error) {
	snapshot, err := s.repo.Snapshot(ctx, CategoryDefaultSnapshotRequest{GroupID: entryID})
	if err != nil {
		return nil, err
	}
	if snapshot == nil || snapshot.Config == nil || snapshot.Config.GroupID != entryID || !snapshot.Config.Enabled {
		return nil, ErrCategoryDefaultSourceUnavailable
	}
	return projectCategoryDefaultCatalog(snapshot)
}

func categoryDefaultUserCanUse(user *User, group *Group) bool {
	return user != nil && user.ID > 0 && user.Status == StatusActive && group != nil && user.CanBindGroup(group.ID, group.IsExclusive)
}

func (s *CategoryDefaultGroupService) ResolveForUser(ctx context.Context, entryID int64, user *User, model string) (*CategoryDefaultModelRoute, error) {
	runtime, err := s.runtime(ctx, entryID)
	if err != nil {
		return nil, err
	}
	key := categoryDefaultModelKey(model)
	if runtime.conflicts[key] {
		return nil, ErrCategoryDefaultModelConflict
	}
	current, exists := runtime.current[key]
	if !exists {
		return nil, ErrCategoryDefaultModelNotAllowed
	}
	group := runtime.groups[current.SourceGroupID]
	if !categoryDefaultUserCanUse(user, group) {
		return nil, ErrCategoryDefaultModelNotAllowed
	}
	if runtime.invalidated[key] {
		return nil, ErrCategoryDefaultPendingSync
	}
	if err := s.validateSourceChannel(ctx, runtime.sources[current.SourceGroupID]); err != nil {
		return nil, err
	}
	return &CategoryDefaultModelRoute{EntryGroupID: entryID, SourceGroup: group, PublicModel: current.PublicModel, SourceModel: current.SourceModel, GeminiNative: current.GeminiNative}, nil
}

func categoryDefaultPlatformAllowed(group *Group, current CategoryDefaultPublishedModel, platform string) bool {
	if platform == "" {
		return true
	}
	if platform == PlatformGemini {
		return current.GeminiNative
	}
	if group.Platform == platform {
		return true
	}
	for _, target := range current.Platforms {
		if target == platform {
			return true
		}
		if platform == PlatformOpenAI && domain.UsesOpenAIGateway(target) {
			return true
		}
	}
	return false
}

func (s *CategoryDefaultGroupService) ModelsForUser(ctx context.Context, entryID int64, user *User, platform string) ([]string, error) {
	runtime, err := s.runtime(ctx, entryID)
	if err != nil {
		return nil, err
	}
	models := make([]string, 0, len(runtime.config.Models))
	for _, current := range runtime.config.Models {
		group := runtime.groups[current.SourceGroupID]
		if !categoryDefaultUserCanUse(user, group) || !categoryDefaultPlatformAllowed(group, current, strings.TrimSpace(platform)) {
			continue
		}
		models = append(models, current.PublicModel)
	}
	sort.Slice(models, func(i, j int) bool { return categoryDefaultModelKey(models[i]) < categoryDefaultModelKey(models[j]) })
	return models, nil
}

func (s *CategoryDefaultGroupService) CanBind(ctx context.Context, entryID int64, user *User) (bool, error) {
	models, err := s.ModelsForUser(ctx, entryID, user, "")
	return len(models) > 0, err
}
