package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newAuthIPBlacklistSettingsHandler(values map[string]string) (*SettingHandler, *settingHandlerRepoStub) {
	repo := &settingHandlerRepoStub{values: values}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	return NewSettingHandler(svc, nil, nil, nil, nil, nil, nil), repo
}

func TestSettingHandlerGetAuthIPBlacklistSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := newAuthIPBlacklistSettingsHandler(map[string]string{
		service.SettingKeyAuthIPBlacklistEnabled:               "true",
		service.SettingKeyAuthIPBlacklistRules:                 `["45.207.193.151","bad-rule","202.8.9.0/24"]`,
		service.SettingKeyAuthIPAutoBlockEnabled:               "true",
		service.SettingKeyAuthIPAutoBlockWindowMinutes:         "10",
		service.SettingKeyAuthIPAutoBlockRegisterThreshold:     "5",
		service.SettingKeyAuthIPAutoBlockVerifyCodeThreshold:   "12",
		service.SettingKeyAuthIPAutoBlockLoginFailureThreshold: "30",
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/auth-ip-blacklist", nil)

	handler.GetAuthIPBlacklistSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data := resp.Data.(map[string]any)
	require.Equal(t, true, data["enabled"])
	require.Equal(t, []any{"45.207.193.151", "202.8.9.0/24"}, data["rules"])
	autoBlock := data["auto_block"].(map[string]any)
	require.Equal(t, true, autoBlock["enabled"])
	require.Equal(t, float64(10), autoBlock["window_minutes"])
	require.Equal(t, float64(5), autoBlock["register_threshold"])
	require.Equal(t, float64(12), autoBlock["verify_code_threshold"])
	require.Equal(t, float64(30), autoBlock["login_failure_threshold"])
}

func TestSettingHandlerUpdateAuthIPBlacklistSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, repo := newAuthIPBlacklistSettingsHandler(map[string]string{})

	body := []byte(`{"enabled":true,"rules":["45.207.193.151","bad-rule","202.8.9.0/24","45.207.193.151"],"auto_block":{"enabled":true,"window_minutes":15,"register_threshold":4,"verify_code_threshold":10,"login_failure_threshold":20}}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/auth-ip-blacklist", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateAuthIPBlacklistSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "true", repo.lastUpdates[service.SettingKeyAuthIPBlacklistEnabled])
	require.JSONEq(t, `["45.207.193.151","202.8.9.0/24"]`, repo.lastUpdates[service.SettingKeyAuthIPBlacklistRules])
	require.Equal(t, "true", repo.lastUpdates[service.SettingKeyAuthIPAutoBlockEnabled])
	require.Equal(t, "15", repo.lastUpdates[service.SettingKeyAuthIPAutoBlockWindowMinutes])
	require.Equal(t, "4", repo.lastUpdates[service.SettingKeyAuthIPAutoBlockRegisterThreshold])
	require.Equal(t, "10", repo.lastUpdates[service.SettingKeyAuthIPAutoBlockVerifyCodeThreshold])
	require.Equal(t, "20", repo.lastUpdates[service.SettingKeyAuthIPAutoBlockLoginFailureThreshold])
}
