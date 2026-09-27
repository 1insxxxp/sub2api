package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type activityHeatmapRepoStub struct {
	service.UsageLogRepository
	days    []usagestats.ActivityHeatmapDay
	called  bool
	start   time.Time
	end     time.Time
	filters usagestats.UsageLogFilters
}

func (s *activityHeatmapRepoStub) GetUserActivityHeatmap(
	ctx context.Context,
	startTime, endTime time.Time,
	filters usagestats.UsageLogFilters,
) ([]usagestats.ActivityHeatmapDay, error) {
	s.called = true
	s.start = startTime
	s.end = endTime
	s.filters = filters
	return s.days, nil
}

func newActivityHeatmapTestRouter(repo *activityHeatmapRepoStub, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	handler := NewUsageHandler(usageSvc, nil, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
		c.Next()
	})
	router.GET("/usage/dashboard/activity", handler.DashboardActivity)
	return router
}

func TestDashboardActivityReturnsUserScopedDailyData(t *testing.T) {
	repo := &activityHeatmapRepoStub{
		days: []usagestats.ActivityHeatmapDay{{
			Date:            "2026-09-27",
			SuccessRequests: 4,
			FailedRequests:  1,
			InputTokens:     10,
			OutputTokens:    20,
			TotalTokens:     30,
			BilledCost:      1.25,
			ModelCount:      2,
		}},
	}
	router := newActivityHeatmapTestRouter(repo, 42)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/activity?start_date=2026-09-01&end_date=2026-09-27", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, repo.called)
	require.Equal(t, int64(42), repo.filters.UserID)
	require.Equal(t, "2026-09-01", repo.start.Format("2006-01-02"))
	require.Equal(t, "2026-09-28", repo.end.Format("2006-01-02"))

	var got struct {
		Data struct {
			Days []usagestats.ActivityHeatmapDay `json:"days"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, repo.days, got.Data.Days)
}
