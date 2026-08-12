package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type systemCustomRouteRepoStub struct {
	SystemCustomGroupRepository
	route        *SystemCustomGroupModel
	models       []SystemCustomGroupModel
	err          error
	listErr      error
	requestedID  int64
	requested    string
	calls        int
	listCalls    int
	enabledOnly  bool
	sourceGroups map[int64]*Group
}

func (s *systemCustomRouteRepoStub) ResolveModel(_ context.Context, groupID int64, publicModel string) (*SystemCustomGroupModel, error) {
	s.calls++
	s.requestedID = groupID
	s.requested = publicModel
	return s.route, s.err
}

func (s *systemCustomRouteRepoStub) ListModels(_ context.Context, groupID int64, enabledOnly bool) ([]SystemCustomGroupModel, error) {
	s.listCalls++
	s.requestedID = groupID
	s.enabledOnly = enabledOnly
	models := append([]SystemCustomGroupModel(nil), s.models...)
	for i := range models {
		if models[i].SourceGroup == nil {
			models[i].SourceGroup = s.sourceGroups[models[i].SourceGroupID]
		}
	}
	return models, s.listErr
}

type systemCustomSourceRepoStub struct {
	GroupRepository
	groups    map[int64]*Group
	err       error
	calls     int
	liteCalls int
}

func (s *systemCustomSourceRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	s.calls++
	return s.get(id)
}

func (s *systemCustomSourceRepoStub) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	s.liteCalls++
	return s.get(id)
}

func (s *systemCustomSourceRepoStub) get(id int64) (*Group, error) {
	if s.err != nil {
		return nil, s.err
	}
	group, ok := s.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	return group, nil
}

func TestResolveSystemCustomGroupModelClonesKeyAndPreservesBillingIdentity(t *testing.T) {
	billingGroupID, sourceGroupID := int64(25), int64(42)
	staleCustomGroupID := int64(99)
	billingGroup := &Group{
		ID: billingGroupID, Platform: PlatformComposite, Status: StatusActive, Hydrated: true,
		SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true,
	}
	sourceGroup := &Group{
		ID: sourceGroupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true,
		IsExclusive: true,
	}
	fallbackID, invalidFallbackID := int64(77), int64(78)
	sourceGroup.FallbackGroupID = &fallbackID
	sourceGroup.FallbackGroupIDOnInvalidRequest = &invalidFallbackID
	original := &APIKey{
		ID: 7, UserID: 9, GroupID: &billingGroupID, CustomGroupID: &staleCustomGroupID, Group: billingGroup,
		User: &User{ID: 9, AllowedGroups: []int64{999}},
	}
	routeRepo := &systemCustomRouteRepoStub{route: &SystemCustomGroupModel{
		GroupID: billingGroupID, PublicModel: "Claude-Monthly", SourceGroupID: sourceGroupID,
		SourceModel: "claude-sonnet-4-5", Enabled: true,
	}}
	groupRepo := &systemCustomSourceRepoStub{groups: map[int64]*Group{sourceGroupID: sourceGroup}}
	svc := &APIKeyService{systemCustomGroupRepo: routeRepo, groupRepo: groupRepo}

	resolution, err := svc.ResolveSystemCustomGroupModel(context.Background(), original, "claude-monthly")

	require.NoError(t, err)
	require.NotNil(t, resolution)
	require.NotSame(t, original, resolution.APIKey)
	require.Equal(t, sourceGroupID, requirePointerValue(t, resolution.APIKey.GroupID))
	require.Nil(t, resolution.APIKey.CustomGroupID, "the authoritative system route must not be resolved again by user-custom middleware")
	require.NotSame(t, sourceGroup, resolution.APIKey.Group, "the live repository group must remain immutable")
	require.Nil(t, resolution.APIKey.Group.FallbackGroupID, "system routes must never escape through the source group's normal fallback")
	require.Nil(t, resolution.APIKey.Group.FallbackGroupIDOnInvalidRequest, "invalid requests must not escape through a fallback either")
	require.Equal(t, fallbackID, requirePointerValue(t, sourceGroup.FallbackGroupID), "the repository entity must not be mutated")
	require.Equal(t, invalidFallbackID, requirePointerValue(t, sourceGroup.FallbackGroupIDOnInvalidRequest), "the repository entity must not be mutated")
	require.Equal(t, billingGroupID, requirePointerValue(t, original.GroupID))
	require.Equal(t, staleCustomGroupID, requirePointerValue(t, original.CustomGroupID))
	require.Same(t, billingGroup, original.Group)
	require.Equal(t, billingGroupID, resolution.BillingGroupID)
	require.Equal(t, sourceGroupID, resolution.SourceGroupID)
	require.Equal(t, "Claude-Monthly", resolution.PublicModel)
	require.Equal(t, "claude-sonnet-4-5", resolution.SourceModel)
	require.Equal(t, PlatformAnthropic, resolution.SourcePlatform)
	require.Equal(t, billingGroupID, routeRepo.requestedID)
	require.Equal(t, "claude-monthly", routeRepo.requested)
	require.Zero(t, groupRepo.calls, "the resolver must avoid the aggregate-heavy group lookup")
	require.Equal(t, 1, groupRepo.liteCalls)
}

func TestResolveSystemCustomGroupModelReturnsUnhandledForOrdinaryKey(t *testing.T) {
	groupID := int64(3)
	routeRepo := &systemCustomRouteRepoStub{}
	groupRepo := &systemCustomSourceRepoStub{}
	svc := &APIKeyService{systemCustomGroupRepo: routeRepo, groupRepo: groupRepo}
	key := &APIKey{GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformAnthropic, Status: StatusActive}}

	resolution, err := svc.ResolveSystemCustomGroupModel(context.Background(), key, "claude-sonnet-4-5")

	require.NoError(t, err)
	require.Nil(t, resolution)
	require.Zero(t, routeRepo.calls)
	require.Zero(t, groupRepo.calls)
	require.Zero(t, groupRepo.liteCalls)
}

func TestResolveSystemCustomGroupModelRejectsUnknownOrDisabledAlias(t *testing.T) {
	billingGroupID := int64(25)
	key := newSystemCustomAPIKey(billingGroupID)
	tests := []struct {
		name  string
		repo  *systemCustomRouteRepoStub
		model string
	}{
		{name: "unknown", repo: &systemCustomRouteRepoStub{err: ErrSystemCustomGroupRouteNotFound}, model: "missing"},
		{name: "disabled", repo: &systemCustomRouteRepoStub{route: &SystemCustomGroupModel{GroupID: billingGroupID, PublicModel: "disabled", SourceGroupID: 42, SourceModel: "claude-sonnet-4-5", Enabled: false}}, model: "disabled"},
		{name: "non exact repository result", repo: &systemCustomRouteRepoStub{route: &SystemCustomGroupModel{GroupID: billingGroupID, PublicModel: "another", SourceGroupID: 42, SourceModel: "claude-sonnet-4-5", Enabled: true}}, model: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groupRepo := &systemCustomSourceRepoStub{groups: map[int64]*Group{42: directSourceGroup(42, PlatformAnthropic)}}
			svc := &APIKeyService{systemCustomGroupRepo: tt.repo, groupRepo: groupRepo}

			resolution, err := svc.ResolveSystemCustomGroupModel(context.Background(), key, tt.model)

			require.Nil(t, resolution)
			require.ErrorIs(t, err, ErrSystemCustomGroupModelNotAllowed)
			require.Equal(t, 403, infraerrors.Code(err))
			require.Zero(t, groupRepo.calls, "a rejected alias must not try any source/fallback group")
			require.Zero(t, groupRepo.liteCalls)
		})
	}
}

func TestResolveSystemCustomGroupModelFailsClosedForUnavailableSources(t *testing.T) {
	billingGroupID := int64(25)
	tests := []struct {
		name      string
		source    *Group
		lookupErr error
	}{
		{name: "missing", lookupErr: ErrGroupNotFound},
		{name: "inactive", source: &Group{ID: 42, Platform: PlatformAnthropic, Status: StatusDisabled, Hydrated: true}},
		{name: "composite", source: &Group{ID: 42, Platform: PlatformComposite, Status: StatusActive, Hydrated: true}},
		{name: "nested system custom", source: &Group{ID: 42, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true, SystemCustomRoutingEnabled: true}},
		{name: "untrusted source snapshot", source: &Group{ID: 42, Platform: PlatformAnthropic, Status: StatusActive}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routeRepo := &systemCustomRouteRepoStub{route: &SystemCustomGroupModel{
				GroupID: billingGroupID, PublicModel: "monthly", SourceGroupID: 42,
				SourceModel: "claude-sonnet-4-5", Enabled: true,
			}}
			groups := map[int64]*Group{}
			if tt.source != nil {
				groups[42] = tt.source
			}
			groupRepo := &systemCustomSourceRepoStub{groups: groups, err: tt.lookupErr}
			svc := &APIKeyService{systemCustomGroupRepo: routeRepo, groupRepo: groupRepo}

			resolution, err := svc.ResolveSystemCustomGroupModel(context.Background(), newSystemCustomAPIKey(billingGroupID), "monthly")

			require.Nil(t, resolution)
			require.ErrorIs(t, err, ErrSystemCustomGroupSourceUnavailable)
			require.Equal(t, 503, infraerrors.Code(err))
			require.Equal(t, 1, routeRepo.calls)
			require.Zero(t, groupRepo.calls)
			require.Equal(t, 1, groupRepo.liteCalls, "resolver must not try another source group")
		})
	}
}

func TestResolveSystemCustomGroupModelTreatsRepositoryFailureAsUnavailable(t *testing.T) {
	billingGroupID := int64(25)
	svc := &APIKeyService{
		systemCustomGroupRepo: &systemCustomRouteRepoStub{err: errors.New("database offline")},
		groupRepo:             &systemCustomSourceRepoStub{},
	}

	resolution, err := svc.ResolveSystemCustomGroupModel(context.Background(), newSystemCustomAPIKey(billingGroupID), "monthly")

	require.Nil(t, resolution)
	require.ErrorIs(t, err, ErrSystemCustomGroupSourceUnavailable)
	require.Equal(t, 503, infraerrors.Code(err))
}

func TestListSystemCustomGroupModelsReturnsOnlyLiveEnabledAliases(t *testing.T) {
	billingGroupID := int64(25)
	sourceGroups := map[int64]*Group{
		10: directSourceGroup(10, PlatformAnthropic),
		20: directSourceGroup(20, PlatformGemini),
		21: directSourceGroup(21, PlatformOpenAI),
		30: {ID: 30, Platform: PlatformAnthropic, Status: StatusDisabled, Hydrated: true},
		40: directSourceGroup(40, PlatformGemini),
	}
	routeRepo := &systemCustomRouteRepoStub{models: []SystemCustomGroupModel{
		{GroupID: billingGroupID, PublicModel: "Zulu", SourceGroupID: 10, SourceModel: "claude-sonnet-4", Enabled: true},
		{GroupID: billingGroupID, PublicModel: "alpha", SourceGroupID: 20, SourceModel: "gemini-2.5-flash", Enabled: true},
		{GroupID: billingGroupID, PublicModel: "ALPHA", SourceGroupID: 21, SourceModel: "gpt-5.6", Enabled: true},
		{GroupID: billingGroupID, PublicModel: "missing-model", SourceGroupID: 10, SourceModel: "claude-opus-missing", Enabled: true},
		{GroupID: billingGroupID, PublicModel: "disabled-route", SourceGroupID: 10, SourceModel: "claude-sonnet-4", Enabled: false},
		{GroupID: billingGroupID, PublicModel: "disabled-source", SourceGroupID: 30, SourceModel: "claude-sonnet-4", Enabled: true},
		{GroupID: billingGroupID, PublicModel: "unschedulable", SourceGroupID: 40, SourceModel: "gemini-2.5-flash", Enabled: true},
	}, sourceGroups: sourceGroups}
	svc := &APIKeyService{
		systemCustomGroupRepo: routeRepo,
		groupRepo:             &systemCustomSourceRepoStub{groups: sourceGroups},
		systemCustomModelCatalog: systemCustomModelCatalogStub{
			models: map[int64][]string{
				10: {"claude-sonnet-4"},
				20: {"gemini-2.5-flash"},
				21: {"gpt-5.6"},
				40: {"gemini-2.5-flash"},
			},
			unschedulable: map[int64]bool{40: true},
		},
	}

	models, err := svc.ListSystemCustomGroupModels(context.Background(), newSystemCustomAPIKey(billingGroupID), "")

	require.NoError(t, err)
	require.Equal(t, []string{"alpha", "Zulu"}, models, "aliases must be case-insensitively deduplicated and stably sorted")
	require.Equal(t, 1, routeRepo.listCalls)
	require.Equal(t, billingGroupID, routeRepo.requestedID)
	require.True(t, routeRepo.enabledOnly)
}

func TestListSystemCustomGroupModelsFiltersByLiveSourcePlatform(t *testing.T) {
	billingGroupID := int64(25)
	sourceGroups := map[int64]*Group{
		10: directSourceGroup(10, PlatformAnthropic),
		20: directSourceGroup(20, PlatformGemini),
	}
	routeRepo := &systemCustomRouteRepoStub{models: []SystemCustomGroupModel{
		{GroupID: billingGroupID, PublicModel: "claude-monthly", SourceGroupID: 10, SourceModel: "claude-sonnet-4", Enabled: true},
		{GroupID: billingGroupID, PublicModel: "gemini-monthly", SourceGroupID: 20, SourceModel: "gemini-2.5-flash", Enabled: true},
	}, sourceGroups: sourceGroups}
	svc := &APIKeyService{
		systemCustomGroupRepo: routeRepo,
		groupRepo:             &systemCustomSourceRepoStub{groups: sourceGroups},
		systemCustomModelCatalog: systemCustomModelCatalogStub{models: map[int64][]string{
			10: {"claude-sonnet-4"}, 20: {"gemini-2.5-flash"},
		}},
	}

	models, err := svc.ListSystemCustomGroupModels(context.Background(), newSystemCustomAPIKey(billingGroupID), PlatformGemini)

	require.NoError(t, err)
	require.Equal(t, []string{"gemini-monthly"}, models)
}

func TestListSystemCustomGroupModelsUsesLiveDefaultAndCustomListSemantics(t *testing.T) {
	billingGroupID := int64(25)
	routeRepo := &systemCustomRouteRepoStub{models: []SystemCustomGroupModel{
		{GroupID: billingGroupID, PublicModel: "default-openai", SourceGroupID: 50, SourceModel: "gpt-5.6", Enabled: true},
		{GroupID: billingGroupID, PublicModel: "selected-gemini", SourceGroupID: 60, SourceModel: "gemini-2.5-pro", Enabled: true},
		{GroupID: billingGroupID, PublicModel: "hidden-gemini", SourceGroupID: 60, SourceModel: "gemini-2.5-flash", Enabled: true},
	}}
	customGemini := directSourceGroup(60, PlatformGemini)
	customGemini.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"gemini-2.5-pro"}}
	sourceGroups := map[int64]*Group{
		50: directSourceGroup(50, PlatformOpenAI),
		60: customGemini,
	}
	routeRepo.sourceGroups = sourceGroups
	svc := &APIKeyService{
		systemCustomGroupRepo: routeRepo,
		groupRepo:             &systemCustomSourceRepoStub{groups: sourceGroups},
		systemCustomModelCatalog: systemCustomModelCatalogStub{models: map[int64][]string{
			50: nil,
			60: {"gemini-2.5-flash", "gemini-2.5-pro"},
		}},
	}

	models, err := svc.ListSystemCustomGroupModels(context.Background(), newSystemCustomAPIKey(billingGroupID), "")

	require.NoError(t, err)
	require.Equal(t, []string{"default-openai", "selected-gemini"}, models)
}

func TestListSystemCustomGroupModelsUsesSchedulerWildcardSupport(t *testing.T) {
	billingGroupID := int64(25)
	source := directSourceGroup(10, PlatformAnthropic)
	routeRepo := &systemCustomRouteRepoStub{models: []SystemCustomGroupModel{
		{GroupID: billingGroupID, PublicModel: "sonnet-monthly", SourceGroupID: source.ID, SourceModel: "claude-sonnet-4-6", SourceGroup: source, Enabled: true},
	}}
	svc := &APIKeyService{
		systemCustomGroupRepo: routeRepo,
		groupRepo:             &systemCustomSourceRepoStub{groups: map[int64]*Group{source.ID: source}},
		systemCustomModelCatalog: systemCustomModelCatalogStub{models: map[int64][]string{
			source.ID: {"claude-*"},
		}},
	}

	models, err := svc.ListSystemCustomGroupModels(context.Background(), newSystemCustomAPIKey(billingGroupID), "")

	require.NoError(t, err)
	require.Equal(t, []string{"sonnet-monthly"}, models, "the model list must use the same wildcard support semantics as account scheduling")
}

func TestListSystemCustomGroupModelsLoadsAllSourcesInBoundedBatches(t *testing.T) {
	const sourceCount = 50
	billingGroupID := int64(25)
	routes := make([]SystemCustomGroupModel, 0, sourceCount)
	groups := make(map[int64]*Group, sourceCount)
	modelsByGroup := make(map[int64][]string, sourceCount)
	for i := 0; i < sourceCount; i++ {
		sourceID := int64(1000 + i)
		sourceModel := fmt.Sprintf("model-%03d", i)
		source := directSourceGroup(sourceID, PlatformAnthropic)
		groups[sourceID] = source
		modelsByGroup[sourceID] = []string{sourceModel}
		routes = append(routes, SystemCustomGroupModel{
			GroupID: billingGroupID, PublicModel: fmt.Sprintf("alias-%03d", i),
			SourceGroupID: sourceID, SourceModel: sourceModel, SourceGroup: source, Enabled: true,
		})
	}
	routeRepo := &systemCustomRouteRepoStub{models: routes}
	groupRepo := &systemCustomSourceRepoStub{groups: groups}
	catalog := &countingSystemCustomModelCatalogStub{models: modelsByGroup}
	svc := &APIKeyService{
		systemCustomGroupRepo:    routeRepo,
		groupRepo:                groupRepo,
		systemCustomModelCatalog: catalog,
	}

	models, err := svc.ListSystemCustomGroupModels(context.Background(), newSystemCustomAPIKey(billingGroupID), "")

	require.NoError(t, err)
	require.Len(t, models, sourceCount)
	require.Equal(t, 1, routeRepo.listCalls)
	require.Zero(t, groupRepo.liteCalls, "source groups already attached to the route snapshot must not be reloaded one by one")
	require.Equal(t, 1, catalog.batchCalls, "all source account/model availability must be loaded in one bounded batch")
	require.Zero(t, catalog.hasCalls+catalog.getCalls, "the list path must not use per-source catalog calls")
}

type countingSystemCustomModelCatalogStub struct {
	models     map[int64][]string
	hasCalls   int
	getCalls   int
	batchCalls int
}

func (s *countingSystemCustomModelCatalogStub) HasSchedulableAccountsForGroupPlatform(_ context.Context, groupID int64, _ string) bool {
	s.hasCalls++
	_, ok := s.models[groupID]
	return ok
}

func (s *countingSystemCustomModelCatalogStub) GetAvailableModels(_ context.Context, groupID *int64, _ string) []string {
	s.getCalls++
	if groupID == nil {
		return nil
	}
	return append([]string(nil), s.models[*groupID]...)
}

func (s *countingSystemCustomModelCatalogStub) ListSystemCustomGroupModelAvailability(_ context.Context, sources []SystemCustomGroupModelListSource) (SystemCustomGroupModelAvailability, error) {
	s.batchCalls++
	availability := make(SystemCustomGroupModelAvailability, len(sources))
	for _, source := range sources {
		available := make(map[string]bool, len(source.Models))
		for _, model := range source.Models {
			available[model] = CustomModelsListAllowsModel(s.models[source.Group.ID], model)
		}
		availability[source.Group.ID] = available
	}
	return availability, nil
}

func TestListSystemCustomGroupModelsFailsClosedWhenContainerOrDependenciesAreInvalid(t *testing.T) {
	ordinaryID := int64(3)
	ordinaryKey := &APIKey{GroupID: &ordinaryID, Group: directSourceGroup(ordinaryID, PlatformAnthropic)}
	configured := &APIKeyService{
		systemCustomGroupRepo:    &systemCustomRouteRepoStub{},
		groupRepo:                &systemCustomSourceRepoStub{},
		systemCustomModelCatalog: systemCustomModelCatalogStub{},
	}

	models, err := configured.ListSystemCustomGroupModels(context.Background(), ordinaryKey, "")
	require.Nil(t, models)
	require.ErrorIs(t, err, ErrSystemCustomGroupNotFound)

	models, err = (&APIKeyService{}).ListSystemCustomGroupModels(context.Background(), newSystemCustomAPIKey(25), "")
	require.Nil(t, models)
	require.ErrorIs(t, err, ErrSystemCustomGroupSourceUnavailable)
}

func newSystemCustomAPIKey(groupID int64) *APIKey {
	return &APIKey{
		ID: 7, UserID: 9, GroupID: &groupID,
		Group: &Group{
			ID: groupID, Platform: PlatformComposite, Status: StatusActive, Hydrated: true,
			SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true,
		},
		User: &User{ID: 9},
	}
}

func directSourceGroup(id int64, platform string) *Group {
	return &Group{ID: id, Platform: platform, Status: StatusActive, Hydrated: true}
}

func requirePointerValue(t *testing.T, value *int64) int64 {
	t.Helper()
	require.NotNil(t, value)
	return *value
}
