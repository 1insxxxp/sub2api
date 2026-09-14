package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type rechargeRankingHandlerRepo struct{ service.UsageLogRepository }

func (rechargeRankingHandlerRepo) GetRechargeRanking(context.Context, time.Time, time.Time, int64, string, string, string, int, int) (*service.RechargeRankingResponse, error) {
	return &service.RechargeRankingResponse{}, nil
}

func TestRechargeRankingRejectsReverseDateRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewUsageService(rechargeRankingHandlerRepo{}, nil, nil, nil)
	h := NewUsageHandler(svc, nil, nil, nil)
	router := gin.New()
	router.GET("/admin/usage/recharge-ranking", h.RechargeRanking)
	req := httptest.NewRequest(http.MethodGet, "/admin/usage/recharge-ranking?start_date=2026-09-15&end_date=2026-09-14", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
