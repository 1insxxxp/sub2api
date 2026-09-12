package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterInternalRoutes(v1 *gin.RouterGroup, h *handler.Handlers, cfg *config.Config) {
	if v1 == nil || h == nil || cfg == nil {
		return
	}

	internal := v1.Group("/internal")
	if h.InternalDujiaoAuth != nil && cfg.DujiaoLogin.Enabled {
		internal.Group("/dujiao/auth").POST("/verify", h.InternalDujiaoAuth.Verify)
	}
	if h.PublicGroupSync != nil {
		internal.GET("/public-group-sync/snapshot", h.PublicGroupSync.Snapshot)
	}
}

func RegisterPublicGroupSyncRoute(api *gin.RouterGroup, h *handler.Handlers) {
	if api != nil && h != nil && h.PublicGroupSync != nil {
		api.GET("/internal/public-group-sync/snapshot", h.PublicGroupSync.Snapshot)
	}
}
