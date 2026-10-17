//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthService_SendVerifyCodeAsync_UniformRegisteredResponse(t *testing.T) {
	for _, tc := range []struct {
		name       string
		repo       *userRepoStub
		wantQueued int
	}{
		{name: "new email", repo: &userRepoStub{}, wantQueued: 1},
		{name: "registered email", repo: &userRepoStub{exists: true}},
		{name: "registered alias", repo: &userRepoStub{aliasExists: true}},
		{name: "deleted identity", repo: &userRepoStub{deletedIdentityExists: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newAuthService(tc.repo, map[string]string{SettingKeyRegistrationEnabled: "true"}, nil, nil)
			// A buffered queue without workers records delivery attempts without sending mail.
			svc.emailQueueService = &EmailQueueService{taskChan: make(chan EmailTask, 1)}
			result, err := svc.SendVerifyCodeAsync(context.Background(), "person@example.com", "zh-CN")
			require.NoError(t, err)
			require.Equal(t, &SendVerifyCodeResult{Countdown: 60}, result)
			require.Len(t, svc.emailQueueService.taskChan, tc.wantQueued)
			if tc.wantQueued > 0 {
				task := <-svc.emailQueueService.taskChan
				require.Equal(t, "person@example.com", task.Email)
				require.Equal(t, "zh-CN", task.Locale)
			}
		})
	}
}

func TestAuthService_SendVerifyCodeAsync_EmailPolicyDoesNotRevealRegistration(t *testing.T) {
	for _, registered := range []bool{false, true} {
		svc := newAuthService(&userRepoStub{exists: registered}, map[string]string{
			SettingKeyRegistrationEnabled:              "true",
			SettingKeyRegistrationEmailSuffixWhitelist: `["@allowed.example"]`,
		}, nil, nil)
		svc.emailQueueService = &EmailQueueService{taskChan: make(chan EmailTask, 1)}
		result, err := svc.SendVerifyCodeAsync(context.Background(), "person@blocked.example")
		require.ErrorIs(t, err, ErrEmailSuffixNotAllowed)
		require.Nil(t, result)
		require.Empty(t, svc.emailQueueService.taskChan)
	}
}

func TestAuthService_SendVerifyCodeAsync_RegistrationDisabledRemainsBlocked(t *testing.T) {
	svc := newAuthService(&userRepoStub{exists: true}, map[string]string{SettingKeyRegistrationEnabled: "false"}, nil, nil)
	result, err := svc.SendVerifyCodeAsync(context.Background(), "person@example.com")
	require.ErrorIs(t, err, ErrRegDisabled)
	require.Nil(t, result)
}

func TestAuthService_SendVerifyCodeAsync_DomainQuotaDoesNotRevealRegistration(t *testing.T) {
	for _, registered := range []bool{false, true} {
		svc := newAuthService(&userRepoStub{exists: registered, domainCounts: map[string]int{"limited.example": 1}}, map[string]string{
			SettingKeyRegistrationEnabled:                 "true",
			SettingKeyRegistrationEmailSuffixWhitelist:    `["@allowed.example"]`,
			SettingKeyRegistrationEmailDomainQuotaEnabled: "true",
		}, nil, nil)
		result, err := svc.SendVerifyCodeAsync(context.Background(), "person@limited.example")
		require.ErrorIs(t, err, ErrEmailDomainRegistrationLimit)
		require.Nil(t, result)
	}
}

func TestAuthService_SendVerifyCodeAsync_QueueFailureDoesNotRevealRegistration(t *testing.T) {
	for _, registered := range []bool{false, true} {
		svc := newAuthService(&userRepoStub{exists: registered}, map[string]string{SettingKeyRegistrationEnabled: "true"}, nil, nil)
		// A queue without capacity simulates saturation without sending any mail.
		svc.emailQueueService = &EmailQueueService{taskChan: make(chan EmailTask)}
		result, err := svc.SendVerifyCodeAsync(context.Background(), "person@example.com")
		require.NoError(t, err)
		require.Equal(t, &SendVerifyCodeResult{Countdown: 60}, result)
	}
}
