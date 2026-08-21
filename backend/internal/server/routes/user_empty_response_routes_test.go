//go:build unit

package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserUsageRoutesDoNotRegisterBulkEmptyResponseClaim(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	noop := func(c *gin.Context) { c.Next() }

	RegisterUserRoutes(
		router.Group("/api/v1"),
		&handler.Handlers{Usage: handler.NewUsageHandler(nil, nil, nil, nil)},
		middleware.JWTAuthMiddleware(noop),
		middleware.AuditLogMiddleware(noop),
		nil,
		&middleware.PanelRateLimiter{},
	)

	for _, route := range router.Routes() {
		require.NotEqual(t, "POST /api/v1/usage/empty-responses/claim", route.Method+" "+route.Path)
	}
}
