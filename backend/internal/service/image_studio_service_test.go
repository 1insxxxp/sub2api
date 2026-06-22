//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type imageStudioConfigReaderStub struct {
	cfg *ImageStudioSettings
	err error
}

func (s *imageStudioConfigReaderStub) GetImageStudioConfig(ctx context.Context) (*ImageStudioSettings, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.cfg != nil {
		return cloneImageStudioSettings(s.cfg), nil
	}
	return defaultImageStudioSettings(), nil
}

type imageStudioRepoStub struct {
	listUserID int64
	listParams pagination.PaginationParams
	listItems  []ImageStudioImageRecord
	listPage   *pagination.PaginationResult
	listErr    error

	deletedID     int64
	deletedUserID int64
	deleteErr     error
}

func (s *imageStudioRepoStub) Create(ctx context.Context, record *ImageStudioImageRecord) error {
	panic("unexpected Create call")
}

func (s *imageStudioRepoStub) ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ImageStudioImageRecord, *pagination.PaginationResult, error) {
	s.listUserID = userID
	s.listParams = params
	if s.listPage == nil {
		s.listPage = &pagination.PaginationResult{Total: int64(len(s.listItems)), Page: params.Page, PageSize: params.Limit()}
	}
	return s.listItems, s.listPage, s.listErr
}

func (s *imageStudioRepoStub) GetByID(ctx context.Context, id int64) (*ImageStudioImageRecord, error) {
	panic("unexpected GetByID call")
}

func (s *imageStudioRepoStub) SoftDelete(ctx context.Context, id int64, userID int64) error {
	s.deletedID = id
	s.deletedUserID = userID
	return s.deleteErr
}

func (s *imageStudioRepoStub) CountSavedByUser(ctx context.Context, userID int64) (int64, error) {
	panic("unexpected CountSavedByUser call")
}

func (s *imageStudioRepoStub) DeleteOldestOverLimit(ctx context.Context, userID int64, limit int) ([]ImageStudioImageRecord, error) {
	panic("unexpected DeleteOldestOverLimit call")
}

func (s *imageStudioRepoStub) ListExpired(ctx context.Context, now time.Time, limit int) ([]ImageStudioImageRecord, error) {
	panic("unexpected ListExpired call")
}

func TestImageStudioServiceGetConfigReturnsPublicConfig(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	settings.AllowedModels = []string{"gpt-image-1", "custom-image-model"}
	settings.DefaultModel = "custom-image-model"
	settings.MaxImagesPerUser = 42
	settings.MaxReferenceImageMB = 12

	svc := NewImageStudioService(&imageStudioRepoStub{}, &imageStudioConfigReaderStub{cfg: settings})

	cfg, err := svc.GetConfig(context.Background())
	require.NoError(t, err)
	require.True(t, cfg.Enabled)
	require.Equal(t, settings.AllowedModels, cfg.AllowedModels)
	require.Equal(t, "custom-image-model", cfg.DefaultModel)
	require.Equal(t, 42, cfg.MaxImagesPerUser)
	require.Equal(t, 12, cfg.MaxReferenceImageMB)
	require.NotEmpty(t, cfg.AspectRatios)
}

func TestImageStudioServiceRejectsListWhenDisabled(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = false

	svc := NewImageStudioService(&imageStudioRepoStub{}, &imageStudioConfigReaderStub{cfg: settings})

	_, _, err := svc.List(context.Background(), 7, pagination.PaginationParams{Page: 1, PageSize: 10})
	require.ErrorIs(t, err, ErrImageStudioDisabled)
}

func TestImageStudioServiceListsCurrentUserImages(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	repo := &imageStudioRepoStub{
		listItems: []ImageStudioImageRecord{{ID: 99, UserID: 7, ImageURL: "https://assets.example.com/image.png"}},
	}
	svc := NewImageStudioService(repo, &imageStudioConfigReaderStub{cfg: settings})

	items, page, err := svc.List(context.Background(), 7, pagination.PaginationParams{Page: 2, PageSize: 5})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.NotNil(t, page)
	require.Equal(t, int64(7), repo.listUserID)
	require.Equal(t, 2, repo.listParams.Page)
	require.Equal(t, 5, repo.listParams.PageSize)
}

func TestImageStudioServiceDeleteChecksCurrentUserThroughRepository(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	repo := &imageStudioRepoStub{}
	svc := NewImageStudioService(repo, &imageStudioConfigReaderStub{cfg: settings})

	require.NoError(t, svc.Delete(context.Background(), 7, 99))
	require.Equal(t, int64(99), repo.deletedID)
	require.Equal(t, int64(7), repo.deletedUserID)

	repo.deleteErr = ErrUserImageNotFound
	err := svc.Delete(context.Background(), 8, 99)
	require.ErrorIs(t, err, ErrUserImageNotFound)
}
