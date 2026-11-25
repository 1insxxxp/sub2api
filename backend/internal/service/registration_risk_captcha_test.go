//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type registrationRiskVerifier struct{ calls int }

func (v *registrationRiskVerifier) VerifyToken(ctx context.Context, secret, token, ip string) (*TurnstileVerifyResponse, error) {
	v.calls++
	if token == "offline" {
		return nil, errors.New("provider unavailable")
	}
	return &TurnstileVerifyResponse{Success: token == "verified"}, nil
}
func TestSignupCaptchaReceiptIsBoundAndSingleUse(t *testing.T) {
	db, client := newRegistrationRiskTestDB(t)
	settings := newCheckinSettingRepoStub()
	settings.values[registrationModeKey] = "enforce"
	settings.values[challengeCountKey] = "1"
	service := NewSettingService(settings, nil)
	verifier := &registrationRiskVerifier{}
	svc := &AuthService{entClient: client, settingService: service, turnstileService: NewTurnstileService(service, verifier)}
	src := &RegistrationRiskSource{IP: "192.0.2.12", BrowserHash: "browser"}
	ctx := WithRegistrationRiskSource(context.Background(), src)
	now := time.Now().UTC()
	require.NoError(t, svc.VerifySignupCaptcha(ctx, CaptchaProof{}, src.IP, "one@example.org", false))
	require.NoError(t, src.recordEvent(ctx, client, 1, "register", now))
	require.ErrorIs(t, svc.VerifySignupCaptcha(ctx, CaptchaProof{}, src.IP, "two@example.org", false), ErrRegistrationCaptchaUnavailable)
	settings.values[SettingKeyTurnstileEnabled] = "false"
	settings.values[SettingKeyTurnstileSiteKey] = "widget-id"
	settings.values[SettingKeyTurnstileSecretKey] = "test-secret"
	provider, err := svc.SignupChallengeProvider(ctx)
	require.NoError(t, err)
	require.Equal(t, "turnstile", provider)
	other := WithRegistrationRiskSource(context.Background(), &RegistrationRiskSource{IP: "192.0.2.44", BrowserHash: "another-browser"})
	provider, err = svc.SignupChallengeProvider(other)
	require.NoError(t, err)
	require.Empty(t, provider)
	verify := func(token, email string, skip bool) error {
		return svc.VerifySignupCaptcha(ctx, CaptchaProof{TurnstileToken: token}, src.IP, email, skip)
	}
	require.ErrorIs(t, verify("", "two@example.org", false), ErrRegistrationCaptchaRequired)
	require.ErrorIs(t, verify("invalid", "two@example.org", false), ErrTurnstileVerificationFailed)
	require.ErrorIs(t, verify("offline", "two@example.org", false), ErrRegistrationCaptchaUnavailable)
	require.NoError(t, verify("verified", "two@example.org", false))
	require.ErrorIs(t, verify("", "other@example.org", true), ErrRegistrationCaptchaRequired)
	require.NoError(t, verify("", "two@example.org", true))
	require.Equal(t, 3, verifier.calls)
	src.OAuth = true
	require.NoError(t, svc.VerifyOAuthStartCaptcha(ctx, CaptchaProof{TurnstileToken: "verified"}, src.IP))
	cfg, err := ReadAccountRiskConfig(ctx, settings)
	require.NoError(t, err)
	require.NoError(t, src.CompleteRegistration(ctx, client, cfg, 12, "oauth-user@example.org", now))
	require.ErrorIs(t, src.CompleteRegistration(ctx, client, cfg, 13, "oauth-user2@example.org", now), ErrRegistrationCaptchaRequired)
	src.OAuth = false
	originalBrowser := src.BrowserHash
	src.BrowserHash = "another-browser"
	require.ErrorIs(t, verify("", "two@example.org", true), ErrRegistrationCaptchaRequired)
	src.BrowserHash = originalBrowser
	require.NoError(t, src.CompleteRegistration(ctx, client, cfg, 2, "two@example.org", now))
	require.ErrorIs(t, src.CompleteRegistration(ctx, client, cfg, 3, "two@example.org", now), ErrRegistrationCaptchaRequired)
	settings.values[registrationModeKey] = "observe"
	settings.values[SettingKeyTurnstileEnabled] = "false"
	require.NoError(t, verify("", "three@example.org", false))
	svc.FinishRegistrationRiskRequest(ctx)
	var hits int
	require.NoError(t, db.QueryRow("SELECT hits FROM account_registration_risk_counts WHERE action='challenge'").Scan(&hits))
	require.Equal(t, 1, hits)
}
