//go:build unit

package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCategoryDefaultManagementRoutesRequireFullAdminBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	fullAdmin := router.Group("/api/v1/admin")
	fullAdmin.Use(func(c *gin.Context) { c.AbortWithStatus(http.StatusForbidden) })
	h := &handler.Handlers{Admin: &handler.AdminHandlers{CategoryDefaultGroup: admin.NewCategoryDefaultGroupHandler(nil)}}
	registerCategoryDefaultGroupRoutes(fullAdmin, h)
	for _, endpoint := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/category-default-groups"},
		{http.MethodGet, "/api/v1/admin/category-default-groups/gemini/candidates"},
		{http.MethodPost, "/api/v1/admin/category-default-groups/gemini/sync-preview"},
		{http.MethodPost, "/api/v1/admin/category-default-groups/gemini/sync-apply"},
		{http.MethodPut, "/api/v1/admin/category-default-groups/gemini/status"},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader(`{}`)))
		require.Equal(t, http.StatusForbidden, recorder.Code, endpoint.path)
	}
}
