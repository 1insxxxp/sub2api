package handler

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/gemini"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// CategoryDefaultCodexModels is a local manifest: it must not request a source
// account's upstream catalogue or accidentally add unpublished model names.
func (h *GatewayHandler) CategoryDefaultCodexModels(c *gin.Context) {
	key, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || key == nil || key.Group == nil || !key.Group.IsCategoryDefault || h.apiKeyService == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "service_unavailable_error", "默认分组暂不可用")
		return
	}
	models, err := h.apiKeyService.ListCategoryDefaultGroupModels(c.Request.Context(), key, "")
	if err != nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "service_unavailable_error", "默认分组暂不可用")
		return
	}
	manifest, err := service.BuildCodexModelsManifest(models)
	if err != nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "service_unavailable_error", "默认分组暂不可用")
		return
	}
	c.Data(http.StatusOK, "application/json", manifest)
}

func (h *GatewayHandler) categoryDefaultModels(c *gin.Context, key *service.APIKey, google bool) {
	if h.apiKeyService == nil {
		h.categoryDefaultModelsError(c, google)
		return
	}
	platform := ""
	if google {
		platform = service.PlatformGemini
	}
	models, err := h.apiKeyService.ListCategoryDefaultGroupModels(c.Request.Context(), key, platform)
	if err != nil {
		h.categoryDefaultModelsError(c, google)
		return
	}
	if google {
		requested := strings.TrimPrefix(strings.TrimSpace(c.Param("model")), "models/")
		if requested != "" {
			for _, model := range models {
				if strings.EqualFold(model, requested) {
					c.JSON(http.StatusOK, gemini.FallbackModel(model))
					return
				}
			}
			googleError(c, http.StatusNotFound, "Model is not available for this default group")
			return
		}
		c.JSON(http.StatusOK, customGroupGeminiModels(models))
		return
	}
	writeOpenAIModelsList(c, models)
}

func (h *GatewayHandler) categoryDefaultModelsError(c *gin.Context, google bool) {
	if google {
		googleError(c, http.StatusServiceUnavailable, "默认分组暂不可用")
		return
	}
	h.errorResponse(c, http.StatusServiceUnavailable, "service_unavailable_error", "默认分组暂不可用")
}
