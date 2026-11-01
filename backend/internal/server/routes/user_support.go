package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// RegisterUserSupportRoutes uses session authentication; the handler requires
// an administrator role and a fresh, independent support capability.
func RegisterUserSupportRoutes(v1 *gin.RouterGroup, h *handler.Handlers, jwtAuth middleware.JWTAuthMiddleware,
	audit middleware.AuditLogMiddleware, settingService *service.SettingService, limiter *middleware.PanelRateLimiter) {
	if h == nil || h.Admin == nil || h.Admin.UserSupport == nil {
		return
	}
	support := v1.Group("/admin/user-support")
	support.Use(gin.HandlerFunc(jwtAuth))
	support.Use(limiter.Global())
	support.Use(gin.HandlerFunc(audit))
	support.Use(middleware.AdminComplianceGuard(settingService))
	support.GET("/access", h.Admin.UserSupport.GetMyAccess)
	support.POST("/lookup", h.Admin.UserSupport.Lookup)
}
