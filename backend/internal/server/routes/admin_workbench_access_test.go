package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type workbenchRouteAccessRepo struct {
	service.AdminWorkbenchAccessRepository
	access service.AdminWorkbenchAccess
	calls  int
}

func (s *workbenchRouteAccessRepo) GetAccess(context.Context, int64) (*service.AdminWorkbenchAccess, error) {
	s.calls++
	return &s.access, nil
}

func TestAdminWorkbenchRoutesRequireEachDataCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []string{service.RoleSubAdmin, service.RoleUser} {
		t.Run(role, func(t *testing.T) {
			repo := &workbenchRouteAccessRepo{access: service.AdminWorkbenchAccess{UserSupport: true, BalanceTransfer: true}}
			h := &handler.Handlers{Admin: &handler.AdminHandlers{
				SubAdminCommission: &adminhandler.SubAdminCommissionHandler{},
				Affiliate:          &adminhandler.AffiliateHandler{},
				WorkbenchAccess:    adminhandler.NewAdminWorkbenchAccessHandler(service.NewAdminWorkbenchAccessService(repo)),
			}}
			r := gin.New()
			deny := func(c *gin.Context) { c.AbortWithStatus(403) }
			noop := func(c *gin.Context) { c.Next() }
			workbenchAuth := middleware.AdminWorkbenchAuthMiddleware(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 8})
				c.Set(string(middleware.ContextKeyUserRole), role)
				c.Next()
			})
			RegisterAdminRoutes(r.Group("/api/v1"), h, middleware.AdminAuthMiddleware(deny), workbenchAuth, middleware.AuditLogMiddleware(noop), middleware.StepUpAuthMiddleware(noop), nil, nil)
			get := func(path string) *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/workbench/"+path, nil))
				return w
			}
			for _, path := range []string{"commission/grants", "commission/calendar", "commission/monthly-report", "commission/days/2026-10-31/groups", "affiliates/leaderboard"} {
				require.Equal(t, 403, get(path).Code, path)
			}
			if role == service.RoleSubAdmin {
				require.Equal(t, 200, get("access").Code)
				repo.access.Commission = true
				// The permission guard now passes to the intentionally unconfigured
				// report service (503); revocation must block the next call instead.
				require.Equal(t, 503, get("commission/monthly-report").Code)
				repo.access.Commission = false
				require.Equal(t, 403, get("commission/monthly-report").Code)
			} else {
				require.Equal(t, 403, get("access").Code)
				require.Zero(t, repo.calls)
			}
		})
	}
}
