package routes

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type supportRouteRepo struct {
	enabled bool
	grants  int
}

func (s *supportRouteRepo) CanAccess(context.Context, int64) (bool, error) { return s.enabled, nil }
func (s *supportRouteRepo) GetAccess(context.Context, int64, int64) (bool, error) {
	return false, service.ErrUserSupportForbidden
}
func (s *supportRouteRepo) SetAccess(context.Context, int64, int64, bool) error {
	return service.ErrUserSupportForbidden
}
func (s *supportRouteRepo) LookupAndGrant(context.Context, int64, string) (*service.UserSupportGrantResult, error) {
	s.grants++
	return &service.UserSupportGrantResult{User: service.UserSupportUser{ID: 12, Email: "customer@example.com", Balance: 15}, Trial: service.UserSupportTrial{Status: "granted", Amount: 15}}, nil
}
func (s *supportRouteRepo) ListUsage(context.Context, int64) ([]service.UserSupportUsageRecord, error) {
	return nil, nil
}
func (s *supportRouteRepo) ListErrors(context.Context, int64) ([]service.UserSupportErrorRecord, error) {
	return nil, nil
}

func TestUserSupportRoutesRequireAdminRoleAndIndependentCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &supportRouteRepo{enabled: true}
	h := &handler.Handlers{Redeem: &handler.RedeemHandler{}, Admin: &handler.AdminHandlers{User: &admin.UserHandler{}, UserSupport: admin.NewUserSupportHandler(service.NewUserSupportService(repo, nil, nil))}}
	r := gin.New()
	v1 := r.Group("/api/v1")
	deny := func(c *gin.Context) { c.AbortWithStatus(403) }
	noop := func(c *gin.Context) { c.Next() }
	role := service.RoleUser
	jwt := middleware.JWTAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("Authorization") != "Bearer fixture" {
			c.AbortWithStatus(401)
			return
		}
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 8})
		c.Set(string(middleware.ContextKeyUserRole), role)
		c.Next()
	})
	RegisterAdminRoutes(v1, h, middleware.AdminAuthMiddleware(deny), middleware.AdminWorkbenchAuthMiddleware(deny), middleware.AuditLogMiddleware(noop), middleware.StepUpAuthMiddleware(noop), nil, nil)
	RegisterUserSupportRoutes(v1, h, jwt, middleware.AuditLogMiddleware(noop), nil, nil)
	request := func(method, path, token string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(`{"account":"12"}`))
		req.Header.Set("Authorization", token)
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	require.Equal(t, 401, request("POST", "/api/v1/admin/user-support/lookup", "").Code)
	require.Zero(t, repo.grants)
	// Even a stale or erroneous explicit capability cannot promote a normal user.
	require.Equal(t, 403, request("POST", "/api/v1/admin/user-support/lookup", "Bearer fixture").Code)
	require.Equal(t, 403, request("GET", "/api/v1/admin/user-support/access", "Bearer fixture").Code)
	require.Zero(t, repo.grants)
	role = service.RoleSubAdmin
	require.Equal(t, 200, request("POST", "/api/v1/admin/user-support/lookup", "Bearer fixture").Code)
	require.Equal(t, 1, repo.grants)
	for _, path := range []string{"/api/v1/admin/users", "/api/v1/admin/workbench/redeem/generated", "/api/v1/admin/users/8/support-access"} {
		require.Equal(t, 403, request("GET", path, "Bearer fixture").Code, path)
	}
	repo.enabled = false
	require.Equal(t, 403, request("POST", "/api/v1/admin/user-support/lookup", "Bearer fixture").Code)
	require.Equal(t, 1, repo.grants)
}
