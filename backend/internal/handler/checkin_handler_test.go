package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type checkinHandlerServiceStub struct {
	status *service.CheckinStatus
	result *service.CheckinResult
	err    error
}

func (s *checkinHandlerServiceStub) GetStatus(ctx context.Context, userID int64) (*service.CheckinStatus, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := *s.status
	out.UserID = userID
	return &out, nil
}

func (s *checkinHandlerServiceStub) Checkin(ctx context.Context, userID int64) (*service.CheckinResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := *s.result
	out.UserID = userID
	return &out, nil
}

func newCheckinHandlerTestContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	return c, w
}

func decodeCheckinHandlerBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func TestCheckinHandlerGetStatus(t *testing.T) {
	handler := newCheckinHandler(&checkinHandlerServiceStub{
		status: &service.CheckinStatus{
			Enabled:             true,
			CheckinDate:         "2026-06-14",
			CheckedInToday:      false,
			CurrentStreak:       6,
			LifetimeCheckinDays: 20,
		},
	})
	c, w := newCheckinHandlerTestContext(http.MethodGet, "/api/v1/user/checkin/status")

	handler.GetStatus(c)

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeCheckinHandlerBody(t, w)
	require.Equal(t, float64(0), body["code"])
	data := body["data"].(map[string]any)
	require.Equal(t, true, data["enabled"])
	require.Equal(t, false, data["checked_in_today"])
	require.Equal(t, "2026-06-14", data["checkin_date"])
	require.Equal(t, float64(6), data["current_streak"])
}

func TestCheckinHandlerCheckin(t *testing.T) {
	handler := newCheckinHandler(&checkinHandlerServiceStub{
		result: &service.CheckinResult{CheckinStatus: service.CheckinStatus{
			Enabled:             true,
			CheckinDate:         "2026-06-14",
			CheckedInToday:      true,
			CurrentStreak:       1,
			LifetimeCheckinDays: 1,
			BaseRewardAmount:    2,
			TotalRewardAmount:   2,
			BalanceAfter:        12,
		}},
	})
	c, w := newCheckinHandlerTestContext(http.MethodPost, "/api/v1/user/checkin")

	handler.Checkin(c)

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeCheckinHandlerBody(t, w)
	data := body["data"].(map[string]any)
	require.Equal(t, true, data["checked_in_today"])
	require.Equal(t, float64(12), data["balance_after"])
}

func TestCheckinHandlerMapsServiceErrors(t *testing.T) {
	handler := newCheckinHandler(&checkinHandlerServiceStub{err: service.ErrCheckinDisabled})
	c, w := newCheckinHandlerTestContext(http.MethodPost, "/api/v1/user/checkin")

	handler.Checkin(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	body := decodeCheckinHandlerBody(t, w)
	require.Equal(t, "CHECKIN_DISABLED", body["reason"])
}
