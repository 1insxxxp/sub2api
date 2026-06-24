//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/png"
	"io"
	"net/http"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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

	getByIDRecord *ImageStudioImageRecord
	getByIDID     int64
	getByIDErr    error

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

	listExpiredNow   time.Time
	listExpiredLimit int
	listExpiredItems []ImageStudioImageRecord
	listExpiredErr   error
}

type imageStudioTaskRepoStub struct {
	nextID int64
	tasks  map[int64]ImageStudioTask
}

func (s *imageStudioTaskRepoStub) ensure() {
	if s.tasks == nil {
		s.tasks = make(map[int64]ImageStudioTask)
	}
}

func (s *imageStudioTaskRepoStub) Create(ctx context.Context, task *ImageStudioTask) error {
	s.ensure()
	s.nextID++
	task.ID = s.nextID
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = task.CreatedAt
	}
	s.tasks[task.ID] = *task
	return nil
}

func (s *imageStudioTaskRepoStub) GetByID(ctx context.Context, userID int64, taskID int64) (*ImageStudioTask, error) {
	s.ensure()
	task, ok := s.tasks[taskID]
	if !ok || (userID > 0 && task.UserID != userID) {
		return nil, ErrImageStudioTaskNotFound
	}
	return &task, nil
}

func (s *imageStudioTaskRepoStub) ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ImageStudioTask, *pagination.PaginationResult, error) {
	s.ensure()
	var out []ImageStudioTask
	for _, task := range s.tasks {
		if task.UserID == userID {
			out = append(out, task)
		}
	}
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: params.Page, PageSize: params.Limit(), Pages: 1}, nil
}

func (s *imageStudioTaskRepoStub) MarkRunning(ctx context.Context, taskID int64, startedAt time.Time) (bool, error) {
	s.ensure()
	task := s.tasks[taskID]
	if task.ID == 0 || task.Status != ImageStudioTaskStatusQueued {
		return false, nil
	}
	task.Status = ImageStudioTaskStatusRunning
	task.StartedAt = &startedAt
	task.UpdatedAt = startedAt
	s.tasks[taskID] = task
	return true, nil
}

func (s *imageStudioTaskRepoStub) MarkSucceeded(ctx context.Context, taskID int64, image *ImageStudioImageRecord, quality string, estimatedCost float64, completedAt time.Time) (*ImageStudioTask, error) {
	s.ensure()
	task := s.tasks[taskID]
	task.Status = ImageStudioTaskStatusSucceeded
	task.CompletedAt = &completedAt
	task.UpdatedAt = completedAt
	if quality != "" {
		task.Quality = quality
	}
	if estimatedCost >= 0 {
		task.EstimatedCost = estimatedCost
	}
	if image != nil {
		task.ImageID = &image.ID
		task.Image = image
		if image.Size != "" {
			task.Size = image.Size
		}
	}
	s.tasks[taskID] = task
	return &task, nil
}

func (s *imageStudioTaskRepoStub) MarkFailed(ctx context.Context, taskID int64, reason string, message string, completedAt time.Time) (*ImageStudioTask, error) {
	s.ensure()
	task := s.tasks[taskID]
	task.Status = ImageStudioTaskStatusFailed
	task.ErrorReason = reason
	task.ErrorMessage = message
	task.CompletedAt = &completedAt
	task.UpdatedAt = completedAt
	s.tasks[taskID] = task
	return &task, nil
}

func (s *imageStudioTaskRepoStub) MarkStaleRunningFailed(ctx context.Context, olderThan time.Time, completedAt time.Time, reason string, message string) (int, error) {
	s.ensure()
	affected := 0
	for id, task := range s.tasks {
		if task.Status != ImageStudioTaskStatusRunning || task.StartedAt == nil || task.StartedAt.After(olderThan) {
			continue
		}
		task.Status = ImageStudioTaskStatusFailed
		task.ErrorReason = reason
		task.ErrorMessage = message
		task.CompletedAt = &completedAt
		task.UpdatedAt = completedAt
		s.tasks[id] = task
		affected++
	}
	return affected, nil
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
	s.getByIDID = id
	if s.getByIDErr != nil {
		return nil, s.getByIDErr
	}
	if s.getByIDRecord != nil {
		record := *s.getByIDRecord
		return &record, nil
	}
	return nil, ErrUserImageNotFound
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
	s.listExpiredNow = now
	s.listExpiredLimit = limit
	if s.listExpiredErr != nil {
		return nil, s.listExpiredErr
	}
	return append([]ImageStudioImageRecord(nil), s.listExpiredItems...), nil
}

type imageStudioExecutorStub struct {
	generateInput  *ImageStudioGenerateInput
	generateInputs []ImageStudioGenerateInput
	editInput      *ImageStudioEditInput
	result         *ImageStudioExecutionResult
	err            error
	results        []*ImageStudioExecutionResult
	errs           []error
}

func (s *imageStudioExecutorStub) Generate(ctx context.Context, input ImageStudioGenerateInput) (*ImageStudioExecutionResult, error) {
	s.generateInput = &input
	s.generateInputs = append(s.generateInputs, input)
	if len(s.errs) > 0 {
		err := s.errs[0]
		s.errs = s.errs[1:]
		if err != nil {
			return nil, err
		}
	}
	if len(s.results) > 0 {
		result := s.results[0]
		s.results = s.results[1:]
		return result, nil
	}
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
	deleteKeys     []string
	openKey        string
	openFile       *ImageStudioStoredFile
	openErr        error
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
	s.deleteKeys = append(s.deleteKeys, objectKey)
	return nil
}

func (s *imageStudioStorageStub) Open(ctx context.Context, objectKey string) (*ImageStudioStoredFile, error) {
	s.openKey = objectKey
	if s.openErr != nil {
		return nil, s.openErr
	}
	if s.openFile != nil {
		return s.openFile, nil
	}
	return &ImageStudioStoredFile{
		Name:        "stored.png",
		ContentType: "image/png",
		Size:        int64(len("stored-bytes")),
		Reader:      bytes.NewReader([]byte("stored-bytes")),
		Close:       func() error { return nil },
	}, nil
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

func TestImageStudioServiceGetOptionsUsesConfiguredDefaultModel(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	settings.AllowedModels = []string{"gpt-image-1", "gpt-image-2"}
	settings.DefaultModel = "gpt-image-2"

	svc := NewImageStudioService(&imageStudioRepoStub{}, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetGroupResolver(&imageStudioGroupResolverStub{groups: []Group{
		{
			ID:                   2,
			Name:                 "Image Pro",
			Platform:             PlatformOpenAI,
			Status:               StatusActive,
			AllowImageGeneration: true,
		},
	}})

	options, err := svc.GetOptions(context.Background(), 7)

	require.NoError(t, err)
	require.Equal(t, "gpt-image-2", options.DefaultModel)
	require.Len(t, options.Groups, 1)
	require.Equal(t, []ImageStudioModelOption{
		{Model: "gpt-image-1", Label: "gpt-image-1", Capabilities: []string{ImageStudioModeGeneration, ImageStudioModeEdit}},
		{Model: "gpt-image-2", Label: "gpt-image-2", Capabilities: []string{ImageStudioModeGeneration, ImageStudioModeEdit}},
	}, options.Groups[0].Models)
}

func TestImageStudioServiceGetOptionsUsesOnlyGroupCustomImageModelsWhenEnabled(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	settings.AllowedModels = []string{"gpt-image-2"}
	settings.DefaultModel = "gpt-image-2"

	svc := NewImageStudioService(&imageStudioRepoStub{}, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetGroupResolver(&imageStudioGroupResolverStub{groups: []Group{
		{
			ID:                   2,
			Name:                 "Image Pro",
			Platform:             PlatformOpenAI,
			Status:               StatusActive,
			AllowImageGeneration: true,
			ModelsListConfig: GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gpt-image-1"},
			},
		},
	}})

	options, err := svc.GetOptions(context.Background(), 7)

	require.NoError(t, err)
	require.Len(t, options.Groups, 1)
	require.Equal(t, []ImageStudioModelOption{{
		Model:        "gpt-image-1",
		Label:        "gpt-image-1",
		Capabilities: []string{ImageStudioModeGeneration, ImageStudioModeEdit},
	}}, options.Groups[0].Models)
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

func TestImageStudioServiceDownloadChecksUserAndOpensStoredImage(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	repo := &imageStudioRepoStub{
		getByIDRecord: &ImageStudioImageRecord{
			ID:               20,
			UserID:           7,
			StorageObjectKey: "images/user-7/2026/06/example.png",
			MimeType:         "image/png",
		},
	}
	storage := &imageStudioStorageStub{
		openFile: &ImageStudioStoredFile{
			Name:        "example.png",
			ContentType: "image/png",
			Size:        int64(len("png-bytes")),
			Reader:      bytes.NewReader([]byte("png-bytes")),
			Close:       func() error { return nil },
		},
	}
	svc := NewImageStudioService(repo, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetStorageFactory(func(ctx context.Context, cfg *ImageStudioSettings) (ImageStorage, error) {
		return storage, nil
	})

	file, err := svc.Download(context.Background(), 7, 20)
	require.NoError(t, err)
	require.Equal(t, int64(20), repo.getByIDID)
	require.Equal(t, "images/user-7/2026/06/example.png", storage.openKey)
	require.Equal(t, "passion-api-image-20.png", file.Name)
	require.Equal(t, "image/png", file.ContentType)
	body, err := io.ReadAll(file.Reader)
	require.NoError(t, err)
	require.Equal(t, []byte("png-bytes"), body)

	_, err = svc.Download(context.Background(), 8, 20)
	require.ErrorIs(t, err, ErrUserImageNotFound)
}

func TestImageStudioServiceCleanupExpiredImagesDeletesExpiredRecordsAndObjects(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	repo := &imageStudioRepoStub{
		listExpiredItems: []ImageStudioImageRecord{
			{ID: 11, UserID: 7, StorageObjectKey: "images/user-7/old-a.png"},
			{ID: 12, UserID: 7, StorageObjectKey: "images/user-7/old-b.png"},
		},
	}
	storage := &imageStudioStorageStub{}
	now := time.Date(2026, 6, 24, 16, 0, 0, 0, time.UTC)
	svc := NewImageStudioService(repo, &imageStudioConfigReaderStub{cfg: settings})
	svc.now = func() time.Time { return now }
	svc.SetStorageFactory(func(ctx context.Context, cfg *ImageStudioSettings) (ImageStorage, error) {
		return storage, nil
	})

	deleted, err := svc.CleanupExpiredImages(context.Background(), 50)

	require.NoError(t, err)
	require.Equal(t, 2, deleted)
	require.Equal(t, now, repo.listExpiredNow)
	require.Equal(t, 50, repo.listExpiredLimit)
	require.Equal(t, []string{"images/user-7/old-a.png", "images/user-7/old-b.png"}, storage.deleteKeys)
	require.Equal(t, int64(12), repo.deletedID)
	require.Equal(t, int64(7), repo.deletedUserID)
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

func TestImageStudioServiceCreateGenerationTaskCompletesAsync(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	settings.AllowedModels = []string{"gpt-image-1"}
	settings.DefaultModel = "gpt-image-1"
	executor := &imageStudioExecutorStub{
		result: &ImageStudioExecutionResult{
			ImageBytes: []byte("png-bytes"),
			MimeType:   "image/png",
			Cost:       0.02,
		},
	}
	storage := &imageStudioStorageStub{publicURL: "https://assets.example.com/task.png"}
	repo := &imageStudioRepoStub{}
	taskRepo := &imageStudioTaskRepoStub{}
	svc := NewImageStudioService(repo, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetExecutor(executor)
	svc.SetTaskRepository(taskRepo)
	svc.SetStorageFactory(func(ctx context.Context, cfg *ImageStudioSettings) (ImageStorage, error) {
		return storage, nil
	})

	task, err := svc.CreateTask(context.Background(), ImageStudioTaskCreateInput{
		Generate: &ImageStudioGenerateInput{
			UserID:      7,
			Model:       "gpt-image-1",
			Prompt:      "blue API portal",
			AspectRatio: "1:1",
			Quality:     ImageBillingSize1K,
		},
	})
	require.NoError(t, err)
	require.Equal(t, ImageStudioTaskStatusQueued, task.Status)
	require.Equal(t, ImageStudioModeGeneration, task.Mode)

	require.Eventually(t, func() bool {
		loaded, loadErr := svc.GetTask(context.Background(), 7, task.ID)
		return loadErr == nil &&
			loaded.Status == ImageStudioTaskStatusSucceeded &&
			loaded.ImageID != nil &&
			loaded.Image != nil &&
			loaded.Image.ImageURL == "https://assets.example.com/task.png"
	}, time.Second, 10*time.Millisecond)
	require.Len(t, repo.createdRecords, 1)
}

func TestImageStudioServiceGenerationTaskRetriesAndDowngrades4KTimeouts(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	settings.AllowedModels = []string{"gpt-image-2"}
	settings.DefaultModel = "gpt-image-2"
	executor := &imageStudioExecutorStub{
		errs: []error{
			io.ErrUnexpectedEOF,
			io.ErrUnexpectedEOF,
			io.ErrUnexpectedEOF,
			nil,
		},
		result: &ImageStudioExecutionResult{
			ImageBytes: []byte("png-bytes"),
			MimeType:   "image/png",
			Cost:       0.03,
		},
	}
	storage := &imageStudioStorageStub{publicURL: "https://assets.example.com/task.png"}
	repo := &imageStudioRepoStub{}
	taskRepo := &imageStudioTaskRepoStub{}
	svc := NewImageStudioService(repo, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetExecutor(executor)
	svc.SetTaskRepository(taskRepo)
	svc.SetStorageFactory(func(ctx context.Context, cfg *ImageStudioSettings) (ImageStorage, error) {
		return storage, nil
	})

	task, err := svc.CreateTask(context.Background(), ImageStudioTaskCreateInput{
		Generate: &ImageStudioGenerateInput{
			UserID:      7,
			Model:       "gpt-image-2",
			Prompt:      "blue API portal",
			AspectRatio: "16:9",
			Quality:     ImageBillingSize4K,
		},
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		loaded, loadErr := svc.GetTask(context.Background(), 7, task.ID)
		return loadErr == nil &&
			loaded.Status == ImageStudioTaskStatusSucceeded &&
			loaded.Image != nil &&
			loaded.Quality == ImageBillingSize1K &&
			loaded.Size == "1024x576"
	}, time.Second, 10*time.Millisecond)
	require.Len(t, executor.generateInputs, 4)
	require.Equal(t, []string{
		ImageBillingSize4K,
		ImageBillingSize4K,
		ImageBillingSize2K,
		ImageBillingSize1K,
	}, []string{
		executor.generateInputs[0].Quality,
		executor.generateInputs[1].Quality,
		executor.generateInputs[2].Quality,
		executor.generateInputs[3].Quality,
	})
	require.Equal(t, []string{
		"3840x2160",
		"3840x2160",
		"2048x1152",
		"1024x576",
	}, []string{
		executor.generateInputs[0].Size,
		executor.generateInputs[1].Size,
		executor.generateInputs[2].Size,
		executor.generateInputs[3].Size,
	})
	require.Len(t, repo.createdRecords, 1)
	require.Equal(t, "1024x576", repo.createdRecords[0].Size)
}

func TestImageStudioServiceMarksStaleRunningTasksInterruptedOnStartup(t *testing.T) {
	now := time.Date(2026, 6, 24, 16, 40, 0, 0, time.UTC)
	staleStartedAt := now.Add(-20 * time.Minute)
	freshStartedAt := now.Add(-2 * time.Minute)
	taskRepo := &imageStudioTaskRepoStub{
		tasks: map[int64]ImageStudioTask{
			1: {
				ID:        1,
				Status:    ImageStudioTaskStatusRunning,
				StartedAt: &staleStartedAt,
			},
			2: {
				ID:        2,
				Status:    ImageStudioTaskStatusRunning,
				StartedAt: &freshStartedAt,
			},
			3: {
				ID:     3,
				Status: ImageStudioTaskStatusQueued,
			},
		},
	}
	svc := NewImageStudioService(&imageStudioRepoStub{}, &imageStudioConfigReaderStub{})
	svc.SetTaskRepository(taskRepo)
	svc.now = func() time.Time { return now }

	affected, err := svc.MarkInterruptedRunningTasks(context.Background())

	require.NoError(t, err)
	require.Equal(t, 1, affected)
	require.Equal(t, ImageStudioTaskStatusFailed, taskRepo.tasks[1].Status)
	require.Equal(t, "IMAGE_TASK_INTERRUPTED_BY_RESTART", taskRepo.tasks[1].ErrorReason)
	require.Equal(t, ImageStudioTaskStatusRunning, taskRepo.tasks[2].Status)
	require.Equal(t, ImageStudioTaskStatusQueued, taskRepo.tasks[3].Status)
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

func TestImageStudioServiceGenerateAugmentsPromptWithAspectRatioForUpstreamOnly(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	settings.AllowedModels = []string{"gpt-image-2"}
	settings.DefaultModel = "gpt-image-2"
	repo := &imageStudioRepoStub{}
	executor := &imageStudioExecutorStub{
		result: &ImageStudioExecutionResult{
			ImageBytes: []byte("png-bytes"),
			MimeType:   "image/png",
		},
	}
	storage := &imageStudioStorageStub{publicURL: "https://assets.example.com/out.png"}
	svc := NewImageStudioService(repo, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetExecutor(executor)
	svc.SetStorageFactory(func(ctx context.Context, cfg *ImageStudioSettings) (ImageStorage, error) {
		return storage, nil
	})

	record, err := svc.Generate(context.Background(), ImageStudioGenerateInput{
		UserID:      7,
		Model:       "gpt-image-2",
		Prompt:      "生成一张海边风景图",
		AspectRatio: "9:16",
		Quality:     ImageBillingSize1K,
	})

	require.NoError(t, err)
	require.NotNil(t, record)
	require.NotNil(t, executor.generateInput)
	require.Equal(t, "生成一张海边风景图", repo.createdRecords[0].Prompt)
	require.Contains(t, executor.generateInput.Prompt, "生成一张海边风景图")
	require.Contains(t, executor.generateInput.Prompt, "9:16")
	require.Contains(t, executor.generateInput.Prompt, "vertical 9:16 portrait canvas")
	require.Contains(t, executor.generateInput.Prompt, "Do not create a wide, panoramic, letterboxed, or cropped composition")
	require.NotEqual(t, repo.createdRecords[0].Prompt, executor.generateInput.Prompt)
}

func TestImageStudioPromptWithAspectRatioGuidanceUsesStrongCanvasInstructions(t *testing.T) {
	prompt := ImageStudioPromptWithAspectRatioGuidance("a golden dragon on white paper", "1:1")

	require.Contains(t, prompt, "a golden dragon on white paper")
	require.Contains(t, prompt, "square 1:1 canvas")
	require.Contains(t, prompt, "final image canvas must be 1:1")
	require.Contains(t, prompt, "Do not create a wide, panoramic, letterboxed, or cropped composition")
	require.NotContains(t, prompt, "璇")
	require.NotContains(t, prompt, "鐢")
}

func TestImageStudioServiceGenerateNormalizesStoredImageToSelectedAspectRatio(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	settings.AllowedModels = []string{"gpt-image-2"}
	settings.DefaultModel = "gpt-image-2"
	sourcePNG := imageStudioTestPNG(t, 160, 90, color.RGBA{R: 25, G: 112, B: 245, A: 255})
	repo := &imageStudioRepoStub{}
	executor := &imageStudioExecutorStub{
		result: &ImageStudioExecutionResult{
			ImageBytes: sourcePNG,
			MimeType:   "image/png",
		},
	}
	storage := &imageStudioStorageStub{publicURL: "https://assets.example.com/out.png"}
	svc := NewImageStudioService(repo, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetExecutor(executor)
	svc.SetStorageFactory(func(ctx context.Context, cfg *ImageStudioSettings) (ImageStorage, error) {
		return storage, nil
	})

	record, err := svc.Generate(context.Background(), ImageStudioGenerateInput{
		UserID:      7,
		Model:       "gpt-image-2",
		Prompt:      "a clean app icon",
		AspectRatio: "1:1",
		Quality:     ImageBillingSize1K,
	})

	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, "1024x1024", record.Size)
	require.Equal(t, "image/png", record.MimeType)
	require.Equal(t, "image/png", storage.putContentType)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(storage.putData))
	require.NoError(t, err)
	require.Equal(t, 1024, cfg.Width)
	require.Equal(t, 1024, cfg.Height)
	require.Equal(t, int64(len(storage.putData)), record.Bytes)
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

func TestImageStudioServiceGenerateClassifiesUpstreamDisconnect(t *testing.T) {
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	settings.AllowedModels = []string{"gpt-image-1"}
	settings.DefaultModel = "gpt-image-1"
	repo := &imageStudioRepoStub{}
	executor := &imageStudioExecutorStub{err: io.ErrUnexpectedEOF}
	svc := NewImageStudioService(repo, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetExecutor(executor)

	_, err := svc.Generate(context.Background(), ImageStudioGenerateInput{
		UserID:      7,
		Model:       "gpt-image-1",
		Prompt:      "blue API portal",
		AspectRatio: "1:1",
	})

	require.Error(t, err)
	require.Equal(t, http.StatusGatewayTimeout, infraerrors.Code(err))
	require.Equal(t, "IMAGE_PROVIDER_TIMEOUT_OR_DISCONNECT", infraerrors.Reason(err))
	require.Contains(t, infraerrors.Message(err), "not charged")
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

func imageStudioTestPNG(t *testing.T, width int, height int, fill color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	stddraw.Draw(img, img.Bounds(), &image.Uniform{C: fill}, image.Point{}, stddraw.Src)
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}
