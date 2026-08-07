package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type emptyResponseClaimReviewRequest struct {
	Note string `json:"note"`
}

type emptyResponseClaimBatchRequest struct {
	IDs    []int64 `json:"ids" binding:"required,min=1,max=100"`
	Action string  `json:"action" binding:"required"`
	Note   string  `json:"note"`
}

func (h *UsageHandler) ListEmptyResponseClaims(c *gin.Context) {
	if h.emptyResponseClaimService == nil {
		response.InternalError(c, "Empty response claim service not available")
		return
	}
	page, pageSize := response.ParsePagination(c)
	filters := service.EmptyResponseClaimListFilters{Status: strings.TrimSpace(c.Query("status")), Model: strings.TrimSpace(c.Query("model"))}
	for name, target := range map[string]*int64{"user_id": &filters.UserID, "group_id": &filters.GroupID, "account_id": &filters.AccountID} {
		if raw := strings.TrimSpace(c.Query(name)); raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || value <= 0 {
				response.BadRequest(c, "Invalid "+name)
				return
			}
			*target = value
		}
		if raw := strings.TrimSpace(c.Query("start_date")); raw != "" {
			parsed, err := time.Parse("2006-01-02", raw)
			if err != nil {
				response.BadRequest(c, "Invalid start_date")
				return
			}
			filters.StartTime = &parsed
		}
		if raw := strings.TrimSpace(c.Query("end_date")); raw != "" {
			parsed, err := time.Parse("2006-01-02", raw)
			if err != nil {
				response.BadRequest(c, "Invalid end_date")
				return
			}
			parsed = parsed.AddDate(0, 0, 1)
			filters.EndTime = &parsed
		}
	}
	claims, result, err := h.emptyResponseClaimService.List(c.Request.Context(), pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: "created_at", SortOrder: "desc"}, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]dto.AdminEmptyResponseClaim, 0, len(claims))
	for i := range claims {
		items = append(items, *dto.EmptyResponseClaimFromServiceAdmin(&claims[i]))
	}
	response.Paginated(c, items, result.Total, page, pageSize)
}

func (h *UsageHandler) GetEmptyResponseClaimMetrics(c *gin.Context) {
	if h.emptyResponseClaimService == nil {
		response.InternalError(c, "Empty response claim service not available")
		return
	}
	now := time.Now()
	start, end := now.AddDate(0, 0, -7), now
	if raw := strings.TrimSpace(c.Query("start_date")); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			response.BadRequest(c, "Invalid start_date")
			return
		}
		start = parsed
	}
	if raw := strings.TrimSpace(c.Query("end_date")); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			response.BadRequest(c, "Invalid end_date")
			return
		}
		end = parsed.AddDate(0, 0, 1)
	}
	metrics, err := h.emptyResponseClaimService.Metrics(c.Request.Context(), start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, metrics)
}

func (h *UsageHandler) ApproveEmptyResponseClaim(c *gin.Context) {
	h.reviewEmptyResponseClaim(c, service.EmptyResponseClaimApproved)
}

func (h *UsageHandler) RejectEmptyResponseClaim(c *gin.Context) {
	h.reviewEmptyResponseClaim(c, service.EmptyResponseClaimRejected)
}

func (h *UsageHandler) reviewEmptyResponseClaim(c *gin.Context, status string) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid claim id")
		return
	}
	var request emptyResponseClaimReviewRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid review request")
		return
	}
	if status == service.EmptyResponseClaimRejected && strings.TrimSpace(request.Note) == "" {
		response.BadRequest(c, "Rejection note is required")
		return
	}
	var claim *service.EmptyResponseClaim
	if status == service.EmptyResponseClaimApproved {
		claim, err = h.emptyResponseClaimService.Approve(c.Request.Context(), id, subject.UserID, request.Note)
	} else {
		claim, err = h.emptyResponseClaimService.Reject(c.Request.Context(), id, subject.UserID, request.Note)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if h.opsService != nil {
		refund := claim.BalanceRefund + claim.SubscriptionRefund
		h.opsService.RecordEmptyResponseClaimAudit(subject.UserID, status, []int64{id}, 1, 0, refund)
	}
	response.Success(c, dto.EmptyResponseClaimFromServiceAdmin(claim))
}

func (h *UsageHandler) BatchEmptyResponseClaims(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	var request emptyResponseClaimBatchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid batch request")
		return
	}
	switch request.Action {
	case "approve":
		request.Action = service.EmptyResponseClaimApproved
	case "reject":
		request.Action = service.EmptyResponseClaimRejected
	}
	result, err := h.emptyResponseClaimService.Batch(c.Request.Context(), service.EmptyResponseClaimBatchInput{
		IDs: request.IDs, Action: request.Action, ReviewerID: subject.UserID, Note: request.Note,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if h.opsService != nil {
		refund := 0.0
		for i := range result.Claims {
			refund += result.Claims[i].BalanceRefund + result.Claims[i].SubscriptionRefund
		}
		h.opsService.RecordEmptyResponseClaimAudit(subject.UserID, request.Action, result.Succeeded, len(result.Succeeded), len(result.Failed), refund)
	}
	response.Success(c, result)
}
