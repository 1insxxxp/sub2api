package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) GetPublicGatewayErrorSettings(c *gin.Context) {
	settings, err := h.settingService.GetPublicGatewayErrorSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *SettingHandler) UpdatePublicGatewayErrorSettings(c *gin.Context) {
	var settings service.PublicGatewayErrorSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		response.BadRequest(c, "错误映射设置格式无效")
		return
	}
	if _, err := service.CompilePublicGatewayErrorSettings(&settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.settingService.SetPublicGatewayErrorSettings(c.Request.Context(), &settings); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *SettingHandler) PreviewPublicGatewayError(c *gin.Context) {
	var req struct {
		Settings   service.PublicGatewayErrorSettings `json:"settings"`
		Status     int                                `json:"status"`
		Diagnostic string                             `json:"diagnostic"`
		Format     string                             `json:"format"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "测试错误格式无效")
		return
	}
	preview, err := service.PreviewPublicGatewayError(&req.Settings, req.Status, req.Diagnostic, req.Format)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, preview)
}
