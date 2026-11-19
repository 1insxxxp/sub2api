package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type cacheTokenStatsRepo struct {
	service.OpsRepository
	calls int
}

func (r *cacheTokenStatsRepo) GetOpenAITokenStats(_ context.Context, f *service.OpsOpenAITokenStatsFilter) (*service.OpsOpenAITokenStatsResponse, error) {
	r.calls++
	return &service.OpsOpenAITokenStatsResponse{StartTime: f.StartTime, EndTime: f.EndTime, Total: int64(r.calls)}, nil
}

func TestOpsTokenStatsCacheReusesRollingRangeAndIsolatesFilters(t *testing.T) {
	repo := &cacheTokenStatsRepo{}
	svc := service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.GET("/stats", NewOpsHandler(svc).GetDashboardOpenAITokenStats)
	get := func(q string) string {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stats?"+q, nil))
		require.Equal(t, 200, w.Code)
		return w.Body.String()
	}
	first := get("group_id=3")
	require.JSONEq(t, first, get("group_id=3&time_range=30d&page=1&page_size=20"))
	require.Equal(t, 1, repo.calls)
	for _, q := range []string{"group_id=4", "group_id=3&platform=gemini", "group_id=3&top_n=20", "group_id=3&page=2", "group_id=3&time_range=1h", "group_id=3&page_size=30"} {
		get(q)
	}
	require.Equal(t, 7, repo.calls)
}
