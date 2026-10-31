package middleware

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type supportGuardChecker struct{ allowed bool }

func (s *supportGuardChecker) CanAccessUserSupport(context.Context, int64) (bool, error) {
	return s.allowed, nil
}

func TestUserSupportBackendModeOnlyAllowsOwnAuthMe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, allowed := range []bool{false, true} {
		for _, path := range []string{"/api/v1/auth/me", "/api/v1/user/profile", "/api/v1/manager/redeem-codes"} {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set(string(ContextKeyUser), AuthSubject{UserID: 7})
				c.Set(string(ContextKeyUserRole), "user")
				c.Next()
			})
			r.Use(BackendModeUserGuard(newBackendModeSettingService(t, "true"), &supportGuardChecker{allowed: allowed}))
			r.GET(path, func(c *gin.Context) { c.Status(204) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			want := 403
			if allowed && path == "/api/v1/auth/me" {
				want = 204
			}
			require.Equal(t, want, w.Code, path)
		}
	}
}
