//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSettingServiceAuthIPBlacklistDefaultsDisabled(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, nil)

	settings, err := svc.GetAuthIPBlacklistSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Empty(t, settings.Rules)

	blocked, err := svc.IsAuthIPBlocked(context.Background(), "203.0.113.10")
	require.NoError(t, err)
	require.False(t, blocked)
}

func TestSettingServiceAuthIPBlacklistMatchesExactIPAndCIDR(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{
		values: map[string]string{
			SettingKeyAuthIPBlacklistEnabled: "true",
			SettingKeyAuthIPBlacklistRules:   `["45.207.193.151","202.8.9.0/24","bad-rule",""]`,
		},
	}, nil)

	settings, err := svc.GetAuthIPBlacklistSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.Equal(t, []string{"45.207.193.151", "202.8.9.0/24"}, settings.Rules)

	blocked, err := svc.IsAuthIPBlocked(context.Background(), "45.207.193.151")
	require.NoError(t, err)
	require.True(t, blocked)

	blocked, err = svc.IsAuthIPBlocked(context.Background(), "202.8.9.242")
	require.NoError(t, err)
	require.True(t, blocked)

	blocked, err = svc.IsAuthIPBlocked(context.Background(), "203.0.113.10")
	require.NoError(t, err)
	require.False(t, blocked)
}
