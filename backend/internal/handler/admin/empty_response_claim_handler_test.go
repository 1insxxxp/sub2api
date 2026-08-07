//go:build unit

package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminClaimRepositoryStub struct {
	claims     []service.EmptyResponseClaim
	seenID     int64
	seenStatus string
	seenAdmin  int64
	seenNote   string
	seenFilter service.EmptyResponseClaimListFilters
}

func (s *adminClaimRepositoryStub) List(_ context.Context, params pagination.PaginationParams, filters service.EmptyResponseClaimListFilters) ([]service.EmptyResponseClaim, *pagination.PaginationResult, error) {
	s.seenFilter = filters
	return s.claims, &pagination.PaginationResult{Total: int64(len(s.claims)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func (s *adminClaimRepositoryStub) Review(_ context.Context, id int64, status string, reviewerID int64, note string) (*service.EmptyResponseClaim, error) {
	s.seenID, s.seenStatus, s.seenAdmin, s.seenNote = id, status, reviewerID, note
	return &service.EmptyResponseClaim{ID: id, Status: status, OriginalActualCost: 1.25}, nil
}

func newAdminClaimHandlerTestRouter(repo *adminClaimRepositoryStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	claimService := service.NewEmptyResponseClaimAdminService(repo, nil)
	handler := NewUsageHandler(nil, nil, nil, nil)
	handler.emptyResponseClaimService = claimService
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 9})
		c.Next()
	})
	router.GET("/admin/usage/empty-response-claims", handler.ListEmptyResponseClaims)
	router.POST("/admin/usage/empty-response-claims/:id/approve", handler.ApproveEmptyResponseClaim)
	router.POST("/admin/usage/empty-response-claims/:id/reject", handler.RejectEmptyResponseClaim)
	router.POST("/admin/usage/empty-response-claims/batch", handler.BatchEmptyResponseClaims)
	return router
}

func TestAdminEmptyResponseClaimListAppliesInclusiveCalendarRange(t *testing.T) {
	repo := &adminClaimRepositoryStub{}
	router := newAdminClaimHandlerTestRouter(repo)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/usage/empty-response-claims?start_date=2026-08-01&end_date=2026-08-07", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, repo.seenFilter.StartTime)
	require.NotNil(t, repo.seenFilter.EndTime)
	require.Equal(t, "2026-08-01", repo.seenFilter.StartTime.Format("2006-01-02"))
	require.Equal(t, "2026-08-08", repo.seenFilter.EndTime.Format("2006-01-02"))
}

func TestAdminEmptyResponseClaimListUsesRequestedTimezone(t *testing.T) {
	repo := &adminClaimRepositoryStub{}
	router := newAdminClaimHandlerTestRouter(repo)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/usage/empty-response-claims?start_date=2026-08-01&end_date=2026-08-07&timezone=America%2FLos_Angeles", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, repo.seenFilter.StartTime)
	require.NotNil(t, repo.seenFilter.EndTime)
	require.Equal(t, "America/Los_Angeles", repo.seenFilter.StartTime.Location().String())
	require.Equal(t, "America/Los_Angeles", repo.seenFilter.EndTime.Location().String())
}

func TestAdminEmptyResponseClaimListExposesStructuredEvidenceOnly(t *testing.T) {
	repo := &adminClaimRepositoryStub{claims: []service.EmptyResponseClaim{{
		ID: 1, Status: service.EmptyResponseClaimManualReview, Model: "gpt-test",
		Evidence: service.ResponseOutcome{HTTPStatus: 200, HasText: false, CollectorVersion: 1},
	}}}
	router := newAdminClaimHandlerTestRouter(repo)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/usage/empty-response-claims", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"has_text":false`)
	require.NotContains(t, recorder.Body.String(), "response_text")
	require.NotContains(t, recorder.Body.String(), "prompt")
}

func TestAdminEmptyResponseClaimRejectRequiresReason(t *testing.T) {
	repo := &adminClaimRepositoryStub{}
	router := newAdminClaimHandlerTestRouter(repo)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/usage/empty-response-claims/12/reject", bytes.NewBufferString(`{"note":""}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Zero(t, repo.seenID)
}

func TestAdminEmptyResponseClaimApproveUsesAuthenticatedReviewer(t *testing.T) {
	repo := &adminClaimRepositoryStub{}
	router := newAdminClaimHandlerTestRouter(repo)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/usage/empty-response-claims/12/approve", bytes.NewBufferString(`{"note":"verified"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(12), repo.seenID)
	require.Equal(t, int64(9), repo.seenAdmin)
	require.Equal(t, service.EmptyResponseClaimApproved, repo.seenStatus)
}

func TestAdminEmptyResponseClaimBatchAcceptsActionVerbs(t *testing.T) {
	repo := &adminClaimRepositoryStub{}
	router := newAdminClaimHandlerTestRouter(repo)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/usage/empty-response-claims/batch", bytes.NewBufferString(`{"ids":[12],"action":"approve","note":"verified"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, service.EmptyResponseClaimApproved, repo.seenStatus)
}
