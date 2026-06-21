//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthServiceRegisterWithMetadataStoresRegistrationSource(t *testing.T) {
	repo := &userRepoStub{nextID: 9}
	service := newAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil, nil)

	_, user, err := service.RegisterWithVerificationAndMetadata(
		context.Background(),
		"user@test.com",
		"password",
		"",
		"",
		"",
		"",
		AuthSourceMetadata{
			IP:        "45.207.193.151",
			UserAgent: "Mozilla/5.0 test",
		},
	)
	require.NoError(t, err)
	require.Equal(t, int64(9), user.ID)
	require.Len(t, repo.created, 1)
	require.Equal(t, "45.207.193.151", repo.created[0].RegistrationIP)
	require.Equal(t, "Mozilla/5.0 test", repo.created[0].RegistrationUserAgent)
}

func TestAuthServiceLoginWithMetadataUpdatesLastLoginSource(t *testing.T) {
	repo := &userRepoStub{}
	service := newAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil, nil)

	passwordHash, err := service.HashPassword("password")
	require.NoError(t, err)
	repo.user = &User{
		ID:           10,
		Email:        "user@test.com",
		PasswordHash: passwordHash,
		Role:         RoleUser,
		Status:       StatusActive,
	}

	_, user, err := service.LoginWithMetadata(context.Background(), "user@test.com", "password", AuthSourceMetadata{
		IP:        "202.8.9.242",
		UserAgent: "curl/8.0",
	})
	require.NoError(t, err)
	require.Equal(t, int64(10), user.ID)
	require.Len(t, repo.updated, 1)
	require.Equal(t, "202.8.9.242", repo.updated[0].LastLoginIP)
	require.Equal(t, "curl/8.0", repo.updated[0].LastLoginUserAgent)
	require.NotNil(t, repo.updated[0].LastLoginAt)
}
