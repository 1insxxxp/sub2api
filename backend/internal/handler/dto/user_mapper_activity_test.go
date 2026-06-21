package dto

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserFromServiceAdmin_MapsActivityTimestamps(t *testing.T) {
	t.Parallel()

	lastLoginAt := time.Date(2026, time.April, 20, 10, 0, 0, 0, time.UTC)
	lastActiveAt := lastLoginAt.Add(15 * time.Minute)
	lastUsedAt := lastLoginAt.Add(45 * time.Minute)

	out := UserFromServiceAdmin(&service.User{
		ID:           42,
		Email:        "admin@example.com",
		Username:     "admin",
		Role:         service.RoleAdmin,
		Status:       service.StatusActive,
		LastActiveAt: &lastActiveAt,
		LastUsedAt:   &lastUsedAt,
	})

	require.NotNil(t, out)
	require.NotNil(t, out.LastActiveAt)
	require.NotNil(t, out.LastUsedAt)
	require.WithinDuration(t, lastActiveAt, *out.LastActiveAt, time.Second)
	require.WithinDuration(t, lastUsedAt, *out.LastUsedAt, time.Second)
}

func TestUserFromServiceAdmin_MapsAuthSourceMetadata(t *testing.T) {
	t.Parallel()

	out := UserFromServiceAdmin(&service.User{
		ID:                    42,
		Email:                 "admin@example.com",
		Username:              "admin",
		Role:                  service.RoleAdmin,
		Status:                service.StatusActive,
		RegistrationIP:        "45.207.193.151",
		RegistrationUserAgent: "register-agent",
		LastLoginIP:           "202.8.9.242",
		LastLoginUserAgent:    "login-agent",
	})

	require.NotNil(t, out)
	require.Equal(t, "45.207.193.151", out.RegistrationIP)
	require.Equal(t, "register-agent", out.RegistrationUserAgent)
	require.Equal(t, "202.8.9.242", out.LastLoginIP)
	require.Equal(t, "login-agent", out.LastLoginUserAgent)
}
