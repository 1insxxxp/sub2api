package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type categoryDefaultAdminStub struct {
	statusCalls int
	enabled     bool
	category    string
	apply       service.CategoryDefaultGroupApplyRequest
	err         error
}

func (s *categoryDefaultAdminStub) List(context.Context) ([]service.CategoryDefaultGroupConfig, error) {
	return []service.CategoryDefaultGroupConfig{}, s.err
}
func (s *categoryDefaultAdminStub) Candidates(context.Context, string) ([]service.CategoryDefaultSourceCandidate, error) {
	return []service.CategoryDefaultSourceCandidate{}, s.err
}
func (s *categoryDefaultAdminStub) Preview(context.Context, string, []int64) (*service.CategoryDefaultGroupPreview, error) {
	return nil, s.err
}
func (s *categoryDefaultAdminStub) Apply(_ context.Context, category string, req service.CategoryDefaultGroupApplyRequest) (*service.CategoryDefaultGroupConfig, error) {
	s.category, s.apply = category, req
	return &service.CategoryDefaultGroupConfig{GroupID: 100, Category: category}, s.err
}
func (s *categoryDefaultAdminStub) SetStatus(_ context.Context, category string, enabled bool) (*service.CategoryDefaultGroupConfig, error) {
	s.statusCalls++
	s.category, s.enabled = category, enabled
	return &service.CategoryDefaultGroupConfig{GroupID: 100, Category: category, Enabled: enabled}, s.err
}

func categoryDefaultAdminRequest(t *testing.T, body string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/gemini/status", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "category", Value: "gemini"}}
	handler(ctx)
	return w
}

func TestCategoryDefaultAdminStatusRequiresExplicitBoolean(t *testing.T) {
	for _, body := range []string{`{}`, `{"enabled":null}`, `{"enabled":"true"}`, `broken`} {
		stub := &categoryDefaultAdminStub{}
		h := &CategoryDefaultGroupHandler{service: stub}
		w := categoryDefaultAdminRequest(t, body, h.SetStatus)
		require.Equal(t, http.StatusBadRequest, w.Code, body)
		require.Zero(t, stub.statusCalls)
	}
	stub := &categoryDefaultAdminStub{enabled: true}
	h := &CategoryDefaultGroupHandler{service: stub}
	w := categoryDefaultAdminRequest(t, `{"enabled":false}`, h.SetStatus)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, stub.statusCalls)
	require.False(t, stub.enabled)
	require.Equal(t, "gemini", stub.category)
}

func TestCategoryDefaultAdminPublishForwardsPreviewAndReturnsConflict(t *testing.T) {
	stub := &categoryDefaultAdminStub{err: service.ErrCategoryDefaultStalePreview}
	h := &CategoryDefaultGroupHandler{service: stub}
	w := categoryDefaultAdminRequest(t, `{"source_group_ids":[7,9],"expected_revision":3,"preview_hash":"preview","models":[{"public_model":"untrusted"}]}`, h.SyncApply)
	require.Equal(t, http.StatusConflict, w.Code)
	require.Equal(t, "gemini", stub.category)
	require.Equal(t, []int64{7, 9}, stub.apply.SourceGroupIDs)
	require.Equal(t, int64(3), stub.apply.ExpectedRevision)
	require.Equal(t, "preview", stub.apply.PreviewHash)
	require.Contains(t, w.Body.String(), "CATEGORY_DEFAULT_STALE_PREVIEW")
}
