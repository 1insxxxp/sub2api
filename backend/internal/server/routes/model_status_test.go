//go:build unit

package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type modelStatusEmptyGroups struct{ service.GroupRepository }

func (modelStatusEmptyGroups) ListActive(context.Context) ([]service.Group, error) {
	return []service.Group{}, nil
}

func (modelStatusEmptyGroups) ListActivePublic(context.Context) ([]service.Group, error) {
	return []service.Group{}, nil
}

func TestModelStatusRouteAllowsVisitors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &handler.Handlers{ModelStatus: handler.NewModelStatusHandler(
		service.NewModelStatusService(nil, modelStatusEmptyGroups{}, nil, nil),
	)}
	RegisterModelStatusRoutes(r.Group("/api/v1"), h, nil)
	for _, authorization := range []string{"", "Bearer expired-token"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/model-status", nil)
		req.Header.Set("Authorization", authorization)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		require.Contains(t, w.Body.String(), `"groups":[]`)
		require.Contains(t, w.Body.String(), `"status":"partial"`)
		require.Contains(t, w.Body.String(), `"terminal_errors_enabled":false`)
	}
}
