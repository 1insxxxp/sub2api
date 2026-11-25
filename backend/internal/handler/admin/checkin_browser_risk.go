package admin

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *CheckinHandler) GetAccountRiskConfig(c *gin.Context) {
	cfg, err := h.checkinService.AccountRiskConfig(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	hits, err := h.checkinService.RegistrationRiskHitsToday(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, struct {
		service.AccountRiskConfig
		RegistrationHits map[string]int64 `json:"registration_hits_today"`
	}{cfg, hits})
}
func (h *CheckinHandler) UpdateAccountRiskConfig(c *gin.Context) {
	var cfg service.AccountRiskConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "Invalid account risk settings")
		return
	}
	if response.ErrorFrom(c, h.checkinService.UpdateAccountRiskConfig(c.Request.Context(), cfg)) {
		return
	}
	response.Success(c, cfg)
}
func (h *CheckinHandler) ListBrowserReviews(c *gin.Context) {
	page, size := response.ParsePagination(c)
	size = min(size, 100)
	date := strings.TrimSpace(c.Query("date"))
	if date != "" {
		if _, err := time.Parse(time.DateOnly, date); err != nil {
			response.BadRequest(c, "Invalid date")
			return
		}
	}
	items, total, err := h.checkinService.ListCheckinBrowserReviews(c.Request.Context(), page, size, c.Query("search"), c.Query("status"), date)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, items, total, page, size)
}
func (h *CheckinHandler) DecideBrowserReview(c *gin.Context) {
	var req struct {
		UserID      int64  `json:"user_id" binding:"required,gt=0"`
		BrowserHash string `json:"browser_hash" binding:"required,len=64,hexadecimal"`
		Date        string `json:"checkin_date" binding:"required,datetime=2006-01-02"`
		Decision    string `json:"decision" binding:"required,oneof=approved rejected"`
		Reason      string `json:"reason" binding:"required,max=1000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid review decision")
		return
	}
	err := h.checkinService.DecideCheckinBrowserReview(c.Request.Context(), req.UserID, req.BrowserHash, req.Date, req.Decision, checkinCampaignAdminID(c), req.Reason)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"status": req.Decision})
}
func (h *CheckinHandler) ListBrowserWhitelist(c *gin.Context) {
	page, size := response.ParsePagination(c)
	size = min(size, 100)
	items, total, err := h.checkinService.ListBrowserWhitelist(c.Request.Context(), page, size, c.Query("search"))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, items, total, page, size)
}
func (h *CheckinHandler) UpdateBrowserWhitelist(c *gin.Context) {
	var req struct {
		Email   string `json:"email" binding:"required,email,max=320"`
		Enabled *bool  `json:"enabled" binding:"required"`
		Reason  string `json:"reason" binding:"required,max=1000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid whitelist request")
		return
	}
	err := h.checkinService.SetBrowserWhitelist(c.Request.Context(), req.Email, *req.Enabled, checkinCampaignAdminID(c), req.Reason)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"enabled": *req.Enabled})
}
