package admin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type userSupportService interface {
	CanAccess(context.Context, int64) (bool, error)
	GetAccess(context.Context, int64, int64) (bool, error)
	SetAccess(context.Context, int64, int64, bool) error
	Lookup(context.Context, int64, string) (*service.UserSupportResult, error)
}

// UserSupportHandler exposes a narrow, privacy-safe support capability.
// No account, Ops, or general-purpose usage DTO is serialized here.
type UserSupportHandler struct{ service userSupportService }

func NewUserSupportHandler(svc *service.UserSupportService) *UserSupportHandler {
	return &UserSupportHandler{service: svc}
}

func supportError(c *gin.Context, err error) {
	status, code, message := http.StatusServiceUnavailable, "SUPPORT_UNAVAILABLE", "User support is temporarily unavailable"
	switch {
	case errors.Is(err, service.ErrUserSupportForbidden):
		status, code, message = http.StatusForbidden, "FORBIDDEN", "User support access denied"
	case errors.Is(err, service.ErrUserSupportUserNotFound):
		status, code, message = http.StatusNotFound, "USER_NOT_FOUND", "User not found"
	case errors.Is(err, service.ErrUserSupportInvalidAccount):
		status, code, message = http.StatusBadRequest, "INVALID_ACCOUNT", "Invalid user support request"
	}
	// Never pass an application error, arbitrary metadata, or err.Error() to the client.
	response.ErrorWithDetails(c, status, message, code, nil)
}

func (h *UserSupportHandler) actor(c *gin.Context, mainAdminOnly bool) (int64, bool) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("Pragma", "no-cache")
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.ErrorWithDetails(c, http.StatusUnauthorized, "Authentication required", "UNAUTHORIZED", nil)
		return 0, false
	}
	role, _ := middleware.GetUserRoleFromContext(c)
	if role != service.RoleAdmin && (mainAdminOnly || role != service.RoleSubAdmin) {
		supportError(c, service.ErrUserSupportForbidden)
		return 0, false
	}
	if h == nil || h.service == nil {
		supportError(c, service.ErrUserSupportUnavailable)
		return 0, false
	}
	return subject.UserID, true
}

func supportDecode(c *gin.Context, out any) bool {
	if c.Request.URL.RawQuery != "" {
		supportError(c, service.ErrUserSupportInvalidAccount)
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2048)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		supportError(c, service.ErrUserSupportInvalidAccount)
		return false
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		supportError(c, service.ErrUserSupportInvalidAccount)
		return false
	}
	return true
}

func (h *UserSupportHandler) GetMyAccess(c *gin.Context) {
	actor, ok := h.actor(c, false)
	if !ok {
		return
	}
	enabled, err := h.service.CanAccess(c.Request.Context(), actor)
	if err != nil {
		supportError(c, err)
		return
	}
	response.Success(c, gin.H{"enabled": enabled})
}

func (h *UserSupportHandler) Lookup(c *gin.Context) {
	actor, ok := h.actor(c, false)
	if !ok {
		return
	}
	allowed, err := h.service.CanAccess(c.Request.Context(), actor)
	if err != nil {
		supportError(c, err)
		return
	}
	if !allowed {
		supportError(c, service.ErrUserSupportForbidden)
		return
	}
	var req struct {
		Account string `json:"account"`
	}
	if !supportDecode(c, &req) {
		return
	}
	req.Account = strings.TrimSpace(req.Account)
	if req.Account == "" || len(req.Account) > 254 {
		supportError(c, service.ErrUserSupportInvalidAccount)
		return
	}
	middleware.SetAuditAction(c, "admin.user_support.lookup")
	result, err := h.service.Lookup(c.Request.Context(), actor, req.Account)
	if err != nil {
		supportError(c, err)
		return
	}
	if result == nil {
		supportError(c, service.ErrUserSupportUnavailable)
		return
	}
	response.Success(c, result)
}

func supportTarget(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		supportError(c, service.ErrUserSupportInvalidAccount)
		return 0, false
	}
	return id, true
}

func (h *UserSupportHandler) GetUserAccess(c *gin.Context) {
	actor, ok := h.actor(c, true)
	if !ok {
		return
	}
	target, ok := supportTarget(c)
	if !ok {
		return
	}
	middleware.SetAuditAction(c, "admin.user_support.access.read")
	enabled, err := h.service.GetAccess(c.Request.Context(), actor, target)
	if err != nil {
		supportError(c, err)
		return
	}
	response.Success(c, gin.H{"enabled": enabled})
}

func (h *UserSupportHandler) SetUserAccess(c *gin.Context) {
	actor, ok := h.actor(c, true)
	if !ok {
		return
	}
	target, ok := supportTarget(c)
	if !ok {
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if !supportDecode(c, &req) {
		return
	}
	if req.Enabled == nil {
		supportError(c, service.ErrUserSupportInvalidAccount)
		return
	}
	middleware.SetAuditAction(c, "admin.user_support.access.update")
	if err := h.service.SetAccess(c.Request.Context(), actor, target, *req.Enabled); err != nil {
		supportError(c, err)
		return
	}
	middleware.SetAuditExtra(c, map[string]any{"enabled": *req.Enabled})
	response.Success(c, gin.H{"enabled": *req.Enabled})
}
