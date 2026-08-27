package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterInternalRoutes(v1 *gin.RouterGroup, h *handler.Handlers, cfg *config.Config) {
	if v1 == nil || h == nil || h.InternalDujiaoAuth == nil || cfg == nil || !cfg.DujiaoLogin.Enabled {
		return
	}

	internal := v1.Group("/internal")
	dujiaoAuth := internal.Group("/dujiao/auth")
	dujiaoAuth.POST("/verify", h.InternalDujiaoAuth.Verify)
}
