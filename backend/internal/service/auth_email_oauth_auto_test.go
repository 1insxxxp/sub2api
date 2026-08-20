//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func newEmailOAuthAutoAuthService(
	userRepo UserRepository,
	settings map[string]string,
	quotaRepo UserPlatformQuotaRepository,
) *AuthService {
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                   "test-secret",
			ExpireHour:               1,
			AccessTokenExpireMinutes: 60,
			RefreshTokenExpireDays:   7,
		},
		Default: config.DefaultConfig{
			UserBalance:     3.5,
			UserConcurrency: 2,
		},
	}

	settingService := NewSettingService(&settingRepoStub{values: settings}, cfg)

	return NewAuthService(
		nil, // entClient — nil, updateUserSignupSource early return
		userRepo,
		nil, // redeemRepo — invitationCode="" 时不触发
		&refreshTokenCacheStub{},
		cfg,
		settingService,
		nil, // emailService
		nil, // turnstileService
		nil, // emailQueueService
		nil, // promoService
		nil, // defaultSubAssigner — nil, assignSubscriptions early return
		nil, // affiliateService — nil, bindOAuthAffiliate early return
		quotaRepo,
	)
}

func TestEmailOAuthAuto_SnapshotsPlatformQuotaDefaults(t *testing.T) {
	userRepo := &userRepoStub{nextID: 88}
	quotaRepo := &userPlatformQuotaRepoStub{}

	svc := newEmailOAuthAutoAuthService(
		userRepo,
		map[string]string{
			SettingKeyRegistrationEnabled:   "true",
			SettingKeyDefaultPlatformQuotas: `{"gemini": {"monthly": 100.0}}`,
		},
		quotaRepo,
	)

	user, created, err := svc.createEmailOAuthUser(
		context.Background(),
		"newoauth@example.com",
		"newoauth",
		"github",
		"", // invitationCode
		"", // affiliateCode
	)
	require.NoError(t, err)
	require.NotNil(t, user)
	require.True(t, created)
	require.Equal(t, int64(88), user.ID)

	require.Len(t, quotaRepo.bulkInsertCalls, 1, "createEmailOAuthUser must snapshot platform quotas via BulkInsertInitial")

	records := quotaRepo.bulkInsertCalls[0]
	var geminiRecord *UserPlatformQuotaRecord
	for i := range records {
		if records[i].Platform == "gemini" {
			geminiRecord = &records[i]
			break
		}
	}
	require.NotNil(t, geminiRecord, "expected gemini platform record")
	require.NotNil(t, geminiRecord.MonthlyLimitUSD)
	require.InDelta(t, 100.0, *geminiRecord.MonthlyLimitUSD, 0.0001)
}

func TestEmailOAuthAuto_BlocksDeletedEmailIdentity(t *testing.T) {
	userRepo := &userRepoStub{deletedIdentityExists: true}
	svc := newEmailOAuthAutoAuthService(
		userRepo,
		map[string]string{
			SettingKeyRegistrationEnabled: "true",
		},
		nil,
	)

	user, created, err := svc.createEmailOAuthUser(
		context.Background(),
		"deleted@example.com",
		"deleted",
		"github",
		"",
		"",
	)

	require.Nil(t, user)
	require.False(t, created)
	require.ErrorIs(t, err, ErrEmailExists)
	require.Empty(t, userRepo.created)
	require.Equal(t, []string{"deleted@example.com"}, userRepo.deletedIdentityChecks)
}

func TestEmailOAuthAuto_ReusesConcurrentActiveCreate(t *testing.T) {
	existing := &User{
		ID:           93,
		Email:        "race-verified@example.com",
		Username:     "race-verified-winner",
		Role:         RoleUser,
		Status:       StatusActive,
		TokenVersion: 4,
	}
	userRepo := &userRepoStub{
		usersByEmail: map[string]*User{existing.Email: existing},
		createErr:    ErrEmailExists,
	}
	quotaRepo := &userPlatformQuotaRepoStub{}
	svc := newEmailOAuthAutoAuthService(
		userRepo,
		map[string]string{
			SettingKeyRegistrationEnabled:   "true",
			SettingKeyDefaultPlatformQuotas: `{"gemini": {"monthly": 100.0}}`,
		},
		quotaRepo,
	)

	user, created, err := svc.createEmailOAuthUser(
		context.Background(),
		existing.Email,
		"race-verified",
		"github",
		"",
		"",
	)

	require.NoError(t, err)
	require.Equal(t, existing, user)
	require.False(t, created)
	require.Empty(t, userRepo.created)
	require.Empty(t, quotaRepo.bulkInsertCalls)
}
