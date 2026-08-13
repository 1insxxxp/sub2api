package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type customGroupValidationUserRepoStub struct {
	UserRepository
	user *User
}

func (s customGroupValidationUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	if s.user == nil {
		return nil, errors.New("user not found")
	}
	return s.user, nil
}

type customGroupValidationGroupRepoStub struct {
	GroupRepository
	groups map[int64]*Group
}

type customGroupReadRepoStub struct {
	UserCustomGroupRepository
	groups []UserCustomGroup
	group  *UserCustomGroup
}

func (s customGroupReadRepoStub) ListByUserID(context.Context, int64) ([]UserCustomGroup, error) {
	return s.groups, nil
}

func (s customGroupReadRepoStub) GetOwned(context.Context, int64, int64) (*UserCustomGroup, error) {
	return s.group, nil
}

func (s customGroupValidationGroupRepoStub) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	group, ok := s.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	return group, nil
}

func newCustomGroupValidationService() *UserCustomGroupService {
	return &UserCustomGroupService{
		userRepo: customGroupValidationUserRepoStub{user: &User{ID: 9, Status: StatusActive}},
		groupRepo: customGroupValidationGroupRepoStub{groups: map[int64]*Group{
			10: {ID: 10, Status: StatusActive, Platform: PlatformAnthropic},
			20: {ID: 20, Status: StatusActive, Platform: PlatformAnthropic},
		}},
	}
}

func TestUserCustomGroupValidateModelsAllowsAliasesForSameRealModelAcrossGroups(t *testing.T) {
	svc := newCustomGroupValidationService()
	models := []UserCustomGroupModelInput{
		{PublicModel: "  claude-opus-4-6-balance  ", SourceGroupID: 10, SourceModel: "claude-opus-4-6"},
		{PublicModel: "claude-opus-4-6-discount", SourceGroupID: 20, SourceModel: "claude-opus-4-6"},
	}

	err := svc.validateModels(context.Background(), 9, models)

	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-6-balance", models[0].PublicModel)
}

func TestUserCustomGroupValidateModelsRejectsCaseInsensitiveCallNameConflict(t *testing.T) {
	svc := newCustomGroupValidationService()
	models := []UserCustomGroupModelInput{
		{PublicModel: "Claude-Balance", SourceGroupID: 10, SourceModel: "claude-opus-4-6"},
		{PublicModel: "claude-balance", SourceGroupID: 20, SourceModel: "claude-opus-4-6"},
	}

	require.ErrorIs(t, svc.validateModels(context.Background(), 9, models), ErrUserCustomGroupInvalidModel)
}

func TestUserCustomGroupValidateModelsRejectsDuplicateSourceMapping(t *testing.T) {
	svc := newCustomGroupValidationService()
	models := []UserCustomGroupModelInput{
		{PublicModel: "claude-primary", SourceGroupID: 10, SourceModel: "claude-opus-4-6"},
		{PublicModel: "claude-secondary", SourceGroupID: 10, SourceModel: "CLAUDE-OPUS-4-6"},
	}

	require.ErrorIs(t, svc.validateModels(context.Background(), 9, models), ErrUserCustomGroupInvalidModel)
}

func TestUserCustomGroupServiceListAnnotatesSourceAvailability(t *testing.T) {
	repo := customGroupReadRepoStub{groups: []UserCustomGroup{{
		ID:     70,
		UserID: 9,
		Models: []UserCustomGroupModel{
			{ID: 1, SourceGroupID: 10, SourceGroup: &Group{ID: 10, Status: StatusActive, Platform: PlatformAnthropic}},
			{ID: 2, SourceGroupID: 20},
			{ID: 3, SourceGroupID: 30, SourceGroup: &Group{ID: 30, Status: StatusDisabled, Platform: PlatformAnthropic}},
			{ID: 4, SourceGroupID: 40, SourceGroup: &Group{ID: 40, Status: StatusActive, Platform: PlatformComposite}},
			{ID: 5, SourceGroupID: 50, SourceGroup: &Group{ID: 50, Status: StatusActive, Platform: PlatformAnthropic, IsExclusive: true}},
		},
	}}}
	svc := &UserCustomGroupService{
		repo:     repo,
		userRepo: customGroupValidationUserRepoStub{user: &User{ID: 9, Status: StatusActive}},
	}

	groups, err := svc.List(context.Background(), 9)

	require.NoError(t, err)
	require.Len(t, groups, 1)
	models := groups[0].Models
	require.True(t, models[0].SourceAvailable)
	require.Empty(t, models[0].SourceIssue)
	require.False(t, models[1].SourceAvailable)
	require.Equal(t, UserCustomGroupSourceIssueUnavailable, models[1].SourceIssue)
	require.False(t, models[2].SourceAvailable)
	require.Equal(t, UserCustomGroupSourceIssueUnavailable, models[2].SourceIssue)
	require.False(t, models[3].SourceAvailable)
	require.Equal(t, UserCustomGroupSourceIssueUnavailable, models[3].SourceIssue)
	require.False(t, models[4].SourceAvailable)
	require.Equal(t, UserCustomGroupSourceIssueNotAllowed, models[4].SourceIssue)
}

func TestUserCustomGroupServiceGetAnnotatesAllowedExclusiveSource(t *testing.T) {
	repo := customGroupReadRepoStub{group: &UserCustomGroup{
		ID:     70,
		UserID: 9,
		Models: []UserCustomGroupModel{{
			ID:            1,
			SourceGroupID: 50,
			SourceGroup:   &Group{ID: 50, Status: StatusActive, Platform: PlatformAnthropic, IsExclusive: true},
		}},
	}}
	svc := &UserCustomGroupService{
		repo:     repo,
		userRepo: customGroupValidationUserRepoStub{user: &User{ID: 9, Status: StatusActive, AllowedGroups: []int64{50}}},
	}

	group, err := svc.Get(context.Background(), 9, 70)

	require.NoError(t, err)
	require.True(t, group.Models[0].SourceAvailable)
	require.Empty(t, group.Models[0].SourceIssue)
}
