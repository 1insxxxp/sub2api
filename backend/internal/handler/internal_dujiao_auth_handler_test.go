package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const internalDujiaoTestSecret = "0123456789abcdef0123456789abcdef"

type internalDujiaoVerifierCall struct {
	email    string
	password string
}

type internalDujiaoVerifierStub struct {
	user  *service.ExternalCredentialUser
	err   error
	calls []internalDujiaoVerifierCall
}

func (s *internalDujiaoVerifierStub) VerifyExternalCredential(_ context.Context, email, password string) (*service.ExternalCredentialUser, error) {
	s.calls = append(s.calls, internalDujiaoVerifierCall{email: email, password: password})
	return s.user, s.err
}

func newTestInternalDujiaoAuthHandler(t *testing.T, verifier *internalDujiaoVerifierStub) *InternalDujiaoAuthHandler {
	t.Helper()
	h := NewInternalDujiaoAuthHandler(&config.Config{
		DujiaoLogin: config.DujiaoLoginConfig{
			Enabled:      true,
			SharedSecret: internalDujiaoTestSecret,
		},
	}, verifier)
	require.NotNil(t, h)
	return h
}

func performInternalDujiaoVerifyRequest(t *testing.T, h *InternalDujiaoAuthHandler, secret, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/verify", h.Verify)

	req := httptest.NewRequest(http.MethodPost, "/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-Sub2-Internal-Secret", secret)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestInternalDujiaoAuthVerifyReturnsSafeUserWithoutToken(t *testing.T) {
	verifier := &internalDujiaoVerifierStub{
		user: &service.ExternalCredentialUser{
			ID:       1,
			Email:    "user@example.com",
			Username: "Alice",
			Role:     "user",
			Status:   "active",
		},
	}
	h := newTestInternalDujiaoAuthHandler(t, verifier)

	rec := performInternalDujiaoVerifyRequest(t, h, internalDujiaoTestSecret, `{"email":"user@example.com","password":"secret-123"}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, verifier.calls, 1)
	require.Equal(t, internalDujiaoVerifierCall{email: "user@example.com", password: "secret-123"}, verifier.calls[0])
	require.NotContains(t, rec.Body.String(), "token")

	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			User service.ExternalCredentialUser `json:"user"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.Equal(t, "success", payload.Message)
	require.Equal(t, service.ExternalCredentialUser{
		ID:       1,
		Email:    "user@example.com",
		Username: "Alice",
		Role:     "user",
		Status:   "active",
	}, payload.Data.User)
}

func TestInternalDujiaoAuthVerifyRejectsMissingWrongOrBlankSecretBeforeCredentials(t *testing.T) {
	tests := []struct {
		name   string
		secret string
	}{
		{name: "missing"},
		{name: "wrong", secret: "wrong-secret"},
		{name: "blank", secret: "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := &internalDujiaoVerifierStub{
				user: &service.ExternalCredentialUser{ID: 1, Email: "user@example.com"},
			}
			h := newTestInternalDujiaoAuthHandler(t, verifier)

			rec := performInternalDujiaoVerifyRequest(t, h, tt.secret, `{"email":"user@example.com","password":"secret-123"}`)

			require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
			require.Empty(t, verifier.calls)
			require.NotContains(t, rec.Body.String(), "INVALID_CREDENTIALS")
		})
	}
}

func TestInternalDujiaoAuthVerifyCredentialFailuresAreGenericUnauthorized(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "invalid credentials", err: service.ErrInvalidCredentials},
		{name: "totp required", err: service.ErrExternalLogin2FARequired},
		{name: "inactive user", err: service.ErrUserNotActive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := &internalDujiaoVerifierStub{err: tt.err}
			h := newTestInternalDujiaoAuthHandler(t, verifier)

			rec := performInternalDujiaoVerifyRequest(t, h, internalDujiaoTestSecret, `{"email":"user@example.com","password":"secret-123"}`)

			require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
			require.Len(t, verifier.calls, 1)
			require.Contains(t, rec.Body.String(), "invalid email or password")
			require.NotContains(t, rec.Body.String(), "USER_NOT_ACTIVE")
			require.NotContains(t, rec.Body.String(), "EXTERNAL_LOGIN_2FA_REQUIRED")
			require.NotContains(t, rec.Body.String(), "INVALID_CREDENTIALS")
		})
	}
}

func TestInternalDujiaoAuthVerifyServiceUnavailable(t *testing.T) {
	verifier := &internalDujiaoVerifierStub{err: service.ErrServiceUnavailable}
	h := newTestInternalDujiaoAuthHandler(t, verifier)

	rec := performInternalDujiaoVerifyRequest(t, h, internalDujiaoTestSecret, `{"email":"user@example.com","password":"secret-123"}`)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "SERVICE_UNAVAILABLE")
}

func TestInternalDujiaoAuthVerifyRejectsBadJSONOrInvalidBody(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "bad json", body: `{"email":`},
		{name: "missing email", body: `{"password":"secret-123"}`},
		{name: "invalid email", body: `{"email":"not-an-email","password":"secret-123"}`},
		{name: "missing password", body: `{"email":"user@example.com"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := &internalDujiaoVerifierStub{}
			h := newTestInternalDujiaoAuthHandler(t, verifier)

			rec := performInternalDujiaoVerifyRequest(t, h, internalDujiaoTestSecret, tt.body)

			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.Empty(t, verifier.calls)
		})
	}
}
