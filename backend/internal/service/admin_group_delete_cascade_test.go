//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type sourceGroupDeleteRepoStub struct {
	GroupRepository
	group        *Group
	affected     []int64
	err          error
	deleteCalls  int
	guardedCalls int
	countCalls   int
}

func (r *sourceGroupDeleteRepoStub) GetByID(context.Context, int64) (*Group, error) {
	return r.group, nil
}

func (r *sourceGroupDeleteRepoStub) CountCustomGroupModelReferences(context.Context, int64) (int, error) {
	r.countCalls++
	return 123, nil
}

func (r *sourceGroupDeleteRepoStub) DeleteCascade(context.Context, int64) ([]int64, error) {
	r.deleteCalls++
	return r.affected, r.err
}

func (r *sourceGroupDeleteRepoStub) DeleteCascadeIfEmpty(context.Context, int64) ([]int64, error) {
	r.guardedCalls++
	return r.affected, r.err
}

type sourceGroupDeleteKeyRepoStub struct{ APIKeyRepository }

func (sourceGroupDeleteKeyRepoStub) ListKeysByGroupID(context.Context, int64) ([]string, error) {
	return []string{"fixture-direct-group-key"}, nil
}

func TestAdminDeleteSourceGroupCascadesReferencesAndInvalidatesOwners(t *testing.T) {
	for _, guarded := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "guarded"}[guarded], func(t *testing.T) {
			repo := &sourceGroupDeleteRepoStub{group: &Group{ID: 102, SubscriptionType: SubscriptionTypeStandard}, affected: []int64{37, 38}}
			cache := &authCacheInvalidatorStub{}
			svc := &adminServiceImpl{
				groupRepo: repo, emptyGroupDeleteRepo: repo,
				apiKeyRepo: sourceGroupDeleteKeyRepoStub{}, authCacheInvalidator: cache,
			}
			var err error
			if guarded {
				err = svc.DeleteGroupIfEmpty(context.Background(), 102)
			} else {
				err = svc.DeleteGroup(context.Background(), 102)
			}
			require.NoError(t, err)
			require.Zero(t, repo.countCalls, "references are removed atomically rather than rejected before deletion")
			require.Equal(t, 1, repo.deleteCalls+repo.guardedCalls)
			require.Equal(t, guarded, repo.guardedCalls == 1)
			require.ElementsMatch(t, []int64{37, 38}, cache.userIDs)
			require.Equal(t, []string{"fixture-direct-group-key"}, cache.keys)
		})
	}
}

func TestAdminDeleteSourceGroupFailureDoesNotInvalidateCaches(t *testing.T) {
	failure := errors.New("cascade transaction rolled back")
	repo := &sourceGroupDeleteRepoStub{group: &Group{ID: 102}, affected: []int64{37}, err: failure}
	cache := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{groupRepo: repo, apiKeyRepo: sourceGroupDeleteKeyRepoStub{}, authCacheInvalidator: cache}

	err := svc.DeleteGroup(context.Background(), 102)

	require.ErrorIs(t, err, failure)
	require.Empty(t, cache.userIDs)
	require.Empty(t, cache.keys)
}
