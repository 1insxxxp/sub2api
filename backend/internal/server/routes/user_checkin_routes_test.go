package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterUserRoutesIncludesCheckinEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterUserRoutes(
		v1,
		&handler.Handlers{
			Checkin: &handler.CheckinHandler{},
		},
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) {
			c.Next()
		}),
		nil,
	)

	routesByMethodAndPath := make(map[string]struct{})
	for _, route := range router.Routes() {
		routesByMethodAndPath[route.Method+" "+route.Path] = struct{}{}
	}

	_, hasStatus := routesByMethodAndPath["GET /api/v1/user/checkin/status"]
	require.True(t, hasStatus)

	_, hasCheckin := routesByMethodAndPath["POST /api/v1/user/checkin"]
	require.True(t, hasCheckin)
}
