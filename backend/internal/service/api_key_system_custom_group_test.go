package service

import (
	"context"
	"errors"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type systemCustomRouteRepoStub struct {
	SystemCustomGroupRepository
	route       *SystemCustomGroupModel
	err         error
	requestedID int64
	requested   string
	calls       int
}

func (s *systemCustomRouteRepoStub) ResolveModel(_ context.Context, groupID int64, publicModel string) (*SystemCustomGroupModel, error) {
	s.calls++
	s.requestedID = groupID
	s.requested = publicModel
	return s.route, s.err
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
