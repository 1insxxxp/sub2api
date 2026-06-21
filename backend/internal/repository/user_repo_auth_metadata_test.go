package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserRepositoryPersistsAuthSourceMetadata(t *testing.T) {
	repo, _ := newUserEntRepo(t)
	ctx := context.Background()

	created := &service.User{
		Email:                 "source-metadata@example.com",
		Username:              "source-metadata",
		PasswordHash:          "hash",
		Role:                  service.RoleUser,
		Status:                service.StatusActive,
		RegistrationIP:        "45.207.193.151",
		RegistrationUserAgent: "Mozilla/5.0 test browser",
		LastLoginIP:           "202.8.9.242",
		LastLoginUserAgent:    "curl/8.0",
	}
	require.NoError(t, repo.Create(ctx, created))

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "45.207.193.151", got.RegistrationIP)
	require.Equal(t, "Mozilla/5.0 test browser", got.RegistrationUserAgent)
	require.Equal(t, "202.8.9.242", got.LastLoginIP)
	require.Equal(t, "curl/8.0", got.LastLoginUserAgent)
}
