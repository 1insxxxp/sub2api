//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type imageStudioSettingRepoStub struct {
	value string
	err   error
	saved map[string]string
}

func (s *imageStudioSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *imageStudioSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.value, nil
}

func (s *imageStudioSettingRepoStub) Set(ctx context.Context, key, value string) error {
	if s.saved == nil {
		s.saved = map[string]string{}
	}
	s.saved[key] = value
	return nil
}

func (s *imageStudioSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *imageStudioSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *imageStudioSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *imageStudioSettingRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func resetImageStudioConfigTestCache(t *testing.T) {
	t.Helper()
	imageStudioConfigCache.Store((*cachedImageStudioConfig)(nil))
	t.Cleanup(func() {
		imageStudioConfigCache.Store((*cachedImageStudioConfig)(nil))
	})
}

func TestGetImageStudioConfigReturnsDefaultsWhenMissing(t *testing.T) {
	resetImageStudioConfigTestCache(t)

	repo := &imageStudioSettingRepoStub{err: ErrSettingNotFound}
	svc := NewSettingService(repo, &config.Config{})

	cfg, err := svc.GetImageStudioConfig(context.Background())
	require.NoError(t, err)
	require.False(t, cfg.Enabled)
	require.Equal(t, []string{"gpt-image-1"}, cfg.AllowedModels)
	require.Equal(t, "gpt-image-1", cfg.DefaultModel)
	require.Equal(t, ImageStorageDriverLocal, cfg.StorageDriver)
	require.Equal(t, 30, cfg.RetentionDays)
	require.Equal(t, 100, cfg.MaxImagesPerUser)
	require.Equal(t, 20, cfg.MaxReferenceImageMB)
	require.NotEmpty(t, cfg.AspectRatios)
}

func TestSaveImageStudioConfigNormalizesAndPersists(t *testing.T) {
	resetImageStudioConfigTestCache(t)

	repo := &imageStudioSettingRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.SaveImageStudioConfig(context.Background(), &ImageStudioSettings{
		Enabled:             true,
		AllowedModels:       []string{" gpt-image-1 ", "", "gpt-image-1", "gpt-image-2"},
		DefaultModel:        "gpt-image-2",
		StorageDriver:       ImageStorageDriverR2,
		R2PublicBaseURL:     " https://assets.example.com/ ",
		RetentionDays:       0,
		MaxImagesPerUser:    -1,
		MaxReferenceImageMB: 0,
	})
	require.NoError(t, err)

	raw := repo.saved[SettingKeyImageStudioConfig]
	require.NotEmpty(t, raw)

	var saved ImageStudioSettings
	require.NoError(t, json.Unmarshal([]byte(raw), &saved))
	require.True(t, saved.Enabled)
	require.Equal(t, []string{"gpt-image-1", "gpt-image-2"}, saved.AllowedModels)
	require.Equal(t, "gpt-image-2", saved.DefaultModel)
	require.Equal(t, ImageStorageDriverR2, saved.StorageDriver)
	require.Equal(t, "https://assets.example.com", saved.R2PublicBaseURL)
	require.Equal(t, 30, saved.RetentionDays)
	require.Equal(t, 100, saved.MaxImagesPerUser)
	require.Equal(t, 20, saved.MaxReferenceImageMB)
}

func TestSaveImageStudioConfigRejectsDefaultModelOutsideAllowedList(t *testing.T) {
	resetImageStudioConfigTestCache(t)

	repo := &imageStudioSettingRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.SaveImageStudioConfig(context.Background(), &ImageStudioSettings{
		AllowedModels: []string{"gpt-image-1"},
		DefaultModel:  "gpt-image-2",
	})
	require.Error(t, err)
	require.Empty(t, repo.saved)
}
