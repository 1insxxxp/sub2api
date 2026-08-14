//go:build unit

package handler

import (
	"bytes"
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type affiliateReferralRepoStub struct {
	service.AffiliateRepository
	summaries map[string]*service.AffiliateSummary
	err       error
	lookups   []string
}

func (r *affiliateReferralRepoStub) GetAffiliateByCode(_ context.Context, code string) (*service.AffiliateSummary, error) {
	r.lookups = append(r.lookups, code)
	if r.err != nil {
		return nil, r.err
	}
	summary := r.summaries[code]
	if summary == nil {
		return nil, service.ErrAffiliateProfileNotFound
	}
	return summary, nil
}

type affiliateReferralSettingRepoStub struct {
	service.SettingRepository
	enabled bool
}

func (r *affiliateReferralSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if key == service.SettingKeyAffiliateEnabled {
		if r.enabled {
			return "true", nil
		}
		return "false", nil
	}
	return "", service.ErrSettingNotFound
}

func newAffiliateReferralTestHandler(t *testing.T, codes ...string) *AuthHandler {
	t.Helper()
	repo := &affiliateReferralRepoStub{summaries: make(map[string]*service.AffiliateSummary)}
	for i, code := range codes {
		repo.summaries[code] = &service.AffiliateSummary{UserID: int64(i + 1), AffCode: code}
	}
	settings := service.NewSettingService(&affiliateReferralSettingRepoStub{enabled: true}, nil)
	affiliateService := service.NewAffiliateService(repo, settings, nil, nil)
	authService := service.NewAuthService(nil, nil, nil, nil, &config.Config{}, settings, nil, nil, nil, nil, nil, affiliateService, nil)
	return NewAuthHandler(&config.Config{JWT: config.JWTConfig{Secret: strings.Repeat("s", 32)}}, authService, nil, settings, nil, nil, nil, nil)
}

func affiliateReferralRequest(t *testing.T, h *AuthHandler, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/resolve", h.ResolveAffiliateReferral)
	router.GET("/status", h.GetAffiliateReferralStatus)
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestAffiliateReferralLockCodec(t *testing.T) {
	secret := strings.Repeat("x", 32)
	issuedAt := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	expiresAt := issuedAt.Add(30 * 24 * time.Hour)

	encoded, err := encodeAffiliateReferralLock(secret, "ABC123", issuedAt, expiresAt)
	require.NoError(t, err)

	code, err := decodeAffiliateReferralLock(secret, encoded, issuedAt.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, "ABC123", code)

	parts := strings.Split(encoded, ".")
	require.Len(t, parts, 2)
	tampered := parts[0] + "." + parts[1][:len(parts[1])-1] + "A"
	_, err = decodeAffiliateReferralLock(secret, tampered, issuedAt.Add(time.Hour))
	require.Error(t, err)

	expired, err := encodeAffiliateReferralLock(secret, "ABC123", issuedAt.Add(-48*time.Hour), issuedAt.Add(-24*time.Hour))
	require.NoError(t, err)
	_, err = decodeAffiliateReferralLock(secret, expired, issuedAt)
	require.Error(t, err)

	wrongVersion, err := encodeAffiliateReferralLockVersion(secret, "ABC123", issuedAt, expiresAt, affiliateReferralLockVersion+1)
	require.NoError(t, err)
	_, err = decodeAffiliateReferralLock(secret, wrongVersion, issuedAt)
	require.Error(t, err)

	futureIssued, err := encodeAffiliateReferralLock(secret, "ABC123", issuedAt.Add(time.Hour), expiresAt)
	require.NoError(t, err)
	_, err = decodeAffiliateReferralLock(secret, futureIssued, issuedAt)
	require.Error(t, err)
}

func TestResolveAffiliateReferral(t *testing.T) {
	t.Run("sets normalized secure httponly cookie for valid code", func(t *testing.T) {
		h := newAffiliateReferralTestHandler(t, "ABC123")
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/resolve", h.ResolveAffiliateReferral)
		request := httptest.NewRequest(http.MethodPost, "https://example.test/resolve", bytes.NewBufferString(`{"aff_code":" abc123 "}`))
		request.Header.Set("Content-Type", "application/json")
		request.TLS = &tls.ConnectionState{}
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Contains(t, recorder.Body.String(), `"valid":true`)
		require.Contains(t, recorder.Body.String(), `"locked":true`)
		cookies := recorder.Result().Cookies()
		require.Len(t, cookies, 1)
		require.Equal(t, affiliateReferralLockCookieName, cookies[0].Name)
		require.True(t, cookies[0].HttpOnly)
		require.True(t, cookies[0].Secure)
		require.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
		require.Equal(t, "/", cookies[0].Path)
		require.Equal(t, affiliateReferralLockMaxAgeSeconds, cookies[0].MaxAge)
		code, err := decodeAffiliateReferralLock(h.cfg.JWT.Secret, cookies[0].Value, time.Now())
		require.NoError(t, err)
		require.Equal(t, "ABC123", code)
	})

	t.Run("replaces an existing lock only with another valid code", func(t *testing.T) {
		h := newAffiliateReferralTestHandler(t, "ABC123", "XYZ789")
		oldValue, err := encodeAffiliateReferralLock(h.cfg.JWT.Secret, "ABC123", time.Now(), time.Now().Add(time.Hour))
		require.NoError(t, err)
		oldCookie := &http.Cookie{Name: affiliateReferralLockCookieName, Value: oldValue}

		recorder := affiliateReferralRequest(t, h, http.MethodPost, "/resolve", `{"aff_code":"xyz789"}`, oldCookie)

		require.Equal(t, http.StatusOK, recorder.Code)
		cookies := recorder.Result().Cookies()
		require.Len(t, cookies, 1)
		code, err := decodeAffiliateReferralLock(h.cfg.JWT.Secret, cookies[0].Value, time.Now())
		require.NoError(t, err)
		require.Equal(t, "XYZ789", code)
	})

	t.Run("invalid code does not overwrite existing lock", func(t *testing.T) {
		h := newAffiliateReferralTestHandler(t, "ABC123")
		oldValue, err := encodeAffiliateReferralLock(h.cfg.JWT.Secret, "ABC123", time.Now(), time.Now().Add(time.Hour))
		require.NoError(t, err)
		oldCookie := &http.Cookie{Name: affiliateReferralLockCookieName, Value: oldValue}

		recorder := affiliateReferralRequest(t, h, http.MethodPost, "/resolve", `{"aff_code":"UNKNOWN"}`, oldCookie)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		require.Empty(t, recorder.Result().Cookies())
	})

	t.Run("http development request does not force secure", func(t *testing.T) {
		h := newAffiliateReferralTestHandler(t, "ABC123")

		recorder := affiliateReferralRequest(t, h, http.MethodPost, "/resolve", `{"aff_code":"ABC123"}`)

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Len(t, recorder.Result().Cookies(), 1)
		require.False(t, recorder.Result().Cookies()[0].Secure)
	})
}

func TestAffiliateReferralStatus(t *testing.T) {
	h := newAffiliateReferralTestHandler(t, "ABC123")
	validValue, err := encodeAffiliateReferralLock(h.cfg.JWT.Secret, "ABC123", time.Now(), time.Now().Add(time.Hour))
	require.NoError(t, err)

	recorder := affiliateReferralRequest(t, h, http.MethodGet, "/status", "", &http.Cookie{
		Name:  affiliateReferralLockCookieName,
		Value: validValue,
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"locked":true`)
	require.NotContains(t, recorder.Body.String(), "ABC123")

	expiredValue, err := encodeAffiliateReferralLock(h.cfg.JWT.Secret, "ABC123", time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	require.NoError(t, err)
	recorder = affiliateReferralRequest(t, h, http.MethodGet, "/status", "", &http.Cookie{
		Name:  affiliateReferralLockCookieName,
		Value: expiredValue,
	})
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"locked":false`)
	require.Len(t, recorder.Result().Cookies(), 1)
	require.Less(t, recorder.Result().Cookies()[0].MaxAge, 0)

	recorder = affiliateReferralRequest(t, h, http.MethodGet, "/status", "")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"locked":false`)
	require.Empty(t, recorder.Result().Cookies())
}

func TestResolveAffiliateReferralPropagatesServiceFailure(t *testing.T) {
	h := newAffiliateReferralTestHandler(t)
	h.authService = service.NewAuthService(nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil)

	recorder := affiliateReferralRequest(t, h, http.MethodPost, "/resolve", `{"aff_code":"ABC123"}`)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestOAuthAffiliateReferralLockPrecedence(t *testing.T) {
	providers := []string{"github", "google", "linuxdo", "wechat", "oidc", "dingtalk"}
	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			h := newAffiliateReferralTestHandler(t)
			value, err := encodeAffiliateReferralLock(h.cfg.JWT.Secret, "LOCK123", time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
			require.NoError(t, err)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			req := httptest.NewRequest(http.MethodPost, "/oauth/complete", nil)
			req.AddCookie(&http.Cookie{Name: affiliateReferralLockCookieName, Value: value})
			ctx.Request = req

			require.Equal(t, "LOCK123", h.oauthAffiliateCodeForRequest(ctx, "MANUAL12", "CAPTURED1"))
		})
	}

	h := newAffiliateReferralTestHandler(t)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/oauth/complete", nil)
	require.Equal(t, "CAPTURED1", h.oauthAffiliateCodeForRequest(ctx, "MANUAL12", "CAPTURED1"))
	require.Equal(t, "MANUAL12", h.oauthAffiliateCodeForRequest(ctx, "MANUAL12", ""))
}
