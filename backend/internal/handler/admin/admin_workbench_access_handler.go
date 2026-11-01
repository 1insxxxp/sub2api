package admin

import (
	"context"
	"errors"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type adminWorkbenchAccessService interface {
	GetAccess(context.Context, int64) (*service.AdminWorkbenchAccess, error)
	GetUserAccess(context.Context, int64, int64) (*service.AdminWorkbenchAccess, error)
	SetUserAccess(context.Context, int64, int64, bool, bool, bool) error
}

type AdminWorkbenchAccessHandler struct{ service adminWorkbenchAccessService }

func NewAdminWorkbenchAccessHandler(svc *service.AdminWorkbenchAccessService) *AdminWorkbenchAccessHandler {
	return &AdminWorkbenchAccessHandler{service: svc}
}

func workbenchAccessError(c *gin.Context, err error) {
	status, code, message := http.StatusServiceUnavailable, "WORKBENCH_UNAVAILABLE", "Workbench access is temporarily unavailable"
	if errors.Is(err, service.ErrUserSupportForbidden) {
		status, code, message = http.StatusForbidden, "FORBIDDEN", "Workbench access denied"
	} else if errors.Is(err, service.ErrUserSupportUserNotFound) {
		status, code, message = http.StatusNotFound, "USER_NOT_FOUND", "User not found"
	}
	response.ErrorWithDetails(c, status, message, code, nil)
}

func (h *AdminWorkbenchAccessHandler) actor(c *gin.Context, mainAdminOnly bool) (int64, bool) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("Pragma", "no-cache")
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.ErrorWithDetails(c, http.StatusUnauthorized, "Authentication required", "UNAUTHORIZED", nil)
		return 0, false
	}
	role, _ := middleware.GetUserRoleFromContext(c)
	if role != service.RoleAdmin && (mainAdminOnly || role != service.RoleSubAdmin) {
		workbenchAccessError(c, service.ErrUserSupportForbidden)
		return 0, false
	}
	if h == nil || h.service == nil {
		workbenchAccessError(c, service.ErrUserSupportUnavailable)
		return 0, false
	}
	return subject.UserID, true
}

func (h *AdminWorkbenchAccessHandler) GetMyAccess(c *gin.Context) {
	actor, ok := h.actor(c, false)
	if !ok {
		return
	}
	access, err := h.service.GetAccess(c.Request.Context(), actor)
	if err != nil || access == nil {
		workbenchAccessError(c, err)
		return
	}
	response.Success(c, access)
}

func (h *AdminWorkbenchAccessHandler) GetUserAccess(c *gin.Context) {
	actor, ok := h.actor(c, true)
	if !ok {
		return
	}
	target, ok := supportTarget(c)
	if !ok {
		return
	}
	access, err := h.service.GetUserAccess(c.Request.Context(), actor, target)
	if err != nil || access == nil {
		workbenchAccessError(c, err)
		return
	}
	response.Success(c, access)
}

func (h *AdminWorkbenchAccessHandler) SetUserAccess(c *gin.Context) {
	actor, ok := h.actor(c, true)
	if !ok {
		return
	}
	target, ok := supportTarget(c)
	if !ok {
		return
	}
	var req struct {
		Commission           *bool `json:"commission"`
		AffiliateLeaderboard *bool `json:"affiliate_leaderboard"`
		UserSupport          *bool `json:"user_support"`
	}
	if !supportDecode(c, &req) {
		return
	}
	if req.Commission == nil || req.AffiliateLeaderboard == nil || req.UserSupport == nil {
		response.ErrorWithDetails(c, http.StatusBadRequest, "Invalid workbench access request", "INVALID_REQUEST", nil)
		return
	}
	if err := h.service.SetUserAccess(c.Request.Context(), actor, target, *req.Commission, *req.AffiliateLeaderboard, *req.UserSupport); err != nil {
		workbenchAccessError(c, err)
		return
	}
	middleware.SetAuditAction(c, "admin.workbench.access.update")
	middleware.SetAuditExtra(c, map[string]any{"commission": *req.Commission, "affiliate_leaderboard": *req.AffiliateLeaderboard, "user_support": *req.UserSupport})
	response.Success(c, service.AdminWorkbenchAccess{Commission: *req.Commission, AffiliateLeaderboard: *req.AffiliateLeaderboard, UserSupport: *req.UserSupport, BalanceTransfer: true})
}

func (h *AdminWorkbenchAccessHandler) require(c *gin.Context, permission func(*service.AdminWorkbenchAccess) bool) {
	actor, ok := h.actor(c, false)
	if !ok {
		c.Abort()
		return
	}
	access, err := h.service.GetAccess(c.Request.Context(), actor)
	if err != nil || access == nil {
		workbenchAccessError(c, err)
		c.Abort()
		return
	}
	if !permission(access) {
		workbenchAccessError(c, service.ErrUserSupportForbidden)
		c.Abort()
		return
	}
	c.Next()
}

func (h *AdminWorkbenchAccessHandler) RequireCommission(c *gin.Context) {
	h.require(c, func(a *service.AdminWorkbenchAccess) bool { return a.Commission })
}

func (h *AdminWorkbenchAccessHandler) RequireAffiliateLeaderboard(c *gin.Context) {
	h.require(c, func(a *service.AdminWorkbenchAccess) bool { return a.AffiliateLeaderboard })
}
