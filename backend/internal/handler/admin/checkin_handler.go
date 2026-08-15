package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

const checkinRewardCampaignBodyMaxBytes = 1 << 20

// CheckinHandler handles admin check-in management APIs.
type CheckinHandler struct {
	checkinService  *service.CheckinService
	campaignService checkinCampaignAdminService
}

func NewCheckinHandler(checkinService *service.CheckinService) *CheckinHandler {
	return &CheckinHandler{checkinService: checkinService, campaignService: checkinService}
}

type checkinCampaignAdminService interface {
	ListRewardCampaigns(context.Context, string) ([]service.CheckinRewardCampaign, error)
	GetRewardCampaign(context.Context, int64) (*service.CheckinRewardCampaign, error)
	CreateRewardCampaign(context.Context, service.CreateCheckinRewardCampaignInput) (*service.CheckinRewardCampaign, error)
	UpdateRewardCampaign(context.Context, int64, service.UpdateCheckinRewardCampaignInput) (*service.CheckinRewardCampaign, error)
	EnableRewardCampaign(context.Context, int64, int64) (*service.CheckinRewardCampaign, error)
	DisableRewardCampaign(context.Context, int64, int64) (*service.CheckinRewardCampaign, error)
	CopyRewardCampaign(context.Context, int64, string, int64) (*service.CheckinRewardCampaign, error)
	DeleteRewardCampaign(context.Context, int64) error
}

var (
	errCheckinRewardCampaignInvalidID = infraerrors.BadRequest(
		"CHECKIN_REWARD_CAMPAIGN_INVALID_ID",
		"campaign id must be a positive integer",
	)
	errCheckinRewardCampaignInvalidRequest = infraerrors.BadRequest(
		"CHECKIN_REWARD_CAMPAIGN_INVALID_REQUEST",
		"invalid check-in reward campaign request",
	)
)

func (h *CheckinHandler) GetStats(c *gin.Context) {
	stats, err := h.checkinService.GetStats(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

func (h *CheckinHandler) GetConfig(c *gin.Context) {
	cfg, err := h.checkinService.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

type UpdateCheckinConfigRequest struct {
	Enabled                bool                        `json:"enabled"`
	MinTotalUsageUSD       float64                     `json:"min_total_usage_usd" binding:"gte=0"`
	MinTotalRechargeUSD    float64                     `json:"min_total_recharge_usd" binding:"gte=0"`
	Tiers                  []service.CheckinRewardTier `json:"tiers"`
	StreakEnabled          bool                        `json:"streak_enabled"`
	StreakRules            []service.CheckinStreakRule `json:"streak_rules"`
	UsageRebateEnabled     bool                        `json:"usage_rebate_enabled"`
	UsageRebateRatePercent float64                     `json:"usage_rebate_rate_percent"`
	UsageRebateCap         float64                     `json:"usage_rebate_cap"`
	TotalRewardCap         float64                     `json:"total_reward_cap"`
}

func (h *CheckinHandler) UpdateConfig(c *gin.Context) {
	var req UpdateCheckinConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.checkinService.UpdateConfig(c.Request.Context(), service.CheckinConfig{
		Enabled:                req.Enabled,
		MinTotalUsageUSD:       req.MinTotalUsageUSD,
		MinTotalRechargeUSD:    req.MinTotalRechargeUSD,
		Tiers:                  req.Tiers,
		StreakEnabled:          req.StreakEnabled,
		StreakRules:            req.StreakRules,
		UsageRebateEnabled:     req.UsageRebateEnabled,
		UsageRebateRatePercent: req.UsageRebateRatePercent,
		UsageRebateCap:         req.UsageRebateCap,
		TotalRewardCap:         req.TotalRewardCap,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
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

type UpsertCheckinRewardCampaignRequest struct {
	Name        string                      `json:"name" binding:"required"`
	StartDate   string                      `json:"start_date" binding:"required"`
	EndDate     string                      `json:"end_date" binding:"required"`
	RewardTiers []service.CheckinRewardTier `json:"reward_tiers" binding:"required"`
}

type CopyCheckinRewardCampaignRequest struct {
	Name string `json:"name" binding:"required"`
}

type DeleteCheckinRewardCampaignResponse struct {
	ID      int64 `json:"id"`
	Deleted bool  `json:"deleted"`
}

func (h *CheckinHandler) ListCampaigns(c *gin.Context) {
	campaigns, err := h.campaignService.ListRewardCampaigns(c.Request.Context(), c.Query("lifecycle"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, campaigns)
}

func (h *CheckinHandler) CreateCampaign(c *gin.Context) {
	var req UpsertCheckinRewardCampaignRequest
	if !decodeCheckinRewardCampaignBody(c, &req) {
		return
	}
	campaign, err := h.campaignService.CreateRewardCampaign(c.Request.Context(), service.CreateCheckinRewardCampaignInput{
		Name: req.Name, StartDate: req.StartDate, EndDate: req.EndDate,
		RewardTiers: req.RewardTiers, AdminID: checkinCampaignAdminID(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, campaign)
}

func (h *CheckinHandler) GetCampaign(c *gin.Context) {
	id, ok := parseCheckinCampaignID(c)
	if !ok {
		return
	}
	campaign, err := h.campaignService.GetRewardCampaign(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, campaign)
}

func (h *CheckinHandler) UpdateCampaign(c *gin.Context) {
	id, ok := parseCheckinCampaignID(c)
	if !ok {
		return
	}
	var req UpsertCheckinRewardCampaignRequest
	if !decodeCheckinRewardCampaignBody(c, &req) {
		return
	}
	campaign, err := h.campaignService.UpdateRewardCampaign(c.Request.Context(), id, service.UpdateCheckinRewardCampaignInput{
		Name: req.Name, StartDate: req.StartDate, EndDate: req.EndDate,
		RewardTiers: req.RewardTiers, AdminID: checkinCampaignAdminID(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, campaign)
}

func (h *CheckinHandler) EnableCampaign(c *gin.Context) {
	h.transitionCampaign(c, h.campaignService.EnableRewardCampaign)
}

func (h *CheckinHandler) DisableCampaign(c *gin.Context) {
	h.transitionCampaign(c, h.campaignService.DisableRewardCampaign)
}

func (h *CheckinHandler) transitionCampaign(c *gin.Context, transition func(context.Context, int64, int64) (*service.CheckinRewardCampaign, error)) {
	id, ok := parseCheckinCampaignID(c)
	if !ok {
		return
	}
	campaign, err := transition(c.Request.Context(), id, checkinCampaignAdminID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, campaign)
}

func (h *CheckinHandler) CopyCampaign(c *gin.Context) {
	id, ok := parseCheckinCampaignID(c)
	if !ok {
		return
	}
	var req CopyCheckinRewardCampaignRequest
	if !decodeCheckinRewardCampaignBody(c, &req) {
		return
	}
	campaign, err := h.campaignService.CopyRewardCampaign(c.Request.Context(), id, req.Name, checkinCampaignAdminID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, campaign)
}

func (h *CheckinHandler) DeleteCampaign(c *gin.Context) {
	id, ok := parseCheckinCampaignID(c)
	if !ok {
		return
	}
	if err := h.campaignService.DeleteRewardCampaign(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, DeleteCheckinRewardCampaignResponse{ID: id, Deleted: true})
}

func parseCheckinCampaignID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, errCheckinRewardCampaignInvalidID)
		return 0, false
	}
	return id, true
}

func checkinCampaignAdminID(c *gin.Context) int64 {
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		return subject.UserID
	}
	return 0
}

func decodeCheckinRewardCampaignBody(c *gin.Context, dst any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, checkinRewardCampaignBodyMaxBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		response.ErrorFrom(c, errCheckinRewardCampaignInvalidRequest)
		return false
	}
	if err := binding.Validator.ValidateStruct(dst); err != nil {
		response.ErrorFrom(c, errCheckinRewardCampaignInvalidRequest)
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		response.ErrorFrom(c, errCheckinRewardCampaignInvalidRequest)
		return false
	}
	return true
}
