package admin

import (
	"context"
	"errors"
	"fmt"
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
func (s *modelProbeHandlerStub) ConfigGroups(context.Context, int64) ([]service.ModelProbeConfigGroup, error) {
	s.calls++
	return []service.ModelProbeConfigGroup{}, s.err
}
func (s *modelProbeHandlerStub) Config(context.Context, int64, int64) (*service.ModelProbeConfig, error) {
	s.calls++
	return &service.ModelProbeConfig{Enabled: true}, s.err
}
func (s *modelProbeHandlerStub) SaveConfig(_ context.Context, _ int64, id int64, enabled bool) (*service.ModelProbeConfigResult, error) {
	s.calls++
	return &service.ModelProbeConfigResult{GroupID: id, Enabled: enabled}, s.err
}
func (s *modelProbeHandlerStub) SaveConfigs(_ context.Context, _ int64, changes []service.ModelProbeConfigChange) ([]service.ModelProbeConfigResult, error) {
	s.calls++
	return changes, s.err
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

func probeConfigHandlerRequest(h *ModelProbeHandler, role, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Params = gin.Params{{Key: "id", Value: "7"}}
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 4})
	c.Set(string(middleware.ContextKeyUserRole), role)
	if method == "PUT" {
		if strings.Split(path, "?")[0] == "/config/groups" {
			h.SaveConfigs(c)
		} else {
			h.SaveConfig(c)
		}
	} else if path == "/config/groups" {
		h.ConfigGroups(c)
	} else {
		h.Config(c)
	}
	return w
}
func TestModelProbeConfigHandlerRequiresRootRole(t *testing.T) {
	for _, role := range []string{"user", "sub_admin"} {
		for _, method := range []string{"GET", "PUT"} {
			s := &modelProbeHandlerStub{}
			w := probeConfigHandlerRequest(&ModelProbeHandler{service: s}, role, method, "/config/groups/7", `{"enabled":false}`)
			require.Equal(t, 403, w.Code)
			require.Zero(t, s.calls)
		}
	}
}
func TestModelProbeConfigHandlerStrictBoundedBody(t *testing.T) {
	for _, body := range []string{
		`{}`, `null`, `{"enabled":null}`, `{"enabled":false,"account_id":3}`, `{"enabled":false,"enabled":true}`,
		`{"enabled":false} {}`, `{"enabled":3}`, `{"enabled":false}` + strings.Repeat(" ", 65536), `{"models":[]}`,
	} {
		s := &modelProbeHandlerStub{}
		w := probeConfigHandlerRequest(&ModelProbeHandler{service: s}, "admin", "PUT", "/config/groups/7", body)
		require.Equal(t, 400, w.Code, "body %s", body[:min(len(body), 80)])
		require.Zero(t, s.calls)
	}
	s := &modelProbeHandlerStub{}
	w := probeConfigHandlerRequest(&ModelProbeHandler{service: s}, "admin", "PUT", "/config/groups/7?account_id=3", `{"enabled":false}`)
	require.Equal(t, 400, w.Code)
	require.Zero(t, s.calls)
	w = probeConfigHandlerRequest(&ModelProbeHandler{service: s}, "admin", "PUT", "/config/groups/7", `{"enabled":false}`)
	require.Equal(t, 200, w.Code)
	require.Equal(t, 1, s.calls)
	require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
}
func TestModelProbeConfigHandlerMapsFreshAuthorizationAndSafeErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{service.ErrUserSupportForbidden, 403}, {service.ErrModelProbeInvalid, 400}, {errors.New("https://secret.invalid sk-private"), 503}, {nil, 200}} {
		s := &modelProbeHandlerStub{err: tc.err}
		for _, method := range []string{"GET", "PUT"} {
			w := probeConfigHandlerRequest(&ModelProbeHandler{service: s}, "admin", method, "/config/groups/7", `{"enabled":false}`)
			require.Equal(t, tc.status, w.Code)
			require.NotContains(t, w.Body.String(), "secret.invalid")
			require.NotContains(t, w.Body.String(), "sk-private")
		}
	}
}

func TestModelProbeGroupSwitchBulkHandlerStrictBoundedChanges(t *testing.T) {
	for _, body := range []string{
		`{}`, `{"groups":null}`, `{"groups":[{"group_id":7}]}`, `{"groups":[{"enabled":true}]}`,
		`{"groups":[{"group_id":7,"enabled":null}]}`, `{"groups":[{"group_id":null,"enabled":true}]}`,
		`{"groups":[{"group_id":7,"enabled":true,"models":[]}]}`, `{"groups":[],"models":[]}`,
		`{"groups":[{"group_id":7,"enabled":true},{"group_id":7,"enabled":false}]}`,
		`{"groups":[{"group_id":7,"group_id":8,"enabled":true}]}`, `{"groups":[],"groups":[]}`,
		`{"groups":[]} {}`, `{"groups":[]}` + strings.Repeat(" ", 65536),
	} {
		s := &modelProbeHandlerStub{}
		w := probeConfigHandlerRequest(&ModelProbeHandler{service: s}, "admin", "PUT", "/config/groups", body)
		require.Equal(t, 400, w.Code, "body %s", body[:min(len(body), 100)])
		require.Zero(t, s.calls)
	}
	items := make([]string, 501)
	for i := range items {
		items[i] = fmt.Sprintf(`{"group_id":%d,"enabled":true}`, i+1)
	}
	for _, tc := range []struct {
		body   string
		status int
	}{{`{"groups":[]}`, 200}, {`{"groups":[` + strings.Join(items[:500], ",") + `]}`, 200}, {`{"groups":[` + strings.Join(items, ",") + `]}`, 400}} {
		s := &modelProbeHandlerStub{}
		w := probeConfigHandlerRequest(&ModelProbeHandler{service: s}, "admin", "PUT", "/config/groups", tc.body)
		require.Equal(t, tc.status, w.Code)
		if tc.status == 400 {
			require.Zero(t, s.calls)
		} else {
			require.Equal(t, 1, s.calls)
		}
	}
	for _, role := range []string{"user", "sub_admin"} {
		s := &modelProbeHandlerStub{}
		w := probeConfigHandlerRequest(&ModelProbeHandler{service: s}, role, "PUT", "/config/groups", `{"groups":[]}`)
		require.Equal(t, 403, w.Code)
		require.Zero(t, s.calls)
	}
}

func TestModelProbeGroupSwitchHandlerAcceptsOnlyEnabled(t *testing.T) {
	s := &modelProbeHandlerStub{}
	w := probeConfigHandlerRequest(&ModelProbeHandler{service: s}, "admin", "PUT", "/config/groups/7", `{"enabled":true}`)
	require.Equal(t, 200, w.Code)
	require.Equal(t, 1, s.calls)
	s = &modelProbeHandlerStub{}
	w = probeConfigHandlerRequest(&ModelProbeHandler{service: s}, "admin", "PUT", "/config/groups/7", `{"models":[]}`)
	require.Equal(t, 400, w.Code)
	require.Zero(t, s.calls)
}
