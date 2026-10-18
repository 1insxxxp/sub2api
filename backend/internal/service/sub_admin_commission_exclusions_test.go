package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type commissionExclusionRepoStub struct {
	SubAdminCommissionRepository
	userID, operatorID int64
	err                error
}

func (r *commissionExclusionRepoStub) ListExclusions(context.Context, pagination.PaginationParams) ([]SubAdminCommissionExclusion, pagination.PaginationResult, error) {
	return []SubAdminCommissionExclusion{{UserID: 7}}, pagination.PaginationResult{Total: 1, Page: 1, PageSize: 10}, r.err
}

func (r *commissionExclusionRepoStub) AddExclusion(_ context.Context, userID, operatorID int64) (*SubAdminCommissionExclusion, error) {
	r.userID, r.operatorID = userID, operatorID
	return &SubAdminCommissionExclusion{UserID: userID}, r.err
}

func (r *commissionExclusionRepoStub) RemoveExclusion(_ context.Context, userID int64) error {
	r.userID = userID
	return r.err
}

func TestSubAdminCommissionExclusionsValidateTargetAndPreserveErrors(t *testing.T) {
	repo := &commissionExclusionRepoStub{}
	svc := NewSubAdminCommissionService(repo, nil, nil)
	ctx := context.Background()
	for _, id := range []int64{0, -1} {
		_, err := svc.AddExclusion(ctx, id, 1)
		require.Error(t, err)
		require.Error(t, svc.RemoveExclusion(ctx, id))
	}
	require.Zero(t, repo.userID)
	entry, err := svc.AddExclusion(ctx, 7, 9)
	require.NoError(t, err)
	require.Equal(t, int64(7), entry.UserID)
	require.Equal(t, int64(9), repo.operatorID)
	require.NoError(t, svc.RemoveExclusion(ctx, 7))
	entries, page, err := svc.ListExclusions(ctx, pagination.PaginationParams{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.EqualValues(t, 1, page.Total)
	repo.err = errors.New("database unavailable")
	_, _, err = svc.ListExclusions(ctx, pagination.PaginationParams{})
	require.ErrorIs(t, err, repo.err)
	_, err = svc.AddExclusion(ctx, 7, 9)
	require.ErrorIs(t, err, repo.err)
	require.ErrorIs(t, svc.RemoveExclusion(ctx, 7), repo.err)
}

func TestSubAdminCommissionExclusionsRequireRepository(t *testing.T) {
	svc := NewSubAdminCommissionService(nil, nil, nil)
	ctx := context.Background()
	_, _, err := svc.ListExclusions(ctx, pagination.PaginationParams{})
	require.ErrorIs(t, err, ErrSubAdminCommissionRepoMissing)
	_, err = svc.AddExclusion(ctx, 7, 1)
	require.ErrorIs(t, err, ErrSubAdminCommissionRepoMissing)
	require.ErrorIs(t, svc.RemoveExclusion(ctx, 7), ErrSubAdminCommissionRepoMissing)
}
