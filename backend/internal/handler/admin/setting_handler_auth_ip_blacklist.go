package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GetAuthIPBlacklistSettings 获取认证入口全局 IP 黑名单配置。
// GET /api/v1/admin/settings/auth-ip-blacklist
func (h *SettingHandler) GetAuthIPBlacklistSettings(c *gin.Context) {
	settings, err := h.settingService.GetAuthIPBlacklistSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

// UpdateAuthIPBlacklistSettings 更新认证入口全局 IP 黑名单配置。
// PUT /api/v1/admin/settings/auth-ip-blacklist
func (h *SettingHandler) UpdateAuthIPBlacklistSettings(c *gin.Context) {
	var req service.AuthIPBlacklistSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings, err := h.settingService.SetAuthIPBlacklistSettings(c.Request.Context(), &req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}
