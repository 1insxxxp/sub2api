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
	subAdminCommissionService
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

type monthlyReportHandlerServiceStub struct {
	subAdminCommissionHandlerServiceStub
	report service.SubAdminCommissionMonthlyReport
	err    error
	calls  int
}

func (s *monthlyReportHandlerServiceStub) ListMonthlyReport(ctx context.Context, id int64, month string, now time.Time) (service.SubAdminCommissionMonthlyReport, error) {
	s.calls++
	s.calendarSubAdminID = id
	s.calendarMonth = month
	return s.report, s.err
}
func monthlyReportHandlerRouter(stub *monthlyReportHandlerServiceStub, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if userID > 0 {
		router.Use(func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
			c.Next()
		})
	}
	h := &SubAdminCommissionHandler{service: stub}
	router.GET("/monthly-report", h.GetWorkbenchMonthlyReport)
	return router
}
func TestSubAdminCommissionHandlerMonthlyReportAuthValidationAndPrivateResponse(t *testing.T) {
	for _, month := range []string{"2026-13", "2026-8", "invalid", "2026-08-01"} {
		stub := &monthlyReportHandlerServiceStub{}
		rec := httptest.NewRecorder()
		monthlyReportHandlerRouter(stub, 88).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/monthly-report?month="+month, nil))
		require.Equal(t, http.StatusBadRequest, rec.Code, month)
		require.Zero(t, stub.calls)
	}
	stub := &monthlyReportHandlerServiceStub{}
	rec := httptest.NewRecorder()
	monthlyReportHandlerRouter(stub, 0).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/monthly-report", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Zero(t, stub.calls)
	day := service.SubAdminCommissionCalendarDay{Date: "2026-08-23", Enabled: true, ActualCost: 12.5, CommissionAmount: 1.0}
	stub.report = service.SubAdminCommissionMonthlyReport{Days: []service.SubAdminCommissionCalendarDay{day}, Groups: []service.SubAdminCommissionMonthlyGroup{{GroupID: 9, GroupName: "Group", Days: []service.SubAdminCommissionCalendarDay{day}}}}
	rec = httptest.NewRecorder()
	monthlyReportHandlerRouter(stub, 88).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/monthly-report?month=2026-08", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(88), stub.calendarSubAdminID)
	require.Equal(t, "2026-08", stub.calendarMonth)
	require.JSONEq(t, `{"code":0,"message":"success","data":{"days":[{"date":"2026-08-23","enabled":true,"actual_cost":12.5,"commission_amount":1}],"groups":[{"group_id":9,"group_name":"Group","days":[{"date":"2026-08-23","enabled":true,"actual_cost":12.5,"commission_amount":1}]}]}}`, rec.Body.String())
}
func TestSubAdminCommissionHandlerMonthlyReportEmptyArraysAndServiceErrors(t *testing.T) {
	stub := &monthlyReportHandlerServiceStub{}
	router := monthlyReportHandlerRouter(stub, 88)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/monthly-report", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"code":0,"message":"success","data":{"days":[],"groups":[]}}`, rec.Body.String())
	stub.err = service.ErrSubAdminCommissionForbidden
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/monthly-report", nil))
	require.Equal(t, http.StatusForbidden, rec.Code)
}
