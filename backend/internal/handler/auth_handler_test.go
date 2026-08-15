//go:build unit

package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newAffiliateReferralRegistrationHandler(
	t *testing.T,
	codeOwners map[string]int64,
) (*AuthHandler, *oauthEmailAffiliateRepoStub) {
	t.Helper()
	handler, _, affiliateRepo := newAffiliateReferralOAuthHandler(t, codeOwners)
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
