package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type slaSummaryCaptureRepo struct {
	service.OpsRepository
	filter *service.OpsErrorLogFilter
	result *service.OpsSLAErrorSummary
}

func (r *slaSummaryCaptureRepo) GetSLAErrorSummary(_ context.Context, filter *service.OpsErrorLogFilter) (*service.OpsSLAErrorSummary, error) {
	r.filter = filter
	if r.result != nil {
		return r.result, nil
	}
	return &service.OpsSLAErrorSummary{Groups: []*service.OpsUpstreamErrorSummaryGroup{}}, nil
}

func newSLASummaryTestRouter(h *OpsHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin/ops/request-errors/summary", h.SummaryRequestErrors)
	r.GET("/admin/ops/request-errors/:id", h.GetRequestError)
	return r
}

func TestOpsSLAErrorSummary_PassesFiltersAndForcesFinalSemantics(t *testing.T) {
	repo := &slaSummaryCaptureRepo{result: &service.OpsSLAErrorSummary{TotalErrors: 2, Groups: []*service.OpsUpstreamErrorSummaryGroup{}}}
	svc := service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := newSLASummaryTestRouter(NewOpsHandler(svc))

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/request-errors/summary?time_range=6h&platform=openai&group_id=12&q=needle&model=gpt-5&status_codes=500,502&status_codes_other=1&include_recovered=true", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotNil(t, repo.filter)
	require.Equal(t, "openai", repo.filter.Platform)
	require.Equal(t, "gpt-5", repo.filter.Model)
	require.Equal(t, "needle", repo.filter.Query)
	require.Equal(t, "errors", repo.filter.View)
	require.False(t, repo.filter.IncludeRecoveredUpstream)
	require.Empty(t, repo.filter.Owner)
	require.Empty(t, repo.filter.ErrorPhasesAny)
	require.NotNil(t, repo.filter.GroupID)
	require.EqualValues(t, 12, *repo.filter.GroupID)
	require.Equal(t, []int{500, 502}, repo.filter.StatusCodes)
	require.True(t, repo.filter.StatusCodesOther)

	var envelope responseEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	var got service.OpsSLAErrorSummary
	require.NoError(t, json.Unmarshal(envelope.Data, &got))
	require.EqualValues(t, 2, got.TotalErrors)
}

func TestOpsSLAErrorSummary_RoutePrecedesID(t *testing.T) {
	repo := &slaSummaryCaptureRepo{}
	r := newSLASummaryTestRouter(NewOpsHandler(service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/ops/request-errors/summary", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestOpsSLAErrorSummary_RejectsInvalidParameters(t *testing.T) {
	r := newSLASummaryTestRouter(NewOpsHandler(service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)))
	for _, query := range []string{"group_id=bad", "status_codes=500,nope", "status_codes_other=maybe", "start_time=not-a-time"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/ops/request-errors/summary?"+query, nil))
		require.Equal(t, http.StatusBadRequest, w.Code, "query %q: %s", query, w.Body.String())
	}
}

func TestOpsSLAErrorSummary_MonitoringDisabled(t *testing.T) {
	svc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: false}}, nil, nil, nil, nil, nil, nil, nil, nil)
	r := newSLASummaryTestRouter(NewOpsHandler(svc))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/ops/request-errors/summary", nil))
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}
