package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type accountExclusionServiceRepoStub struct {
	err       error
	committed bool
}

func (r *accountExclusionServiceRepoStub) List(context.Context, int64, pagination.PaginationParams) ([]GroupAccountUserExclusion, pagination.PaginationResult, error) {
	return nil, pagination.PaginationResult{}, r.err
}
func (r *accountExclusionServiceRepoStub) Add(_ context.Context, g, a, u int64) (*GroupAccountUserExclusion, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.committed = true
	return &GroupAccountUserExclusion{GroupID: g, AccountID: a, UserID: u}, nil
}
func (r *accountExclusionServiceRepoStub) Remove(context.Context, int64, int64) (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	r.committed = true
	return 7, nil
}

type accountExclusionInvalidatorStub struct {
	t     *testing.T
	repo  *accountExclusionServiceRepoStub
	users []int64
}

func (s *accountExclusionInvalidatorStub) InvalidateAuthCacheByUserID(_ context.Context, id int64) {
	require.True(s.t, s.repo.committed)
	s.users = append(s.users, id)
}
func (*accountExclusionInvalidatorStub) InvalidateAuthCacheByGroupID(context.Context, int64) {}
func (*accountExclusionInvalidatorStub) InvalidateAuthCacheByKey(context.Context, string)    {}

func TestGroupAccountUserExclusionServiceInvalidatesOnlyCommittedChanges(t *testing.T) {
	for _, operation := range []string{"add", "remove"} {
		for _, repoErr := range []error{nil, errors.New("transaction rolled back")} {
			repo := &accountExclusionServiceRepoStub{err: repoErr}
			invalidator := &accountExclusionInvalidatorStub{t: t, repo: repo}
			svc := &GroupAccountUserExclusionService{repo: repo, invalidator: invalidator}
			var err error
			if operation == "add" {
				_, err = svc.Add(context.Background(), 10, 21, 7)
			} else {
				err = svc.Remove(context.Background(), 10, 1)
			}
			if repoErr != nil {
				require.ErrorIs(t, err, repoErr)
				require.Empty(t, invalidator.users)
			} else {
				require.NoError(t, err)
				require.Equal(t, []int64{7}, invalidator.users)
			}
		}
	}
}

func TestGroupAccountUserExclusionServiceWithoutImmediateInvalidator(t *testing.T) {
	svc := NewGroupAccountUserExclusionService(&accountExclusionServiceRepoStub{}, nil)
	require.NotPanics(t, func() { _, err := svc.Add(context.Background(), 10, 21, 7); require.NoError(t, err) })
}
