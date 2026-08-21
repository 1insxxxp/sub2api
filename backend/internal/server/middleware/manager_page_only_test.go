//go:build unit

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestManagerPageOnlyAllowsAdminAndSubAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		role string
		want int
	}{
		{name: "admin", role: service.RoleAdmin, want: http.StatusOK},
		{name: "sub admin", role: service.RoleSubAdmin, want: http.StatusOK},
		{name: "user", role: service.RoleUser, want: http.StatusForbidden},
		{name: "missing role", role: "", want: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				if tt.role != "" {
					c.Set(string(ContextKeyUserRole), tt.role)
				}
				c.Next()
			})
			router.Use(ManagerPageOnly())
			router.GET("/t", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/t", nil)
			router.ServeHTTP(w, req)

			require.Equal(t, tt.want, w.Code)
		})
	}
}

func TestAdminOnlyRejectsSubAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUserRole), service.RoleSubAdmin)
		c.Next()
	})
	router.Use(AdminOnly())
	router.GET("/t", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
}
