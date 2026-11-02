package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *ModelProbeHandler) configActor(c *gin.Context) (int64, bool) {
	id, ok := h.actor(c)
	if !ok {
		return 0, false
	}
	role, _ := middleware.GetUserRoleFromContext(c)
	if role != service.RoleAdmin {
		probeError(c, service.ErrUserSupportForbidden)
		return 0, false
	}
	return id, true
}

func probeConfigGroupID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		probeError(c, service.ErrModelProbeInvalid)
		return 0, false
	}
	return id, true
}

func (h *ModelProbeHandler) ConfigGroups(c *gin.Context) {
	actor, ok := h.configActor(c)
	if !ok || !probeQuery(c) {
		return
	}
	items, err := h.service.ConfigGroups(c.Request.Context(), actor)
	if err != nil {
		probeError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *ModelProbeHandler) Config(c *gin.Context) {
	actor, ok := h.configActor(c)
	if !ok || !probeQuery(c) {
		return
	}
	id, ok := probeConfigGroupID(c)
	if !ok {
		return
	}
	value, err := h.service.Config(c.Request.Context(), actor, id)
	if err != nil {
		probeError(c, err)
		return
	}
	response.Success(c, value)
}

// Decode the single allowed field explicitly so duplicate fields, null, extra
// objects and oversized trailing whitespace cannot bypass the request bound.
func decodeProbeConfig(c *gin.Context) ([]string, bool) {
	invalid := func() ([]string, bool) {
		probeError(c, service.ErrModelProbeInvalid)
		return nil, false
	}
	if c.Request.URL.RawQuery != "" {
		return invalid()
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	d := json.NewDecoder(c.Request.Body)
	token, err := d.Token()
	if err != nil || token != json.Delim('{') || !d.More() {
		return invalid()
	}
	token, err = d.Token()
	if err != nil || token != "models" {
		return invalid()
	}
	var models []string
	if d.Decode(&models) != nil || models == nil || len(models) > 200 || d.More() {
		return invalid()
	}
	token, err = d.Token()
	if err != nil || token != json.Delim('}') {
		return invalid()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return invalid()
	}
	return models, true
}

func (h *ModelProbeHandler) SaveConfig(c *gin.Context) {
	actor, ok := h.configActor(c)
	if !ok {
		return
	}
	id, ok := probeConfigGroupID(c)
	if !ok {
		return
	}
	models, ok := decodeProbeConfig(c)
	if !ok {
		return
	}
	middleware.SetAuditAction(c, "admin.model_probe.configure")
	value, err := h.service.SaveConfig(c.Request.Context(), actor, id, models)
	if err != nil {
		probeError(c, err)
		return
	}
	response.Success(c, value)
}
