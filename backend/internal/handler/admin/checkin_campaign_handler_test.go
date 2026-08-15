//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type checkinCampaignAdminServiceStub struct {
	listFn    func(context.Context, string) ([]service.CheckinRewardCampaign, error)
	getFn     func(context.Context, int64) (*service.CheckinRewardCampaign, error)
	createFn  func(context.Context, service.CreateCheckinRewardCampaignInput) (*service.CheckinRewardCampaign, error)
	updateFn  func(context.Context, int64, service.UpdateCheckinRewardCampaignInput) (*service.CheckinRewardCampaign, error)
	enableFn  func(context.Context, int64, int64) (*service.CheckinRewardCampaign, error)
	disableFn func(context.Context, int64, int64) (*service.CheckinRewardCampaign, error)
	copyFn    func(context.Context, int64, string, int64) (*service.CheckinRewardCampaign, error)
	deleteFn  func(context.Context, int64) error
}

func (s *checkinCampaignAdminServiceStub) ListRewardCampaigns(ctx context.Context, lifecycle string) ([]service.CheckinRewardCampaign, error) {
	return s.listFn(ctx, lifecycle)
}

func (s *checkinCampaignAdminServiceStub) GetRewardCampaign(ctx context.Context, id int64) (*service.CheckinRewardCampaign, error) {
	return s.getFn(ctx, id)
}

func (s *checkinCampaignAdminServiceStub) CreateRewardCampaign(ctx context.Context, input service.CreateCheckinRewardCampaignInput) (*service.CheckinRewardCampaign, error) {
	return s.createFn(ctx, input)
}

func (s *checkinCampaignAdminServiceStub) UpdateRewardCampaign(ctx context.Context, id int64, input service.UpdateCheckinRewardCampaignInput) (*service.CheckinRewardCampaign, error) {
	return s.updateFn(ctx, id, input)
}

func (s *checkinCampaignAdminServiceStub) EnableRewardCampaign(ctx context.Context, id, adminID int64) (*service.CheckinRewardCampaign, error) {
	return s.enableFn(ctx, id, adminID)
}

func (s *checkinCampaignAdminServiceStub) DisableRewardCampaign(ctx context.Context, id, adminID int64) (*service.CheckinRewardCampaign, error) {
	return s.disableFn(ctx, id, adminID)
}

func (s *checkinCampaignAdminServiceStub) CopyRewardCampaign(ctx context.Context, id int64, name string, adminID int64) (*service.CheckinRewardCampaign, error) {
	return s.copyFn(ctx, id, name, adminID)
}

func (s *checkinCampaignAdminServiceStub) DeleteRewardCampaign(ctx context.Context, id int64) error {
	return s.deleteFn(ctx, id)
}

func completeCheckinCampaignAdminServiceStub() *checkinCampaignAdminServiceStub {
	campaign := &service.CheckinRewardCampaign{
		ID:              42,
		Name:            "Summer bonus",
		Status:          "draft",
		LifecycleStatus: "draft",
		StartDate:       "2026-08-16",
		EndDate:         "2026-08-18",
		RewardTiers: []service.CheckinRewardTier{
			{Amount: 1, Probability: 100, SortOrder: 1},
		},
		ProbabilityTotal: 100,
	}
	return &checkinCampaignAdminServiceStub{
		listFn: func(context.Context, string) ([]service.CheckinRewardCampaign, error) {
			return []service.CheckinRewardCampaign{*campaign}, nil
		},
		getFn: func(context.Context, int64) (*service.CheckinRewardCampaign, error) { return campaign, nil },
		createFn: func(context.Context, service.CreateCheckinRewardCampaignInput) (*service.CheckinRewardCampaign, error) {
			return campaign, nil
		},
		updateFn: func(context.Context, int64, service.UpdateCheckinRewardCampaignInput) (*service.CheckinRewardCampaign, error) {
			return campaign, nil
		},
		enableFn:  func(context.Context, int64, int64) (*service.CheckinRewardCampaign, error) { return campaign, nil },
		disableFn: func(context.Context, int64, int64) (*service.CheckinRewardCampaign, error) { return campaign, nil },
		copyFn: func(context.Context, int64, string, int64) (*service.CheckinRewardCampaign, error) {
			return campaign, nil
		},
		deleteFn: func(context.Context, int64) error { return nil },
	}
}

func newCheckinCampaignHandlerTestRouter(svc checkinCampaignAdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 77})
		c.Next()
	})
	h := newCheckinHandlerForCampaignService(svc)
	router.GET("/", h.ListCampaigns)
	router.POST("/", h.CreateCampaign)
	router.GET("/:id", h.GetCampaign)
	router.PUT("/:id", h.UpdateCampaign)
	router.POST("/:id/enable", h.EnableCampaign)
	router.POST("/:id/disable", h.DisableCampaign)
	router.POST("/:id/copy", h.CopyCampaign)
	router.DELETE("/:id", h.DeleteCampaign)
	return router
}

func performCheckinCampaignRequest(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeCheckinCampaignEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) struct {
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

func TestCheckinCampaignHandlerEightEndpointsSuccessAndActor(t *testing.T) {
	svc := completeCheckinCampaignAdminServiceStub()
	var gotLifecycle string
	var gotIDs []int64
	var gotAdminIDs []int64
	var gotCreate service.CreateCheckinRewardCampaignInput
	var gotUpdate service.UpdateCheckinRewardCampaignInput
	var gotCopyName string
	svc.listFn = func(_ context.Context, lifecycle string) ([]service.CheckinRewardCampaign, error) {
		gotLifecycle = lifecycle
		return []service.CheckinRewardCampaign{}, nil
	}
	svc.getFn = func(_ context.Context, id int64) (*service.CheckinRewardCampaign, error) {
		gotIDs = append(gotIDs, id)
		return &service.CheckinRewardCampaign{ID: id}, nil
	}
	svc.createFn = func(_ context.Context, input service.CreateCheckinRewardCampaignInput) (*service.CheckinRewardCampaign, error) {
		gotCreate = input
		gotAdminIDs = append(gotAdminIDs, input.AdminID)
		return &service.CheckinRewardCampaign{ID: 42}, nil
	}
	svc.updateFn = func(_ context.Context, id int64, input service.UpdateCheckinRewardCampaignInput) (*service.CheckinRewardCampaign, error) {
		gotIDs = append(gotIDs, id)
		gotUpdate = input
		gotAdminIDs = append(gotAdminIDs, input.AdminID)
		return &service.CheckinRewardCampaign{ID: id}, nil
	}
	svc.enableFn = func(_ context.Context, id, adminID int64) (*service.CheckinRewardCampaign, error) {
		gotIDs = append(gotIDs, id)
		gotAdminIDs = append(gotAdminIDs, adminID)
		return &service.CheckinRewardCampaign{ID: id}, nil
	}
	svc.disableFn = func(_ context.Context, id, adminID int64) (*service.CheckinRewardCampaign, error) {
		gotIDs = append(gotIDs, id)
		gotAdminIDs = append(gotAdminIDs, adminID)
		return &service.CheckinRewardCampaign{ID: id}, nil
	}
	svc.copyFn = func(_ context.Context, id int64, name string, adminID int64) (*service.CheckinRewardCampaign, error) {
		gotIDs = append(gotIDs, id)
		gotCopyName = name
		gotAdminIDs = append(gotAdminIDs, adminID)
		return &service.CheckinRewardCampaign{ID: 43}, nil
	}
	svc.deleteFn = func(_ context.Context, id int64) error {
		gotIDs = append(gotIDs, id)
		return nil
	}
	router := newCheckinCampaignHandlerTestRouter(svc)
	upsertBody := `{"name":"Summer bonus","start_date":"2026-08-16","end_date":"2026-08-18","reward_tiers":[{"amount":1,"probability":100,"sort_order":1}]}`

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantData   string
	}{
		{name: "list", method: http.MethodGet, path: "/?lifecycle=active", wantStatus: http.StatusOK, wantData: `[]`},
		{name: "create", method: http.MethodPost, path: "/", body: upsertBody, wantStatus: http.StatusCreated},
		{name: "get", method: http.MethodGet, path: "/42", wantStatus: http.StatusOK},
		{name: "update", method: http.MethodPut, path: "/42", body: upsertBody, wantStatus: http.StatusOK},
		{name: "enable", method: http.MethodPost, path: "/42/enable", wantStatus: http.StatusOK},
		{name: "disable", method: http.MethodPost, path: "/42/disable", wantStatus: http.StatusOK},
		{name: "copy", method: http.MethodPost, path: "/42/copy", body: `{"name":"Summer bonus copy"}`, wantStatus: http.StatusCreated},
		{name: "delete", method: http.MethodDelete, path: "/42", wantStatus: http.StatusOK, wantData: `{"id":42,"deleted":true}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := performCheckinCampaignRequest(t, router, tt.method, tt.path, tt.body)
			require.Equal(t, tt.wantStatus, recorder.Code)
			envelope := decodeCheckinCampaignEnvelope(t, recorder)
			require.Zero(t, envelope.Code)
			require.Equal(t, "success", envelope.Message)
			if tt.wantData != "" {
				require.JSONEq(t, tt.wantData, string(envelope.Data))
			}
		})
	}

	require.Equal(t, "active", gotLifecycle)
	require.Equal(t, []int64{42, 42, 42, 42, 42, 42}, gotIDs)
	require.Equal(t, []int64{77, 77, 77, 77, 77}, gotAdminIDs)
	require.Equal(t, "Summer bonus", gotCreate.Name)
	require.Equal(t, "2026-08-16", gotUpdate.StartDate)
	require.Equal(t, []service.CheckinRewardTier{{Amount: 1, Probability: 100, SortOrder: 1}}, gotCreate.RewardTiers)
	require.Equal(t, "Summer bonus copy", gotCopyName)
}

func TestCheckinCampaignHandlerRejectsInvalidIDsAndJSON(t *testing.T) {
	router := newCheckinCampaignHandlerTestRouter(completeCheckinCampaignAdminServiceStub())
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		reason string
	}{
		{name: "invalid id", method: http.MethodGet, path: "/not-an-id", reason: "CHECKIN_REWARD_CAMPAIGN_INVALID_ID"},
		{name: "zero id", method: http.MethodDelete, path: "/0", reason: "CHECKIN_REWARD_CAMPAIGN_INVALID_ID"},
		{name: "malformed upsert", method: http.MethodPost, path: "/", body: `{"name":`, reason: "CHECKIN_REWARD_CAMPAIGN_INVALID_REQUEST"},
		{name: "malformed copy", method: http.MethodPost, path: "/42/copy", body: `{"name":`, reason: "CHECKIN_REWARD_CAMPAIGN_INVALID_REQUEST"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := performCheckinCampaignRequest(t, router, tt.method, tt.path, tt.body)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
			envelope := decodeCheckinCampaignEnvelope(t, recorder)
			require.Equal(t, tt.reason, envelope.Reason)
		})
	}
}

func TestCheckinCampaignHandlerMapsValidationConflictAndTransitionErrors(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantStatus   int
		wantReason   string
		wantMetadata map[string]string
	}{
		{
			name:       "invalid date",
			err:        service.ErrCheckinRewardCampaignInvalidDate.WithMetadata(map[string]string{"field": "start_date", "value": "2026-13-40"}),
			wantStatus: http.StatusBadRequest,
			wantReason: "CHECKIN_REWARD_CAMPAIGN_INVALID_DATE",
			wantMetadata: map[string]string{
				"field": "start_date", "value": "2026-13-40",
			},
		},
		{
			name:       "invalid probability",
			err:        infraerrors.BadRequest("CHECKIN_REWARD_CONFIG_INVALID_TOTAL", "reward probabilities must add up to exactly 100"),
			wantStatus: http.StatusBadRequest,
			wantReason: "CHECKIN_REWARD_CONFIG_INVALID_TOTAL",
		},
		{
			name:       "overlap",
			err:        service.ErrCheckinRewardCampaignOverlap.WithMetadata(map[string]string{"conflict_campaign_id": "9", "conflict_campaign_name": "Existing"}),
			wantStatus: http.StatusConflict,
			wantReason: "CHECKIN_REWARD_CAMPAIGN_OVERLAP",
			wantMetadata: map[string]string{
				"conflict_campaign_id": "9", "conflict_campaign_name": "Existing",
			},
		},
		{
			name:         "invalid transition",
			err:          service.ErrCheckinRewardCampaignInvalidStateTransition.WithMetadata(map[string]string{"campaign_id": "42", "lifecycle_status": "active"}),
			wantStatus:   http.StatusConflict,
			wantReason:   "CHECKIN_REWARD_CAMPAIGN_INVALID_STATE_TRANSITION",
			wantMetadata: map[string]string{"campaign_id": "42", "lifecycle_status": "active"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := completeCheckinCampaignAdminServiceStub()
			svc.createFn = func(context.Context, service.CreateCheckinRewardCampaignInput) (*service.CheckinRewardCampaign, error) {
				return nil, tt.err
			}
			recorder := performCheckinCampaignRequest(t, newCheckinCampaignHandlerTestRouter(svc), http.MethodPost, "/", `{"name":"Campaign","start_date":"2026-08-16","end_date":"2026-08-18","reward_tiers":[{"amount":1,"probability":100}]}`)
			require.Equal(t, tt.wantStatus, recorder.Code)
			envelope := decodeCheckinCampaignEnvelope(t, recorder)
			require.Equal(t, tt.wantReason, envelope.Reason)
			require.Equal(t, tt.wantMetadata, envelope.Metadata)
		})
	}
}

func TestCheckinCampaignHandlerUnexpectedErrorIsRedacted(t *testing.T) {
	svc := completeCheckinCampaignAdminServiceStub()
	svc.enableFn = func(context.Context, int64, int64) (*service.CheckinRewardCampaign, error) {
		return nil, errors.New("pq: password=secret internal campaign row")
	}

	recorder := performCheckinCampaignRequest(t, newCheckinCampaignHandlerTestRouter(svc), http.MethodPost, "/42/enable", "")

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	envelope := decodeCheckinCampaignEnvelope(t, recorder)
	require.Equal(t, "internal error", envelope.Message)
	require.Empty(t, envelope.Reason)
	require.NotContains(t, recorder.Body.String(), "secret")
}
