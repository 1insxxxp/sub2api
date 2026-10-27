package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type pendingUsageDetailRepoStub struct {
	service.UsageLogRepository
	owner       int64
	requestedID int64
}

func (s *pendingUsageDetailRepoStub) GetByID(_ context.Context, id int64) (*service.UsageLog, error) {
	s.requestedID = id
	return &service.UsageLog{ID: id, UserID: s.owner, BillingStatus: service.UsageBillingStatusPendingReview, BillingPendingReason: service.UsagePendingReasonUpstreamMissing}, nil
}

func TestPendingUsageDetailPreservesOwnership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, owner := range []int64{42, 43} {
		repo := &pendingUsageDetailRepoStub{owner: owner}
		handler := NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), nil, nil, nil)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		})
		router.GET("/usage/:id", handler.GetByID)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/usage/-9", nil))
		require.EqualValues(t, -9, repo.requestedID)
		if owner == 42 {
			require.Equal(t, http.StatusOK, rec.Code)
			require.Contains(t, rec.Body.String(), `"billing_status":"pending_review"`)
			require.Contains(t, rec.Body.String(), `"compensation_eligible":false`)
		} else {
			require.Equal(t, http.StatusForbidden, rec.Code)
			require.NotContains(t, rec.Body.String(), `"billing_status"`)
		}
	}
}

func TestNormalUsageListIncludesPendingWithForcedOwner(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/usage?user_id=9999", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, repo.listFilters.IncludePending)
	require.NotEqualValues(t, 9999, repo.listFilters.UserID)
	require.Positive(t, repo.listFilters.UserID)
}
