package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type commissionExclusionsHandlerStub struct {
	subAdminCommissionHandlerServiceStub
	called             int
	userID, operatorID int64
	params             pagination.PaginationParams
	err                error
}

func (s *commissionExclusionsHandlerStub) ListExclusions(_ context.Context, params pagination.PaginationParams) ([]service.SubAdminCommissionExclusion, pagination.PaginationResult, error) {
	s.called++
	s.params = params
	return []service.SubAdminCommissionExclusion{{ID: 1, UserID: 7, UserEmail: "test@example.com", StartedAt: time.Date(2026, 10, 18, 4, 0, 0, 0, time.UTC)}}, pagination.PaginationResult{Page: params.Page, PageSize: params.PageSize, Total: 1}, s.err
}

func (s *commissionExclusionsHandlerStub) AddExclusion(_ context.Context, userID, operatorID int64) (*service.SubAdminCommissionExclusion, error) {
	s.called++
	s.userID, s.operatorID = userID, operatorID
	return &service.SubAdminCommissionExclusion{ID: 1, UserID: userID, StartedAt: time.Date(2026, 10, 18, 4, 0, 0, 0, time.UTC)}, s.err
}

func (s *commissionExclusionsHandlerStub) RemoveExclusion(_ context.Context, userID int64) error {
	s.called++
	s.userID = userID
	return s.err
}

func commissionExclusionsTestRouter(stub *commissionExclusionsHandlerStub, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if role != "" {
		router.Use(func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
			c.Set(string(middleware.ContextKeyUserRole), role)
			c.Next()
		})
	}
	h := &SubAdminCommissionHandler{service: stub}
	router.GET("/exclusions", h.ListExclusions)
	router.POST("/exclusions", h.AddExclusion)
	router.DELETE("/exclusions/:user_id", h.RemoveExclusion)
	return router
}

func TestSubAdminCommissionExclusionHandlersRequireAdmin(t *testing.T) {
	for _, role := range []string{"", "user", "sub_admin"} {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
			stub := &commissionExclusionsHandlerStub{}
			router := commissionExclusionsTestRouter(stub, role)
			path := "/exclusions"
			if method == http.MethodDelete {
				path += "/7"
			}
			req := httptest.NewRequest(method, path, strings.NewReader(`{"user_id":7}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			want := http.StatusForbidden
			if role == "" {
				want = http.StatusUnauthorized
			}
			require.Equal(t, want, rec.Code, role+" "+method)
			require.Zero(t, stub.called)
		}
	}
}

func TestSubAdminCommissionExclusionHandlersValidateAndPaginate(t *testing.T) {
	stub := &commissionExclusionsHandlerStub{}
	router := commissionExclusionsTestRouter(stub, "admin")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/exclusions?page=2&page_size=10", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 2, stub.params.Page)
	var response struct {
		Data struct {
			Items []struct {
				UserID    int64  `json:"user_id"`
				StartedAt string `json:"started_at"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Len(t, response.Data.Items, 1)
	require.Equal(t, int64(7), response.Data.Items[0].UserID)
	require.Equal(t, "2026-10-18T04:00:00Z", response.Data.Items[0].StartedAt)
	for _, body := range []string{`{}`, `{"user_id":0}`, `{"user_id":-1}`, `{"user_id":"7"}`} {
		req := httptest.NewRequest(http.MethodPost, "/exclusions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
	require.Equal(t, 1, stub.called)
	req := httptest.NewRequest(http.MethodPost, "/exclusions", strings.NewReader(`{"user_id":7}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(7), stub.userID)
	require.Equal(t, int64(99), stub.operatorID)
	for _, id := range []string{"-1", "0", "bad"} {
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/exclusions/"+id, nil))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/exclusions/7", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 3, stub.called)
}

func TestSubAdminCommissionExclusionHandlerPropagatesNotFound(t *testing.T) {
	stub := &commissionExclusionsHandlerStub{err: service.ErrUserNotFound}
	router := commissionExclusionsTestRouter(stub, "admin")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/exclusions", strings.NewReader(`{"user_id":7}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}
