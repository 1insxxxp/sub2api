//go:build unit

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type claimHandlerRepositoryStub struct {
	seenUserID int64
	seenUsage  int64
	claim      *service.EmptyResponseClaim
	err        error
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

func (s *claimHandlerRepositoryStub) CountUserClaims(context.Context, int64, time.Time, time.Time) (int, error) {
	return 0, nil
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
