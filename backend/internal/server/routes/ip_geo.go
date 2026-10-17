package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func registerIPGeoRoute(user *gin.RouterGroup, userHandler *handler.UserHandler, heavy gin.HandlerFunc) {
	// Skip before the limiter: rejected lookups must not persist customer-IP lists either.
	user.POST("/ip-geo", middleware.SkipAudit, heavy, userHandler.GetIPGeo)
}
