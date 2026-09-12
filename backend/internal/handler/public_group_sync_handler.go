package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type PublicGroupSyncHandler struct {
	service *service.PublicGroupSyncService
	secret  string
}

func NewPublicGroupSyncHandler(s *service.PublicGroupSyncService, cfg *config.Config) *PublicGroupSyncHandler {
	secret := ""
	if cfg != nil {
		secret = strings.TrimSpace(cfg.PublicGroupSync.Secret)
	}
	if strings.TrimSpace(secret) == "" {
		secret = strings.TrimSpace(os.Getenv("PUBLIC_GROUP_SYNC_SECRET"))
	}
	return &PublicGroupSyncHandler{service: s, secret: secret}
}
func (h *PublicGroupSyncHandler) Snapshot(c *gin.Context) {
	if strings.TrimSpace(h.secret) == "" {
		c.Status(http.StatusNotFound)
		return
	}
	ts, err := strconv.ParseInt(c.GetHeader("X-Public-Group-Sync-Timestamp"), 10, 64)
	if err != nil || time.Since(time.Unix(ts, 0)).Abs() > 5*time.Minute {
		c.Status(http.StatusUnauthorized)
		return
	}
	mac := hmac.New(sha256.New, []byte(h.secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(ts, 10)))
	_, _ = mac.Write([]byte("\n"))
	provided, _ := hex.DecodeString(c.GetHeader("X-Public-Group-Sync-Signature"))
	if !hmac.Equal(mac.Sum(nil), provided) {
		c.Status(http.StatusUnauthorized)
		return
	}
	snap, err := h.service.Snapshot(c.Request.Context())
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": 1, "snapshots": snap})
}
