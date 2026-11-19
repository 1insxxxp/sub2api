package admin

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type acceptanceHandlerStub struct {
	channelAcceptanceService
	calls int
	err   error
}

func (s *acceptanceHandlerStub) Preview(_ context.Context, _ int64, _ service.AcceptancePreviewRequest) (*service.AcceptancePreview, error) {
	s.calls++
	return &service.AcceptancePreview{}, s.err
}
func acceptanceRequest(t *testing.T, role, body string, restricted bool) (*httptest.ResponseRecorder, *acceptanceHandlerStub) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(r)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))
	if role != "" {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
		c.Set(string(middleware.ContextKeyUserRole), role)
	}
	if restricted {
		c.Request = c.Request.WithContext(adminaccess.WithPrincipal(c.Request.Context(), &adminaccess.Principal{}))
	}
	s := &acceptanceHandlerStub{}
	h := &ChannelAcceptanceHandler{service: s}
	h.Preview(c)
	return r, s
}
func TestAcceptanceHandlerFullAdminOnly(t *testing.T) {
	for _, tc := range []struct {
		role       string
		restricted bool
		want       int
	}{{"", false, 401}, {"user", false, 403}, {"sub_admin", false, 403}, {"admin", true, 403}} {
		r, s := acceptanceRequest(t, tc.role, `{}`, tc.restricted)
		require.Equal(t, tc.want, r.Code)
		require.Zero(t, s.calls)
	}
}
func TestAcceptanceHandlerRejectsUnknownBodyAndHidesErrors(t *testing.T) {
	for _, body := range []string{`{"url":"https://evil.invalid"}`, `{} {}`, strings.Repeat("x", 100000)} {
		r, s := acceptanceRequest(t, "admin", body, false)
		require.Equal(t, 400, r.Code)
		require.Zero(t, s.calls)
	}
	r := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(r)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{}`))
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	c.Set(string(middleware.ContextKeyUserRole), "admin")
	h := &ChannelAcceptanceHandler{service: &acceptanceHandlerStub{err: errors.New("api_key=private-token")}}
	h.Preview(c)
	require.Equal(t, 503, r.Code)
	require.NotContains(t, r.Body.String(), "private-token")
	require.Equal(t, "private, no-store", r.Header().Get("Cache-Control"))
}
