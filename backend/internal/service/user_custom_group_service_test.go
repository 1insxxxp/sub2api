package service

import (
	"context"
	"errors"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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

type customGroupDeleteRepoStub struct {
	UserCustomGroupRepository
	boundCount       int
	deleteCalls      int
	forceDeleteCalls int
	forceDeleteCount int
}

type customGroupDeleteAuthCacheStub struct {
	invalidatedUserIDs []int64
}

func (*customGroupDeleteAuthCacheStub) InvalidateAuthCacheByKey(context.Context, string) {}

func (s *customGroupDeleteAuthCacheStub) InvalidateAuthCacheByUserID(_ context.Context, userID int64) {
	s.invalidatedUserIDs = append(s.invalidatedUserIDs, userID)
}

func (*customGroupDeleteAuthCacheStub) InvalidateAuthCacheByGroupID(context.Context, int64) {}

func (s *customGroupDeleteRepoStub) GetOwned(context.Context, int64, int64) (*UserCustomGroup, error) {
	return &UserCustomGroup{ID: 21, UserID: 9}, nil
}

func (s *customGroupDeleteRepoStub) CountBoundAPIKeys(context.Context, int64) (int, error) {
	return s.boundCount, nil
}

func (s *customGroupDeleteRepoStub) Delete(context.Context, int64, int64) error {
	s.deleteCalls++
	return nil
}

func (s *customGroupDeleteRepoStub) DeleteAndUnbindAPIKeys(context.Context, int64, int64) (int, error) {
	s.forceDeleteCalls++
	return s.forceDeleteCount, nil
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

func TestUserCustomGroupDeleteReturnsBoundKeyCount(t *testing.T) {
	repo := &customGroupDeleteRepoStub{boundCount: 2}
	svc := &UserCustomGroupService{repo: repo}

	unboundCount, err := svc.Delete(context.Background(), 9, 21, false)

	require.ErrorIs(t, err, ErrUserCustomGroupInUse)
	require.Zero(t, unboundCount)
	require.Equal(t, "2", infraerrors.FromError(err).Metadata["bound_api_key_count"])
	require.Zero(t, repo.deleteCalls)
	require.Zero(t, repo.forceDeleteCalls)
}

func TestUserCustomGroupForceDeleteUnbindsAtomically(t *testing.T) {
	repo := &customGroupDeleteRepoStub{boundCount: 2, forceDeleteCount: 2}
	cache := &customGroupDeleteAuthCacheStub{}
	svc := &UserCustomGroupService{repo: repo, authCacheInvalidator: cache}

	unboundCount, err := svc.Delete(context.Background(), 9, 21, true)

	require.NoError(t, err)
	require.Equal(t, 2, unboundCount)
	require.Zero(t, repo.deleteCalls)
	require.Equal(t, 1, repo.forceDeleteCalls)
	require.Equal(t, []int64{9}, cache.invalidatedUserIDs)
}
