//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type canceledRequestCaptureRepo struct {
	service.OpsRepository
	filter *service.OpsErrorLogFilter
}

func (r *canceledRequestCaptureRepo) ListErrorLogs(_ context.Context, f *service.OpsErrorLogFilter) (*service.OpsErrorLogList, error) {
	r.filter = f
	return &service.OpsErrorLogList{Errors: []*service.OpsErrorLog{}, Page: 1, PageSize: 20}, nil
}

func TestOpsErrorEndpointsCancellationVisibility(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		want       bool
	}{
		{"provider health default", "/logs?view=all", false},
		{"usage explicit", "/logs?view=all&include_canceled=true", true},
		{"usage explicit off", "/logs?view=all&include_canceled=false", false},
		{"cancellation category", "/logs?category=client_cancel", true},
		{"request errors", "/requests", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			repo := &canceledRequestCaptureRepo{}
			h := NewOpsHandler(service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
			r := gin.New()
			r.GET("/logs", h.GetErrorLogs)
			r.GET("/requests", h.ListRequestErrors)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.NotNil(t, repo.filter)
			encoded, err := json.Marshal(repo.filter)
			require.NoError(t, err)
			var fields map[string]any
			require.NoError(t, json.Unmarshal(encoded, &fields))
			if tc.path == "/requests" || tc.path == "/logs?view=all&include_canceled=false" {
				require.True(t, repo.filter.ExcludeCanceledRequests, "usage errors exclude even legacy canceled error statuses")
			}
			if tc.want {
				require.Equal(t, true, fields["IncludeCanceledRequests"])
			} else {
				require.NotEqual(t, true, fields["IncludeCanceledRequests"])
			}
		})
	}
}

func TestOpsErrorEndpointsRejectMalformedCancellationOptIn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &canceledRequestCaptureRepo{}
	h := NewOpsHandler(service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
	r := gin.New()
	r.GET("/logs", h.GetErrorLogs)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/logs?include_canceled=maybe", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Nil(t, repo.filter)
}
