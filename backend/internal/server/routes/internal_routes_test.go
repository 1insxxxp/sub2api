package routes

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const internalDujiaoRouteTestSecret = "0123456789abcdef0123456789abcdef"

type internalDujiaoRouteVerifierStub struct {
	user  *service.ExternalCredentialUser
	calls int
}

func (s *internalDujiaoRouteVerifierStub) VerifyExternalCredential(context.Context, string, string) (*service.ExternalCredentialUser, error) {
	s.calls++
	return s.user, nil
}

func newInternalDujiaoRouteRequest(t *testing.T, secret string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/internal/dujiao/auth/verify",
		bytes.NewBufferString(`{"email":"user@example.com","password":"secret-123"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-Sub2-Internal-Secret", secret)
	}
	return req
}

func TestInternalDujiaoRoutesRegisterVerifyWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		DujiaoLogin: config.DujiaoLoginConfig{
			Enabled:      true,
			SharedSecret: internalDujiaoRouteTestSecret,
		},
	}
	verifier := &internalDujiaoRouteVerifierStub{
		user: &service.ExternalCredentialUser{
			ID:       1,
			Email:    "user@example.com",
			Username: "Alice",
			Role:     "user",
			Status:   "active",
		},
	}
	internalHandler := handler.NewInternalDujiaoAuthHandler(cfg, verifier)

	router := gin.New()
	RegisterInternalRoutes(router.Group("/api/v1"), &handler.Handlers{InternalDujiaoAuth: internalHandler}, cfg)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, newInternalDujiaoRouteRequest(t, internalDujiaoRouteTestSecret))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 1, verifier.calls)
}

func TestInternalDujiaoRoutesDoNotRegisterVerifyWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabledCfg := &config.Config{
		DujiaoLogin: config.DujiaoLoginConfig{
			Enabled:      true,
			SharedSecret: internalDujiaoRouteTestSecret,
		},
	}
	disabledCfg := &config.Config{
		DujiaoLogin: config.DujiaoLoginConfig{
			Enabled:      false,
			SharedSecret: internalDujiaoRouteTestSecret,
		},
	}
	verifier := &internalDujiaoRouteVerifierStub{
		user: &service.ExternalCredentialUser{ID: 1, Email: "user@example.com"},
	}
	internalHandler := handler.NewInternalDujiaoAuthHandler(enabledCfg, verifier)

	router := gin.New()
	RegisterInternalRoutes(router.Group("/api/v1"), &handler.Handlers{InternalDujiaoAuth: internalHandler}, disabledCfg)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, newInternalDujiaoRouteRequest(t, internalDujiaoRouteTestSecret))

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	require.Zero(t, verifier.calls)
}

func TestInternalDujiaoRoutesDoNotRegisterVerifyWithoutHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		DujiaoLogin: config.DujiaoLoginConfig{
			Enabled:      true,
			SharedSecret: internalDujiaoRouteTestSecret,
		},
	}

	router := gin.New()
	RegisterInternalRoutes(router.Group("/api/v1"), &handler.Handlers{}, cfg)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, newInternalDujiaoRouteRequest(t, internalDujiaoRouteTestSecret))

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}
