package admin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type channelAcceptanceService interface {
	Targets(context.Context, []int64) ([]service.AcceptanceTarget, error)
	Preview(context.Context, int64, service.AcceptancePreviewRequest) (*service.AcceptancePreview, error)
	Start(context.Context, int64, string) (*service.AcceptanceRun, error)
	Get(context.Context, string) (*service.AcceptanceRun, error)
	History(context.Context, []int64) ([]service.AcceptanceRun, error)
	Cancel(context.Context, string) (*service.AcceptanceRun, error)
}
type ChannelAcceptanceHandler struct{ service channelAcceptanceService }

func NewChannelAcceptanceHandler(s *service.ChannelAcceptanceService) *ChannelAcceptanceHandler {
	return &ChannelAcceptanceHandler{service: s}
}

func (h *ChannelAcceptanceHandler) actor(c *gin.Context) (int64, bool) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("Pragma", "no-cache")
	actor, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || actor.UserID <= 0 {
		response.Error(c, 401, "请先登录管理员账号")
		return 0, false
	}
	role, _ := middleware.GetUserRoleFromContext(c)
	if role != service.RoleAdmin || adminaccess.RestrictedFromContext(c.Request.Context()) != nil {
		response.Error(c, 403, "仅完整管理员可使用渠道验收")
		return 0, false
	}
	if h == nil || h.service == nil {
		response.Error(c, 503, "渠道验收暂不可用")
		return 0, false
	}
	return actor.UserID, true
}
func acceptanceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrAcceptanceInvalid):
		response.ErrorWithDetails(c, 400, "验收参数无效", "ACCEPTANCE_INVALID", nil)
	case errors.Is(err, service.ErrAcceptancePriceRequired):
		response.ErrorWithDetails(c, 400, "请先补充上游成本价格", "ACCEPTANCE_PRICE_REQUIRED", nil)
	case errors.Is(err, service.ErrAcceptanceNotFound):
		response.ErrorWithDetails(c, 404, "验收记录不存在", "ACCEPTANCE_NOT_FOUND", nil)
	case errors.Is(err, service.ErrAcceptanceStale):
		response.ErrorWithDetails(c, 409, "预览已过期，或账号配置、价格已变化，请重新预览", "ACCEPTANCE_STALE", nil)
	default:
		response.ErrorWithDetails(c, 503, "渠道验收暂不可用，请稍后重试", "ACCEPTANCE_UNAVAILABLE", nil)
	}
}
func acceptanceDecode(c *gin.Context, v any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		acceptanceError(c, service.ErrAcceptanceInvalid)
		return false
	}
	var tail any
	if err := d.Decode(&tail); err != io.EOF {
		acceptanceError(c, service.ErrAcceptanceInvalid)
		return false
	}
	return true
}
func acceptanceIDs(c *gin.Context, required bool) ([]int64, bool) {
	for key, values := range c.Request.URL.Query() {
		if (key != "account_ids" && key != "timezone") || len(values) != 1 {
			acceptanceError(c, service.ErrAcceptanceInvalid)
			return nil, false
		}
	}
	value := c.Query("account_ids")
	if value == "" {
		if required {
			acceptanceError(c, service.ErrAcceptanceInvalid)
			return nil, false
		}
		return nil, true
	}
	parts := strings.Split(value, ",")
	if len(parts) > 50 {
		acceptanceError(c, service.ErrAcceptanceInvalid)
		return nil, false
	}
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			acceptanceError(c, service.ErrAcceptanceInvalid)
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}
func acceptanceRunID(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		acceptanceError(c, service.ErrAcceptanceInvalid)
		return "", false
	}
	return id, true
}
func (h *ChannelAcceptanceHandler) Targets(c *gin.Context) {
	if _, ok := h.actor(c); !ok {
		return
	}
	ids, ok := acceptanceIDs(c, true)
	if !ok {
		return
	}
	items, err := h.service.Targets(c.Request.Context(), ids)
	if err != nil {
		acceptanceError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *ChannelAcceptanceHandler) Preview(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var req service.AcceptancePreviewRequest
	if !acceptanceDecode(c, &req) {
		return
	}
	p, err := h.service.Preview(c.Request.Context(), actor, req)
	if err != nil {
		acceptanceError(c, err)
		return
	}
	response.Success(c, p)
}
func (h *ChannelAcceptanceHandler) Start(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var req struct {
		PreviewID string `json:"preview_id"`
	}
	if !acceptanceDecode(c, &req) {
		return
	}
	if _, err := uuid.Parse(req.PreviewID); err != nil {
		acceptanceError(c, service.ErrAcceptanceInvalid)
		return
	}
	r, err := h.service.Start(c.Request.Context(), actor, req.PreviewID)
	if err != nil {
		acceptanceError(c, err)
		return
	}
	response.Success(c, r)
}
func (h *ChannelAcceptanceHandler) History(c *gin.Context) {
	if _, ok := h.actor(c); !ok {
		return
	}
	ids, ok := acceptanceIDs(c, false)
	if !ok {
		return
	}
	items, err := h.service.History(c.Request.Context(), ids)
	if err != nil {
		acceptanceError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *ChannelAcceptanceHandler) Get(c *gin.Context) {
	if _, ok := h.actor(c); !ok {
		return
	}
	id, ok := acceptanceRunID(c)
	if !ok {
		return
	}
	r, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		acceptanceError(c, err)
		return
	}
	response.Success(c, r)
}
func (h *ChannelAcceptanceHandler) Cancel(c *gin.Context) {
	if _, ok := h.actor(c); !ok {
		return
	}
	id, ok := acceptanceRunID(c)
	if !ok {
		return
	}
	r, err := h.service.Cancel(c.Request.Context(), id)
	if err != nil {
		acceptanceError(c, err)
		return
	}
	response.Success(c, r)
}
func (h *ChannelAcceptanceHandler) Export(c *gin.Context) {
	if _, ok := h.actor(c); !ok {
		return
	}
	id, ok := acceptanceRunID(c)
	if !ok {
		return
	}
	r, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		acceptanceError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="channel-acceptance-`+id+`.json"`)
	c.JSON(200, r)
}
