//go:build unit

package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type systemCustomRouteRepoStub struct {
	SystemCustomGroupRepository
	group        *SystemCustomGroup
	route        *SystemCustomGroupModel
	models       []SystemCustomGroupModel
	err          error
	listErr      error
	requestedID  int64
	requested    string
	calls        int
	listCalls    int
	getCalls     int
	runtimeCalls int
	enabledOnly  bool
	sourceGroups map[int64]*Group
}

func (s *systemCustomRouteRepoStub) Get(_ context.Context, groupID int64) (*SystemCustomGroup, error) {
	s.getCalls++
	s.requestedID = groupID
	if s.err != nil {
		return nil, s.err
	}
	if s.group == nil {
		return nil, ErrSystemCustomGroupNotFound
	}
	return s.group, nil
}

func (s *systemCustomRouteRepoStub) GetRuntime(_ context.Context, groupID int64) (*SystemCustomGroup, error) {
	s.runtimeCalls++
	s.requestedID = groupID
	if s.err != nil {
		return nil, s.err
	}
	if s.group == nil {
		return nil, ErrSystemCustomGroupNotFound
	}
	clone := *s.group
	clone.Models = nil
	return &clone, nil
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

func TestResolveSystemCustomGroupModelTreatsRepositoryFailureAsUnavailable(t *testing.T) {
	billingGroupID := int64(25)
	svc := &APIKeyService{
		systemCustomGroupRepo:    &systemCustomRouteRepoStub{err: errors.New("database offline")},
		systemCustomModelCatalog: &systemCustomRuntimeCatalogStub{catalog: runtimeCatalogForTests(nil)},
	}

	resolution, err := svc.ResolveSystemCustomGroupModel(context.Background(), newSystemCustomAPIKey(billingGroupID), "monthly")

	require.Nil(t, resolution)
	require.ErrorIs(t, err, ErrSystemCustomGroupSourceUnavailable)
	require.Equal(t, 503, infraerrors.Code(err))
}

type systemCustomRuntimeCatalogStub struct {
	catalog        *SystemCustomGroupRuntimeCatalog
	err            error
	calls          int
	sources        []SystemCustomGroupSource
	platform       string
	priced         bool
	resolveCalls   int
	requestedModel string
}

func (s *systemCustomRuntimeCatalogStub) BuildSystemCustomGroupModelCatalog(ctx context.Context, sources []SystemCustomGroupSource, platform string) (*SystemCustomGroupRuntimeCatalog, error) {
	s.calls++
	s.sources = append([]SystemCustomGroupSource(nil), sources...)
	s.platform = platform
	_, s.priced = gatewayTokenRequestPricingAtFromContext(ctx)
	return s.catalog, s.err
}

func (s *systemCustomRuntimeCatalogStub) ResolveSystemCustomGroupModelCatalog(ctx context.Context, sources []SystemCustomGroupSource, platform, model string) ([]SystemCustomGroupRuntimeCandidate, bool, error) {
	s.resolveCalls++
	s.sources = append([]SystemCustomGroupSource(nil), sources...)
	s.platform = platform
	s.requestedModel = model
	_, s.priced = gatewayTokenRequestPricingAtFromContext(ctx)
	if s.err != nil {
		return nil, false, s.err
	}
	candidates, advertised := s.catalog.Resolve(model)
	return candidates, advertised, nil
}

func runtimeCatalogForTests(entries map[string][]SystemCustomGroupRuntimeCandidate, advertised ...string) *SystemCustomGroupRuntimeCatalog {
	catalog := &SystemCustomGroupRuntimeCatalog{
		availableModels: make(map[string]string),
		candidates:      make(map[string][]SystemCustomGroupRuntimeCandidate),
		advertised:      make(map[string]struct{}),
	}
	for _, model := range advertised {
		catalog.advertised[strings.ToLower(model)] = struct{}{}
	}
	for model, candidates := range entries {
		key := strings.ToLower(model)
		catalog.advertised[key] = struct{}{}
		catalog.candidates[key] = append([]SystemCustomGroupRuntimeCandidate(nil), candidates...)
		if len(candidates) > 0 {
			catalog.availableModels[key] = candidates[0].PublicModel
		}
	}
	return catalog
}

func TestResolveSystemCustomGroupModelUsesDynamicCatalogAndNeverStaticRoutes(t *testing.T) {
	billingGroupID := int64(25)
	source := directSourceGroup(42, PlatformAnthropic)
	fallbackID, invalidFallbackID := int64(77), int64(78)
	source.FallbackGroupID = &fallbackID
	source.FallbackGroupIDOnInvalidRequest = &invalidFallbackID
	container := &SystemCustomGroup{
		Group:   *newSystemCustomAPIKey(billingGroupID).Group,
		Sources: []SystemCustomGroupSource{{SourceGroupID: source.ID, Priority: 1, SourceGroup: source}},
		Models:  []SystemCustomGroupModel{{PublicModel: "retained-static", SourceGroupID: 999, SourceModel: "must-not-run", Enabled: true}},
	}
	repo := &systemCustomRouteRepoStub{group: container}
	catalog := &systemCustomRuntimeCatalogStub{catalog: runtimeCatalogForTests(map[string][]SystemCustomGroupRuntimeCandidate{
		"Claude-Live": {{
			SourceGroup: *source, PublicModel: "Claude-Live", SourceModel: "Claude-Live",
			AllowedAccounts: NewSystemCustomGroupAccountAllowlist([]int64{91, 92}),
		}},
	})}
	svc := &APIKeyService{systemCustomGroupRepo: repo, systemCustomModelCatalog: catalog}

	resolution, err := svc.ResolveSystemCustomGroupModel(context.Background(), newSystemCustomAPIKey(billingGroupID), "claude-live")

	require.NoError(t, err)
	require.Equal(t, int64(42), resolution.SourceGroupID)
	require.Equal(t, billingGroupID, resolution.BillingGroupID)
	require.Equal(t, "Claude-Live", resolution.PublicModel)
	require.Equal(t, "Claude-Live", resolution.SourceModel)
	require.Equal(t, PlatformAnthropic, resolution.SourcePlatform)
	require.Equal(t, []int64{91, 92}, resolution.AllowedAccounts.IDs())
	require.Equal(t, int64(42), requirePointerValue(t, resolution.APIKey.GroupID))
	require.Nil(t, resolution.APIKey.CustomGroupID)
	require.Nil(t, resolution.APIKey.Group.FallbackGroupID)
	require.Nil(t, resolution.APIKey.Group.FallbackGroupIDOnInvalidRequest)
	require.Equal(t, fallbackID, requirePointerValue(t, source.FallbackGroupID))
	require.Equal(t, invalidFallbackID, requirePointerValue(t, source.FallbackGroupIDOnInvalidRequest))
	require.Equal(t, 1, repo.runtimeCalls)
	require.Zero(t, repo.getCalls, "runtime resolution must not eager-load retained rollback routes")
	require.Zero(t, repo.calls, "retained ResolveModel must remain rollback-only")
	require.Zero(t, repo.listCalls, "retained ListModels must remain rollback-only")
	require.Zero(t, catalog.calls, "runtime resolution must not build the full model catalog")
	require.Equal(t, 1, catalog.resolveCalls)
	require.Equal(t, "claude-live", catalog.requestedModel)
	require.True(t, catalog.priced, "runtime resolution must evaluate the same profit-control request context as dispatch")
}

func TestResolveSystemCustomGroupModelDistinguishesAbsentFromUnavailable(t *testing.T) {
	billingGroupID := int64(25)
	container := &SystemCustomGroup{Group: *newSystemCustomAPIKey(billingGroupID).Group}
	for _, tt := range []struct {
		name    string
		catalog *SystemCustomGroupRuntimeCatalog
		want    error
	}{
		{name: "absent", catalog: runtimeCatalogForTests(nil), want: ErrSystemCustomGroupModelNotAllowed},
		{name: "all unavailable", catalog: runtimeCatalogForTests(nil, "known-model"), want: ErrSystemCustomGroupSourceUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &systemCustomRouteRepoStub{group: container}
			catalog := &systemCustomRuntimeCatalogStub{catalog: tt.catalog}
			svc := &APIKeyService{systemCustomGroupRepo: repo, systemCustomModelCatalog: catalog}

			resolution, err := svc.ResolveSystemCustomGroupModel(context.Background(), newSystemCustomAPIKey(billingGroupID), "known-model")

			require.Nil(t, resolution)
			require.ErrorIs(t, err, tt.want)
			require.Equal(t, 1, repo.runtimeCalls)
			require.Zero(t, repo.getCalls)
			require.Zero(t, repo.calls+repo.listCalls)
			require.Zero(t, catalog.calls)
			require.Equal(t, 1, catalog.resolveCalls)
		})
	}
}

func TestListSystemCustomGroupModelsUsesOneDynamicSnapshotAndPlatformFilter(t *testing.T) {
	billingGroupID := int64(25)
	source := directSourceGroup(42, PlatformGemini)
	container := &SystemCustomGroup{
		Group:   *newSystemCustomAPIKey(billingGroupID).Group,
		Sources: []SystemCustomGroupSource{{SourceGroupID: source.ID, Priority: 1, SourceGroup: source}},
		Models:  []SystemCustomGroupModel{{PublicModel: "retained-static", Enabled: true}},
	}
	repo := &systemCustomRouteRepoStub{group: container}
	catalog := &systemCustomRuntimeCatalogStub{catalog: runtimeCatalogForTests(map[string][]SystemCustomGroupRuntimeCandidate{
		"gemini-live": {{SourceGroup: *source, PublicModel: "gemini-live", SourceModel: "gemini-live"}},
	})}
	svc := &APIKeyService{systemCustomGroupRepo: repo, systemCustomModelCatalog: catalog}

	models, err := svc.ListSystemCustomGroupModels(context.Background(), newSystemCustomAPIKey(billingGroupID), PlatformGemini)

	require.NoError(t, err)
	require.Equal(t, []string{"gemini-live"}, models)
	require.Equal(t, PlatformGemini, catalog.platform)
	require.Equal(t, container.Sources, catalog.sources)
	require.Equal(t, 1, repo.runtimeCalls)
	require.Zero(t, repo.getCalls, "runtime listing must not eager-load retained rollback routes")
	require.Zero(t, repo.calls+repo.listCalls)
	require.Equal(t, 1, catalog.calls)
	require.False(t, catalog.priced, "model listing is metadata and must not install token-request profit control")
}

func TestListSystemCustomGroupModelsFailsClosedWhenContainerOrDependenciesAreInvalid(t *testing.T) {
	ordinaryID := int64(3)
	ordinaryKey := &APIKey{GroupID: &ordinaryID, Group: directSourceGroup(ordinaryID, PlatformAnthropic)}
	configured := &APIKeyService{
		systemCustomGroupRepo:    &systemCustomRouteRepoStub{},
		systemCustomModelCatalog: &systemCustomRuntimeCatalogStub{catalog: runtimeCatalogForTests(nil)},
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
