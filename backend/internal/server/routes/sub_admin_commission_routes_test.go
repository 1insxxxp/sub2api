package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSubAdminCommissionRoutesHideLogsAndProtectExclusions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := &handler.Handlers{Admin: &handler.AdminHandlers{SubAdminCommission: &adminhandler.SubAdminCommissionHandler{}}}
	adminAuth := middleware.AdminAuthMiddleware(func(c *gin.Context) { c.AbortWithStatus(http.StatusForbidden) })
	workbenchAuth := middleware.AdminWorkbenchAuthMiddleware(func(c *gin.Context) { c.Next() })
	audit := middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	stepUp := middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	RegisterAdminRoutes(router.Group("/api/v1"), h, adminAuth, workbenchAuth, audit, stepUp, nil, &middleware.PanelRateLimiter{})
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		path := "/api/v1/admin/sub-admin-commissions/exclusions"
		if method == http.MethodDelete {
			path += "/7"
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		require.Equal(t, http.StatusForbidden, rec.Code, method)
	}
	for _, path := range []string{
		"/api/v1/admin/workbench/commission/exclusions",
		"/api/v1/admin/workbench/commission/days/2026-10-18/groups/1/logs",
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusNotFound, rec.Code, path)
	}
}
