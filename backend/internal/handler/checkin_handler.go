package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// CheckinHandler handles user daily check-in requests.
type CheckinHandler struct {
	checkinService *service.CheckinService
}

func NewCheckinHandler(checkinService *service.CheckinService) *CheckinHandler {
	return &CheckinHandler{checkinService: checkinService}
}

func (h *CheckinHandler) GetStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if _, _, err := checkinBrowserIdentity(c); err != nil {
		response.InternalError(c, "Failed to initialize browser identity")
		return
	}
	status, err := h.checkinService.GetStatus(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func (h *CheckinHandler) Checkin(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	browserHash, browserCookiePresent, err := checkinBrowserIdentity(c)
	if err != nil {
		response.InternalError(c, "Failed to initialize browser identity")
		return
	}
	result, err := h.checkinService.CheckinWithBrowser(c.Request.Context(), subject.UserID, service.CheckinBrowser{Hash: browserHash, CookiePresent: browserCookiePresent, IP: ip.GetSecurityClientIP(c, false), UserAgent: c.Request.UserAgent()})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
