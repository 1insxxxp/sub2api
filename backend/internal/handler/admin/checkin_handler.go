package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// CheckinHandler handles admin check-in management APIs.
type CheckinHandler struct {
	checkinService *service.CheckinService
}

func NewCheckinHandler(checkinService *service.CheckinService) *CheckinHandler {
	return &CheckinHandler{checkinService: checkinService}
}

func (h *CheckinHandler) GetStats(c *gin.Context) {
	stats, err := h.checkinService.GetStats(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

func (h *CheckinHandler) ListRecords(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	var userID int64
	if raw := strings.TrimSpace(c.Query("user_id")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		userID = parsed
	}
	records, total, err := h.checkinService.ListRecords(c.Request.Context(), page, pageSize, service.CheckinListFilters{
		UserID: userID,
		Date:   c.Query("date"),
		Search: c.Query("search"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, records, total, page, pageSize)
}

func (h *CheckinHandler) ListBlacklist(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	activeOnly := c.DefaultQuery("active_only", "true") != "false"
	items, total, err := h.checkinService.ListBlacklist(c.Request.Context(), page, pageSize, activeOnly, c.Query("search"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

type AddCheckinBlacklistRequest struct {
	UserID int64  `json:"user_id" binding:"required,gt=0"`
	Reason string `json:"reason"`
}

func (h *CheckinHandler) AddBlacklist(c *gin.Context) {
	var req AddCheckinBlacklistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	var createdBy int64
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		createdBy = subject.UserID
	}
	item, err := h.checkinService.AddBlacklist(c.Request.Context(), service.AddCheckinBlacklistInput{
		UserID:    req.UserID,
		Reason:    req.Reason,
		CreatedBy: createdBy,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *CheckinHandler) RemoveBlacklist(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	var removedBy int64
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		removedBy = subject.UserID
	}
	if err := h.checkinService.RemoveBlacklist(c.Request.Context(), userID, removedBy); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Blacklist entry removed"})
}
