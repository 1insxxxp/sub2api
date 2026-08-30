package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type subAdminCommissionHandlerServiceStub struct {
	setRates []float64

	replaceInput service.ReplaceSubAdminCommissionGrantsInput

	calendarSubAdminID int64
	calendarMonth      string

	logsErr error
}

func (s *subAdminCommissionHandlerServiceStub) GetSettings(ctx context.Context) (float64, error) {
	return 0.08, nil
}

func (s *subAdminCommissionHandlerServiceStub) SetSettings(ctx context.Context, rate float64) (float64, error) {
	s.setRates = append(s.setRates, rate)
	return rate, nil
}

func (s *subAdminCommissionHandlerServiceStub) ListAllGrants(ctx context.Context) ([]service.SubAdminCommissionGrant, error) {
	return []service.SubAdminCommissionGrant{}, nil
}

func (s *subAdminCommissionHandlerServiceStub) ReplaceGrants(ctx context.Context, input service.ReplaceSubAdminCommissionGrantsInput) ([]service.SubAdminCommissionGrant, error) {
	s.replaceInput = input
	return []service.SubAdminCommissionGrant{}, nil
}

func (s *subAdminCommissionHandlerServiceStub) ListWorkbenchGrants(ctx context.Context, subAdminID int64) ([]service.SubAdminCommissionGrant, error) {
	return []service.SubAdminCommissionGrant{}, nil
}

func (s *subAdminCommissionHandlerServiceStub) ListCalendar(ctx context.Context, subAdminID int64, month string, now time.Time) ([]service.SubAdminCommissionCalendarDay, error) {
	s.calendarSubAdminID = subAdminID
	s.calendarMonth = month
	return []service.SubAdminCommissionCalendarDay{}, nil
}

func (s *subAdminCommissionHandlerServiceStub) ListDayGroups(ctx context.Context, subAdminID int64, date string) ([]service.SubAdminCommissionDayGroup, error) {
	return []service.SubAdminCommissionDayGroup{}, nil
}

func (s *subAdminCommissionHandlerServiceStub) ListDayGroupLogs(ctx context.Context, subAdminID, groupID int64, date string, params pagination.PaginationParams) ([]service.SubAdminCommissionUsageLog, pagination.PaginationResult, error) {
	if s.logsErr != nil {
		return nil, pagination.PaginationResult{}, s.logsErr
	}
	return []service.SubAdminCommissionUsageLog{}, pagination.PaginationResult{Page: params.Page, PageSize: params.PageSize}, nil
}

func setupSubAdminCommissionHandlerRouter(stub *subAdminCommissionHandlerServiceStub, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if userID > 0 {
		router.Use(func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
			c.Next()
		})
	}

	handler := &SubAdminCommissionHandler{service: stub}
	router.PUT("/settings", handler.UpdateSettings)
	router.PUT("/grants", handler.ReplaceGrants)
	router.PUT("/grants/:sub_admin_id", handler.ReplaceGrants)
	router.GET("/workbench/calendar", handler.GetWorkbenchCalendar)
	router.GET("/workbench/days/:date/groups/:group_id/logs", handler.GetWorkbenchDayGroupLogs)
	return router
}

func TestSubAdminCommissionHandlerUpdateSettingsRejectsInvalidRate(t *testing.T) {
	for _, body := range []string{`{"commission_rate":-0.01}`, `{"commission_rate":1.01}`} {
		stub := &subAdminCommissionHandlerServiceStub{}
		router := setupSubAdminCommissionHandlerRouter(stub, 7)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/settings", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Empty(t, stub.setRates)
	}
}

func TestSubAdminCommissionHandlerReplaceGrantsUsesBodyAndOperator(t *testing.T) {
	stub := &subAdminCommissionHandlerServiceStub{}
	router := setupSubAdminCommissionHandlerRouter(stub, 99)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/grants", bytes.NewBufferString(`{"group_ids":[3,4]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Zero(t, stub.replaceInput.SubAdminID)
	require.Equal(t, []int64{3, 4}, stub.replaceInput.GroupIDs)
	require.Equal(t, int64(99), stub.replaceInput.OperatorID)
}

func TestSubAdminCommissionHandlerWorkbenchCalendarUsesAuthSubject(t *testing.T) {
	stub := &subAdminCommissionHandlerServiceStub{}
	router := setupSubAdminCommissionHandlerRouter(stub, 88)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/workbench/calendar?month=2026-08", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(88), stub.calendarSubAdminID)
	require.Equal(t, "2026-08", stub.calendarMonth)
}

func TestSubAdminCommissionHandlerWorkbenchLogsForbidden(t *testing.T) {
	stub := &subAdminCommissionHandlerServiceStub{logsErr: service.ErrSubAdminCommissionForbidden}
	router := setupSubAdminCommissionHandlerRouter(stub, 88)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/workbench/days/2026-08-22/groups/9/logs", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}
