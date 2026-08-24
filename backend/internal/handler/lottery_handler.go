package handler

import (
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

func (h *LotteryHandler) GetState(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	state, err := h.lotteryService.GetPublicState(c.Request.Context(), subject.UserID, time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, state)
}

type drawLotteryRequest struct {
	AttemptKey string `json:"attempt_key" binding:"required"`
}

func (h *LotteryHandler) Draw(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req drawLotteryRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.AttemptKey) == "" {
		response.BadRequest(c, "attempt_key is required")
		return
	}
	result, err := h.lotteryService.Draw(c.Request.Context(), subject.UserID, req.AttemptKey, time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *LotteryHandler) History(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	draws, total, err := h.lotteryService.ListUserDraws(
		c.Request.Context(), subject.UserID,
		(page-1)*pageSize, pageSize,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, draws, int64(total), page, pageSize)
}
