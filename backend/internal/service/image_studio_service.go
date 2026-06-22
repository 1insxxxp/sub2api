package service

import (
	"context"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrImageStudioDisabled = infraerrors.Forbidden("IMAGE_STUDIO_DISABLED", "image studio is disabled")
	ErrUserImageNotFound   = infraerrors.NotFound("USER_IMAGE_NOT_FOUND", "image record not found")
)

type ImageStudioConfigReader interface {
	GetImageStudioConfig(ctx context.Context) (*ImageStudioSettings, error)
}

type UserImageRepository interface {
	Create(ctx context.Context, record *ImageStudioImageRecord) error
	ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ImageStudioImageRecord, *pagination.PaginationResult, error)
	GetByID(ctx context.Context, id int64) (*ImageStudioImageRecord, error)
	SoftDelete(ctx context.Context, id int64, userID int64) error
	CountSavedByUser(ctx context.Context, userID int64) (int64, error)
	DeleteOldestOverLimit(ctx context.Context, userID int64, limit int) ([]ImageStudioImageRecord, error)
	ListExpired(ctx context.Context, now time.Time, limit int) ([]ImageStudioImageRecord, error)
}

type ImageStudioService struct {
	repo         UserImageRepository
	configReader ImageStudioConfigReader
}

func NewImageStudioService(repo UserImageRepository, configReader ImageStudioConfigReader) *ImageStudioService {
	return &ImageStudioService{
		repo:         repo,
		configReader: configReader,
	}
}

func (s *ImageStudioService) GetConfig(ctx context.Context) (*ImageStudioConfig, error) {
	cfg, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	return imageStudioPublicConfig(cfg), nil
}

func (s *ImageStudioService) List(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ImageStudioImageRecord, *pagination.PaginationResult, error) {
	if err := s.ensureEnabled(ctx); err != nil {
		return nil, nil, err
	}
	items, page, err := s.repo.ListByUser(ctx, userID, params)
	if err != nil {
		return nil, nil, fmt.Errorf("list user images: %w", err)
	}
	return items, page, nil
}

func (s *ImageStudioService) Delete(ctx context.Context, userID int64, imageID int64) error {
	if err := s.ensureEnabled(ctx); err != nil {
		return err
	}
	if err := s.repo.SoftDelete(ctx, imageID, userID); err != nil {
		return fmt.Errorf("delete user image: %w", err)
	}
	return nil
}

func (s *ImageStudioService) ensureEnabled(ctx context.Context) error {
	cfg, err := s.loadSettings(ctx)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return ErrImageStudioDisabled
	}
	return nil
}

func (s *ImageStudioService) loadSettings(ctx context.Context) (*ImageStudioSettings, error) {
	if s.configReader == nil {
		return defaultImageStudioSettings(), nil
	}
	cfg, err := s.configReader.GetImageStudioConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get image studio config: %w", err)
	}
	normalized, err := normalizeImageStudioSettings(cfg)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}

func imageStudioPublicConfig(cfg *ImageStudioSettings) *ImageStudioConfig {
	if cfg == nil {
		cfg = defaultImageStudioSettings()
	}
	return &ImageStudioConfig{
		Enabled:             cfg.Enabled,
		AllowedModels:       append([]string(nil), cfg.AllowedModels...),
		DefaultModel:        cfg.DefaultModel,
		AspectRatios:        append([]ImageStudioAspectRatio(nil), cfg.AspectRatios...),
		MaxReferenceImageMB: cfg.MaxReferenceImageMB,
		RetentionDays:       cfg.RetentionDays,
		MaxImagesPerUser:    cfg.MaxImagesPerUser,
	}
}
