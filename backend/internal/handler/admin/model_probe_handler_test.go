package admin

import (
	"context"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

type modelProbeHandlerStub struct {
	calls int
	err   error
}

func (s *modelProbeHandlerStub) Groups(context.Context, int64) ([]service.ModelProbeGroup, error) {
	s.calls++
	return []service.ModelProbeGroup{}, s.err
}
func (s *modelProbeHandlerStub) Models(context.Context, int64, int64) (*service.ModelProbeCatalog, error) {
	s.calls++
	return &service.ModelProbeCatalog{}, s.err
}
func (s *modelProbeHandlerStub) Run(context.Context, int64, int64, string) (*service.ModelProbeResult, error) {
	s.calls++
	return &service.ModelProbeResult{Status: "success"}, s.err
}
func probeHandlerRequest(h *ModelProbeHandler, role, path, body string, authenticated bool) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", path, strings.NewReader(body))
	c.Params = gin.Params{{Key: "id", Value: "7"}}
	if authenticated {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 4})
		c.Set(string(middleware.ContextKeyUserRole), role)
	}
	h.Run(c)
	return w
}
func TestModelProbeHandlerRejectsNormalUserAndMissingSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &modelProbeHandlerStub{}
	h := &ModelProbeHandler{service: s}
	require.Equal(t, 401, probeHandlerRequest(h, "admin", "/", `{"group_id":7,"model":"public-model"}`, false).Code)
	require.Equal(t, 403, probeHandlerRequest(h, "user", "/", `{"group_id":7,"model":"public-model"}`, true).Code)
	require.Zero(t, s.calls)
}
func TestModelProbeHandlerStrictInputNeverAcceptsAccountOrURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{`{"group_id":7,"model":"public-model","account_id":99}`, `{"group_id":7,"model":"public-model","url":"https://secret.invalid"}`, `{"group_id":7,"model":"public-model"}{}`, strings.Repeat("x", 3000)} {
		s := &modelProbeHandlerStub{}
		w := probeHandlerRequest(&ModelProbeHandler{service: s}, "sub_admin", "/", body, true)
		require.Equal(t, 400, w.Code)
		require.Zero(t, s.calls)
	}
	s := &modelProbeHandlerStub{}
	require.Equal(t, 400, probeHandlerRequest(&ModelProbeHandler{service: s}, "admin", "/?account_id=99", `{"group_id":7,"model":"public-model"}`, true).Code)
	require.Zero(t, s.calls)
}
func TestModelProbeHandlerRechecksCapabilityAndHidesInternalErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code int
	}{{service.ErrUserSupportForbidden, 403}, {service.ErrModelProbeInvalid, 400}, {errors.New("https://secret.invalid key sk-private"), 503}, {nil, 200}} {
		s := &modelProbeHandlerStub{err: tc.err}
		w := probeHandlerRequest(&ModelProbeHandler{service: s}, "sub_admin", "/", `{"group_id":7,"model":"public-model"}`, true)
		require.Equal(t, tc.code, w.Code)
		require.NotContains(t, w.Body.String(), "secret.invalid")
		require.NotContains(t, w.Body.String(), "sk-private")
		require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
		require.Equal(t, 1, s.calls)
	}
}
