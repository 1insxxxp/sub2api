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
func probeConfigRequest(c *gin.Context, read func(*json.Decoder) error) bool {
	if c.Request.URL.RawQuery != "" {
		probeError(c, service.ErrModelProbeInvalid)
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	d := json.NewDecoder(c.Request.Body)
	var extra any
	if read(d) != nil || d.Decode(&extra) != io.EOF {
		probeError(c, service.ErrModelProbeInvalid)
		return false
	}
	return true
}

func probeConfigObject(d *json.Decoder, fields map[string]func() error) error {
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return service.ErrModelProbeInvalid
	}
	seen := map[string]bool{}
	for d.More() {
		token, err := d.Token()
		key, ok := token.(string)
		read := fields[key]
		if err != nil || !ok || read == nil || seen[key] {
			return service.ErrModelProbeInvalid
		}
		seen[key] = true
		if err := read(); err != nil {
			return service.ErrModelProbeInvalid
		}
	}
	token, err = d.Token()
	if err != nil || token != json.Delim('}') || len(seen) != len(fields) {
		return service.ErrModelProbeInvalid
	}
	return nil
}

func probeConfigBool(d *json.Decoder, target *bool) error {
	var value *bool
	if err := d.Decode(&value); err != nil || value == nil {
		return service.ErrModelProbeInvalid
	}
	*target = *value
	return nil
}

func decodeProbeConfig(c *gin.Context) (bool, bool) {
	var enabled bool
	ok := probeConfigRequest(c, func(d *json.Decoder) error {
		return probeConfigObject(d, map[string]func() error{"enabled": func() error { return probeConfigBool(d, &enabled) }})
	})
	return enabled, ok
}

func decodeProbeConfigs(c *gin.Context) ([]service.ModelProbeConfigChange, bool) {
	changes := []service.ModelProbeConfigChange{}
	ok := probeConfigRequest(c, func(d *json.Decoder) error {
		return probeConfigObject(d, map[string]func() error{"groups": func() error {
			token, err := d.Token()
			if err != nil || token != json.Delim('[') {
				return service.ErrModelProbeInvalid
			}
			seen := map[int64]bool{}
			for d.More() {
				if len(changes) == 500 {
					return service.ErrModelProbeInvalid
				}
				var change service.ModelProbeConfigChange
				err := probeConfigObject(d, map[string]func() error{
					"group_id": func() error { return d.Decode(&change.GroupID) },
					"enabled":  func() error { return probeConfigBool(d, &change.Enabled) },
				})
				if err != nil || change.GroupID <= 0 || seen[change.GroupID] {
					return service.ErrModelProbeInvalid
				}
				seen[change.GroupID] = true
				changes = append(changes, change)
			}
			token, err = d.Token()
			if err != nil || token != json.Delim(']') {
				return service.ErrModelProbeInvalid
			}
			return nil
		}})
	})
	return changes, ok
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
	enabled, ok := decodeProbeConfig(c)
	if !ok {
		return
	}
	middleware.SetAuditAction(c, "admin.model_probe.configure")
	value, err := h.service.SaveConfig(c.Request.Context(), actor, id, enabled)
	if err != nil {
		probeError(c, err)
		return
	}
	response.Success(c, value)
}

func (h *ModelProbeHandler) SaveConfigs(c *gin.Context) {
	actor, ok := h.configActor(c)
	if !ok {
		return
	}
	changes, ok := decodeProbeConfigs(c)
	if !ok {
		return
	}
	middleware.SetAuditAction(c, "admin.model_probe.configure")
	items, err := h.service.SaveConfigs(c.Request.Context(), actor, changes)
	if err != nil {
		probeError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}
