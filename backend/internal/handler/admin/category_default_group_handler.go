package admin

import (
	"context"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type categoryDefaultGroupAdminService interface {
	List(context.Context) ([]service.CategoryDefaultGroupConfig, error)
	Candidates(context.Context, string) ([]service.CategoryDefaultSourceCandidate, error)
	Preview(context.Context, string, []int64) (*service.CategoryDefaultGroupPreview, error)
	Apply(context.Context, string, service.CategoryDefaultGroupApplyRequest) (*service.CategoryDefaultGroupConfig, error)
	SetStatus(context.Context, string, bool) (*service.CategoryDefaultGroupConfig, error)
}

type CategoryDefaultGroupHandler struct {
	service categoryDefaultGroupAdminService
}

func NewCategoryDefaultGroupHandler(svc *service.CategoryDefaultGroupService) *CategoryDefaultGroupHandler {
	return &CategoryDefaultGroupHandler{service: svc}
}

func (h *CategoryDefaultGroupHandler) List(c *gin.Context) {
	configs, err := h.service.List(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, configs)
}

func (h *CategoryDefaultGroupHandler) Candidates(c *gin.Context) {
	candidates, err := h.service.Candidates(c.Request.Context(), c.Param("category"))
	if response.ErrorFrom(c, err) {
		return
	}
	type candidateResponse struct {
		Group         *dto.AdminGroup `json:"group"`
		Models        []string        `json:"models"`
		ModelsCount   int             `json:"models_count"`
		InvalidReason string          `json:"invalid_reason,omitempty"`
	}
	out := make([]candidateResponse, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, candidateResponse{Group: dto.GroupFromServiceAdmin(candidate.Group), Models: candidate.Models, ModelsCount: candidate.ModelsCount, InvalidReason: candidate.InvalidReason})
	}
	response.Success(c, out)
}

func (h *CategoryDefaultGroupHandler) SyncPreview(c *gin.Context) {
	var req struct {
		SourceGroupIDs []int64 `json:"source_group_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid synchronization preview request")
		return
	}
	preview, err := h.service.Preview(c.Request.Context(), c.Param("category"), req.SourceGroupIDs)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, preview)
}

func (h *CategoryDefaultGroupHandler) SyncApply(c *gin.Context) {
	var req service.CategoryDefaultGroupApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid synchronization publish request")
		return
	}
	config, err := h.service.Apply(c.Request.Context(), c.Param("category"), req)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, config)
}

func (h *CategoryDefaultGroupHandler) SetStatus(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.Error(c, http.StatusBadRequest, "enabled is required")
		return
	}
	config, err := h.service.SetStatus(c.Request.Context(), c.Param("category"), *req.Enabled)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, config)
}
