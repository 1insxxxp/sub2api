//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type workbenchAccessRepoStub struct {
	access        *AdminWorkbenchAccess
	err           error
	calls         int
	actor, target int64
	flags         [3]bool
}

func (r *workbenchAccessRepoStub) GetAccess(_ context.Context, actor int64) (*AdminWorkbenchAccess, error) {
	r.calls++
	r.actor = actor
	return r.access, r.err
}
func (r *workbenchAccessRepoStub) GetUserAccess(_ context.Context, actor, target int64) (*AdminWorkbenchAccess, error) {
	r.calls++
	r.actor, r.target = actor, target
	return r.access, r.err
}
func (r *workbenchAccessRepoStub) SetUserAccess(_ context.Context, actor, target int64, commission, leaderboard, support bool) error {
	r.calls++
	r.actor, r.target = actor, target
	r.flags = [3]bool{commission, leaderboard, support}
	return r.err
}

func TestAdminWorkbenchAccessServiceSafeContractAndDelegation(t *testing.T) {
	r := &workbenchAccessRepoStub{access: &AdminWorkbenchAccess{Commission: true, UserSupport: true, BalanceTransfer: true}}
	s := NewAdminWorkbenchAccessService(r)
	got, err := s.GetAccess(context.Background(), 8)
	require.NoError(t, err)
	require.Equal(t, int64(8), r.actor)
	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.ElementsMatch(t, []string{"commission", "affiliate_leaderboard", "user_support", "balance_transfer"}, supportKeys(fields))
	got, err = s.GetUserAccess(context.Background(), 8, 12)
	require.NoError(t, err)
	require.True(t, got.UserSupport)
	require.Equal(t, int64(12), r.target)
	require.NoError(t, s.SetUserAccess(context.Background(), 8, 12, true, false, true))
	require.Equal(t, [3]bool{true, false, true}, r.flags)
}

func TestAdminWorkbenchAccessServiceRejectsInvalidIDsWithoutRepository(t *testing.T) {
	r := &workbenchAccessRepoStub{}
	s := NewAdminWorkbenchAccessService(r)
	_, err := s.GetAccess(context.Background(), 0)
	require.ErrorIs(t, err, ErrUserSupportForbidden)
	_, err = s.GetUserAccess(context.Background(), 8, -1)
	require.ErrorIs(t, err, ErrUserSupportForbidden)
	require.ErrorIs(t, s.SetUserAccess(context.Background(), 0, 12, true, true, true), ErrUserSupportForbidden)
	require.Zero(t, r.calls)
}

func TestAdminWorkbenchAccessServiceDoesNotExposeRepositoryErrors(t *testing.T) {
	for _, original := range []error{errors.New("postgres://private/sk-secret"), ErrUserSupportForbidden, ErrUserSupportUserNotFound} {
		r := &workbenchAccessRepoStub{err: original}
		s := NewAdminWorkbenchAccessService(r)
		_, err := s.GetAccess(context.Background(), 8)
		want := original
		if original != ErrUserSupportForbidden && original != ErrUserSupportUserNotFound {
			want = ErrUserSupportUnavailable
		}
		require.ErrorIs(t, err, want)
		_, err = s.GetUserAccess(context.Background(), 8, 12)
		require.ErrorIs(t, err, want)
		require.ErrorIs(t, s.SetUserAccess(context.Background(), 8, 12, false, false, false), want)
		require.NotContains(t, err.Error(), "private")
	}
	r := &workbenchAccessRepoStub{}
	_, err := NewAdminWorkbenchAccessService(r).GetAccess(context.Background(), 8)
	require.ErrorIs(t, err, ErrUserSupportUnavailable)
}
