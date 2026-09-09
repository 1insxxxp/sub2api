//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type wiredAdminClaimRepositoryStub struct {
	service.EmptyResponseClaimAdminRepository
	listed bool
}

func (s *wiredAdminClaimRepositoryStub) List(_ context.Context, params pagination.PaginationParams, _ service.EmptyResponseClaimListFilters) ([]service.EmptyResponseClaim, *pagination.PaginationResult, error) {
	s.listed = true
	return []service.EmptyResponseClaim{{ID: 123}}, &pagination.PaginationResult{
		Total: 1, Page: params.Page, PageSize: params.PageSize, Pages: 1,
	}, nil
}

func TestProvideAdminUsageHandlerWiresEmptyResponseClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &wiredAdminClaimRepositoryStub{}
	claimService := service.NewEmptyResponseClaimAdminService(repo, nil)
	handler := ProvideAdminUsageHandler(nil, nil, nil, nil, claimService)
	router := gin.New()
	router.GET("/admin/usage/empty-response-claims", handler.ListEmptyResponseClaims)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/usage/empty-response-claims", nil))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.True(t, repo.listed)
	require.Contains(t, recorder.Body.String(), `"id":123`)
}
