package admin

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserHandlerCreateSubAdminRoleBindsAndPassesToService(t *testing.T) {
	router, adminSvc := setupAdminRouter()

	rec := doJSON(t, router, http.MethodPost, "/api/v1/admin/users", map[string]any{
		"email":    "sub-admin@example.com",
		"password": "pass123",
		"role":     service.RoleSubAdmin,
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, adminSvc.lastCreateUserInput)
	require.Equal(t, service.RoleSubAdmin, adminSvc.lastCreateUserInput.Role)
}

func TestUserHandlerUpdateSubAdminRoleBindsAndPassesToService(t *testing.T) {
	router, adminSvc := setupAdminRouter()

	rec := doJSON(t, router, http.MethodPut, "/api/v1/admin/users/100", map[string]any{
		"role": service.RoleSubAdmin,
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, adminSvc.lastUpdateUserInput)
	require.Equal(t, service.RoleSubAdmin, adminSvc.lastUpdateUserInput.Role)
}
