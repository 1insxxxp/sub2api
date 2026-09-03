package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type LotteryHandler struct {
	lotteryService *service.LotteryService
}

func NewLotteryHandler(lotteryService *service.LotteryService) *LotteryHandler {
	return &LotteryHandler{lotteryService: lotteryService}
}

func (h *LotteryHandler) GetConfig(c *gin.Context) {
	config, err := h.lotteryService.GetAdminConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

func (h *LotteryHandler) ListDraws(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	draws, total, err := h.lotteryService.ListAdminDraws(
		c.Request.Context(), (page-1)*pageSize, pageSize,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, draws, int64(total), page, pageSize)
}

type SaveLotteryActivityRequest struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Status       string     `json:"status"`
	AttemptMode  string     `json:"attempt_mode"`
	AttemptLimit int        `json:"attempt_limit"`
	StartsAt     *time.Time `json:"starts_at"`
	EndsAt       *time.Time `json:"ends_at"`
}

func (h *LotteryHandler) SaveActivity(c *gin.Context) {
	var req SaveLotteryActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	activity, err := h.lotteryService.SaveActivity(c.Request.Context(), req.ID, service.LotteryActivityInput{
		Name: req.Name, Description: req.Description, Status: req.Status,
		AttemptMode: req.AttemptMode, AttemptLimit: req.AttemptLimit,
		StartsAt: req.StartsAt, EndsAt: req.EndsAt,
	}, &subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, activity)
}

type SaveLotteryPrizeRequest struct {
	ActivityID    int64    `json:"activity_id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Type          string   `json:"type"`
	Weight        int      `json:"weight"`
	BalanceAmount *float64 `json:"balance_amount"`
	Enabled       bool     `json:"enabled"`
	SortOrder     int      `json:"sort_order"`
}

func (h *LotteryHandler) CreatePrize(c *gin.Context) {
	var req SaveLotteryPrizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	activityID := req.ActivityID
	if activityID <= 0 {
		if parsed, ok := parseLotteryID(c.Query("activity_id")); ok {
			activityID = parsed
		}
	}
	if activityID <= 0 {
		response.BadRequest(c, "activity_id is required")
		return
	}
	prize, err := h.lotteryService.CreatePrize(c.Request.Context(), activityID, lotteryPrizeInputFromRequest(req))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, prize)
}

type UpdateLotteryPrizeRequest struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Type          string   `json:"type"`
	Weight        int      `json:"weight"`
	BalanceAmount *float64 `json:"balance_amount"`
	Enabled       bool     `json:"enabled"`
	SortOrder     int      `json:"sort_order"`
}

func (h *LotteryHandler) UpdatePrize(c *gin.Context) {
	id, ok := parseLotteryID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid prize ID")
		return
	}
	var req UpdateLotteryPrizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	prize, err := h.lotteryService.UpdatePrize(c.Request.Context(), service.LotteryPrizeInput{
		ID: id, Name: req.Name, Description: req.Description, Type: req.Type,
		Weight: req.Weight, BalanceAmount: req.BalanceAmount, Enabled: req.Enabled, SortOrder: req.SortOrder,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, prize)
}

func (h *LotteryHandler) DeletePrize(c *gin.Context) {
	id, ok := parseLotteryID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid prize ID")
		return
	}
	if err := h.lotteryService.DeletePrize(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

type AppendLotteryItemsRequest struct {
	Contents []string `json:"contents"`
}

func (h *LotteryHandler) AppendItems(c *gin.Context) {
	id, ok := parseLotteryID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid prize ID")
		return
	}
	var req AppendLotteryItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	added, err := h.lotteryService.AppendPrizeItems(c.Request.Context(), id, req.Contents)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"added": added})
}

func (h *LotteryHandler) ListItems(c *gin.Context) {
	id, ok := parseLotteryID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid prize ID")
		return
	}
	items, err := h.lotteryService.ListPrizeItems(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

type DeleteLotteryItemsRequest struct {
	ItemIDs []int64 `json:"item_ids"`
}

func (h *LotteryHandler) DeleteItems(c *gin.Context) {
	id, ok := parseLotteryID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid prize ID")
		return
	}
	var req DeleteLotteryItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	deleted, err := h.lotteryService.DeleteAvailablePrizeItems(c.Request.Context(), id, req.ItemIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": deleted})
}

func lotteryPrizeInputFromRequest(req SaveLotteryPrizeRequest) service.LotteryPrizeInput {
	return service.LotteryPrizeInput{
		Name: req.Name, Description: req.Description, Type: req.Type, Weight: req.Weight,
		BalanceAmount: req.BalanceAmount, Enabled: req.Enabled, SortOrder: req.SortOrder,
	}
}

func parseLotteryID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return id, err == nil && id > 0
}
