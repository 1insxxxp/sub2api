//go:build unit

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type claimHandlerRepositoryStub struct {
	seenUserID      int64
	seenUsage       int64
	recent          []service.EmptyResponseRecentCandidate
	recentPageParam pagination.PaginationParams
	dailyCount      int
	claim           *service.EmptyResponseClaim
	err             error
}

func (s *claimHandlerRepositoryStub) LoadEvaluation(_ context.Context, userID, usageLogID int64) (*service.EmptyResponseClaimEvaluation, error) {
	s.seenUserID, s.seenUsage = userID, usageLogID
	if s.err != nil {
		return nil, s.err
	}
	return &service.EmptyResponseClaimEvaluation{
		Usage: service.UsageLog{ID: usageLogID, UserID: userID, ActualCost: 1.25, CreatedAt: time.Now()},
		Group: service.Group{EmptyResponseCompensationEnabled: true},
		Outcome: &service.ResponseOutcome{
			HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true, CollectorVersion: 1,
		},
	}, nil
}

func (s *claimHandlerRepositoryStub) ListRecentEvaluations(_ context.Context, userID int64, _, _ time.Time, params pagination.PaginationParams) ([]service.EmptyResponseRecentCandidate, *pagination.PaginationResult, error) {
	s.seenUserID = userID
	s.recentPageParam = params
	return s.recent, &pagination.PaginationResult{Total: int64(len(s.recent)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, s.err
}

func (s *claimHandlerRepositoryStub) CountUserClaims(context.Context, int64, time.Time, time.Time) (int, error) {
	return s.dailyCount, nil
}

func (s *claimHandlerRepositoryStub) Create(_ context.Context, input *service.EmptyResponseClaimCreateInput) (*service.EmptyResponseClaim, bool, error) {
	if s.claim == nil {
		s.claim = &service.EmptyResponseClaim{
			ID: 7, UsageLogID: input.Evaluation.Usage.ID, UserID: input.Evaluation.Usage.UserID,
			Status: input.Decision.Status, ReasonCode: input.Decision.ReasonCode,
			OriginalActualCost: input.Evaluation.Usage.ActualCost,
		}
	}
	return s.claim, true, nil
}

func newClaimHandlerTestRouter(repo *claimHandlerRepositoryStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	claimService := service.NewEmptyResponseClaimService(repo, nil)
	handler := NewUsageHandler(nil, nil, nil, nil)
	handler.emptyResponseClaimService = claimService
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage/empty-responses", handler.ListRecentEmptyResponses)
	router.POST("/usage/:id/empty-response-claim", handler.SubmitEmptyResponseClaim)
	return router
}

func TestSubmitEmptyResponseClaimUsesAuthenticatedOwnerAndServerRefund(t *testing.T) {
	repo := &claimHandlerRepositoryStub{}
	router := newClaimHandlerTestRouter(repo)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/usage/99/empty-response-claim", bytes.NewBufferString(`{"reason":"empty in client"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Equal(t, int64(42), repo.seenUserID)
	require.Equal(t, int64(99), repo.seenUsage)
	require.Contains(t, recorder.Body.String(), `"estimated_refund":1.25`)
	require.NotContains(t, recorder.Body.String(), "evidence")
}

func TestSubmitEmptyResponseClaimRejectsClientProvidedAmount(t *testing.T) {
	repo := &claimHandlerRepositoryStub{}
	router := newClaimHandlerTestRouter(repo)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/usage/99/empty-response-claim", bytes.NewBufferString(`{"reason":"empty","amount":999}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Zero(t, repo.seenUserID)
}

func TestSubmitEmptyResponseClaimCannotClaimAnotherUsersRecord(t *testing.T) {
	repo := &claimHandlerRepositoryStub{err: service.ErrEmptyResponseClaimNotFound}
	router := newClaimHandlerTestRouter(repo)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/usage/100/empty-response-claim", bytes.NewBufferString(`{}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Equal(t, int64(42), repo.seenUserID)
}

func TestListRecentEmptyResponsesUsesAuthenticatedOwner(t *testing.T) {
	now := time.Now().UTC()
	repo := &claimHandlerRepositoryStub{recent: []service.EmptyResponseRecentCandidate{{
		Evaluation: service.EmptyResponseClaimEvaluation{
			Usage: service.UsageLog{
				ID: 99, UserID: 42, APIKeyID: 8, AccountID: 9, Model: "claude-opus-4-6",
				InputTokens: 1234, OutputTokens: 0, CacheCreationTokens: 12, CacheReadTokens: 34,
				ActualCost: 1.25, CreatedAt: now.Add(-time.Hour),
			},
			Outcome: &service.ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true, CollectorVersion: 1},
			Group:   service.Group{EmptyResponseCompensationEnabled: true},
		},
		APIKeyName:      "cli",
		GroupName:       "cc",
		InboundEndpoint: "/v1/messages",
	}}}
	router := newClaimHandlerTestRouter(repo)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/usage/empty-responses?page=2&page_size=10", nil)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(42), repo.seenUserID)
	require.Equal(t, 2, repo.recentPageParam.Page)
	require.Equal(t, 10, repo.recentPageParam.PageSize)
	require.Contains(t, recorder.Body.String(), `"items":[`)
	require.Contains(t, recorder.Body.String(), `"total":1`)
	require.Contains(t, recorder.Body.String(), `"page":2`)
	require.Contains(t, recorder.Body.String(), `"page_size":10`)
	require.Contains(t, recorder.Body.String(), `"usage_log_id":99`)
	require.Contains(t, recorder.Body.String(), `"status":"claimable"`)
	require.Contains(t, recorder.Body.String(), `"api_key_name":"cli"`)
	require.Contains(t, recorder.Body.String(), `"input_tokens":1234`)
	require.Contains(t, recorder.Body.String(), `"output_tokens":0`)
	require.Contains(t, recorder.Body.String(), `"total_tokens":1280`)
	require.NotContains(t, recorder.Body.String(), "evidence")
}
