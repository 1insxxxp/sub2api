//go:build unit

package admin

import (
	"bytes"
	"context"
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

func newImageStudioSettingsHandler(values map[string]string) (*SettingHandler, *settingHandlerRepoStub) {
	repo := &settingHandlerRepoStub{values: values}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	return NewSettingHandler(svc, nil, nil, nil, nil, nil, nil), repo
}

func resetImageStudioHandlerTestCache(t *testing.T) {
	t.Helper()
}

func TestSettingHandlerGetImageStudioConfig(t *testing.T) {
	resetImageStudioHandlerTestCache(t)
	gin.SetMode(gin.TestMode)
	handler, _ := newImageStudioSettingsHandler(map[string]string{})
	require.NoError(t, handler.settingService.SaveImageStudioConfig(context.Background(), &service.ImageStudioSettings{}))

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/image-studio", nil)

	handler.GetImageStudioConfig(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data := resp.Data.(map[string]any)
	require.Equal(t, false, data["enabled"])
	require.Equal(t, "gpt-image-1", data["default_model"])
	require.Equal(t, "local", data["storage_driver"])
	require.NotEmpty(t, data["aspect_ratios"])
	status := data["storage_status"].(map[string]any)
	require.Equal(t, "local", status["driver"])
	require.Equal(t, true, status["configured"])
}

func TestSettingHandlerUpdateImageStudioConfig(t *testing.T) {
	resetImageStudioHandlerTestCache(t)
	gin.SetMode(gin.TestMode)
	handler, repo := newImageStudioSettingsHandler(map[string]string{})

	body := []byte(`{"enabled":true,"allowed_models":[" gpt-image-1 ","gpt-image-2","gpt-image-1"],"default_model":"gpt-image-2","storage_driver":"r2","r2_public_base_url":" https://assets.example.com/ ","retention_days":0,"max_images_per_user":0,"max_reference_image_mb":0}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/image-studio", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateImageStudioConfig(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotEmpty(t, repo.values[service.SettingKeyImageStudioConfig])

	var saved service.ImageStudioSettings
	require.NoError(t, json.Unmarshal([]byte(repo.values[service.SettingKeyImageStudioConfig]), &saved))
	require.True(t, saved.Enabled)
	require.Equal(t, []string{"gpt-image-1", "gpt-image-2"}, saved.AllowedModels)
	require.Equal(t, "gpt-image-2", saved.DefaultModel)
	require.Equal(t, service.ImageStorageDriverR2, saved.StorageDriver)
	require.Equal(t, "https://assets.example.com", saved.R2PublicBaseURL)
	require.Equal(t, 30, saved.RetentionDays)
	require.Equal(t, 100, saved.MaxImagesPerUser)
	require.Equal(t, 20, saved.MaxReferenceImageMB)
}
