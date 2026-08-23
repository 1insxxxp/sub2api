package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type externalCredentialUserRepoStub struct {
	service.UserRepository

	usersByEmail map[string]*service.User
	lastEmail    string
}

func (s *externalCredentialUserRepoStub) GetByEmail(_ context.Context, email string) (*service.User, error) {
	s.lastEmail = email
	user, ok := s.usersByEmail[email]
	if !ok {
		return nil, service.ErrUserNotFound
	}
	cloned := *user
	return &cloned, nil
}

func TestAuthServiceVerifyExternalCredentialReturnsSafeUser(t *testing.T) {
	passwordHash := hashExternalCredentialPassword(t, "correct-password")
	repo := &externalCredentialUserRepoStub{
		usersByEmail: map[string]*service.User{
			"alice@example.com": {
				ID:             42,
				Email:          "alice@example.com",
				Username:       "alice",
				Role:           service.RoleUser,
				Status:         service.StatusActive,
				PasswordHash:   passwordHash,
				Balance:        99,
				Concurrency:    7,
				TokenVersion:   3,
				APIKeys:        []service.APIKey{{ID: 1, Key: "sk-secret"}},
				Subscriptions:  []service.UserSubscription{{ID: 2}},
				FrozenBalance:  5,
				AllowedGroups:  []int64{10},
				TotpEnabled:    false,
				TotalRecharged: 123,
			},
		},
	}
	svc := newExternalCredentialAuthService(repo)

	got, err := svc.VerifyExternalCredential(context.Background(), "  alice@example.com  ", "correct-password")

	require.NoError(t, err)
	require.Equal(t, &service.ExternalCredentialUser{
		ID:       42,
		Email:    "alice@example.com",
		Username: "alice",
		Role:     service.RoleUser,
		Status:   service.StatusActive,
	}, got)
	require.Equal(t, "alice@example.com", repo.lastEmail)

	body, err := json.Marshal(got)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":42,"email":"alice@example.com","username":"alice","role":"user","status":"active"}`, string(body))
}

func TestAuthServiceVerifyExternalCredentialRejectsUnknownUserAndWrongPassword(t *testing.T) {
	passwordHash := hashExternalCredentialPassword(t, "correct-password")

	tests := []struct {
		name         string
		usersByEmail map[string]*service.User
		email        string
		password     string
	}{
		{
			name:         "unknown user",
			usersByEmail: map[string]*service.User{},
			email:        "missing@example.com",
			password:     "correct-password",
		},
		{
			name: "wrong password",
			usersByEmail: map[string]*service.User{
				"alice@example.com": {
					ID:           42,
					Email:        "alice@example.com",
					Username:     "alice",
					Role:         service.RoleUser,
					Status:       service.StatusActive,
					PasswordHash: passwordHash,
				},
			},
			email:    "alice@example.com",
			password: "wrong-password",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &externalCredentialUserRepoStub{usersByEmail: tc.usersByEmail}
			svc := newExternalCredentialAuthService(repo)

			got, err := svc.VerifyExternalCredential(context.Background(), tc.email, tc.password)

			require.Nil(t, got)
			require.ErrorIs(t, err, service.ErrInvalidCredentials)
		})
	}
}

func TestAuthServiceVerifyExternalCredentialRejectsDisabledUser(t *testing.T) {
	passwordHash := hashExternalCredentialPassword(t, "correct-password")
	repo := &externalCredentialUserRepoStub{
		usersByEmail: map[string]*service.User{
			"alice@example.com": {
				ID:           42,
				Email:        "alice@example.com",
				Username:     "alice",
				Role:         service.RoleUser,
				Status:       service.StatusDisabled,
				PasswordHash: passwordHash,
			},
		},
	}
	svc := newExternalCredentialAuthService(repo)

	got, err := svc.VerifyExternalCredential(context.Background(), "alice@example.com", "correct-password")

	require.Nil(t, got)
	require.ErrorIs(t, err, service.ErrUserNotActive)
}

func TestAuthServiceVerifyExternalCredentialRejectsTotpEnabledUser(t *testing.T) {
	passwordHash := hashExternalCredentialPassword(t, "correct-password")
	repo := &externalCredentialUserRepoStub{
		usersByEmail: map[string]*service.User{
			"alice@example.com": {
				ID:           42,
				Email:        "alice@example.com",
				Username:     "alice",
				Role:         service.RoleUser,
				Status:       service.StatusActive,
				PasswordHash: passwordHash,
				TotpEnabled:  true,
			},
		},
	}
	svc := newExternalCredentialAuthService(repo)

	got, err := svc.VerifyExternalCredential(context.Background(), "alice@example.com", "correct-password")

	require.Nil(t, got)
	require.ErrorIs(t, err, service.ErrExternalLogin2FARequired)
}

func newExternalCredentialAuthService(userRepo service.UserRepository) *service.AuthService {
	return service.NewAuthService(
		nil,
		userRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
}

func hashExternalCredentialPassword(t *testing.T, password string) string {
	t.Helper()

	hash, err := new(service.AuthService).HashPassword(password)
	require.NoError(t, err)
	return hash
}
