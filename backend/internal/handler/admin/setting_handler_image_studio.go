package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GetImageStudioConfig 获取 AI Image Studio 配置。
// GET /api/v1/admin/settings/image-studio
func (h *SettingHandler) GetImageStudioConfig(c *gin.Context) {
	cfg, err := h.settingService.GetImageStudioConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, imageStudioConfigResponse(cfg))
}

// UpdateImageStudioConfig 更新 AI Image Studio 配置。
// PUT /api/v1/admin/settings/image-studio
func (h *SettingHandler) UpdateImageStudioConfig(c *gin.Context) {
	var req service.ImageStudioSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.settingService.SaveImageStudioConfig(c.Request.Context(), &req); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	cfg, err := h.settingService.GetImageStudioConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, imageStudioConfigResponse(cfg))
}

// TestImageStudioStorage returns the storage status for the current Image Studio config.
// POST /api/v1/admin/settings/image-studio/storage/test
func (h *SettingHandler) TestImageStudioStorage(c *gin.Context) {
	cfg, err := h.settingService.GetImageStudioConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, service.ImageStudioStorageStatusForConfig(cfg))
}

func imageStudioConfigResponse(cfg *service.ImageStudioSettings) gin.H {
	if cfg == nil {
		cfg = &service.ImageStudioSettings{}
	}
	return gin.H{
		"enabled":                cfg.Enabled,
		"allowed_models":         cfg.AllowedModels,
		"default_model":          cfg.DefaultModel,
		"storage_driver":         cfg.StorageDriver,
		"local_root_dir":         cfg.LocalRootDir,
		"local_public_base_url":  cfg.LocalPublicBaseURL,
		"r2_public_base_url":     cfg.R2PublicBaseURL,
		"retention_days":         cfg.RetentionDays,
		"max_images_per_user":    cfg.MaxImagesPerUser,
		"max_reference_image_mb": cfg.MaxReferenceImageMB,
		"aspect_ratios":          cfg.AspectRatios,
		"storage_status":         service.ImageStudioStorageStatusForConfig(cfg),
	}
}
