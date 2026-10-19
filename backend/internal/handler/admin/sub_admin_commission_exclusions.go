package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type commissionExclusionResponse struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	UserEmail string `json:"user_email"`
	Username  string `json:"username"`
	Notes     string `json:"notes"`
	StartedAt string `json:"started_at"`
}

func commissionExclusionDTO(entry service.SubAdminCommissionExclusion) commissionExclusionResponse {
	return commissionExclusionResponse{
		ID: entry.ID, UserID: entry.UserID, UserEmail: entry.UserEmail,
		Username: entry.Username, Notes: entry.Notes, StartedAt: formatCommissionTime(entry.StartedAt),
	}
}

func requireCommissionAdmin(c *gin.Context) (middleware.AuthSubject, bool) {
	subject, ok := requireSubAdminCommissionSubject(c)
	if !ok {
		return middleware.AuthSubject{}, false
	}
	role, ok := middleware.GetUserRoleFromContext(c)
	if !ok || role != service.RoleAdmin {
		response.Forbidden(c, "Admin access required")
		return middleware.AuthSubject{}, false
	}
	return subject, true
}

func (h *SubAdminCommissionHandler) ListExclusions(c *gin.Context) {
	if _, ok := requireCommissionAdmin(c); !ok || !h.ensureService(c) {
		return
	}
	page, pageSize := response.ParsePagination(c)
	entries, result, err := h.service.ListExclusions(c.Request.Context(), pagination.PaginationParams{Page: page, PageSize: pageSize})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]commissionExclusionResponse, 0, len(entries))
	for _, entry := range entries {
		items = append(items, commissionExclusionDTO(entry))
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *SubAdminCommissionHandler) AddExclusion(c *gin.Context) {
	subject, ok := requireCommissionAdmin(c)
	if !ok || !h.ensureService(c) {
		return
	}
	var req struct {
		UserID int64 `json:"user_id" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "A positive user_id is required")
		return
	}
	entry, err := h.service.AddExclusion(c.Request.Context(), req.UserID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, commissionExclusionDTO(*entry))
}

func (h *SubAdminCommissionHandler) RemoveExclusion(c *gin.Context) {
	if _, ok := requireCommissionAdmin(c); !ok || !h.ensureService(c) {
		return
	}
	userID, ok := parsePositiveInt64Param(c, "user_id")
	if !ok {
		return
	}
	if err := h.service.RemoveExclusion(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "success"})
}
