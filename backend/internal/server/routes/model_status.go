package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterModelStatusRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	limiter *middleware.PanelRateLimiter,
) {
	status := v1.Group("/model-status")
	status.Use(limiter.PublicIP())
	status.GET("", h.ModelStatus.Get)
	status.GET("/png", h.ModelStatus.GetPNG)
}
