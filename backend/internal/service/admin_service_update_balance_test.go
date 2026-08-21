//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type balanceUserRepoStub struct {
	*userRepoStub
	updateErr        error
	activeAdminTotal int64
	listFilters      []UserListFilters
	updated          []*User
}

func (s *balanceUserRepoStub) Update(ctx context.Context, user *User) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	if user == nil {
		return nil
	}
	clone := *user
	s.updated = append(s.updated, &clone)
	if s.userRepoStub != nil {
		s.userRepoStub.user = &clone
	}
	return nil
}

func (s *balanceUserRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters UserListFilters) ([]User, *pagination.PaginationResult, error) {
	s.listFilters = append(s.listFilters, filters)
	return nil, &pagination.PaginationResult{
		Total:    s.activeAdminTotal,
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    1,
	}, nil
}

type balanceRedeemRepoStub struct {
	*redeemRepoStub
	created []*RedeemCode
}

func (s *balanceRedeemRepoStub) Create(ctx context.Context, code *RedeemCode) error {
	if code == nil {
		return nil
	}
	clone := *code
	s.created = append(s.created, &clone)
	return nil
}

type authCacheInvalidatorStub struct {
	userIDs  []int64
	groupIDs []int64
	keys     []string
}

func (s *authCacheInvalidatorStub) InvalidateAuthCacheByKey(ctx context.Context, key string) {
	s.keys = append(s.keys, key)
}

func (s *authCacheInvalidatorStub) InvalidateAuthCacheByUserID(ctx context.Context, userID int64) {
	s.userIDs = append(s.userIDs, userID)
}

func (s *authCacheInvalidatorStub) InvalidateAuthCacheByGroupID(ctx context.Context, groupID int64) {
	s.groupIDs = append(s.groupIDs, groupID)
}

func TestAdminService_UpdateUserBalance_InvalidatesAuthCache(t *testing.T) {
	baseRepo := &userRepoStub{user: &User{ID: 7, Balance: 10}}
	repo := &balanceUserRepoStub{userRepoStub: baseRepo}
	redeemRepo := &balanceRedeemRepoStub{redeemRepoStub: &redeemRepoStub{}}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		redeemCodeRepo:       redeemRepo,
		authCacheInvalidator: invalidator,
	}

	_, err := svc.UpdateUserBalance(context.Background(), 7, 5, "add", "")
	require.NoError(t, err)
	require.Equal(t, []int64{7}, invalidator.userIDs)
	require.Len(t, redeemRepo.created, 1)
}

func TestAdminService_UpdateUserBalance_NoChangeNoInvalidate(t *testing.T) {
	baseRepo := &userRepoStub{user: &User{ID: 7, Balance: 10}}
	repo := &balanceUserRepoStub{userRepoStub: baseRepo}
	redeemRepo := &balanceRedeemRepoStub{redeemRepoStub: &redeemRepoStub{}}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		redeemCodeRepo:       redeemRepo,
		authCacheInvalidator: invalidator,
	}

	_, err := svc.UpdateUserBalance(context.Background(), 7, 10, "set", "")
	require.NoError(t, err)
	require.Empty(t, invalidator.userIDs)
	require.Empty(t, redeemRepo.created)
}

func TestAdminService_UpdateUserRole_ToSubAdminInvalidatesAuthCache(t *testing.T) {
	baseRepo := &userRepoStub{user: &User{ID: 7, Role: RoleUser, Status: StatusActive, Concurrency: 1}}
	repo := &balanceUserRepoStub{userRepoStub: baseRepo}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		authCacheInvalidator: invalidator,
	}

	user, err := svc.UpdateUser(context.Background(), 7, &UpdateUserInput{Role: RoleSubAdmin})

	require.NoError(t, err)
	require.Equal(t, RoleSubAdmin, user.Role)
	require.Equal(t, []int64{7}, invalidator.userIDs)
	require.Len(t, repo.updated, 1)
}

func TestAdminService_UpdateUserRole_RejectsDowngradingLastActiveAdmin(t *testing.T) {
	baseRepo := &userRepoStub{user: &User{ID: 7, Role: RoleAdmin, Status: StatusActive, Concurrency: 1}}
	repo := &balanceUserRepoStub{userRepoStub: baseRepo, activeAdminTotal: 1}
	svc := &adminServiceImpl{userRepo: repo}

	_, err := svc.UpdateUser(context.Background(), 7, &UpdateUserInput{Role: RoleSubAdmin})

	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot downgrade the last active admin user")
	require.Empty(t, repo.updated)
	require.Len(t, repo.listFilters, 1)
	require.Equal(t, RoleAdmin, repo.listFilters[0].Role)
	require.Equal(t, StatusActive, repo.listFilters[0].Status)
}

func TestAdminService_UpdateUserRole_AllowsDowngradingWhenAnotherActiveAdminExists(t *testing.T) {
	baseRepo := &userRepoStub{user: &User{ID: 7, Role: RoleAdmin, Status: StatusActive, Concurrency: 1}}
	repo := &balanceUserRepoStub{userRepoStub: baseRepo, activeAdminTotal: 2}
	svc := &adminServiceImpl{userRepo: repo}

	user, err := svc.UpdateUser(context.Background(), 7, &UpdateUserInput{Role: RoleSubAdmin})

	require.NoError(t, err)
	require.Equal(t, RoleSubAdmin, user.Role)
	require.Len(t, repo.updated, 1)
}
