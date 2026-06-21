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
		service.SettingKeyAuthIPBlacklistEnabled: "true",
		service.SettingKeyAuthIPBlacklistRules:   `["45.207.193.151","bad-rule","202.8.9.0/24"]`,
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
}

func TestSettingHandlerUpdateAuthIPBlacklistSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, repo := newAuthIPBlacklistSettingsHandler(map[string]string{})

	body := []byte(`{"enabled":true,"rules":["45.207.193.151","bad-rule","202.8.9.0/24","45.207.193.151"]}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/auth-ip-blacklist", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateAuthIPBlacklistSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "true", repo.lastUpdates[service.SettingKeyAuthIPBlacklistEnabled])
	require.JSONEq(t, `["45.207.193.151","202.8.9.0/24"]`, repo.lastUpdates[service.SettingKeyAuthIPBlacklistRules])
}
