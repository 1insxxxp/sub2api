//go:build unit

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type deleteGroupAdminServiceStub struct {
	service.AdminService
	err error
}

func (s deleteGroupAdminServiceStub) DeleteGroup(context.Context, int64) error { return s.err }

func TestDeleteGroupHandlerReturnsCustomGroupSourceConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewGroupHandler(deleteGroupAdminServiceStub{
		err: service.ErrGroupCustomGroupSourceInUse.WithMetadata(map[string]string{
			"reference_count": "7",
		}),
	}, nil, nil)
	router.DELETE("/api/v1/admin/groups/:id", handler.Delete)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/groups/42", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.JSONEq(t, `{
		"code": 409,
		"message": "group is referenced by one or more custom group models",
		"reason": "CUSTOM_GROUP_SOURCE_IN_USE",
		"metadata": {"reference_count": "7"}
	}`, recorder.Body.String())
	require.Equal(t, http.StatusConflict, infraerrors.Code(service.ErrGroupCustomGroupSourceInUse))
}
