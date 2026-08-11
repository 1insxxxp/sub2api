//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type systemCustomGroupAdminServiceStub struct {
	createFn      func(context.Context, service.CreateSystemCustomGroupRequest) (*service.SystemCustomGroup, error)
	getFn         func(context.Context, int64) (*service.SystemCustomGroup, error)
	updateFn      func(context.Context, int64, service.UpdateSystemCustomGroupRequest) (*service.SystemCustomGroup, error)
	candidatesFn  func(context.Context) ([]service.SystemCustomGroupCandidate, error)
	syncPreviewFn func(context.Context, int64) (*service.SystemCustomGroupSyncPreview, error)
	deleteFn      func(context.Context, int64) error
}

func (s *systemCustomGroupAdminServiceStub) Create(ctx context.Context, req service.CreateSystemCustomGroupRequest) (*service.SystemCustomGroup, error) {
	return s.createFn(ctx, req)
}

func (s *systemCustomGroupAdminServiceStub) Get(ctx context.Context, id int64) (*service.SystemCustomGroup, error) {
	return s.getFn(ctx, id)
}

func (s *systemCustomGroupAdminServiceStub) Update(ctx context.Context, id int64, req service.UpdateSystemCustomGroupRequest) (*service.SystemCustomGroup, error) {
	return s.updateFn(ctx, id, req)
}

func (s *systemCustomGroupAdminServiceStub) Candidates(ctx context.Context) ([]service.SystemCustomGroupCandidate, error) {
	return s.candidatesFn(ctx)
}

func (s *systemCustomGroupAdminServiceStub) SyncPreview(ctx context.Context, id int64) (*service.SystemCustomGroupSyncPreview, error) {
	return s.syncPreviewFn(ctx, id)
}

func (s *systemCustomGroupAdminServiceStub) Delete(ctx context.Context, id int64) error {
	return s.deleteFn(ctx, id)
}

func completeSystemCustomGroupAdminServiceStub() *systemCustomGroupAdminServiceStub {
	group := &service.SystemCustomGroup{Group: service.Group{ID: 42, Name: "Tavern Monthly", SystemCustomRoutingEnabled: true}, Models: []service.SystemCustomGroupModel{}}
	return &systemCustomGroupAdminServiceStub{
		createFn: func(context.Context, service.CreateSystemCustomGroupRequest) (*service.SystemCustomGroup, error) {
			return group, nil
		},
		getFn: func(context.Context, int64) (*service.SystemCustomGroup, error) { return group, nil },
		updateFn: func(context.Context, int64, service.UpdateSystemCustomGroupRequest) (*service.SystemCustomGroup, error) {
			return group, nil
		},
		candidatesFn: func(context.Context) ([]service.SystemCustomGroupCandidate, error) {
			return []service.SystemCustomGroupCandidate{{Group: service.Group{ID: 7, Name: "Source"}, Models: []string{"claude-sonnet"}}}, nil
		},
		syncPreviewFn: func(context.Context, int64) (*service.SystemCustomGroupSyncPreview, error) {
			return &service.SystemCustomGroupSyncPreview{
				Added:       []service.SystemCustomGroupSyncAdded{},
				Missing:     []service.SystemCustomGroupModel{},
				Conflicting: []service.SystemCustomGroupSyncConflict{},
			}, nil
		},
		deleteFn: func(context.Context, int64) error { return nil },
	}
}

func newSystemCustomGroupHandlerTestRouter(svc systemCustomGroupAdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := newSystemCustomGroupHandlerForService(svc)
	router.GET("/candidates", h.Candidates)
	router.POST("/", h.Create)
	router.GET("/:id", h.Get)
	router.PUT("/:id", h.Update)
	router.GET("/:id/sync-preview", h.SyncPreview)
	router.DELETE("/:id", h.Delete)
	return router
}

func performSystemCustomGroupRequest(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeSystemCustomGroupEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) struct {
	Code     int               `json:"code"`
	Message  string            `json:"message"`
	Reason   string            `json:"reason"`
	Metadata map[string]string `json:"metadata"`
	Data     json.RawMessage   `json:"data"`
} {
	t.Helper()
	var envelope struct {
		Code     int               `json:"code"`
		Message  string            `json:"message"`
		Reason   string            `json:"reason"`
		Metadata map[string]string `json:"metadata"`
		Data     json.RawMessage   `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	return envelope
}

func TestSystemCustomGroupHandlerSixEndpointsSuccess(t *testing.T) {
	svc := completeSystemCustomGroupAdminServiceStub()
	router := newSystemCustomGroupHandlerTestRouter(svc)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantData   string
	}{
		{name: "candidates", method: http.MethodGet, path: "/candidates", wantStatus: http.StatusOK, wantData: `[{"group":{"id":7,"name":"Source"},"models":["claude-sonnet"]}]`},
		{name: "create", method: http.MethodPost, path: "/", body: `{"name":"Tavern Monthly","default_validity_days":30,"models":[{"public_model":"sonnet","source_group_id":7,"source_model":"claude-sonnet","enabled":true}]}`, wantStatus: http.StatusCreated},
		{name: "get", method: http.MethodGet, path: "/42", wantStatus: http.StatusOK},
		{name: "update", method: http.MethodPut, path: "/42", body: `{"name":"Tavern Monthly","default_validity_days":30,"models":[{"public_model":"sonnet","source_group_id":7,"source_model":"claude-sonnet","enabled":true}]}`, wantStatus: http.StatusOK},
		{name: "sync preview", method: http.MethodGet, path: "/42/sync-preview", wantStatus: http.StatusOK, wantData: `{"added":[],"missing":[],"conflicting":[]}`},
		{name: "delete", method: http.MethodDelete, path: "/42", wantStatus: http.StatusOK, wantData: `{"id":42,"deleted":true}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := performSystemCustomGroupRequest(t, router, tt.method, tt.path, tt.body)
			require.Equal(t, tt.wantStatus, recorder.Code)
			envelope := decodeSystemCustomGroupEnvelope(t, recorder)
			require.Zero(t, envelope.Code)
			require.Equal(t, "success", envelope.Message)
			if tt.wantData != "" {
				require.JSONEq(t, tt.wantData, string(envelope.Data))
			}
		})
	}
}

func TestSystemCustomGroupHandlerMapsStableErrors(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantStatus   int
		wantReason   string
		wantMsg      string
		wantMetadata map[string]string
	}{
		{name: "duplicate public alias", err: &service.SystemCustomGroupRouteError{Kind: service.ErrSystemCustomGroupDuplicatePublicModel, PublicModel: "sonnet", SourceGroupID: 7, SourceModel: "claude-sonnet"}, wantStatus: http.StatusConflict, wantReason: "SYSTEM_CUSTOM_GROUP_DUPLICATE_PUBLIC_MODEL", wantMsg: "system custom group public model already exists", wantMetadata: map[string]string{"public_model": "sonnet", "source_group_id": "7", "source_model": "claude-sonnet"}},
		{name: "duplicate source model", err: &service.SystemCustomGroupRouteError{Kind: service.ErrSystemCustomGroupDuplicateSourceModel, SourceGroupID: 7, SourceModel: "claude-sonnet"}, wantStatus: http.StatusConflict, wantReason: "SYSTEM_CUSTOM_GROUP_DUPLICATE_SOURCE_MODEL", wantMsg: "system custom group source model already exists", wantMetadata: map[string]string{"public_model": "", "source_group_id": "7", "source_model": "claude-sonnet"}},
		{name: "invalid source", err: &service.SystemCustomGroupRouteError{Kind: service.ErrSystemCustomGroupInvalidSourceGroup, PublicModel: "sonnet", SourceGroupID: 7, SourceModel: "claude-sonnet"}, wantStatus: http.StatusBadRequest, wantReason: "SYSTEM_CUSTOM_GROUP_INVALID_SOURCE_GROUP", wantMsg: "system custom group source group is invalid", wantMetadata: map[string]string{"public_model": "sonnet", "source_group_id": "7", "source_model": "claude-sonnet"}},
		{name: "missing source model", err: &service.SystemCustomGroupRouteError{Kind: service.ErrSystemCustomGroupMissingSourceModel, PublicModel: "sonnet", SourceGroupID: 7, SourceModel: "claude-missing"}, wantStatus: http.StatusBadRequest, wantReason: "SYSTEM_CUSTOM_GROUP_MISSING_SOURCE_MODEL", wantMsg: "system custom group source model is unavailable", wantMetadata: map[string]string{"public_model": "sonnet", "source_group_id": "7", "source_model": "claude-missing"}},
		{name: "self reference", err: &service.SystemCustomGroupRouteError{Kind: service.ErrSystemCustomGroupSelfReference, PublicModel: "sonnet", SourceGroupID: 42, SourceModel: "claude-sonnet"}, wantStatus: http.StatusBadRequest, wantReason: "SYSTEM_CUSTOM_GROUP_SELF_REFERENCE", wantMsg: "system custom group cannot route to itself", wantMetadata: map[string]string{"public_model": "sonnet", "source_group_id": "42", "source_model": "claude-sonnet"}},
		{name: "invalid route", err: &service.SystemCustomGroupRouteError{Kind: service.ErrSystemCustomGroupInvalidRoute, PublicModel: "", SourceGroupID: 7, SourceModel: "claude-sonnet"}, wantStatus: http.StatusBadRequest, wantReason: "SYSTEM_CUSTOM_GROUP_INVALID_ROUTE", wantMsg: "system custom group model route is invalid", wantMetadata: map[string]string{"public_model": "", "source_group_id": "7", "source_model": "claude-sonnet"}},
		{name: "invalid input", err: service.ErrSystemCustomGroupInvalidInput, wantStatus: http.StatusBadRequest, wantReason: "SYSTEM_CUSTOM_GROUP_INVALID_INPUT", wantMsg: "system custom group input is invalid"},
		{name: "not found", err: service.ErrSystemCustomGroupNotFound, wantStatus: http.StatusNotFound, wantReason: "SYSTEM_CUSTOM_GROUP_NOT_FOUND", wantMsg: "system custom group not found"},
		{name: "unexpected", err: errors.New("pq: secret sql detail"), wantStatus: http.StatusInternalServerError, wantReason: "", wantMsg: "internal error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := completeSystemCustomGroupAdminServiceStub()
			svc.createFn = func(context.Context, service.CreateSystemCustomGroupRequest) (*service.SystemCustomGroup, error) {
				return nil, tt.err
			}
			recorder := performSystemCustomGroupRequest(t, newSystemCustomGroupHandlerTestRouter(svc), http.MethodPost, "/", `{"name":"Monthly","default_validity_days":30,"models":[{"public_model":"sonnet","source_group_id":7,"source_model":"claude-sonnet","enabled":true}]}`)
			require.Equal(t, tt.wantStatus, recorder.Code)
			envelope := decodeSystemCustomGroupEnvelope(t, recorder)
			require.Equal(t, tt.wantStatus, envelope.Code)
			require.Equal(t, tt.wantReason, envelope.Reason)
			require.Equal(t, tt.wantMsg, envelope.Message)
			require.Equal(t, tt.wantMetadata, envelope.Metadata)
			require.NotContains(t, recorder.Body.String(), "secret sql detail")
		})
	}
}

func TestSystemCustomGroupHandlerRejectsBodyOverLimitEvenWhenExcessIsWhitespace(t *testing.T) {
	router := newSystemCustomGroupHandlerTestRouter(completeSystemCustomGroupAdminServiceStub())
	valid := `{"name":"Monthly","default_validity_days":30,"models":[{"public_model":"sonnet","source_group_id":7,"source_model":"claude-sonnet","enabled":true}]}`
	body := valid + strings.Repeat(" ", (1<<20)-len(valid)+1)

	recorder := performSystemCustomGroupRequest(t, router, http.MethodPost, "/", body)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	envelope := decodeSystemCustomGroupEnvelope(t, recorder)
	require.Equal(t, http.StatusBadRequest, envelope.Code)
	require.Equal(t, "SYSTEM_CUSTOM_GROUP_INVALID_INPUT", envelope.Reason)
}

func TestSystemCustomGroupHandlerRejectsInvalidIDAndBodies(t *testing.T) {
	router := newSystemCustomGroupHandlerTestRouter(completeSystemCustomGroupAdminServiceStub())

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "invalid id", method: http.MethodGet, path: "/not-an-id"},
		{name: "zero id", method: http.MethodDelete, path: "/0"},
		{name: "malformed json", method: http.MethodPost, path: "/", body: `{"name":`},
		{name: "unknown field", method: http.MethodPut, path: "/42", body: `{"name":"Monthly","unknown":true}`},
		{name: "multiple json values", method: http.MethodPost, path: "/", body: `{}` + `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := performSystemCustomGroupRequest(t, router, tc.method, tc.path, tc.body)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
			envelope := decodeSystemCustomGroupEnvelope(t, recorder)
			require.Equal(t, http.StatusBadRequest, envelope.Code)
			require.Equal(t, "SYSTEM_CUSTOM_GROUP_INVALID_INPUT", envelope.Reason)
		})
	}
}

func TestSystemCustomGroupHandlerDeleteInUseReturnsConflict(t *testing.T) {
	svc := completeSystemCustomGroupAdminServiceStub()
	svc.deleteFn = func(context.Context, int64) error {
		return infraerrors.Conflict("SYSTEM_CUSTOM_GROUP_IN_USE", "system custom group is in use")
	}

	recorder := performSystemCustomGroupRequest(t, newSystemCustomGroupHandlerTestRouter(svc), http.MethodDelete, "/42", "")
	require.Equal(t, http.StatusConflict, recorder.Code)
	envelope := decodeSystemCustomGroupEnvelope(t, recorder)
	require.Equal(t, "SYSTEM_CUSTOM_GROUP_IN_USE", envelope.Reason)
}
