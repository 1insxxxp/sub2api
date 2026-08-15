package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newAffiliateReferralOAuthHandler(
	t *testing.T,
	codeOwners map[string]int64,
) (*AuthHandler, *dbent.Client, *oauthEmailAffiliateRepoStub) {
	t.Helper()
	affiliateRepo := newOAuthEmailAffiliateRepoStub(codeOwners)
	handler, client := newOAuthPendingFlowTestHandlerWithDependencies(t, oauthPendingFlowTestHandlerOptions{
		settingValues: map[string]string{
			service.SettingKeyAffiliateEnabled: "true",
		},
		affiliateFactory: func(_ *dbent.Client, settingSvc *service.SettingService) *service.AffiliateService {
			return service.NewAffiliateService(affiliateRepo, settingSvc, nil, nil)
		},
	})
	handler.cfg = &config.Config{JWT: config.JWTConfig{Secret: strings.Repeat("r", 32)}}
	return handler, client, affiliateRepo
}

func lockedAffiliateCookie(t *testing.T, handler *AuthHandler, code string) *http.Cookie {
	t.Helper()
	value, err := encodeAffiliateReferralLock(
		handler.cfg.JWT.Secret,
		code,
		time.Now().Add(-time.Minute),
		time.Now().Add(time.Hour),
	)
	require.NoError(t, err)
	return &http.Cookie{Name: affiliateReferralLockCookieName, Value: value, Path: "/"}
}

func requireAffiliateLockCleared(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == affiliateReferralLockCookieName {
			require.Less(t, cookie.MaxAge, 0)
			return
		}
	}
	t.Fatalf("expected %s cookie to be cleared", affiliateReferralLockCookieName)
}
