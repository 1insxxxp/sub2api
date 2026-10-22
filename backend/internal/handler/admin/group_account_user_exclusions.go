package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

// NewGroupHandlerWithAccountExclusions preserves the existing constructor for
// callers while requiring the private authorization service in production DI.
func NewGroupHandlerWithAccountExclusions(adminService service.AdminService, dashboard *service.DashboardService, capacity *service.GroupCapacityService, pricing *service.GroupPricingCoverageService, cfg *config.Config, exclusions *service.GroupAccountUserExclusionService) *GroupHandler {
	h := NewGroupHandlerWithDependencies(adminService, dashboard, capacity, pricing, cfg)
	h.accountUserExclusions = exclusions
	return h
}

func (h *GroupHandler) requireAccountExclusionAdmin(c *gin.Context) bool {
	if _, ok := middleware.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return false
	}
	if role, ok := middleware.GetUserRoleFromContext(c); !ok || role != service.RoleAdmin {
		response.Forbidden(c, "Admin access required")
		return false
	}
	if h.accountUserExclusions == nil {
		response.Error(c, http.StatusServiceUnavailable, "Account access settings unavailable")
		return false
	}
	return true
}

func (h *GroupHandler) ListAccountUserExclusions(c *gin.Context) {
	if !h.requireAccountExclusionAdmin(c) {
		return
	}
	groupID, ok := parsePositiveInt64Param(c, "id")
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	entries, result, err := h.accountUserExclusions.List(c.Request.Context(), groupID, pagination.PaginationParams{Page: page, PageSize: pageSize})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, entries, result.Total, result.Page, result.PageSize)
}

func (h *GroupHandler) AddAccountUserExclusion(c *gin.Context) {
	if !h.requireAccountExclusionAdmin(c) {
		return
	}
	groupID, ok := parsePositiveInt64Param(c, "id")
	if !ok {
		return
	}
	var req struct {
		AccountID int64 `json:"account_id" binding:"required,gt=0"`
		UserID    int64 `json:"user_id" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Positive account_id and user_id are required")
		return
	}
	item, err := h.accountUserExclusions.Add(c.Request.Context(), groupID, req.AccountID, req.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *GroupHandler) RemoveAccountUserExclusion(c *gin.Context) {
	if !h.requireAccountExclusionAdmin(c) {
		return
	}
	groupID, ok := parsePositiveInt64Param(c, "id")
	if !ok {
		return
	}
	exclusionID, ok := parsePositiveInt64Param(c, "exclusion_id")
	if !ok {
		return
	}
	if err := h.accountUserExclusions.Remove(c.Request.Context(), groupID, exclusionID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "success"})
}
