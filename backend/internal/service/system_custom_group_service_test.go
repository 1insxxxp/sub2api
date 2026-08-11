package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type systemCustomGroupRepositoryStub struct {
	SystemCustomGroupRepository
	createdGroup  *Group
	createdModels []SystemCustomGroupModelInput
	updatedGroup  *Group
	updatedModels []SystemCustomGroupModelInput
	stored        *SystemCustomGroup
}

func (s *systemCustomGroupRepositoryStub) Update(_ context.Context, group *Group, models []SystemCustomGroupModelInput) error {
	copy := *group
	s.updatedGroup = &copy
	s.updatedModels = append([]SystemCustomGroupModelInput(nil), models...)
	s.stored = &SystemCustomGroup{Group: copy, Models: make([]SystemCustomGroupModel, 0, len(models))}
	for i, model := range models {
		s.stored.Models = append(s.stored.Models, SystemCustomGroupModel{
			ID: int64(i + 1), GroupID: group.ID, PublicModel: model.PublicModel,
			SourceGroupID: model.SourceGroupID, SourceModel: model.SourceModel, Enabled: model.Enabled,
		})
	}
	return nil
}

func (s *systemCustomGroupRepositoryStub) Create(_ context.Context, group *Group, models []SystemCustomGroupModelInput) error {
	s.createdGroup = group
	s.createdModels = append([]SystemCustomGroupModelInput(nil), models...)
	group.ID = 99
	return nil
}

func (s *systemCustomGroupRepositoryStub) Get(_ context.Context, groupID int64) (*SystemCustomGroup, error) {
	if s.stored != nil {
		return s.stored, nil
	}
	if s.createdGroup == nil || s.createdGroup.ID != groupID {
		return nil, ErrSystemCustomGroupNotFound
	}
	return &SystemCustomGroup{Group: *s.createdGroup}, nil
}

type systemCustomSourceGroupRepositoryStub struct {
	GroupRepository
	groups map[int64]*Group
}

func (s systemCustomSourceGroupRepositoryStub) ExistsByName(context.Context, string) (bool, error) {
	return false, nil
}

func (s systemCustomSourceGroupRepositoryStub) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	group, ok := s.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	copy := *group
	return &copy, nil
}

func (s systemCustomSourceGroupRepositoryStub) ListActive(context.Context) ([]Group, error) {
	groups := make([]Group, 0, len(s.groups))
	for _, group := range s.groups {
		groups = append(groups, *group)
	}
	return groups, nil
}

type systemCustomModelCatalogStub struct {
	models        map[int64][]string
	unschedulable map[int64]bool
}

func (s systemCustomModelCatalogStub) HasSchedulableAccountsForGroupPlatform(_ context.Context, groupID int64, _ string) bool {
	return !s.unschedulable[groupID]
}

func (s systemCustomModelCatalogStub) GetAvailableModels(_ context.Context, groupID *int64, _ string) []string {
	if groupID == nil {
		return nil
	}
	return append([]string(nil), s.models[*groupID]...)
}

func newSystemCustomGroupValidationService(groups map[int64]*Group, models map[int64][]string) *SystemCustomGroupService {
	return NewSystemCustomGroupService(
		&systemCustomGroupRepositoryStub{},
		systemCustomSourceGroupRepositoryStub{groups: groups},
		systemCustomModelCatalogStub{models: models},
	)
}

func activeDirectSystemCustomSource(id int64, platform string) *Group {
	return &Group{ID: id, Name: "source", Status: StatusActive, Platform: platform, Hydrated: true}
}

func TestCreateSystemCustomGroupNormalizesContainer(t *testing.T) {
	repo := &systemCustomGroupRepositoryStub{}
	svc := NewSystemCustomGroupService(
		repo,
		systemCustomSourceGroupRepositoryStub{groups: map[int64]*Group{
			10: activeDirectSystemCustomSource(10, PlatformAnthropic),
		}},
		systemCustomModelCatalogStub{models: map[int64][]string{10: {"claude-sonnet-4"}}},
	)
	description := " shared tavern subscription "
	daily := 20.0

	created, err := svc.Create(context.Background(), CreateSystemCustomGroupRequest{
		Name: "  Tavern Pass  ", Description: &description, DailyLimitUSD: &daily,
		DefaultValidityDays: 30,
		Models:              []SystemCustomGroupModelInput{{PublicModel: " tavern-sonnet ", SourceGroupID: 10, SourceModel: " claude-sonnet-4 ", Enabled: true}},
	})

	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, "Tavern Pass", repo.createdGroup.Name)
	require.Equal(t, "shared tavern subscription", repo.createdGroup.Description)
	require.Equal(t, PlatformComposite, repo.createdGroup.Platform)
	require.Equal(t, SubscriptionTypeSubscription, repo.createdGroup.SubscriptionType)
	require.True(t, repo.createdGroup.IsExclusive)
	require.Equal(t, 1.0, repo.createdGroup.RateMultiplier)
	require.True(t, repo.createdGroup.SystemCustomRoutingEnabled)
	require.True(t, repo.createdGroup.IsSystemCustomRouteGroup())
	require.Equal(t, "tavern-sonnet", repo.createdModels[0].PublicModel)
	require.Equal(t, "claude-sonnet-4", repo.createdModels[0].SourceModel)
}

func TestUpdateSystemCustomGroupNormalizesContainer(t *testing.T) {
	repo := &systemCustomGroupRepositoryStub{stored: &SystemCustomGroup{Group: Group{
		ID: 99, Name: "old", Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription,
		SystemCustomRoutingEnabled: true, IsExclusive: true, RateMultiplier: 1, Status: StatusActive,
	}}}
	svc := NewSystemCustomGroupService(
		repo,
		systemCustomSourceGroupRepositoryStub{groups: map[int64]*Group{10: activeDirectSystemCustomSource(10, PlatformAnthropic)}},
		systemCustomModelCatalogStub{models: map[int64][]string{10: {"model-a"}}},
	)
	description := " changed "

	updated, err := svc.Update(context.Background(), 99, UpdateSystemCustomGroupRequest{
		Name: " changed ", Description: &description, DefaultValidityDays: 60,
		Models: []SystemCustomGroupModelInput{{PublicModel: " alias ", SourceGroupID: 10, SourceModel: " model-a ", Enabled: true}},
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, PlatformComposite, repo.updatedGroup.Platform)
	require.Equal(t, SubscriptionTypeSubscription, repo.updatedGroup.SubscriptionType)
	require.True(t, repo.updatedGroup.IsExclusive)
	require.Equal(t, 1.0, repo.updatedGroup.RateMultiplier)
	require.True(t, repo.updatedGroup.SystemCustomRoutingEnabled)
	require.Equal(t, "alias", repo.updatedModels[0].PublicModel)
}

func TestValidateSystemCustomRoutesRejectsDuplicatePublicNameCaseInsensitive(t *testing.T) {
	svc := newSystemCustomGroupValidationService(
		map[int64]*Group{10: activeDirectSystemCustomSource(10, PlatformAnthropic), 20: activeDirectSystemCustomSource(20, PlatformAnthropic)},
		map[int64][]string{10: {"model-a"}, 20: {"model-b"}},
	)
	models := []SystemCustomGroupModelInput{
		{PublicModel: "Claude-Premium", SourceGroupID: 10, SourceModel: "model-a", Enabled: true},
		{PublicModel: "claude-premium", SourceGroupID: 20, SourceModel: "model-b", Enabled: true},
	}

	err := svc.ValidateRoutes(context.Background(), 0, models)

	require.ErrorIs(t, err, ErrSystemCustomGroupDuplicatePublicModel)
	require.Contains(t, err.Error(), "claude-premium")
}

func TestValidateSystemCustomRoutesRejectsDuplicateSourceModel(t *testing.T) {
	svc := newSystemCustomGroupValidationService(
		map[int64]*Group{10: activeDirectSystemCustomSource(10, PlatformAnthropic)},
		map[int64][]string{10: {"model-a"}},
	)
	models := []SystemCustomGroupModelInput{
		{PublicModel: "alias-a", SourceGroupID: 10, SourceModel: "model-a", Enabled: true},
		{PublicModel: "alias-b", SourceGroupID: 10, SourceModel: "MODEL-A", Enabled: true},
	}

	err := svc.ValidateRoutes(context.Background(), 0, models)

	require.ErrorIs(t, err, ErrSystemCustomGroupDuplicateSourceModel)
	require.Contains(t, err.Error(), "MODEL-A")
}

func TestValidateSystemCustomRoutesRejectsSelfReference(t *testing.T) {
	svc := newSystemCustomGroupValidationService(nil, nil)
	models := []SystemCustomGroupModelInput{{PublicModel: "model-a", SourceGroupID: 42, SourceModel: "model-a", Enabled: true}}

	err := svc.ValidateRoutes(context.Background(), 42, models)

	require.ErrorIs(t, err, ErrSystemCustomGroupSelfReference)
	require.Contains(t, err.Error(), "model-a")
}

func TestValidateSystemCustomRoutesRejectsInactiveOrNestedSource(t *testing.T) {
	tests := []struct {
		name  string
		group *Group
	}{
		{name: "inactive", group: &Group{ID: 10, Status: StatusDisabled, Platform: PlatformAnthropic, Hydrated: true}},
		{name: "composite", group: &Group{ID: 10, Status: StatusActive, Platform: PlatformComposite, Hydrated: true}},
		{name: "nested", group: &Group{ID: 10, Status: StatusActive, Platform: PlatformAnthropic, SystemCustomRoutingEnabled: true, Hydrated: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newSystemCustomGroupValidationService(map[int64]*Group{10: tt.group}, map[int64][]string{10: {"model-a"}})
			err := svc.ValidateRoutes(context.Background(), 0, []SystemCustomGroupModelInput{{PublicModel: "alias", SourceGroupID: 10, SourceModel: "model-a", Enabled: true}})
			require.ErrorIs(t, err, ErrSystemCustomGroupInvalidSourceGroup)
			require.Contains(t, err.Error(), "model-a")
		})
	}
}

func TestValidateSystemCustomRoutesPreservesExplicitAliases(t *testing.T) {
	svc := newSystemCustomGroupValidationService(
		map[int64]*Group{10: activeDirectSystemCustomSource(10, PlatformAnthropic), 20: activeDirectSystemCustomSource(20, PlatformAnthropic)},
		map[int64][]string{10: {"same-model"}, 20: {"same-model"}},
	)
	models := []SystemCustomGroupModelInput{
		{PublicModel: " same-model@premium ", SourceGroupID: 10, SourceModel: " same-model ", Enabled: true},
		{PublicModel: "same-model@value", SourceGroupID: 20, SourceModel: "same-model", Enabled: true},
	}

	err := svc.ValidateRoutes(context.Background(), 0, models)

	require.NoError(t, err)
	require.Equal(t, "same-model@premium", models[0].PublicModel)
	require.Equal(t, "same-model", models[0].SourceModel)
}

func TestValidateSystemCustomRoutesRejectsMissingSourceModel(t *testing.T) {
	svc := newSystemCustomGroupValidationService(
		map[int64]*Group{10: activeDirectSystemCustomSource(10, PlatformAnthropic)},
		map[int64][]string{10: {"model-a"}},
	)

	err := svc.ValidateRoutes(context.Background(), 0, []SystemCustomGroupModelInput{{PublicModel: "alias", SourceGroupID: 10, SourceModel: "missing", Enabled: true}})

	require.ErrorIs(t, err, ErrSystemCustomGroupMissingSourceModel)
	require.Contains(t, err.Error(), "missing")
}

func TestSyncPreviewMarksAddedMissingAndConflictingModels(t *testing.T) {
	repo := &systemCustomGroupRepositoryStub{stored: &SystemCustomGroup{
		Group: Group{ID: 99, Name: "pass", Status: StatusActive, Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true},
		Models: []SystemCustomGroupModel{
			{ID: 1, GroupID: 99, PublicModel: "existing", SourceGroupID: 10, SourceModel: "existing", Enabled: true},
			{ID: 2, GroupID: 99, PublicModel: "missing", SourceGroupID: 10, SourceModel: "removed", Enabled: true},
		},
	}}
	svc := NewSystemCustomGroupService(
		repo,
		systemCustomSourceGroupRepositoryStub{groups: map[int64]*Group{
			10: activeDirectSystemCustomSource(10, PlatformAnthropic),
			20: activeDirectSystemCustomSource(20, PlatformAnthropic),
		}},
		systemCustomModelCatalogStub{models: map[int64][]string{
			10: {"existing", "unique", "shared"},
			20: {"shared"},
		}},
	)

	preview, err := svc.SyncPreview(context.Background(), 99)

	require.NoError(t, err)
	require.Len(t, preview.Added, 1)
	require.Equal(t, "unique", preview.Added[0].PublicModel)
	require.False(t, preview.Added[0].Selected, "new models are preview-only and unselected")
	require.Len(t, preview.Missing, 1)
	require.Equal(t, "missing", preview.Missing[0].PublicModel)
	require.True(t, preview.Missing[0].Enabled, "missing routes remain stored until explicit replacement")
	require.Len(t, preview.Conflicting, 2)
	for _, conflict := range preview.Conflicting {
		require.Equal(t, "shared", conflict.PublicModel)
		require.True(t, errors.Is(conflict.Err, ErrSystemCustomGroupDuplicatePublicModel))
	}
}

func TestSystemCustomGroupCandidatesOnlyIncludeActiveDirectSourcesAndRespectCustomModelList(t *testing.T) {
	direct := activeDirectSystemCustomSource(10, PlatformAnthropic)
	direct.Name = "direct"
	direct.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"model-b", "not-available"}}
	svc := NewSystemCustomGroupService(
		&systemCustomGroupRepositoryStub{},
		systemCustomSourceGroupRepositoryStub{groups: map[int64]*Group{
			10: direct,
			20: {ID: 20, Name: "inactive", Status: StatusDisabled, Platform: PlatformAnthropic},
			30: {ID: 30, Name: "composite", Status: StatusActive, Platform: PlatformComposite},
			40: {ID: 40, Name: "nested", Status: StatusActive, Platform: PlatformAnthropic, SystemCustomRoutingEnabled: true},
		}},
		systemCustomModelCatalogStub{models: map[int64][]string{
			10: {"model-a", "model-b"}, 20: {"inactive-model"}, 30: {"composite-model"}, 40: {"nested-model"},
		}},
	)

	candidates, err := svc.Candidates(context.Background())

	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, int64(10), candidates[0].Group.ID)
	require.Equal(t, []string{"model-b"}, candidates[0].Models)
}

func TestSystemCustomGroupModelsWildcardMappingAllowsConcreteSelectedModel(t *testing.T) {
	source := activeDirectSystemCustomSource(10, PlatformAnthropic)
	source.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"claude-sonnet-4-6"}}
	svc := NewSystemCustomGroupService(
		&systemCustomGroupRepositoryStub{},
		systemCustomSourceGroupRepositoryStub{groups: map[int64]*Group{10: source}},
		systemCustomModelCatalogStub{models: map[int64][]string{10: {"claude-*"}}},
	)

	candidates, err := svc.Candidates(context.Background())
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, []string{"claude-sonnet-4-6"}, candidates[0].Models)
	require.NoError(t, svc.ValidateRoutes(context.Background(), 0, []SystemCustomGroupModelInput{
		{PublicModel: "sonnet", SourceGroupID: 10, SourceModel: "claude-sonnet-4-6", Enabled: true},
	}))
}

func TestSystemCustomGroupModelsEmptyMappingFallsBackToPlatformDefaultsAndFiltersSelection(t *testing.T) {
	source := activeDirectSystemCustomSource(10, PlatformOpenAI)
	source.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"gpt-5.5", "legacy-gpt-2024", "gpt-5.4"}}
	withoutAccounts := activeDirectSystemCustomSource(20, PlatformOpenAI)
	withoutAccounts.ModelsListConfig = source.ModelsListConfig
	svc := NewSystemCustomGroupService(
		&systemCustomGroupRepositoryStub{},
		systemCustomSourceGroupRepositoryStub{groups: map[int64]*Group{10: source, 20: withoutAccounts}},
		systemCustomModelCatalogStub{models: map[int64][]string{10: nil, 20: nil}, unschedulable: map[int64]bool{20: true}},
	)

	candidates, err := svc.Candidates(context.Background())
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, []string{"gpt-5.5", "gpt-5.4"}, candidates[0].Models)
	require.NoError(t, svc.ValidateRoutes(context.Background(), 0, []SystemCustomGroupModelInput{
		{PublicModel: "gpt", SourceGroupID: 10, SourceModel: "gpt-5.5", Enabled: true},
	}))
}

func TestSystemCustomGroupModelsAnthropicCustomListMergesDefaultsAndMappedModels(t *testing.T) {
	source := activeDirectSystemCustomSource(10, PlatformAnthropic)
	source.ModelsListConfig = GroupModelsListConfig{
		Enabled: true,
		Models:  []string{"claude-fable-5", "claude-opus-4-8", "deepseek-v4-pro"},
	}
	svc := NewSystemCustomGroupService(
		&systemCustomGroupRepositoryStub{},
		systemCustomSourceGroupRepositoryStub{groups: map[int64]*Group{10: source}},
		systemCustomModelCatalogStub{models: map[int64][]string{10: {"deepseek-v4-pro"}}},
	)

	candidates, err := svc.Candidates(context.Background())
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, []string{"claude-fable-5", "claude-opus-4-8", "deepseek-v4-pro"}, candidates[0].Models)
	for _, model := range candidates[0].Models {
		require.NoError(t, svc.ValidateRoutes(context.Background(), 0, []SystemCustomGroupModelInput{
			{PublicModel: model, SourceGroupID: 10, SourceModel: model, Enabled: true},
		}))
	}
}
