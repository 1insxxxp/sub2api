package admin

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type subAdminCommissionService interface {
	GetSettings(ctx context.Context) (float64, error)
	SetSettings(ctx context.Context, rate float64) (float64, error)
	ListAllGrants(ctx context.Context) ([]service.SubAdminCommissionGrant, error)
	ReplaceGrants(ctx context.Context, input service.ReplaceSubAdminCommissionGrantsInput) ([]service.SubAdminCommissionGrant, error)
	ListWorkbenchGrants(ctx context.Context, subAdminID int64) ([]service.SubAdminCommissionGrant, error)
	ListCalendar(ctx context.Context, subAdminID int64, month string, now time.Time) ([]service.SubAdminCommissionCalendarDay, error)
	ListDayGroups(ctx context.Context, subAdminID int64, date string) ([]service.SubAdminCommissionDayGroup, error)
	ListDayGroupLogs(ctx context.Context, subAdminID, groupID int64, date string, params pagination.PaginationParams) ([]service.SubAdminCommissionUsageLog, pagination.PaginationResult, error)
}

// SubAdminCommissionHandler exposes commission management and secondary-admin workbench reporting.
type SubAdminCommissionHandler struct {
	service subAdminCommissionService
}

func NewSubAdminCommissionHandler(svc *service.SubAdminCommissionService) *SubAdminCommissionHandler {
	return &SubAdminCommissionHandler{service: svc}
}

type subAdminCommissionSettingsRequest struct {
	CommissionRate float64 `json:"commission_rate"`
}

type subAdminCommissionSettingsResponse struct {
	CommissionRate float64 `json:"commission_rate"`
}

type replaceSubAdminCommissionGrantsRequest struct {
	GroupIDs []int64 `json:"group_ids" binding:"omitempty,dive,gt=0"`
}

type subAdminCommissionGrantResponse struct {
	ID            int64  `json:"id"`
	SubAdminID    int64  `json:"sub_admin_id"`
	SubAdminEmail string `json:"sub_admin_email,omitempty"`
	GroupID       int64  `json:"group_id"`
	GroupName     string `json:"group_name"`
	GrantedDate   string `json:"granted_date"`
	Enabled       bool   `json:"enabled"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type subAdminCommissionCalendarDayResponse struct {
	Date             string  `json:"date"`
	Enabled          bool    `json:"enabled"`
	ActualCost       float64 `json:"actual_cost"`
	CommissionAmount float64 `json:"commission_amount"`
}

type subAdminCommissionDayGroupResponse struct {
	GroupID          int64   `json:"group_id"`
	GroupName        string  `json:"group_name"`
	Requests         int64   `json:"requests"`
	TotalTokens      int64   `json:"total_tokens"`
	ActualCost       float64 `json:"actual_cost"`
	CommissionAmount float64 `json:"commission_amount"`
}

type subAdminCommissionUsageLogResponse struct {
	ID                  int64   `json:"id"`
	RequestID           string  `json:"request_id"`
	CreatedAt           string  `json:"created_at"`
	UserID              int64   `json:"user_id"`
	UserEmail           string  `json:"user_email"`
	APIKeyID            int64   `json:"api_key_id"`
	APIKeyName          string  `json:"api_key_name"`
	GroupID             int64   `json:"group_id"`
	GroupName           string  `json:"group_name"`
	Model               string  `json:"model"`
	RequestedModel      *string `json:"requested_model,omitempty"`
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
	ActualCost          float64 `json:"actual_cost"`
	TotalTokens         int     `json:"total_tokens"`
}

func (h *SubAdminCommissionHandler) GetSettings(c *gin.Context) {
	if !h.ensureService(c) {
		return
	}
	rate, err := h.service.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, subAdminCommissionSettingsResponse{CommissionRate: rate})
}

func (h *SubAdminCommissionHandler) UpdateSettings(c *gin.Context) {
	if !h.ensureService(c) {
		return
	}
	var req subAdminCommissionSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.CommissionRate < 0 || req.CommissionRate > 1 {
		response.BadRequest(c, "commission_rate must be between 0 and 1")
		return
	}
	rate, err := h.service.SetSettings(c.Request.Context(), req.CommissionRate)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, subAdminCommissionSettingsResponse{CommissionRate: rate})
}

func (h *SubAdminCommissionHandler) ListGrants(c *gin.Context) {
	if !h.ensureService(c) {
		return
	}
	grants, err := h.service.ListAllGrants(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, subAdminCommissionGrantsResponse(grants))
}

func (h *SubAdminCommissionHandler) ReplaceGrants(c *gin.Context) {
	if !h.ensureService(c) {
		return
	}
	subAdminID, ok := parsePositiveInt64Param(c, "sub_admin_id")
	if !ok {
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req replaceSubAdminCommissionGrantsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	grants, err := h.service.ReplaceGrants(c.Request.Context(), service.ReplaceSubAdminCommissionGrantsInput{
		SubAdminID: subAdminID,
		GroupIDs:   req.GroupIDs,
		OperatorID: subject.UserID,
		Now:        time.Now(),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, subAdminCommissionGrantsResponse(grants))
}

func (h *SubAdminCommissionHandler) GetWorkbenchGrants(c *gin.Context) {
	if !h.ensureService(c) {
		return
	}
	subject, ok := requireSubAdminCommissionSubject(c)
	if !ok {
		return
	}
	grants, err := h.service.ListWorkbenchGrants(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, subAdminCommissionGrantsResponse(grants))
}

func (h *SubAdminCommissionHandler) GetWorkbenchCalendar(c *gin.Context) {
	if !h.ensureService(c) {
		return
	}
	subject, ok := requireSubAdminCommissionSubject(c)
	if !ok {
		return
	}
	month := strings.TrimSpace(c.Query("month"))
	if month != "" {
		if _, err := time.Parse("2006-01", month); err != nil {
			response.BadRequest(c, "Invalid month")
			return
		}
	}
	days, err := h.service.ListCalendar(c.Request.Context(), subject.UserID, month, time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, subAdminCommissionCalendarDaysResponse(days))
}

func (h *SubAdminCommissionHandler) GetWorkbenchDayGroups(c *gin.Context) {
	if !h.ensureService(c) {
		return
	}
	subject, ok := requireSubAdminCommissionSubject(c)
	if !ok {
		return
	}
	date := strings.TrimSpace(c.Param("date"))
	if _, err := service.ParseGroupUsageDate(date); err != nil {
		response.BadRequest(c, "Invalid date")
		return
	}
	groups, err := h.service.ListDayGroups(c.Request.Context(), subject.UserID, date)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, subAdminCommissionDayGroupsResponse(groups))
}

func (h *SubAdminCommissionHandler) GetWorkbenchDayGroupLogs(c *gin.Context) {
	if !h.ensureService(c) {
		return
	}
	subject, ok := requireSubAdminCommissionSubject(c)
	if !ok {
		return
	}
	groupID, ok := parsePositiveInt64Param(c, "group_id")
	if !ok {
		return
	}
	date := strings.TrimSpace(c.Param("date"))
	if _, err := service.ParseGroupUsageDate(date); err != nil {
		response.BadRequest(c, "Invalid date")
		return
	}
	page, pageSize := response.ParsePagination(c)
	logs, result, err := h.service.ListDayGroupLogs(c.Request.Context(), subject.UserID, groupID, date, pagination.PaginationParams{Page: page, PageSize: pageSize})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, subAdminCommissionUsageLogsResponse(logs), result.Total, result.Page, result.PageSize)
}

func (h *SubAdminCommissionHandler) ensureService(c *gin.Context) bool {
	if h == nil || h.service == nil {
		response.Error(c, http.StatusServiceUnavailable, "sub-admin commission service not configured")
		return false
	}
	return true
}

func requireSubAdminCommissionSubject(c *gin.Context) (middleware.AuthSubject, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return middleware.AuthSubject{}, false
	}
	return subject, true
}

func parsePositiveInt64Param(c *gin.Context, name string) (int64, bool) {
	value, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || value <= 0 {
		response.BadRequest(c, "Invalid "+strings.ReplaceAll(name, "_", " "))
		return 0, false
	}
	return value, true
}

func subAdminCommissionGrantsResponse(grants []service.SubAdminCommissionGrant) []subAdminCommissionGrantResponse {
	out := make([]subAdminCommissionGrantResponse, 0, len(grants))
	for i := range grants {
		out = append(out, subAdminCommissionGrantResponse{
			ID:            grants[i].ID,
			SubAdminID:    grants[i].SubAdminID,
			SubAdminEmail: grants[i].SubAdminEmail,
			GroupID:       grants[i].GroupID,
			GroupName:     grants[i].GroupName,
			GrantedDate:   grants[i].GrantedDate,
			Enabled:       grants[i].Enabled,
			CreatedAt:     formatCommissionTime(grants[i].CreatedAt),
			UpdatedAt:     formatCommissionTime(grants[i].UpdatedAt),
		})
	}
	return out
}

func subAdminCommissionCalendarDaysResponse(days []service.SubAdminCommissionCalendarDay) []subAdminCommissionCalendarDayResponse {
	out := make([]subAdminCommissionCalendarDayResponse, 0, len(days))
	for i := range days {
		out = append(out, subAdminCommissionCalendarDayResponse(days[i]))
	}
	return out
}

func subAdminCommissionDayGroupsResponse(groups []service.SubAdminCommissionDayGroup) []subAdminCommissionDayGroupResponse {
	out := make([]subAdminCommissionDayGroupResponse, 0, len(groups))
	for i := range groups {
		out = append(out, subAdminCommissionDayGroupResponse(groups[i]))
	}
	return out
}

func subAdminCommissionUsageLogsResponse(logs []service.SubAdminCommissionUsageLog) []subAdminCommissionUsageLogResponse {
	out := make([]subAdminCommissionUsageLogResponse, 0, len(logs))
	for i := range logs {
		totalTokens := logs[i].InputTokens + logs[i].OutputTokens + logs[i].CacheCreationTokens + logs[i].CacheReadTokens
		out = append(out, subAdminCommissionUsageLogResponse{
			ID:                  logs[i].ID,
			RequestID:           logs[i].RequestID,
			CreatedAt:           formatCommissionTime(logs[i].CreatedAt),
			UserID:              logs[i].UserID,
			UserEmail:           logs[i].UserEmail,
			APIKeyID:            logs[i].APIKeyID,
			APIKeyName:          logs[i].APIKeyName,
			GroupID:             logs[i].GroupID,
			GroupName:           logs[i].GroupName,
			Model:               logs[i].Model,
			RequestedModel:      logs[i].RequestedModel,
			InputTokens:         logs[i].InputTokens,
			OutputTokens:        logs[i].OutputTokens,
			CacheCreationTokens: logs[i].CacheCreationTokens,
			CacheReadTokens:     logs[i].CacheReadTokens,
			ActualCost:          logs[i].ActualCost,
			TotalTokens:         totalTokens,
		})
	}
	return out
}

func formatCommissionTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
