package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"golang.org/x/sync/singleflight"
)

const (
	imageStudioDefaultModel          = "gpt-image-1"
	imageStudioDefaultRetentionDays  = 30
	imageStudioDefaultMaxImages      = 100
	imageStudioDefaultMaxReferenceMB = 20

	imageStudioConfigCacheTTL        = 60 * time.Second
	imageStudioConfigErrorTTL        = 5 * time.Second
	imageStudioConfigDBTimeout       = 5 * time.Second
	imageStudioConfigSingleflightKey = "image_studio_config"

	imageStudioStorageStatusOK        = "ok"
	imageStudioStorageStatusUntested  = "untested"
	imageStudioStorageStatusMisconfig = "misconfigured"
)

type ImageStudioSettings struct {
	Enabled             bool                     `json:"enabled"`
	AllowedModels       []string                 `json:"allowed_models"`
	DefaultModel        string                   `json:"default_model"`
	StorageDriver       string                   `json:"storage_driver"`
	LocalRootDir        string                   `json:"local_root_dir,omitempty"`
	LocalPublicBaseURL  string                   `json:"local_public_base_url,omitempty"`
	R2PublicBaseURL     string                   `json:"r2_public_base_url,omitempty"`
	RetentionDays       int                      `json:"retention_days"`
	MaxImagesPerUser    int                      `json:"max_images_per_user"`
	MaxReferenceImageMB int                      `json:"max_reference_image_mb"`
	AspectRatios        []ImageStudioAspectRatio `json:"aspect_ratios"`
}

type ImageStudioStorageStatus struct {
	Driver     string `json:"driver"`
	Status     string `json:"status"`
	Configured bool   `json:"configured"`
	Message    string `json:"message,omitempty"`
}

type cachedImageStudioConfig struct {
	config    *ImageStudioSettings
	expiresAt int64
}

var imageStudioConfigCache atomic.Value // *cachedImageStudioConfig
var imageStudioConfigSF singleflight.Group

func defaultImageStudioSettings() *ImageStudioSettings {
	return &ImageStudioSettings{
		Enabled:             false,
		AllowedModels:       []string{imageStudioDefaultModel},
		DefaultModel:        imageStudioDefaultModel,
		StorageDriver:       ImageStorageDriverLocal,
		RetentionDays:       imageStudioDefaultRetentionDays,
		MaxImagesPerUser:    imageStudioDefaultMaxImages,
		MaxReferenceImageMB: imageStudioDefaultMaxReferenceMB,
		AspectRatios:        SupportedImageStudioAspectRatios(),
	}
}

func cloneImageStudioSettings(cfg *ImageStudioSettings) *ImageStudioSettings {
	if cfg == nil {
		return nil
	}
	out := *cfg
	out.AllowedModels = append([]string(nil), cfg.AllowedModels...)
	out.AspectRatios = append([]ImageStudioAspectRatio(nil), cfg.AspectRatios...)
	return &out
}

func normalizeImageStudioSettings(cfg *ImageStudioSettings) (*ImageStudioSettings, error) {
	base := defaultImageStudioSettings()
	if cfg == nil {
		return base, nil
	}

	out := *cfg
	out.AllowedModels = normalizeImageStudioModels(cfg.AllowedModels)
	if len(out.AllowedModels) == 0 {
		out.AllowedModels = append([]string(nil), base.AllowedModels...)
	}

	out.DefaultModel = strings.TrimSpace(out.DefaultModel)
	if out.DefaultModel == "" {
		out.DefaultModel = out.AllowedModels[0]
	}
	if err := ValidateImageStudioModel(out.DefaultModel, out.AllowedModels); err != nil {
		return nil, err
	}

	out.StorageDriver = strings.ToLower(strings.TrimSpace(out.StorageDriver))
	if out.StorageDriver == "" {
		out.StorageDriver = ImageStorageDriverLocal
	}
	switch out.StorageDriver {
	case ImageStorageDriverLocal, ImageStorageDriverR2:
	default:
		return nil, fmt.Errorf("unsupported image storage driver: %s", out.StorageDriver)
	}

	out.LocalRootDir = strings.TrimSpace(out.LocalRootDir)
	out.LocalPublicBaseURL = strings.TrimRight(strings.TrimSpace(out.LocalPublicBaseURL), "/")
	out.R2PublicBaseURL = strings.TrimRight(strings.TrimSpace(out.R2PublicBaseURL), "/")

	if out.RetentionDays <= 0 {
		out.RetentionDays = base.RetentionDays
	}
	if out.MaxImagesPerUser <= 0 {
		out.MaxImagesPerUser = base.MaxImagesPerUser
	}
	if out.MaxReferenceImageMB <= 0 {
		out.MaxReferenceImageMB = base.MaxReferenceImageMB
	}
	out.AspectRatios = SupportedImageStudioAspectRatios()
	return &out, nil
}

func normalizeImageStudioModels(models []string) []string {
	normalized := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		normalized = append(normalized, model)
	}
	return normalized
}

func parseImageStudioSettingsJSON(raw string) *ImageStudioSettings {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultImageStudioSettings()
	}
	var cfg ImageStudioSettings
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		slog.Warn("image studio: failed to parse config JSON", "error", err)
		return defaultImageStudioSettings()
	}
	normalized, err := normalizeImageStudioSettings(&cfg)
	if err != nil {
		slog.Warn("image studio: invalid config JSON, falling back to defaults", "error", err)
		return defaultImageStudioSettings()
	}
	return normalized
}

func (s *SettingService) GetImageStudioConfig(ctx context.Context) (*ImageStudioSettings, error) {
	if cached := imageStudioConfigCache.Load(); cached != nil {
		if entry, ok := cached.(*cachedImageStudioConfig); ok && entry != nil && time.Now().UnixNano() < entry.expiresAt {
			return cloneImageStudioSettings(entry.config), nil
		}
	}

	result, err, _ := imageStudioConfigSF.Do(imageStudioConfigSingleflightKey, func() (any, error) {
		return s.loadImageStudioConfigFromDB(ctx)
	})
	if err != nil {
		return defaultImageStudioSettings(), err
	}
	cfg, ok := result.(*ImageStudioSettings)
	if !ok || cfg == nil {
		return defaultImageStudioSettings(), nil
	}
	return cloneImageStudioSettings(cfg), nil
}

func (s *SettingService) loadImageStudioConfigFromDB(ctx context.Context) (*ImageStudioSettings, error) {
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), imageStudioConfigDBTimeout)
	defer cancel()

	raw, err := s.settingRepo.GetValue(dbCtx, SettingKeyImageStudioConfig)
	if err != nil {
		cfg := defaultImageStudioSettings()
		if errors.Is(err, ErrSettingNotFound) {
			imageStudioConfigCache.Store(&cachedImageStudioConfig{
				config:    cfg,
				expiresAt: time.Now().Add(imageStudioConfigCacheTTL).UnixNano(),
			})
			return cfg, nil
		}
		imageStudioConfigCache.Store(&cachedImageStudioConfig{
			config:    cfg,
			expiresAt: time.Now().Add(imageStudioConfigErrorTTL).UnixNano(),
		})
		return cfg, fmt.Errorf("image studio: get config: %w", err)
	}

	cfg := parseImageStudioSettingsJSON(raw)
	imageStudioConfigCache.Store(&cachedImageStudioConfig{
		config:    cfg,
		expiresAt: time.Now().Add(imageStudioConfigCacheTTL).UnixNano(),
	})
	return cfg, nil
}

func (s *SettingService) SaveImageStudioConfig(ctx context.Context, cfg *ImageStudioSettings) error {
	normalized, err := normalizeImageStudioSettings(cfg)
	if err != nil {
		return infraerrors.BadRequest("INVALID_IMAGE_STUDIO_CONFIG", err.Error())
	}

	data, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("image studio: marshal config: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyImageStudioConfig, string(data)); err != nil {
		return fmt.Errorf("image studio: save config: %w", err)
	}

	imageStudioConfigSF.Forget(imageStudioConfigSingleflightKey)
	imageStudioConfigCache.Store(&cachedImageStudioConfig{
		config:    cloneImageStudioSettings(normalized),
		expiresAt: time.Now().Add(imageStudioConfigCacheTTL).UnixNano(),
	})
	return nil
}

func ImageStudioStorageStatusForConfig(cfg *ImageStudioSettings) ImageStudioStorageStatus {
	cfg, _ = normalizeImageStudioSettings(cfg)
	status := ImageStudioStorageStatus{
		Driver:     cfg.StorageDriver,
		Status:     imageStudioStorageStatusUntested,
		Configured: true,
	}
	switch cfg.StorageDriver {
	case ImageStorageDriverR2:
		if strings.TrimSpace(cfg.R2PublicBaseURL) == "" {
			status.Status = imageStudioStorageStatusMisconfig
			status.Configured = false
			status.Message = "r2 public base url is required"
		}
	case ImageStorageDriverLocal:
		status.Status = imageStudioStorageStatusOK
	}
	return status
}
