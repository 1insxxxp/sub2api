//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountRiskConfigDefaultsAndUpdates(t *testing.T) {
	ctx := context.Background()
	repo := newCheckinSettingRepoStub()
	got, err := ReadAccountRiskConfig(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, AccountRiskConfig{CheckinMode: "observe", RegistrationMode: "observe", WindowMinutes: 5, ChallengeCount: 3, CooldownCount: 6, CooldownMinutes: 15}, got)
	svc := NewCheckinService(nil, nil, nil)
	svc.SetSettingRepository(repo)
	got.CheckinMode = "enforce"
	require.NoError(t, svc.UpdateAccountRiskConfig(ctx, got))
	stored, err := ReadAccountRiskConfig(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, got, stored)
	got.CooldownCount = 1
	require.Error(t, svc.UpdateAccountRiskConfig(ctx, got))
}
