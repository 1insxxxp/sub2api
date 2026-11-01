package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type workbenchHandlerAccessStub struct {
	access service.AdminWorkbenchAccess
	err    error
	writes int
}

func (s *workbenchHandlerAccessStub) GetAccess(context.Context, int64) (*service.AdminWorkbenchAccess, error) {
	return &s.access, s.err
}
func (s *workbenchHandlerAccessStub) GetUserAccess(context.Context, int64, int64) (*service.AdminWorkbenchAccess, error) {
	return &s.access, s.err
}
func (s *workbenchHandlerAccessStub) SetUserAccess(_ context.Context, _, _ int64, commission, leaderboard, support bool) error {
	s.writes++
	s.access = service.AdminWorkbenchAccess{Commission: commission, AffiliateLeaderboard: leaderboard, UserSupport: support, BalanceTransfer: true}
	return s.err
}

func TestAdminWorkbenchAccessManagementIsMainAdminOnlyAndStrict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []string{"user", "sub_admin", "admin"} {
		s := &workbenchHandlerAccessStub{}
		h := &AdminWorkbenchAccessHandler{service: s}
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
			c.Set(string(middleware.ContextKeyUserRole), role)
			c.Next()
		})
		r.PUT("/users/:id", h.SetUserAccess)
		request := func(body string) *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("PUT", "/users/2", strings.NewReader(body))
			r.ServeHTTP(w, req)
			return w
		}
		body := `{"commission":false,"affiliate_leaderboard":false,"user_support":true}`
		if role != "admin" {
			require.Equal(t, 403, request(body).Code)
			require.Zero(t, s.writes)
			continue
		}
		for _, invalid := range []string{`{}`, `{"commission":true}`, `{"commission":false,"affiliate_leaderboard":false,"user_support":true,"role":"admin"}`} {
			require.Equal(t, 400, request(invalid).Code)
		}
		w := request(body)
		require.Equal(t, 200, w.Code)
		require.Equal(t, 1, s.writes)
		var envelope struct {
			Data map[string]json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		require.Len(t, envelope.Data, 4)
		s.err = errors.New("http://private-preview.invalid sk-secret 10.20.30.40")
		w = request(body)
		require.Equal(t, 503, w.Code)
		require.NotContains(t, w.Body.String(), "private-preview")
		require.NotContains(t, w.Body.String(), "sk-secret")
	}
}
