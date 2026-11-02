package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// This narrow facade is JWT-only and rechecks user-support access per operation.
func RegisterModelProbeRoutes(v1 *gin.RouterGroup, h *handler.Handlers, jwt middleware.JWTAuthMiddleware, audit middleware.AuditLogMiddleware, settings *service.SettingService, limiter *middleware.PanelRateLimiter) {
	if h == nil || h.Admin == nil || h.Admin.ModelProbe == nil {
		return
	}
	g := v1.Group("/admin/model-probes")
	g.Use(gin.HandlerFunc(jwt))
	g.Use(limiter.Global())
	g.Use(gin.HandlerFunc(audit))
	g.Use(middleware.AdminComplianceGuard(settings))
	g.GET("/groups", h.Admin.ModelProbe.Groups)
	g.GET("/groups/:id/models", h.Admin.ModelProbe.Models)
	g.POST("/run", h.Admin.ModelProbe.Run)
	g.GET("/config/groups", h.Admin.ModelProbe.ConfigGroups)
	g.GET("/config/groups/:id", h.Admin.ModelProbe.Config)
	g.PUT("/config/groups/:id", h.Admin.ModelProbe.SaveConfig)
}
