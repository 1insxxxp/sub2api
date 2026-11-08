package routes

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

type probeRouteGroups struct{}

func (probeRouteGroups) GetByID(context.Context, int64) (*service.Group, error) {
	return &service.Group{ID: 7, Name: "Group", Platform: service.PlatformGemini, Status: service.StatusActive, Hydrated: true}, nil
}
func (probeRouteGroups) ListActive(context.Context) ([]service.Group, error) {
	g, _ := (probeRouteGroups{}).GetByID(context.Background(), 7)
	return []service.Group{*g}, nil
}

func (probeRouteGroups) GetAvailableGroups(ctx context.Context, _ int64) ([]service.Group, error) {
	return (probeRouteGroups{}).ListActive(ctx)
}

type probeRouteGateway struct{ selected int }

func (*probeRouteGateway) ModelsForProbe(context.Context, *service.Group) ([]string, error) {
	return []string{"gemini-2.5-flash"}, nil
}
func (s *probeRouteGateway) ExecuteForProbe(context.Context, *service.Group, string) (service.ModelProbeExecution, error) {
	s.selected++
	return service.ModelProbeExecution{Status: "success"}, nil
}

type probeRouteSettings struct{}

func (probeRouteSettings) GetValue(context.Context, string) (string, error) {
	return `{"version":1,"models":["gemini-2.5-flash"]}`, nil
}
func (probeRouteSettings) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := map[string]string{}
	for _, key := range keys {
		values[key] = `{"version":1,"models":["gemini-2.5-flash"]}`
	}
	return values, nil
}
func (probeRouteSettings) Set(context.Context, string, string) error            { return nil }
func (probeRouteSettings) SetMultiple(context.Context, map[string]string) error { return nil }

type probeRouteUsers struct{ role string }

func (s *probeRouteUsers) GetByID(_ context.Context, id int64) (*service.User, error) {
	return &service.User{ID: id, Role: s.role, Status: service.StatusActive}, nil
}

func TestModelProbeRoutesUseJWTAndFreshSupportCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	access := &supportRouteRepo{enabled: true}
	gateway := &probeRouteGateway{}
	users := &probeRouteUsers{role: service.RoleAdmin}
	svc := service.NewModelProbeService(probeRouteGroups{}, access, gateway, probeRouteGroups{}, probeRouteSettings{}, users)
	h := &handler.Handlers{Admin: &handler.AdminHandlers{ModelProbe: admin.NewModelProbeHandler(svc)}}
	r := gin.New()
	role := service.RoleUser
	jwt := middleware.JWTAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("Authorization") != "Bearer session" {
			c.AbortWithStatus(401)
			return
		}
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 8})
		c.Set(string(middleware.ContextKeyUserRole), role)
		c.Next()
	})
	RegisterModelProbeRoutes(r.Group("/api/v1"), h, jwt, middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), nil, nil)
	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", token)
		req.Header.Set("x-api-key", "admin-fixture")
		r.ServeHTTP(w, req)
		return w
	}
	for _, path := range []string{"/groups", "/groups/7/models", "/config/groups", "/config/groups/7"} {
		require.Equal(t, 401, request("GET", "/api/v1/admin/model-probes"+path, "", "").Code)
		require.Equal(t, 403, request("GET", "/api/v1/admin/model-probes"+path, "Bearer session", "").Code)
	}
	role = service.RoleSubAdmin
	require.Equal(t, 403, request("GET", "/api/v1/admin/model-probes/config/groups", "Bearer session", "").Code)
	require.Equal(t, 403, request("PUT", "/api/v1/admin/model-probes/config/groups/7", "Bearer session", `{"enabled":false}`).Code)
	require.Equal(t, 401, request("PUT", "/api/v1/admin/model-probes/config/groups", "", `{"groups":[]}`).Code)
	require.Equal(t, 403, request("PUT", "/api/v1/admin/model-probes/config/groups", "Bearer session", `{"groups":[]}`).Code)
	require.Equal(t, 200, request("GET", "/api/v1/admin/model-probes/groups?timezone=Asia%2FShanghai", "Bearer session", "").Code)
	require.Equal(t, 200, request("POST", "/api/v1/admin/model-probes/run", "Bearer session", `{"group_id":7,"model":"gemini-2.5-flash"}`).Code)
	require.Equal(t, 1, gateway.selected)
	role = service.RoleAdmin
	require.Equal(t, 200, request("GET", "/api/v1/admin/model-probes/config/groups", "Bearer session", "").Code)
	require.Equal(t, 200, request("GET", "/api/v1/admin/model-probes/config/groups/7", "Bearer session", "").Code)
	require.Equal(t, 200, request("PUT", "/api/v1/admin/model-probes/config/groups/7", "Bearer session", `{"enabled":false}`).Code)
	require.Equal(t, 400, request("PUT", "/api/v1/admin/model-probes/config/groups/7", "Bearer session", `{"models":[]}`).Code)
	require.Equal(t, 200, request("PUT", "/api/v1/admin/model-probes/config/groups", "Bearer session", `{"groups":[{"group_id":7,"enabled":true}]}`).Code)
	users.role = service.RoleSubAdmin // Stale root JWT does not grant configuration.
	require.Equal(t, 403, request("PUT", "/api/v1/admin/model-probes/config/groups/7", "Bearer session", `{"enabled":false}`).Code)
	require.Equal(t, 403, request("PUT", "/api/v1/admin/model-probes/config/groups", "Bearer session", `{"groups":[{"group_id":7,"enabled":true}]}`).Code)
	role = service.RoleSubAdmin
	access.enabled = false
	require.Equal(t, 403, request("GET", "/api/v1/admin/model-probes/groups", "Bearer session", "").Code)
	require.Equal(t, 403, request("POST", "/api/v1/admin/model-probes/run", "Bearer session", `{"group_id":7,"model":"gemini-2.5-flash"}`).Code)
	require.Equal(t, 1, gateway.selected)
}
