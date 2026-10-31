//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type apiKeyBalanceRecheckRepo struct {
	UserRepository
	user *User
	err  error
}

func (r apiKeyBalanceRecheckRepo) GetByID(context.Context, int64) (*User, error) {
	return r.user, r.err
}

func TestAPIKeyServiceRecheckUserBalance(t *testing.T) {
	for _, tc := range []struct {
		name string
		svc  *APIKeyService
		user *User
	}{
		{name: "unavailable_service"},
		{name: "unavailable_repository", svc: &APIKeyService{}},
		{name: "repository_failure", svc: &APIKeyService{userRepo: apiKeyBalanceRecheckRepo{err: errors.New("database down")}}},
		{name: "missing_user", svc: &APIKeyService{userRepo: apiKeyBalanceRecheckRepo{}}},
		{name: "authoritative_balance", svc: &APIKeyService{userRepo: apiKeyBalanceRecheckRepo{user: &User{ID: 7, Balance: 0.01}}}, user: &User{ID: 7, Balance: 0.01}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user, err := tc.svc.RecheckUserBalance(context.Background(), 7)
			if tc.user == nil {
				require.ErrorIs(t, err, ErrBillingServiceUnavailable)
				require.Nil(t, user)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.user, user)
		})
	}
}
