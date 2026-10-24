//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminWorkbenchAuthRejectsBalanceRedeemOnlyUserForSensitiveRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "test-secret", ExpireHour: 1}}
	authService := service.NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	user := &service.User{
		ID: 7, Email: "balance-redeem-only@example.com", Role: service.RoleUser,
		Status: service.StatusActive, TokenVersion: 1, Concurrency: 1,
		BalanceRedeemCodeEnabled: true,
	}
	userService := service.NewUserService(&stubUserRepo{
		getByID: func(_ context.Context, id int64) (*service.User, error) {
			if id != user.ID {
				return nil, service.ErrUserNotFound
			}
			clone := *user
			return &clone, nil
		},
	}, nil, nil, nil)
	token, err := authService.GenerateToken(context.Background(), user)
	require.NoError(t, err)

	router := gin.New()
	workbench := router.Group("/api/v1/admin/workbench")
	workbench.Use(gin.HandlerFunc(NewAdminWorkbenchAuthMiddleware(authService, userService, nil, nil)))
	handlerReached := false
	for _, path := range []string{
		"/commission/grants", "/commission/calendar", "/commission/monthly-report",
		"/commission/days/:date/groups", "/affiliates/leaderboard",
	} {
		workbench.GET(path, func(c *gin.Context) {
			handlerReached = true
			c.Status(http.StatusOK)
		})
	}
	for _, path := range []string{
		"/commission/grants", "/commission/calendar", "/commission/monthly-report",
		"/commission/days/2026-10-25/groups", "/affiliates/leaderboard",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/workbench"+path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusForbidden, rec.Code)
			require.Contains(t, rec.Body.String(), "FORBIDDEN")
			require.False(t, handlerReached)
		})
	}
}
