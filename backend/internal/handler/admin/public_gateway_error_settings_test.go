package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type publicGatewayErrorSettingsRepository struct {
	service.SettingRepository
	value  string
	err    error
	writes int
}

func (r *publicGatewayErrorSettingsRepository) GetValue(context.Context, string) (string, error) {
	return r.value, r.err
}

func (r *publicGatewayErrorSettingsRepository) Set(_ context.Context, _ string, value string) error {
	if r.err != nil {
		return r.err
	}
	r.value = value
	r.writes++
	return nil
}

func TestPublicGatewayErrorSettingsAPI(t *testing.T) {
	repo := &publicGatewayErrorSettingsRepository{}
	h := &SettingHandler{settingService: service.NewSettingService(repo, &config.Config{})}
	router := gin.New()
	router.GET("/settings", h.GetPublicGatewayErrorSettings)
	router.PUT("/settings", h.UpdatePublicGatewayErrorSettings)
	router.POST("/preview", h.PreviewPublicGatewayError)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		out := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(out, req)
		return out
	}
	out := request(http.MethodGet, "/settings", "")
	require.Equal(t, 200, out.Code)
	require.Equal(t, int64(21), gjson.Get(out.Body.String(), "data.rules.#").Int())
	settings := service.DefaultPublicGatewayErrorSettings()
	settings.Fallback.Message = "请稍后重试"
	settings.Rules[0].Enabled = false
	body, err := json.Marshal(settings)
	require.NoError(t, err)
	out = request(http.MethodPut, "/settings", string(body))
	require.Equal(t, 200, out.Code)
	require.Equal(t, 1, repo.writes)
	require.Equal(t, "请稍后重试", gjson.Get(out.Body.String(), "data.fallback.message").String())
	require.False(t, gjson.Get(out.Body.String(), "data.rules.0.enabled").Bool())
	out = request(http.MethodGet, "/settings", "")
	require.Equal(t, "请稍后重试", gjson.Get(out.Body.String(), "data.fallback.message").String())
	preview, err := json.Marshal(map[string]any{"settings": settings, "status": 502, "diagnostic": "secret upstream", "format": "responses"})
	require.NoError(t, err)
	out = request(http.MethodPost, "/preview", string(preview))
	require.Equal(t, 200, out.Code)
	require.Equal(t, "response.failed", gjson.Get(out.Body.String(), "data.payload.type").String())
	require.NotContains(t, out.Body.String(), "secret upstream")
	require.Equal(t, 1, repo.writes)
	settings.Rules[0].Pattern = "["
	body, err = json.Marshal(settings)
	require.NoError(t, err)
	out = request(http.MethodPut, "/settings", string(body))
	require.Equal(t, 400, out.Code)
	require.Equal(t, 1, repo.writes)
	for _, method := range []string{http.MethodPut, http.MethodPost} {
		path := "/settings"
		if method == http.MethodPost {
			path = "/preview"
		}
		require.Equal(t, 400, request(method, path, "{").Code)
	}
	repo.err = errors.New("storage failed")
	require.Equal(t, 500, request(http.MethodGet, "/settings", "").Code)
	body, err = json.Marshal(service.DefaultPublicGatewayErrorSettings())
	require.NoError(t, err)
	require.Equal(t, 500, request(http.MethodPut, "/settings", string(body)).Code)
}
