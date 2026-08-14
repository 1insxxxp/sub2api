//go:build unit

package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newAffiliateReferralRegistrationHandler(
	t *testing.T,
	codeOwners map[string]int64,
) (*AuthHandler, *oauthEmailAffiliateRepoStub) {
	t.Helper()
	affiliateRepo := newOAuthEmailAffiliateRepoStub(codeOwners)
	handler, _ := newOAuthPendingFlowTestHandlerWithDependencies(t, oauthPendingFlowTestHandlerOptions{
		settingValues: map[string]string{
			service.SettingKeyAffiliateEnabled: "true",
		},
		affiliateFactory: func(_ *dbent.Client, settingSvc *service.SettingService) *service.AffiliateService {
			return service.NewAffiliateService(affiliateRepo, settingSvc, nil, nil)
		},
	})
	handler.cfg = &config.Config{JWT: config.JWTConfig{Secret: strings.Repeat("r", 32)}}
	return handler, affiliateRepo
}

func performAffiliateAuthRequest(
	t *testing.T,
	handler gin.HandlerFunc,
	path string,
	body string,
	cookies ...*http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST(path, handler)
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
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

func TestRegisterUsesLockedAffiliateReferralOverSubmittedCode(t *testing.T) {
	handler, affiliateRepo := newAffiliateReferralRegistrationHandler(t, map[string]int64{
		"LOCK123":  101,
		"MANUAL12": 202,
	})

	recorder := performAffiliateAuthRequest(
		t,
		handler.Register,
		"/register",
		`{"email":"locked@example.com","password":"secret-123","aff_code":"MANUAL12"}`,
		lockedAffiliateCookie(t, handler, "LOCK123"),
	)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, affiliateRepo.bindCalls, 1)
	require.Equal(t, int64(101), affiliateRepo.bindCalls[0].inviterID)
	requireAffiliateLockCleared(t, recorder)
}

func TestRegisterPreservesManualAffiliateReferralWithoutLock(t *testing.T) {
	handler, affiliateRepo := newAffiliateReferralRegistrationHandler(t, map[string]int64{
		"MANUAL12": 202,
	})

	recorder := performAffiliateAuthRequest(
		t,
		handler.Register,
		"/register",
		`{"email":"manual@example.com","password":"secret-123","aff_code":"MANUAL12"}`,
	)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, affiliateRepo.bindCalls, 1)
	require.Equal(t, int64(202), affiliateRepo.bindCalls[0].inviterID)
}

func TestRegisterFallsBackToManualCodeForTamperedLock(t *testing.T) {
	handler, affiliateRepo := newAffiliateReferralRegistrationHandler(t, map[string]int64{
		"MANUAL12": 202,
	})
	cookie := lockedAffiliateCookie(t, handler, "LOCK123")
	cookie.Value += "tampered"

	recorder := performAffiliateAuthRequest(
		t,
		handler.Register,
		"/register",
		`{"email":"tampered@example.com","password":"secret-123","aff_code":"MANUAL12"}`,
		cookie,
	)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, affiliateRepo.bindCalls, 1)
	require.Equal(t, int64(202), affiliateRepo.bindCalls[0].inviterID)
	requireAffiliateLockCleared(t, recorder)
}

func TestFailedRegisterKeepsAffiliateReferralLock(t *testing.T) {
	handler, _ := newAffiliateReferralRegistrationHandler(t, map[string]int64{"LOCK123": 101})

	recorder := performAffiliateAuthRequest(
		t,
		handler.Register,
		"/register",
		`{"email":"invalid@example.com","password":"short","aff_code":"MANUAL12"}`,
		lockedAffiliateCookie(t, handler, "LOCK123"),
	)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Empty(t, recorder.Result().Cookies())
}

func TestSuccessfulLoginClearsAffiliateReferralLock(t *testing.T) {
	handler, _ := newAffiliateReferralRegistrationHandler(t, nil)
	registerRecorder := performAffiliateAuthRequest(
		t,
		handler.Register,
		"/register",
		`{"email":"login@example.com","password":"secret-123"}`,
	)
	require.Equal(t, http.StatusOK, registerRecorder.Code, registerRecorder.Body.String())

	loginRecorder := performAffiliateAuthRequest(
		t,
		handler.Login,
		"/login",
		`{"email":"login@example.com","password":"secret-123"}`,
		lockedAffiliateCookie(t, handler, "LOCK123"),
	)

	require.Equal(t, http.StatusOK, loginRecorder.Code, loginRecorder.Body.String())
	requireAffiliateLockCleared(t, loginRecorder)
}
