//go:build unit

package service

import (
	"context"
	"errors"
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
	createdRecords []ImageStudioImageRecord
	createErr      error

	listUserID int64
	listParams pagination.PaginationParams
	listItems  []ImageStudioImageRecord
	listPage   *pagination.PaginationResult
	listErr    error

	deletedID     int64
	deletedUserID int64
	deleteErr     error

	deleteOldestUserID int64
	deleteOldestLimit  int
	deleteOldestItems  []ImageStudioImageRecord
	deleteOldestErr    error
}

func (s *imageStudioRepoStub) Create(ctx context.Context, record *ImageStudioImageRecord) error {
	if s.createErr != nil {
		return s.createErr
	}
	if record.ID == 0 {
		record.ID = int64(len(s.createdRecords) + 1)
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	s.createdRecords = append(s.createdRecords, *record)
	return nil
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
	return int64(len(s.listItems) + len(s.createdRecords)), nil
}

func (s *imageStudioRepoStub) DeleteOldestOverLimit(ctx context.Context, userID int64, limit int) ([]ImageStudioImageRecord, error) {
	s.deleteOldestUserID = userID
	s.deleteOldestLimit = limit
	if s.deleteOldestErr != nil {
		return nil, s.deleteOldestErr
	}
	return s.deleteOldestItems, nil
}

func (s *imageStudioRepoStub) ListExpired(ctx context.Context, now time.Time, limit int) ([]ImageStudioImageRecord, error) {
	panic("unexpected ListExpired call")
}

type imageStudioExecutorStub struct {
	generateInput *ImageStudioGenerateInput
	editInput     *ImageStudioEditInput
	result        *ImageStudioExecutionResult
	err           error
}

func (s *imageStudioExecutorStub) Generate(ctx context.Context, input ImageStudioGenerateInput) (*ImageStudioExecutionResult, error) {
	s.generateInput = &input
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func (s *imageStudioExecutorStub) Edit(ctx context.Context, input ImageStudioEditInput) (*ImageStudioExecutionResult, error) {
	s.editInput = &input
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

type imageStudioGroupResolverStub struct {
	groups []Group
	err    error
}

func (s *imageStudioGroupResolverStub) GetAvailableGroups(ctx context.Context, userID int64) ([]Group, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]Group(nil), s.groups...), nil
}

type imageStudioStorageStub struct {
	putKey         string
	putContentType string
	putData        []byte
	deleteKey      string
	publicURL      string
	putErr         error
}

func (s *imageStudioStorageStub) Put(ctx context.Context, objectKey string, contentType string, data []byte) (string, error) {
	s.putKey = objectKey
	s.putContentType = contentType
	s.putData = append([]byte(nil), data...)
	if s.putErr != nil {
		return "", s.putErr
	}
	if s.publicURL != "" {
		return s.publicURL, nil
	}
	return "https://assets.example.com/" + objectKey, nil
}

func (s *imageStudioStorageStub) Delete(ctx context.Context, objectKey string) error {
	s.deleteKey = objectKey
	return nil
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

func TestImageStudioServiceGetOptionsReturnsSelectableGroupsModelsQualitiesAndPrices(t *testing.T) {
	price1K := 0.03
	price2K := 0.07
	price4K := 0.16
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	settings.AllowedModels = []string{"gpt-image-2"}
	settings.DefaultModel = "gpt-image-2"

	svc := NewImageStudioService(&imageStudioRepoStub{}, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetGroupResolver(&imageStudioGroupResolverStub{groups: []Group{
		{
			ID:                   2,
			Name:                 "Image Pro",
			Description:          "image capable",
			Platform:             PlatformOpenAI,
			Status:               StatusActive,
			AllowImageGeneration: true,
			ImagePrice1K:         &price1K,
			ImagePrice2K:         &price2K,
			ImagePrice4K:         &price4K,
		},
		{
			ID:                   3,
			Name:                 "Text Only",
			Platform:             PlatformOpenAI,
			Status:               StatusActive,
			AllowImageGeneration: false,
		},
	}})

	options, err := svc.GetOptions(context.Background(), 7)

	require.NoError(t, err)
	require.True(t, options.Enabled)
	require.NotNil(t, options.DefaultGroupID)
	require.Equal(t, int64(2), *options.DefaultGroupID)
	require.Equal(t, "gpt-image-2", options.DefaultModel)
	require.Len(t, options.Groups, 1)
	group := options.Groups[0]
	require.Equal(t, int64(2), group.ID)
	require.Equal(t, "Image Pro", group.Name)
	require.Equal(t, []ImageStudioModelOption{{
		Model:        "gpt-image-2",
		Label:        "gpt-image-2",
		Capabilities: []string{ImageStudioModeGeneration, ImageStudioModeEdit},
	}}, group.Models)
	require.Equal(t, []ImageStudioQualityOption{
		{Quality: ImageBillingSize1K, Label: ImageBillingSize1K, BillingTier: ImageBillingSize1K, EstimatedCost: price1K},
		{Quality: ImageBillingSize2K, Label: ImageBillingSize2K, BillingTier: ImageBillingSize2K, EstimatedCost: price2K},
		{Quality: ImageBillingSize4K, Label: ImageBillingSize4K, BillingTier: ImageBillingSize4K, EstimatedCost: price4K},
	}, group.Qualities)
	require.Contains(t, group.Prices, ImageStudioPricePreviewItem{
		Ratio:         "16:9",
		Quality:       ImageBillingSize4K,
		Size:          "3840x2160",
		BillingTier:   ImageBillingSize4K,
		EstimatedCost: price4K,
	})
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

func TestImageStudioServiceGenerateStoresExecutionResult(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	settings.AllowedModels = []string{"gpt-image-1"}
	settings.DefaultModel = "gpt-image-1"
	settings.RetentionDays = 7
	executor := &imageStudioExecutorStub{
		result: &ImageStudioExecutionResult{
			ImageBytes:       []byte("png-bytes"),
			MimeType:         "image/png",
			Cost:             0.02,
			UsageLogID:       imageStudioInt64Ptr(88),
			RequestID:        "req-image",
			SourceImageCount: 0,
		},
	}
	storage := &imageStudioStorageStub{publicURL: "https://assets.example.com/out.png"}
	repo := &imageStudioRepoStub{}
	svc := NewImageStudioService(repo, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetExecutor(executor)
	svc.SetStorageFactory(func(ctx context.Context, cfg *ImageStudioSettings) (ImageStorage, error) {
		return storage, nil
	})

	record, err := svc.Generate(context.Background(), ImageStudioGenerateInput{
		UserID:      7,
		Model:       "gpt-image-1",
		Prompt:      "  blue API portal  ",
		AspectRatio: "16:9",
	})
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, int64(7), record.UserID)
	require.Equal(t, ImageStudioModeGeneration, record.Mode)
	require.Equal(t, "blue API portal", record.Prompt)
	require.Equal(t, "1536x864", record.Size)
	require.Equal(t, "2K", executor.generateInput.BillingTier)
	require.Equal(t, []byte("png-bytes"), storage.putData)
	require.Equal(t, "image/png", storage.putContentType)
	require.Equal(t, "https://assets.example.com/out.png", record.ImageURL)
	require.Equal(t, 0.02, record.Cost)
	require.NotNil(t, record.UsageLogID)
	require.Equal(t, int64(88), *record.UsageLogID)
	require.Len(t, repo.createdRecords, 1)
	require.NotNil(t, record.ExpiresAt)
}

func TestImageStudioServiceGenerateUsesSelectedGroupQualityAndRatio(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	settings.AllowedModels = []string{"gpt-image-2"}
	settings.DefaultModel = "gpt-image-2"
	groupID := int64(2)
	executor := &imageStudioExecutorStub{
		result: &ImageStudioExecutionResult{
			ImageBytes: []byte("png-bytes"),
			MimeType:   "image/png",
		},
	}
	storage := &imageStudioStorageStub{publicURL: "https://assets.example.com/out.png"}
	svc := NewImageStudioService(&imageStudioRepoStub{}, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetGroupResolver(&imageStudioGroupResolverStub{groups: []Group{{
		ID:                   groupID,
		Name:                 "Image Pro",
		Platform:             PlatformOpenAI,
		Status:               StatusActive,
		AllowImageGeneration: true,
	}}})
	svc.SetExecutor(executor)
	svc.SetStorageFactory(func(ctx context.Context, cfg *ImageStudioSettings) (ImageStorage, error) {
		return storage, nil
	})

	record, err := svc.Generate(context.Background(), ImageStudioGenerateInput{
		UserID:      7,
		GroupID:     &groupID,
		Model:       "gpt-image-2",
		Prompt:      "blue API portal",
		AspectRatio: "16:9",
		Quality:     ImageBillingSize4K,
	})

	require.NoError(t, err)
	require.NotNil(t, record)
	require.NotNil(t, executor.generateInput.GroupID)
	require.Equal(t, groupID, *executor.generateInput.GroupID)
	require.Equal(t, ImageBillingSize4K, executor.generateInput.Quality)
	require.Equal(t, "3840x2160", executor.generateInput.Size)
	require.Equal(t, ImageBillingSize4K, executor.generateInput.BillingTier)
}

func TestImageStudioServiceGenerateAllowsGPTImage2FromSelectedImageGroup(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	settings.AllowedModels = []string{"gpt-image-1"}
	settings.DefaultModel = "gpt-image-1"
	groupID := int64(2)
	executor := &imageStudioExecutorStub{
		result: &ImageStudioExecutionResult{
			ImageBytes: []byte("png-bytes"),
			MimeType:   "image/png",
		},
	}
	storage := &imageStudioStorageStub{publicURL: "https://assets.example.com/out.png"}
	svc := NewImageStudioService(&imageStudioRepoStub{}, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetGroupResolver(&imageStudioGroupResolverStub{groups: []Group{{
		ID:                   groupID,
		Name:                 "Image Pro",
		Platform:             PlatformOpenAI,
		Status:               StatusActive,
		AllowImageGeneration: true,
	}}})
	svc.SetExecutor(executor)
	svc.SetStorageFactory(func(ctx context.Context, cfg *ImageStudioSettings) (ImageStorage, error) {
		return storage, nil
	})

	record, err := svc.Generate(context.Background(), ImageStudioGenerateInput{
		UserID:      7,
		GroupID:     &groupID,
		Model:       "gpt-image-2",
		Prompt:      "blue API portal",
		AspectRatio: "1:1",
		Quality:     ImageBillingSize1K,
	})

	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, "gpt-image-2", executor.generateInput.Model)
}

func TestImageStudioServiceGenerateCommitsUsageOnlyAfterStorageSucceeds(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	settings.AllowedModels = []string{"gpt-image-1"}
	settings.DefaultModel = "gpt-image-1"

	commitCalls := 0
	executor := &imageStudioExecutorStub{
		result: &ImageStudioExecutionResult{
			ImageBytes: []byte("png-bytes"),
			MimeType:   "image/png",
			CommitUsage: func(ctx context.Context) error {
				commitCalls++
				return nil
			},
		},
	}
	storage := &imageStudioStorageStub{publicURL: "https://assets.example.com/out.png"}
	svc := NewImageStudioService(&imageStudioRepoStub{}, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetExecutor(executor)
	svc.SetStorageFactory(func(ctx context.Context, cfg *ImageStudioSettings) (ImageStorage, error) {
		return storage, nil
	})

	_, err := svc.Generate(context.Background(), ImageStudioGenerateInput{
		UserID:      7,
		Model:       "gpt-image-1",
		Prompt:      "blue API portal",
		AspectRatio: "1:1",
	})
	require.NoError(t, err)
	require.Equal(t, 1, commitCalls)

	commitCalls = 0
	storage.putErr = errors.New("storage down")
	_, err = svc.Generate(context.Background(), ImageStudioGenerateInput{
		UserID:      7,
		Model:       "gpt-image-1",
		Prompt:      "blue API portal",
		AspectRatio: "1:1",
	})
	require.Error(t, err)
	require.Equal(t, 0, commitCalls)
}

func TestImageStudioServiceGenerateRejectsDisabled(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = false
	executor := &imageStudioExecutorStub{}
	svc := NewImageStudioService(&imageStudioRepoStub{}, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetExecutor(executor)

	_, err := svc.Generate(context.Background(), ImageStudioGenerateInput{
		UserID:      7,
		Model:       "gpt-image-1",
		Prompt:      "blue API portal",
		AspectRatio: "1:1",
	})
	require.ErrorIs(t, err, ErrImageStudioDisabled)
	require.Nil(t, executor.generateInput)
}

func TestImageStudioServiceGenerateDoesNotStoreOnExecutorFailure(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	repo := &imageStudioRepoStub{}
	executor := &imageStudioExecutorStub{err: errors.New("upstream failed")}
	svc := NewImageStudioService(repo, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetExecutor(executor)

	_, err := svc.Generate(context.Background(), ImageStudioGenerateInput{
		UserID:      7,
		Model:       "gpt-image-1",
		Prompt:      "blue API portal",
		AspectRatio: "1:1",
	})
	require.Error(t, err)
	require.Empty(t, repo.createdRecords)
}

func TestImageStudioServiceEditRequiresReferenceImage(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	executor := &imageStudioExecutorStub{}
	svc := NewImageStudioService(&imageStudioRepoStub{}, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetExecutor(executor)

	_, err := svc.Edit(context.Background(), ImageStudioEditInput{
		UserID:      7,
		Model:       "gpt-image-1",
		Prompt:      "make it cleaner",
		AspectRatio: "1:1",
	})
	require.Error(t, err)
	require.Nil(t, executor.editInput)
}

func imageStudioInt64Ptr(v int64) *int64 {
	return &v
}
