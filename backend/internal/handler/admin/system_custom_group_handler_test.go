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
	"time"

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

func TestSystemCustomGroupHandlerCreateAndUpdateDecodeOrderedSourceGroupIDs(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path:   "/",
			body:   `{"name":"Monthly","default_validity_days":30,"source_group_ids":[9,7,12]}`,
		},
		{
			name:   "update",
			method: http.MethodPut,
			path:   "/42",
			body:   `{"name":"Monthly","default_validity_days":30,"source_group_ids":[9,7,12]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := completeSystemCustomGroupAdminServiceStub()
			var got []int64
			svc.createFn = func(_ context.Context, req service.CreateSystemCustomGroupRequest) (*service.SystemCustomGroup, error) {
				got = append([]int64(nil), req.SourceGroupIDs...)
				return &service.SystemCustomGroup{}, nil
			}
			svc.updateFn = func(_ context.Context, _ int64, req service.UpdateSystemCustomGroupRequest) (*service.SystemCustomGroup, error) {
				got = append([]int64(nil), req.SourceGroupIDs...)
				return &service.SystemCustomGroup{}, nil
			}

			recorder := performSystemCustomGroupRequest(t, newSystemCustomGroupHandlerTestRouter(svc), tt.method, tt.path, tt.body)

			require.Contains(t, []int{http.StatusOK, http.StatusCreated}, recorder.Code)
			require.Equal(t, []int64{9, 7, 12}, got)
		})
	}
}

func TestSystemCustomGroupHandlerKeepsLegacyModelsRequestCompatibility(t *testing.T) {
	svc := completeSystemCustomGroupAdminServiceStub()
	var got []service.SystemCustomGroupModelInput
	svc.createFn = func(_ context.Context, req service.CreateSystemCustomGroupRequest) (*service.SystemCustomGroup, error) {
		got = append([]service.SystemCustomGroupModelInput(nil), req.Models...)
		return &service.SystemCustomGroup{}, nil
	}
	body := `{"name":"Monthly","default_validity_days":30,"models":[{"public_model":"sonnet","source_group_id":7,"source_model":"claude-sonnet","enabled":true}]}`

	recorder := performSystemCustomGroupRequest(t, newSystemCustomGroupHandlerTestRouter(svc), http.MethodPost, "/", body)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Equal(t, []service.SystemCustomGroupModelInput{{
		PublicModel: "sonnet", SourceGroupID: 7, SourceModel: "claude-sonnet", Enabled: true,
	}}, got)
}

func TestSystemCustomGroupHandlerCreateGetUpdateExposeSourceBasedAggregate(t *testing.T) {
	createdAt := time.Date(2026, time.August, 31, 9, 30, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	aggregate := &service.SystemCustomGroup{
		Group: service.Group{ID: 42, Name: "Tavern Monthly", SystemCustomRoutingEnabled: true},
		Sources: []service.SystemCustomGroupSource{
			{
				ID: 201, GroupID: 42, SourceGroupID: 9, Priority: 0, CreatedAt: createdAt, UpdatedAt: updatedAt,
				SourceGroup: &service.Group{ID: 9, Name: "Claude Primary", Description: "primary", Platform: service.PlatformAnthropic, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard},
			},
			{ID: 202, GroupID: 42, SourceGroupID: 7, Priority: 1, CreatedAt: createdAt, UpdatedAt: updatedAt},
			{
				ID: 203, GroupID: 42, SourceGroupID: 12, Priority: 2, CreatedAt: createdAt, UpdatedAt: updatedAt,
				SourceGroup: &service.Group{ID: 12, Name: "Inactive", Platform: service.PlatformOpenAI, Status: "inactive", SubscriptionType: service.SubscriptionTypeStandard},
			},
			{
				ID: 204, GroupID: 42, SourceGroupID: 15, Priority: 3, CreatedAt: createdAt, UpdatedAt: updatedAt,
				SourceGroup: &service.Group{ID: 15, Name: "Unsupported", Platform: service.PlatformKiro, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard},
			},
		},
		Models: []service.SystemCustomGroupModel{
			{ID: 301, GroupID: 42, PublicModel: "Claude-Sonnet", SourceGroupID: 9, SourceModel: "claude-sonnet", Enabled: true},
			{ID: 302, GroupID: 42, PublicModel: " claude-sonnet ", SourceGroupID: 7, SourceModel: "claude-sonnet", Enabled: true},
			{ID: 303, GroupID: 42, PublicModel: "gpt", SourceGroupID: 12, SourceModel: "gpt", Enabled: true},
			{ID: 304, GroupID: 42, PublicModel: "GPT", SourceGroupID: 15, SourceModel: "gpt", Enabled: false},
			{ID: 305, GroupID: 42, PublicModel: "   ", SourceGroupID: 9, SourceModel: "blank", Enabled: true},
		},
	}
	svc := completeSystemCustomGroupAdminServiceStub()
	svc.createFn = func(context.Context, service.CreateSystemCustomGroupRequest) (*service.SystemCustomGroup, error) {
		return aggregate, nil
	}
	svc.getFn = func(context.Context, int64) (*service.SystemCustomGroup, error) { return aggregate, nil }
	svc.updateFn = func(context.Context, int64, service.UpdateSystemCustomGroupRequest) (*service.SystemCustomGroup, error) {
		return aggregate, nil
	}
	router := newSystemCustomGroupHandlerTestRouter(svc)

	for _, tt := range []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{name: "create", method: http.MethodPost, path: "/", body: `{"name":"Monthly","source_group_ids":[9,7,12,15]}`, wantStatus: http.StatusCreated},
		{name: "get", method: http.MethodGet, path: "/42", wantStatus: http.StatusOK},
		{name: "update", method: http.MethodPut, path: "/42", body: `{"name":"Monthly","source_group_ids":[9,7,12,15]}`, wantStatus: http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := performSystemCustomGroupRequest(t, router, tt.method, tt.path, tt.body)
			require.Equal(t, tt.wantStatus, recorder.Code)
			envelope := decodeSystemCustomGroupEnvelope(t, recorder)
			var data struct {
				Group struct {
					ID int64 `json:"id"`
				} `json:"group"`
				Sources []struct {
					ID            int64     `json:"id"`
					GroupID       int64     `json:"group_id"`
					SourceGroupID int64     `json:"source_group_id"`
					Priority      int       `json:"priority"`
					CreatedAt     time.Time `json:"created_at"`
					UpdatedAt     time.Time `json:"updated_at"`
					Group         *struct {
						ID               int64  `json:"id"`
						Name             string `json:"name"`
						Description      string `json:"description"`
						Platform         string `json:"platform"`
						Status           string `json:"status"`
						SubscriptionType string `json:"subscription_type"`
					} `json:"group"`
				} `json:"sources"`
				Summary struct {
					UniqueModels       int `json:"unique_models"`
					FallbackRoutes     int `json:"fallback_routes"`
					UnavailableSources int `json:"unavailable_sources"`
					UnpricedRoutes     int `json:"unpriced_routes"`
				} `json:"summary"`
				Models []service.SystemCustomGroupModel `json:"models"`
			}
			require.NoError(t, json.Unmarshal(envelope.Data, &data))
			require.Equal(t, int64(42), data.Group.ID)
			require.Len(t, data.Sources, 4)
			require.Equal(t, []int{0, 1, 2, 3}, []int{data.Sources[0].Priority, data.Sources[1].Priority, data.Sources[2].Priority, data.Sources[3].Priority})
			require.Equal(t, []int64{9, 7, 12, 15}, []int64{data.Sources[0].SourceGroupID, data.Sources[1].SourceGroupID, data.Sources[2].SourceGroupID, data.Sources[3].SourceGroupID})
			require.Equal(t, int64(201), data.Sources[0].ID)
			require.Equal(t, int64(42), data.Sources[0].GroupID)
			require.Equal(t, createdAt, data.Sources[0].CreatedAt)
			require.Equal(t, updatedAt, data.Sources[0].UpdatedAt)
			require.Equal(t, &struct {
				ID               int64  `json:"id"`
				Name             string `json:"name"`
				Description      string `json:"description"`
				Platform         string `json:"platform"`
				Status           string `json:"status"`
				SubscriptionType string `json:"subscription_type"`
			}{
				ID: 9, Name: "Claude Primary", Description: "primary", Platform: service.PlatformAnthropic,
				Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard,
			}, data.Sources[0].Group)
			require.Nil(t, data.Sources[1].Group)
			require.Equal(t, 2, data.Summary.UniqueModels)
			require.Equal(t, 1, data.Summary.FallbackRoutes)
			require.Equal(t, 3, data.Summary.UnavailableSources)
			require.Zero(t, data.Summary.UnpricedRoutes)
			require.Len(t, data.Models, 5)
			require.Equal(t, int64(301), data.Models[0].ID)
			require.Equal(t, "Claude-Sonnet", data.Models[0].PublicModel)
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
