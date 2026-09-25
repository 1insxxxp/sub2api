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
)

type upstreamSummaryCaptureRepo struct {
	service.OpsRepository
	filter *service.OpsErrorLogFilter
	result *service.OpsUpstreamErrorSummary
}

func (r *upstreamSummaryCaptureRepo) GetUpstreamErrorSummary(_ context.Context, filter *service.OpsErrorLogFilter) (*service.OpsUpstreamErrorSummary, error) {
	r.filter = filter
	if r.result != nil {
		return r.result, nil
	}
	return &service.OpsUpstreamErrorSummary{Groups: []*service.OpsUpstreamErrorSummaryGroup{}}, nil
}

func newUpstreamSummaryTestRouter(h *OpsHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin/ops/upstream-errors/summary", h.SummaryUpstreamErrors)
	r.GET("/admin/ops/upstream-errors/:id", h.GetUpstreamError)
	return r
}

func TestOpsUpstreamErrorSummary_PassesFiltersAndFixedSemantics(t *testing.T) {
	repo := &upstreamSummaryCaptureRepo{}
	svc := service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	want := &service.OpsUpstreamErrorSummary{TotalErrors: 2, Groups: []*service.OpsUpstreamErrorSummaryGroup{}}
	repo.result = want
	r := newUpstreamSummaryTestRouter(NewOpsHandler(svc))

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/upstream-errors/summary?time_range=6h&platform=openai&group_id=12&account_id=34&q=needle&model=gpt-5&status_codes=500,502&status_codes_other=1&phase=gateway&error_owner=client&error_source=upstream_http&view=all&resolved=yes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200: %s", w.Code, w.Body.String())
	}
	if repo.filter == nil {
		t.Fatal("summary repository was not called")
	}
	filter := repo.filter
	if filter.Platform != "openai" || filter.Query != "needle" || filter.Model != "gpt-5" || filter.Source != "upstream_http" || filter.Phase != "gateway" {
		t.Fatalf("filter did not preserve query fields: %+v", filter)
	}
	if filter.GroupID == nil || *filter.GroupID != 12 || filter.AccountID == nil || *filter.AccountID != 34 {
		t.Fatalf("filter ids = group %v account %v", filter.GroupID, filter.AccountID)
	}
	if filter.Owner != "provider" || !filter.IncludeRecoveredUpstream || len(filter.ErrorPhasesAny) != 2 || filter.ErrorPhasesAny[0] != "upstream" || filter.ErrorPhasesAny[1] != "account_auth" {
		t.Fatalf("fixed provider semantics not enforced: %+v", filter)
	}
	if filter.Resolved == nil || !*filter.Resolved {
		t.Fatalf("resolved filter = %+v", filter.Resolved)
	}
	if len(filter.StatusCodes) != 2 || filter.StatusCodes[0] != 500 || filter.StatusCodes[1] != 502 || !filter.StatusCodesOther {
		t.Fatalf("status filters = %+v other=%v", filter.StatusCodes, filter.StatusCodesOther)
	}

	var envelope responseEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var got service.OpsUpstreamErrorSummary
	if err := json.Unmarshal(envelope.Data, &got); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if got.TotalErrors != want.TotalErrors {
		t.Fatalf("summary = %+v, want %+v", got, want)
	}
}

func TestOpsUpstreamErrorSummary_RoutePrecedesID(t *testing.T) {
	repo := &upstreamSummaryCaptureRepo{}
	svc := service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := newUpstreamSummaryTestRouter(NewOpsHandler(svc))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/ops/upstream-errors/summary", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("summary was routed as :id: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestOpsUpstreamErrorSummary_RejectsInvalidParameters(t *testing.T) {
	h := NewOpsHandler(service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
	r := newUpstreamSummaryTestRouter(h)
	for _, query := range []string{"group_id=bad", "status_codes=500,nope", "resolved=maybe"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/ops/upstream-errors/summary?"+query, nil))
		if w.Code != http.StatusBadRequest {
			t.Errorf("query %q status=%d, want 400", query, w.Code)
		}
	}
}

func TestOpsUpstreamErrorSummary_MonitoringDisabled(t *testing.T) {
	svc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: false}}, nil, nil, nil, nil, nil, nil, nil, nil)
	r := newUpstreamSummaryTestRouter(NewOpsHandler(svc))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/ops/upstream-errors/summary", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", w.Code)
	}
}
