package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type liveSidebandAccountAccessStore struct {
	service.GatewayCache
	service.LiveCallStore
	record *service.LiveCallRecord
}

func (s *liveSidebandAccountAccessStore) GetLiveCall(context.Context, string) (*service.LiveCallRecord, error) {
	return s.record, nil
}

func TestLiveSidebandChecksCurrentAccountAccessBeforeWebSocketUpgrade(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const groupID, userID, accountID = int64(42), int64(7), int64(9901)
	for _, tc := range []struct {
		name       string
		exclusions service.GroupAccountExclusions
		status     int
	}{
		{"newly excluded bound account", service.GroupAccountExclusions{groupID: {accountID}}, http.StatusServiceUnavailable},
		{"same account allowed in this group", service.GroupAccountExclusions{groupID + 1: {accountID}}, http.StatusUpgradeRequired},
		{"other account excluded", service.GroupAccountExclusions{groupID: {accountID + 1}}, http.StatusUpgradeRequired},
		{"user without exclusions", nil, http.StatusUpgradeRequired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowLive: true}
			key := &service.APIKey{ID: 11, UserID: userID, GroupID: &group.ID, Group: group, User: &service.User{ID: userID}}
			store := &liveSidebandAccountAccessStore{record: &service.LiveCallRecord{
				CallID: "existing-live-call", AccountID: accountID, APIKeyID: key.ID, UserID: userID,
				GroupID: groupID, Controller: service.LiveControllerObserver, ExpiresAt: time.Now().Add(time.Hour),
			}}
			gateway := service.NewOpenAIGatewayService(
				nil, nil, nil, nil, nil, nil, store, &config.Config{}, nil, nil, nil,
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			)
			handler := &OpenAIGatewayHandler{gatewayService: gateway}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/live/existing-live-call", nil)
			// A different routing-context group must not bypass the bound call's group.
			requestCtx := context.WithValue(c.Request.Context(), ctxkey.Group, &service.Group{ID: groupID + 1})
			requestCtx = service.WithGroupAccountExclusions(requestCtx, userID, tc.exclusions)
			c.Request = c.Request.WithContext(requestCtx)
			c.Params = gin.Params{{Key: "call_id", Value: store.record.CallID}}
			c.Set(string(middleware.ContextKeyAPIKey), key)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})

			handler.LiveSideband(c)

			// Allowed joins reach the ordinary missing-WebSocket-header response;
			// denied joins must stop before the upgrade, controller claim or dial.
			require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
			if tc.status == http.StatusServiceUnavailable {
				require.Contains(t, recorder.Body.String(), "Live is unavailable")
				require.NotContains(t, recorder.Body.String(), "9901")
				require.NotContains(t, recorder.Body.String(), "excluded")
			}
		})
	}
}
