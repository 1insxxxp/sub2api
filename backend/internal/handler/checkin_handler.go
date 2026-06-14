package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type checkinService interface {
	GetStatus(ctx context.Context, userID int64) (*service.CheckinStatus, error)
	Checkin(ctx context.Context, userID int64) (*service.CheckinResult, error)
}

// CheckinHandler handles user daily check-in requests.
type CheckinHandler struct {
	checkinService checkinService
}

func NewCheckinHandler(checkinService *service.CheckinService) *CheckinHandler {
	return newCheckinHandler(checkinService)
}

func newCheckinHandler(checkinService checkinService) *CheckinHandler {
	return &CheckinHandler{checkinService: checkinService}
}

func (h *CheckinHandler) GetStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
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
	result, err := h.checkinService.Checkin(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
