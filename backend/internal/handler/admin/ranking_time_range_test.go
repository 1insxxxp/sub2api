package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type rankingTimeRangeRepo struct {
	service.UsageLogRepository
	start time.Time
	end   time.Time
}

func (r *rankingTimeRangeRepo) GetUserBreakdownRanking(_ context.Context, start, end time.Time, dim usagestats.UserBreakdownDimension) (*usagestats.UserBreakdownRankingResponse, error) {
	r.start, r.end = start, end
	return &usagestats.UserBreakdownRankingResponse{Page: dim.Page, PageSize: dim.PageSize}, nil
}

func (r *rankingTimeRangeRepo) GetRechargeRanking(_ context.Context, start, end time.Time, _ int64, _, _, _ string, _, _ int) (*service.RechargeRankingResponse, error) {
	r.start, r.end = start, end
	return &service.RechargeRankingResponse{}, nil
}

func TestRankingDateRangesUseInclusiveLocalDays(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name, zone, from, through, wantStart, wantEnd string
	}{
		{"same day", "Asia/Shanghai", "2026-09-15", "2026-09-15", "2026-09-15T00:00:00+08:00", "2026-09-16T00:00:00+08:00"},
		{"yesterday", "Asia/Shanghai", "2026-09-14", "2026-09-14", "2026-09-14T00:00:00+08:00", "2026-09-15T00:00:00+08:00"},
		{"seven days across month", "Asia/Shanghai", "2026-08-29", "2026-09-04", "2026-08-29T00:00:00+08:00", "2026-09-05T00:00:00+08:00"},
		{"spring daylight saving", "America/New_York", "2026-03-08", "2026-03-08", "2026-03-08T00:00:00-05:00", "2026-03-09T00:00:00-04:00"},
		{"autumn daylight saving", "America/New_York", "2026-11-01", "2026-11-01", "2026-11-01T00:00:00-04:00", "2026-11-02T00:00:00-05:00"},
	}
	for _, tc := range cases {
		for _, kind := range []string{"consumption", "recharge"} {
			t.Run(tc.name+"/"+kind, func(t *testing.T) {
				repo := &rankingTimeRangeRepo{}
				router := gin.New()
				if kind == "consumption" {
					h := NewDashboardHandler(service.NewDashboardService(repo, nil, nil, nil), nil)
					router.GET("/ranking", h.GetUserBreakdown)
				} else {
					h := NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), nil, nil, nil)
					router.GET("/ranking", h.RechargeRanking)
				}
				params := url.Values{"start_date": {tc.from}, "end_date": {tc.through}, "timezone": {tc.zone}, "page": {"1"}, "page_size": {"20"}}
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ranking?"+params.Encode(), nil))
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				require.Equal(t, tc.wantStart, repo.start.Format(time.RFC3339))
				require.Equal(t, tc.wantEnd, repo.end.Format(time.RFC3339))
				if kind == "consumption" {
					var body struct {
						Data struct {
							StartDate string `json:"start_date"`
							EndDate   string `json:"end_date"`
						} `json:"data"`
					}
					require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
					require.Equal(t, tc.from, body.Data.StartDate)
					require.Equal(t, tc.through, body.Data.EndDate)
				}
			})
		}
	}
}
