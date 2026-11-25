package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *CheckinHandler) ListRiskAssociations(c *gin.Context) {
	page, size := response.ParsePagination(c)
	items, total, err := h.checkinService.ListRiskAssociations(c.Request.Context(), page, size, service.RiskAssociationFilters{
		Search: c.Query("search"), Reason: c.Query("reason"), Status: c.Query("status"), StartDate: c.Query("start_date"), EndDate: c.Query("end_date"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, size)
}
