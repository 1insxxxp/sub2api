//go:build unit

package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIPGeoRateLimitedRequestSkipsAuditBeforeHeavyLimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	audited := false
	router.Use(func(c *gin.Context) {
		c.Next()
		audited = !c.GetBool("audit_skip")
	})
	registerIPGeoRoute(router.Group("/api/v1/user"), &handler.UserHandler{}, func(c *gin.Context) {
		c.AbortWithStatus(http.StatusTooManyRequests)
	})
	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/user/ip-geo", strings.NewReader(`{"ips":["8.8.8.8"]}`))
	router.ServeHTTP(w, request)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.False(t, audited, "rate-limited read-only lookups must not save the queried customer-IP list")
}
