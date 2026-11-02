package admin

import (
	"context"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type modelProbeService interface {
	Groups(context.Context, int64) ([]service.ModelProbeGroup, error)
	Models(context.Context, int64, int64) (*service.ModelProbeCatalog, error)
	Run(context.Context, int64, int64, string) (*service.ModelProbeResult, error)
	ConfigGroups(context.Context, int64) ([]service.ModelProbeGroup, error)
	Config(context.Context, int64, int64) (*service.ModelProbeConfig, error)
	SaveConfig(context.Context, int64, int64, []string) (*service.ModelProbeConfigResult, error)
}
type ModelProbeHandler struct{ service modelProbeService }

func NewModelProbeHandler(s *service.ModelProbeService) *ModelProbeHandler {
	return &ModelProbeHandler{service: s}
}
func probeError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrModelProbeInvalid) {
		response.ErrorWithDetails(c, http.StatusBadRequest, "Invalid model probe request", "INVALID_PROBE", nil)
		return
	}
	supportError(c, err)
}
func (h *ModelProbeHandler) actor(c *gin.Context) (int64, bool) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("Pragma", "no-cache")
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.ErrorWithDetails(c, 401, "Authentication required", "UNAUTHORIZED", nil)
		return 0, false
	}
	role, _ := middleware.GetUserRoleFromContext(c)
	if role != service.RoleAdmin && role != service.RoleSubAdmin {
		probeError(c, service.ErrUserSupportForbidden)
		return 0, false
	}
	if h == nil || h.service == nil {
		probeError(c, service.ErrUserSupportUnavailable)
		return 0, false
	}
	return subject.UserID, true
}
func probeQuery(c *gin.Context) bool {
	for k, v := range c.Request.URL.Query() {
		if k != "timezone" || len(v) != 1 || len(v[0]) > 100 {
			probeError(c, service.ErrModelProbeInvalid)
			return false
		}
	}
	return true
}
func (h *ModelProbeHandler) Groups(c *gin.Context) {
	id, ok := h.actor(c)
	if !ok || !probeQuery(c) {
		return
	}
	items, err := h.service.Groups(c.Request.Context(), id)
	if err != nil {
		probeError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *ModelProbeHandler) Models(c *gin.Context) {
	id, ok := h.actor(c)
	if !ok || !probeQuery(c) {
		return
	}
	group, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || group <= 0 {
		probeError(c, service.ErrModelProbeInvalid)
		return
	}
	v, err := h.service.Models(c.Request.Context(), id, group)
	if err != nil {
		probeError(c, err)
		return
	}
	response.Success(c, v)
}
func (h *ModelProbeHandler) Run(c *gin.Context) {
	id, ok := h.actor(c)
	if !ok {
		return
	}
	var req struct {
		GroupID int64  `json:"group_id"`
		Model   string `json:"model"`
	}
	if !supportDecode(c, &req) {
		return
	}
	middleware.SetAuditAction(c, "admin.model_probe.run")
	v, err := h.service.Run(c.Request.Context(), id, req.GroupID, req.Model)
	if err != nil {
		probeError(c, err)
		return
	}
	response.Success(c, v)
}
