//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type supportRefreshUserRepoStub struct {
	service.UserRepository
	user         *service.User
	requestedIDs []int64
}

func (s *supportRefreshUserRepoStub) GetByID(_ context.Context, id int64) (*service.User, error) {
	s.requestedIDs = append(s.requestedIDs, id)
	if id != s.user.ID {
		return nil, service.ErrUserNotFound
	}
	u := *s.user
	return &u, nil
}

type supportRefreshTokenCacheStub struct {
	userHandlerRefreshTokenCacheStub
	tokens map[string]service.RefreshTokenData
}

func (s *supportRefreshTokenCacheStub) StoreRefreshToken(_ context.Context, hash string, data *service.RefreshTokenData, _ time.Duration) error {
	s.tokens[hash] = *data
	return nil
}

func (s *supportRefreshTokenCacheStub) GetRefreshToken(_ context.Context, hash string) (*service.RefreshTokenData, error) {
	data, ok := s.tokens[hash]
	if !ok {
		return nil, service.ErrRefreshTokenNotFound
	}
	return &data, nil
}

func (s *supportRefreshTokenCacheStub) DeleteRefreshToken(_ context.Context, hash string) error {
	delete(s.tokens, hash)
	return nil
}

type supportRefreshAccessRepoStub struct {
	service.UserSupportRepository
	userID     int64
	enabled    bool
	checkedIDs []int64
}

func (s *supportRefreshAccessRepoStub) CanAccess(_ context.Context, id int64) (bool, error) {
	s.checkedIDs = append(s.checkedIDs, id)
	return s.enabled && id == s.userID, nil
}

func TestUserSupportBackendModeRefreshUsesServerIdentityAndRechecksAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &service.User{ID: 731, Email: "support-refresh@example.com", Role: service.RoleUser, Status: service.StatusActive}
	users := &supportRefreshUserRepoStub{user: user}
	cache := &supportRefreshTokenCacheStub{tokens: map[string]service.RefreshTokenData{}}
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "support-refresh-test-secret", ExpireHour: 1, RefreshTokenExpireDays: 7}}
	auth := service.NewAuthService(nil, users, nil, cache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	access := &supportRefreshAccessRepoStub{userID: user.ID, enabled: true}
	h := newBackendModeAuthHandler()
	h.authService = auth
	h.userSupport = service.NewUserSupportService(access, nil, nil)
	router := gin.New()
	router.POST("/api/v1/auth/refresh", h.RefreshToken)

	pair, err := auth.GenerateTokenPair(context.Background(), user, "support-refresh-session")
	require.NoError(t, err)
	refresh := func(token string) *httptest.ResponseRecorder {
		// Client-supplied identity must not replace the user attached to the refresh token.
		body, marshalErr := json.Marshal(map[string]any{"refresh_token": token, "user_id": 1, "role": service.RoleAdmin})
		require.NoError(t, marshalErr)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}

	response := refresh(pair.RefreshToken)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var success struct {
		Code int                        `json:"code"`
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &success))
	require.Zero(t, success.Code)
	require.Len(t, success.Data, 4)
	for _, field := range []string{"access_token", "refresh_token", "expires_in", "token_type"} {
		require.Contains(t, success.Data, field)
	}
	var rotated service.TokenPair
	require.NoError(t, json.Unmarshal(mustMarshalSupportRefreshData(t, success.Data), &rotated))
	require.NotEmpty(t, rotated.AccessToken)
	require.NotEmpty(t, rotated.RefreshToken)
	require.NotEqual(t, pair.RefreshToken, rotated.RefreshToken)
	claims, err := auth.ValidateToken(rotated.AccessToken)
	require.NoError(t, err)
	require.Equal(t, user.ID, claims.UserID)
	require.Equal(t, service.RoleUser, claims.Role)
	require.False(t, user.CanAccessAdminWorkbench())
	require.Equal(t, []int64{user.ID}, users.requestedIDs)
	require.Equal(t, []int64{user.ID}, access.checkedIDs)

	// A still-valid refresh token does not preserve a revoked support capability.
	access.enabled = false
	response = refresh(rotated.RefreshToken)
	require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), "BACKEND_MODE_MANAGER_ONLY")
	require.NotContains(t, response.Body.String(), "access_token")
	require.NotContains(t, response.Body.String(), "refresh_token")
	require.Equal(t, []int64{user.ID, user.ID}, users.requestedIDs)
	require.Equal(t, []int64{user.ID, user.ID}, access.checkedIDs)
}

func mustMarshalSupportRefreshData(t *testing.T, data map[string]json.RawMessage) []byte {
	t.Helper()
	body, err := json.Marshal(data)
	require.NoError(t, err)
	return body
}

func TestUserSupportRefreshIdentityStaysInternal(t *testing.T) {
	body, err := json.Marshal(service.TokenPairWithUser{UserID: 731})
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &fields))
	require.NotContains(t, fields, "UserID")
	require.NotContains(t, fields, "user_id")
}
