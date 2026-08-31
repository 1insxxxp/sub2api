//go:build unit

package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type managedSystemCustomGroupAdminServiceStub struct {
	service.AdminService
}

func (s *managedSystemCustomGroupAdminServiceStub) UpdateGroup(context.Context, int64, *service.UpdateGroupInput) (*service.Group, error) {
	return nil, service.ErrSystemCustomGroupManagedOnly
}

func (s *managedSystemCustomGroupAdminServiceStub) DeleteGroup(context.Context, int64) error {
	return service.ErrSystemCustomGroupManagedOnly
}

func TestOrdinaryGroupHandlersExposeSystemCustomManagedOnlyError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewGroupHandler(&managedSystemCustomGroupAdminServiceStub{}, nil, nil, nil)
	router := gin.New()
	router.PUT("/groups/:id", h.Update)
	router.DELETE("/groups/:id", h.Delete)

	for _, tc := range []struct {
		name   string
		method string
		body   string
	}{
		{name: "update", method: http.MethodPut, body: `{"platform":"anthropic","rate_multiplier":9,"subscription_type":"standard"}`},
		{name: "delete", method: http.MethodDelete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, "/groups/42", bytes.NewBufferString(tc.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusConflict, recorder.Code)
			require.JSONEq(t, `{"code":409,"message":"system custom groups must be managed through the dedicated API","reason":"SYSTEM_CUSTOM_GROUP_MANAGED_ONLY"}`, recorder.Body.String())
		})
	}
}
