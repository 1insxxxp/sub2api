package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrImageStudioDisabled              = infraerrors.Forbidden("IMAGE_STUDIO_DISABLED", "image studio is disabled")
	ErrUserImageNotFound                = infraerrors.NotFound("USER_IMAGE_NOT_FOUND", "image record not found")
	ErrImageStudioFileNotFound          = infraerrors.NotFound("IMAGE_FILE_NOT_FOUND", "image file not found")
	ErrImageStudioExecutorMissing       = infraerrors.ServiceUnavailable("IMAGE_STUDIO_EXECUTOR_MISSING", "image studio executor is not configured")
	ErrImageStudioStorageMissing        = infraerrors.ServiceUnavailable("IMAGE_STUDIO_STORAGE_MISSING", "image studio storage is not configured")
	ErrImageStudioTaskNotFound          = infraerrors.NotFound("IMAGE_STUDIO_TASK_NOT_FOUND", "image task not found")
	ErrImageStudioTaskRepoMissing       = infraerrors.ServiceUnavailable("IMAGE_STUDIO_TASK_REPO_MISSING", "image studio task repository is not configured")
	ErrImageProviderTimeoutOrDisconnect = infraerrors.GatewayTimeout(
		"IMAGE_PROVIDER_TIMEOUT_OR_DISCONNECT",
		"image provider timed out or disconnected before returning an image; this request was not charged",
	)
)

type ImageStudioConfigReader interface {
	GetImageStudioConfig(ctx context.Context) (*ImageStudioSettings, error)
}

type ImageStudioGroupResolver interface {
	GetAvailableGroups(ctx context.Context, userID int64) ([]Group, error)
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

type UserImageTaskRepository interface {
	Create(ctx context.Context, task *ImageStudioTask) error
	GetByID(ctx context.Context, userID int64, taskID int64) (*ImageStudioTask, error)
	ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ImageStudioTask, *pagination.PaginationResult, error)
	MarkRunning(ctx context.Context, taskID int64, startedAt time.Time) (bool, error)
	MarkSucceeded(ctx context.Context, taskID int64, image *ImageStudioImageRecord, quality string, estimatedCost float64, completedAt time.Time) (*ImageStudioTask, error)
	MarkFailed(ctx context.Context, taskID int64, reason string, message string, completedAt time.Time) (*ImageStudioTask, error)
}

type ImageStudioExecutor interface {
	Generate(ctx context.Context, input ImageStudioGenerateInput) (*ImageStudioExecutionResult, error)
	Edit(ctx context.Context, input ImageStudioEditInput) (*ImageStudioExecutionResult, error)
}

type ImageStudioStorageFactory func(ctx context.Context, cfg *ImageStudioSettings) (ImageStorage, error)

type ImageStudioService struct {
	repo           UserImageRepository
	taskRepo       UserImageTaskRepository
	configReader   ImageStudioConfigReader
	groupResolver  ImageStudioGroupResolver
	executor       ImageStudioExecutor
	storageFactory ImageStudioStorageFactory
	taskQueue      chan int64
	workersStarted atomic.Bool
	now            func() time.Time
}

func NewImageStudioService(repo UserImageRepository, configReader ImageStudioConfigReader) *ImageStudioService {
	return &ImageStudioService{
		repo:         repo,
		configReader: configReader,
		taskQueue:    make(chan int64, 128),
		now:          time.Now,
	}
}

func (s *ImageStudioService) SetExecutor(executor ImageStudioExecutor) {
	s.executor = executor
}

func (s *ImageStudioService) SetGroupResolver(resolver ImageStudioGroupResolver) {
	s.groupResolver = resolver
}

func (s *ImageStudioService) SetTaskRepository(repo UserImageTaskRepository) {
	s.taskRepo = repo
}

func (s *ImageStudioService) SetStorageFactory(factory ImageStudioStorageFactory) {
	s.storageFactory = factory
}

func (s *ImageStudioService) GetConfig(ctx context.Context) (*ImageStudioConfig, error) {
	cfg, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	return imageStudioPublicConfig(cfg), nil
}

func (s *ImageStudioService) GetOptions(ctx context.Context, userID int64) (*ImageStudioOptions, error) {
	cfg, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	options := &ImageStudioOptions{
		Enabled:      cfg.Enabled,
		DefaultModel: firstImageStudioModel(cfg),
		Groups:       []ImageStudioGroupOption{},
	}
	if !cfg.Enabled {
		return options, nil
	}
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_IMAGE_STUDIO_USER", "user id is required")
	}
	groups, err := s.availableImageStudioGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		group := groups[i]
		option := imageStudioGroupOptionFromGroup(&group, cfg)
		if len(option.Models) == 0 {
			continue
		}
		if options.DefaultGroupID == nil {
			id := option.ID
			options.DefaultGroupID = &id
		}
		options.Groups = append(options.Groups, option)
	}
	return options, nil
}

func (s *ImageStudioService) CreateTask(ctx context.Context, input ImageStudioTaskCreateInput) (*ImageStudioTask, error) {
	if s.taskRepo == nil {
		return nil, ErrImageStudioTaskRepoMissing
	}
	var task *ImageStudioTask
	if input.Generate != nil {
		cfg, normalized, err := s.prepareGenerateInput(ctx, *input.Generate)
		if err != nil {
			return nil, err
		}
		task = s.taskFromGenerateInput(cfg, normalized)
	} else if input.Edit != nil {
		cfg, normalized, err := s.prepareEditInput(ctx, *input.Edit)
		if err != nil {
			return nil, err
		}
		task = s.taskFromEditInput(cfg, normalized)
	} else {
		return nil, infraerrors.BadRequest("INVALID_IMAGE_TASK", "image task payload is required")
	}
	if err := s.taskRepo.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("create image task: %w", err)
	}
	s.enqueueTask(task.ID)
	return task, nil
}

func (s *ImageStudioService) GetTask(ctx context.Context, userID int64, taskID int64) (*ImageStudioTask, error) {
	if s.taskRepo == nil {
		return nil, ErrImageStudioTaskRepoMissing
	}
	if userID <= 0 || taskID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_IMAGE_TASK", "invalid image task id")
	}
	return s.taskRepo.GetByID(ctx, userID, taskID)
}

func (s *ImageStudioService) ListTasks(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ImageStudioTask, *pagination.PaginationResult, error) {
	if s.taskRepo == nil {
		return nil, nil, ErrImageStudioTaskRepoMissing
	}
	if userID <= 0 {
		return nil, nil, infraerrors.BadRequest("INVALID_IMAGE_STUDIO_USER", "user id is required")
	}
	return s.taskRepo.ListByUser(ctx, userID, params)
}

func (s *ImageStudioService) StartTaskWorkers(workerCount int) {
	if workerCount <= 0 {
		workerCount = 2
	}
	if !s.workersStarted.CompareAndSwap(false, true) {
		return
	}
	for i := 0; i < workerCount; i++ {
		go func() {
			for taskID := range s.taskQueue {
				s.processTask(context.Background(), taskID)
			}
		}()
	}
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

func (s *ImageStudioService) Generate(ctx context.Context, input ImageStudioGenerateInput) (*ImageStudioImageRecord, error) {
	cfg, normalized, err := s.prepareGenerateInput(ctx, input)
	if err != nil {
		return nil, err
	}
	if s.executor == nil {
		return nil, ErrImageStudioExecutorMissing
	}
	executionInput := normalized
	executionInput.Prompt = ImageStudioPromptWithAspectRatioGuidance(normalized.Prompt, normalized.AspectRatio)
	result, err := s.executor.Generate(ctx, executionInput)
	if err != nil {
		return nil, imageStudioExecutionError("generate image", err)
	}
	return s.storeExecutionResult(ctx, cfg, ImageStudioModeGeneration, normalized.UserID, normalized.Model, normalized.Prompt, normalized.AspectRatio, normalized.Size, normalized.OutputFormat, normalized.Background, result)
}

func (s *ImageStudioService) Edit(ctx context.Context, input ImageStudioEditInput) (*ImageStudioImageRecord, error) {
	cfg, normalized, err := s.prepareEditInput(ctx, input)
	if err != nil {
		return nil, err
	}
	if s.executor == nil {
		return nil, ErrImageStudioExecutorMissing
	}
	executionInput := normalized
	executionInput.Prompt = ImageStudioPromptWithAspectRatioGuidance(normalized.Prompt, normalized.AspectRatio)
	result, err := s.executor.Edit(ctx, executionInput)
	if err != nil {
		return nil, imageStudioExecutionError("edit image", err)
	}
	return s.storeExecutionResult(ctx, cfg, ImageStudioModeEdit, normalized.UserID, normalized.Model, normalized.Prompt, normalized.AspectRatio, normalized.Size, normalized.OutputFormat, normalized.Background, result)
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

func (s *ImageStudioService) OpenLocalFile(ctx context.Context, objectKey string) (*ImageStudioLocalFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cfg, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, ErrImageStudioDisabled
	}
	if cfg.StorageDriver != ImageStorageDriverLocal {
		return nil, ErrImageStudioFileNotFound
	}
	objectKey = strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if err := validateImageStorageObjectKey(objectKey); err != nil {
		return nil, ErrImageStudioFileNotFound
	}
	storage, err := NewLocalImageStorage(LocalImageStorageConfig{RootDir: localImageStorageRootDir(cfg)})
	if err != nil {
		return nil, fmt.Errorf("open local image storage: %w", err)
	}
	target, err := storage.resolvePath(objectKey)
	if err != nil {
		return nil, ErrImageStudioFileNotFound
	}
	file, err := os.Open(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrImageStudioFileNotFound
		}
		return nil, fmt.Errorf("open image file: %w", err)
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat image file: %w", err)
	}
	if stat.IsDir() {
		_ = file.Close()
		return nil, ErrImageStudioFileNotFound
	}
	contentType := mime.TypeByExtension(filepath.Ext(objectKey))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return &ImageStudioLocalFile{
		Name:        filepath.Base(objectKey),
		ContentType: contentType,
		ModTime:     stat.ModTime(),
		Size:        stat.Size(),
		Reader:      file,
		Close:       file.Close,
	}, nil
}

func (s *ImageStudioService) prepareGenerateInput(ctx context.Context, input ImageStudioGenerateInput) (*ImageStudioSettings, ImageStudioGenerateInput, error) {
	cfg, err := s.loadSettings(ctx)
	if err != nil {
		return nil, ImageStudioGenerateInput{}, err
	}
	if !cfg.Enabled {
		return nil, ImageStudioGenerateInput{}, ErrImageStudioDisabled
	}
	if input.UserID <= 0 {
		return nil, ImageStudioGenerateInput{}, infraerrors.BadRequest("INVALID_IMAGE_STUDIO_USER", "user id is required")
	}
	prompt, err := NormalizeImageStudioPrompt(input.Prompt)
	if err != nil {
		return nil, ImageStudioGenerateInput{}, infraerrors.BadRequest("INVALID_IMAGE_PROMPT", err.Error())
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = cfg.DefaultModel
	}
	group, err := s.resolveSelectedImageStudioGroup(ctx, input.UserID, input.GroupID, model, cfg)
	if err != nil {
		return nil, ImageStudioGenerateInput{}, err
	}
	if group == nil {
		if err := ValidateImageStudioModel(model, cfg.AllowedModels); err != nil {
			return nil, ImageStudioGenerateInput{}, infraerrors.BadRequest("INVALID_IMAGE_MODEL", err.Error())
		}
	}
	if group != nil && input.GroupID == nil {
		input.GroupID = &group.ID
	}
	aspectRatio := firstNonEmptyTrimmed(input.AspectRatio, "1:1")
	quality := NormalizeImageStudioQuality(input.Quality)
	size, billingTier, err := ResolveImageStudioRenderSpec(aspectRatio, quality)
	if err != nil {
		return nil, ImageStudioGenerateInput{}, infraerrors.BadRequest("INVALID_IMAGE_RENDER_SPEC", err.Error())
	}
	outputFormat := NormalizeImageStudioOutputFormat(input.OutputFormat)
	background := NormalizeImageStudioBackground(input.Background)
	if err := ValidateImageStudioOutputOptionsForModel(model, outputFormat, background); err != nil {
		return nil, ImageStudioGenerateInput{}, infraerrors.BadRequest("INVALID_IMAGE_OUTPUT_OPTIONS", err.Error())
	}
	input.Model = model
	input.Prompt = prompt
	input.AspectRatio = aspectRatio
	input.Quality = firstNonEmptyTrimmed(quality, billingTier)
	input.OutputFormat = outputFormat
	input.Background = background
	input.Size = size
	input.BillingTier = billingTier
	return cfg, input, nil
}

func (s *ImageStudioService) prepareEditInput(ctx context.Context, input ImageStudioEditInput) (*ImageStudioSettings, ImageStudioEditInput, error) {
	cfg, err := s.loadSettings(ctx)
	if err != nil {
		return nil, ImageStudioEditInput{}, err
	}
	if !cfg.Enabled {
		return nil, ImageStudioEditInput{}, ErrImageStudioDisabled
	}
	if input.UserID <= 0 {
		return nil, ImageStudioEditInput{}, infraerrors.BadRequest("INVALID_IMAGE_STUDIO_USER", "user id is required")
	}
	prompt, err := NormalizeImageStudioPrompt(input.Prompt)
	if err != nil {
		return nil, ImageStudioEditInput{}, infraerrors.BadRequest("INVALID_IMAGE_PROMPT", err.Error())
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = cfg.DefaultModel
	}
	group, err := s.resolveSelectedImageStudioGroup(ctx, input.UserID, input.GroupID, model, cfg)
	if err != nil {
		return nil, ImageStudioEditInput{}, err
	}
	if group == nil {
		if err := ValidateImageStudioModel(model, cfg.AllowedModels); err != nil {
			return nil, ImageStudioEditInput{}, infraerrors.BadRequest("INVALID_IMAGE_MODEL", err.Error())
		}
	}
	if group != nil && input.GroupID == nil {
		input.GroupID = &group.ID
	}
	aspectRatio := firstNonEmptyTrimmed(input.AspectRatio, "1:1")
	quality := NormalizeImageStudioQuality(input.Quality)
	size, billingTier, err := ResolveImageStudioRenderSpec(aspectRatio, quality)
	if err != nil {
		return nil, ImageStudioEditInput{}, infraerrors.BadRequest("INVALID_IMAGE_RENDER_SPEC", err.Error())
	}
	outputFormat := NormalizeImageStudioOutputFormat(input.OutputFormat)
	background := NormalizeImageStudioBackground(input.Background)
	if err := ValidateImageStudioOutputOptionsForModel(model, outputFormat, background); err != nil {
		return nil, ImageStudioEditInput{}, infraerrors.BadRequest("INVALID_IMAGE_OUTPUT_OPTIONS", err.Error())
	}
	if len(input.ReferenceImages) == 0 {
		return nil, ImageStudioEditInput{}, infraerrors.BadRequest("IMAGE_REFERENCE_REQUIRED", "reference image is required")
	}
	maxBytes := int64(cfg.MaxReferenceImageMB) << 20
	for _, image := range input.ReferenceImages {
		if len(image.Data) == 0 {
			return nil, ImageStudioEditInput{}, infraerrors.BadRequest("INVALID_REFERENCE_IMAGE", "reference image is empty")
		}
		if maxBytes > 0 && int64(len(image.Data)) > maxBytes {
			return nil, ImageStudioEditInput{}, infraerrors.BadRequest("REFERENCE_IMAGE_TOO_LARGE", fmt.Sprintf("reference image exceeds %dMB", cfg.MaxReferenceImageMB))
		}
		contentType := strings.TrimSpace(image.ContentType)
		if contentType == "" {
			contentType = mime.TypeByExtension(filepath.Ext(image.FileName))
		}
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
			return nil, ImageStudioEditInput{}, infraerrors.BadRequest("INVALID_REFERENCE_IMAGE", "reference file must be an image")
		}
	}
	input.Model = model
	input.Prompt = prompt
	input.AspectRatio = aspectRatio
	input.Quality = firstNonEmptyTrimmed(quality, billingTier)
	input.OutputFormat = outputFormat
	input.Background = background
	input.Size = size
	input.BillingTier = billingTier
	return cfg, input, nil
}

func (s *ImageStudioService) resolveSelectedImageStudioGroup(ctx context.Context, userID int64, groupID *int64, model string, cfg *ImageStudioSettings) (*Group, error) {
	if s.groupResolver == nil {
		return nil, nil
	}
	groups, err := s.availableImageStudioGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, infraerrors.Forbidden("IMAGE_STUDIO_NO_GROUP", "no image-enabled group is available")
	}
	selectedID := int64(0)
	if groupID != nil {
		selectedID = *groupID
	} else if len(groups) == 1 {
		selectedID = groups[0].ID
	} else {
		return nil, infraerrors.BadRequest("IMAGE_STUDIO_GROUP_REQUIRED", "image group is required")
	}
	for i := range groups {
		group := groups[i]
		if group.ID != selectedID {
			continue
		}
		if err := ValidateImageStudioModel(model, imageStudioModelsForGroup(&group, cfg)); err != nil {
			return nil, infraerrors.BadRequest("IMAGE_STUDIO_MODEL_NOT_AVAILABLE", err.Error())
		}
		return &group, nil
	}
	return nil, infraerrors.BadRequest("IMAGE_STUDIO_GROUP_NOT_AVAILABLE", "selected image group is not available")
}

func (s *ImageStudioService) availableImageStudioGroups(ctx context.Context, userID int64) ([]Group, error) {
	if s.groupResolver == nil {
		return nil, nil
	}
	groups, err := s.groupResolver.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list image studio groups: %w", err)
	}
	out := make([]Group, 0, len(groups))
	for _, group := range groups {
		if group.ID <= 0 || !group.IsActive() || !GroupAllowsImageGeneration(&group) {
			continue
		}
		out = append(out, group)
	}
	return out, nil
}

func imageStudioGroupOptionFromGroup(group *Group, cfg *ImageStudioSettings) ImageStudioGroupOption {
	if group == nil {
		return ImageStudioGroupOption{}
	}
	models := imageStudioModelsForGroup(group, cfg)
	qualityOptions := imageStudioQualityOptionsForGroup(group)
	return ImageStudioGroupOption{
		ID:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		Platform:    group.Platform,
		Models:      imageStudioModelOptions(models),
		Qualities:   qualityOptions,
		Prices:      imageStudioPricePreviewItems(group, qualityOptions),
	}
}

func imageStudioModelsForGroup(group *Group, cfg *ImageStudioSettings) []string {
	var models []string
	if group != nil && group.CustomModelsListEnabled() {
		for _, model := range group.ModelsListConfig.Models {
			if isOpenAIImageGenerationModel(model) {
				models = append(models, model)
			}
		}
		return dedupeImageStudioModels(models)
	}
	if len(models) == 0 && cfg != nil {
		models = append(models, cfg.AllowedModels...)
	}
	models = append(models, "gpt-image-2")
	return dedupeImageStudioModels(models)
}

func imageStudioModelOptions(models []string) []ImageStudioModelOption {
	out := make([]ImageStudioModelOption, 0, len(models))
	for _, model := range dedupeImageStudioModels(models) {
		out = append(out, ImageStudioModelOption{
			Model:        model,
			Label:        model,
			Capabilities: []string{ImageStudioModeGeneration, ImageStudioModeEdit},
		})
	}
	return out
}

func imageStudioQualityOptionsForGroup(group *Group) []ImageStudioQualityOption {
	options := SupportedImageStudioQualities()
	for i := range options {
		options[i].EstimatedCost = estimateImageStudioCost(group, options[i].BillingTier)
	}
	return options
}

func imageStudioPricePreviewItems(group *Group, qualities []ImageStudioQualityOption) []ImageStudioPricePreviewItem {
	items := make([]ImageStudioPricePreviewItem, 0, len(SupportedImageStudioAspectRatios())*len(qualities))
	for _, ratio := range SupportedImageStudioAspectRatios() {
		for _, quality := range qualities {
			size, billingTier, err := ResolveImageStudioRenderSpec(ratio.Ratio, quality.Quality)
			if err != nil {
				continue
			}
			items = append(items, ImageStudioPricePreviewItem{
				Ratio:         ratio.Ratio,
				Quality:       quality.Quality,
				Size:          size,
				BillingTier:   billingTier,
				EstimatedCost: estimateImageStudioCost(group, billingTier),
			})
		}
	}
	return items
}

func estimateImageStudioCost(group *Group, billingTier string) float64 {
	if group != nil {
		if price := group.GetImagePrice(billingTier); price != nil {
			return *price * normalizeImageStudioImageMultiplier(group)
		}
	}
	base := 0.134
	switch NormalizeImageBillingTierOrDefault(billingTier) {
	case ImageBillingSize2K:
		base *= 1.5
	case ImageBillingSize4K:
		base *= 2
	}
	return base * normalizeImageStudioImageMultiplier(group)
}

func normalizeImageStudioImageMultiplier(group *Group) float64 {
	if group == nil {
		return 1
	}
	if group.ImageRateIndependent {
		if group.ImageRateMultiplier < 0 {
			return 0
		}
		return group.ImageRateMultiplier
	}
	if group.RateMultiplier > 0 {
		return group.RateMultiplier
	}
	return 1
}

func dedupeImageStudioModels(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	out := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" || !isOpenAIImageGenerationModel(model) {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}
	return out
}

func firstImageStudioModel(cfg *ImageStudioSettings) string {
	models := imageStudioModelsForGroup(nil, cfg)
	if len(models) > 0 {
		return models[0]
	}
	return "gpt-image-2"
}

func (s *ImageStudioService) storeExecutionResult(
	ctx context.Context,
	cfg *ImageStudioSettings,
	mode string,
	userID int64,
	model string,
	prompt string,
	aspectRatio string,
	size string,
	outputFormat string,
	background string,
	result *ImageStudioExecutionResult,
) (*ImageStudioImageRecord, error) {
	if result == nil || len(result.ImageBytes) == 0 {
		return nil, infraerrors.ServiceUnavailable("IMAGE_GENERATION_EMPTY_RESULT", "image provider returned no image")
	}
	storage, err := s.resolveStorage(ctx, cfg)
	if err != nil {
		return nil, err
	}
	mimeType := strings.TrimSpace(result.MimeType)
	if mimeType == "" {
		mimeType = "image/png"
	}
	finalOutputFormat := firstNonEmptyTrimmed(result.OutputFormat, outputFormat, imageStudioOutputFormatFromMIMEType(mimeType))
	finalOutputFormat = NormalizeImageStudioOutputFormat(finalOutputFormat)
	finalBackground := firstNonEmptyTrimmed(result.Background, background, "auto")
	finalBackground = NormalizeImageStudioBackground(finalBackground)
	now := time.Now()
	objectKey, err := GenerateImageStorageObjectKey(userID, mimeType, now)
	if err != nil {
		return nil, fmt.Errorf("generate image object key: %w", err)
	}
	imageURL, err := storage.Put(ctx, objectKey, mimeType, result.ImageBytes)
	if err != nil {
		return nil, fmt.Errorf("store generated image: %w", err)
	}
	if result.CommitUsage != nil {
		if err := result.CommitUsage(ctx); err != nil {
			_ = storage.Delete(context.WithoutCancel(ctx), objectKey)
			return nil, fmt.Errorf("commit image usage: %w", err)
		}
	}
	expiresAt := now.Add(time.Duration(cfg.RetentionDays) * 24 * time.Hour)
	record := &ImageStudioImageRecord{
		UserID:           userID,
		Mode:             mode,
		Model:            strings.TrimSpace(model),
		Prompt:           strings.TrimSpace(prompt),
		AspectRatio:      strings.TrimSpace(aspectRatio),
		Size:             strings.TrimSpace(size),
		ImageURL:         imageURL,
		StorageDriver:    cfg.StorageDriver,
		StorageObjectKey: objectKey,
		MimeType:         mimeType,
		OutputFormat:     finalOutputFormat,
		Background:       finalBackground,
		Bytes:            int64(len(result.ImageBytes)),
		Cost:             result.Cost,
		UsageLogID:       result.UsageLogID,
		SourceImageCount: result.SourceImageCount,
		ExpiresAt:        &expiresAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.repo.Create(ctx, record); err != nil {
		_ = storage.Delete(context.WithoutCancel(ctx), objectKey)
		return nil, fmt.Errorf("create user image record: %w", err)
	}
	if cfg.MaxImagesPerUser > 0 {
		if oldRecords, cleanupErr := s.repo.DeleteOldestOverLimit(ctx, userID, cfg.MaxImagesPerUser); cleanupErr == nil {
			for _, old := range oldRecords {
				if old.StorageObjectKey != "" {
					_ = storage.Delete(context.WithoutCancel(ctx), old.StorageObjectKey)
				}
			}
		}
	}
	return record, nil
}

func (s *ImageStudioService) resolveStorage(ctx context.Context, cfg *ImageStudioSettings) (ImageStorage, error) {
	if s.storageFactory != nil {
		storage, err := s.storageFactory(ctx, cfg)
		if err != nil {
			return nil, err
		}
		if storage == nil {
			return nil, ErrImageStudioStorageMissing
		}
		return storage, nil
	}
	return defaultImageStudioStorageFactory(ctx, cfg)
}

func defaultImageStudioStorageFactory(ctx context.Context, cfg *ImageStudioSettings) (ImageStorage, error) {
	cfg, _ = normalizeImageStudioSettings(cfg)
	switch cfg.StorageDriver {
	case ImageStorageDriverLocal:
		rootDir := localImageStorageRootDir(cfg)
		publicBaseURL := strings.TrimSpace(cfg.LocalPublicBaseURL)
		if publicBaseURL == "" {
			publicBaseURL = "/api/v1/user/images/files"
		}
		return NewLocalImageStorage(LocalImageStorageConfig{RootDir: rootDir, PublicBaseURL: publicBaseURL})
	case ImageStorageDriverR2:
		return nil, infraerrors.ServiceUnavailable("IMAGE_STUDIO_R2_NOT_CONFIGURED", "r2 storage requires server credentials")
	default:
		return nil, ErrImageStudioStorageMissing
	}
}

func localImageStorageRootDir(cfg *ImageStudioSettings) string {
	if cfg != nil {
		if rootDir := strings.TrimSpace(cfg.LocalRootDir); rootDir != "" {
			return rootDir
		}
	}
	return filepath.Join(os.TempDir(), "sub2api-image-studio")
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

func (s *ImageStudioService) taskFromGenerateInput(_ *ImageStudioSettings, input ImageStudioGenerateInput) *ImageStudioTask {
	now := s.now()
	return &ImageStudioTask{
		UserID:        input.UserID,
		APIKeyID:      input.APIKeyID,
		GroupID:       input.GroupID,
		Mode:          ImageStudioModeGeneration,
		Status:        ImageStudioTaskStatusQueued,
		Model:         strings.TrimSpace(input.Model),
		Prompt:        strings.TrimSpace(input.Prompt),
		AspectRatio:   strings.TrimSpace(input.AspectRatio),
		Quality:       strings.TrimSpace(input.Quality),
		OutputFormat:  strings.TrimSpace(input.OutputFormat),
		Background:    strings.TrimSpace(input.Background),
		Size:          strings.TrimSpace(input.Size),
		EstimatedCost: estimateImageStudioCost(nil, input.BillingTier),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func (s *ImageStudioService) taskFromEditInput(_ *ImageStudioSettings, input ImageStudioEditInput) *ImageStudioTask {
	now := s.now()
	return &ImageStudioTask{
		UserID:           input.UserID,
		APIKeyID:         input.APIKeyID,
		GroupID:          input.GroupID,
		Mode:             ImageStudioModeEdit,
		Status:           ImageStudioTaskStatusQueued,
		Model:            strings.TrimSpace(input.Model),
		Prompt:           strings.TrimSpace(input.Prompt),
		AspectRatio:      strings.TrimSpace(input.AspectRatio),
		Quality:          strings.TrimSpace(input.Quality),
		OutputFormat:     strings.TrimSpace(input.OutputFormat),
		Background:       strings.TrimSpace(input.Background),
		Size:             strings.TrimSpace(input.Size),
		EstimatedCost:    estimateImageStudioCost(nil, input.BillingTier),
		SourceImageCount: len(input.ReferenceImages),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func (s *ImageStudioService) enqueueTask(taskID int64) {
	if taskID <= 0 {
		return
	}
	if !s.workersStarted.Load() {
		go s.processTask(context.Background(), taskID)
		return
	}
	select {
	case s.taskQueue <- taskID:
	default:
		go s.processTask(context.Background(), taskID)
	}
}

func (s *ImageStudioService) processTask(ctx context.Context, taskID int64) {
	if s.taskRepo == nil || taskID <= 0 {
		return
	}
	startedAt := s.now()
	claimed, err := s.taskRepo.MarkRunning(ctx, taskID, startedAt)
	if err != nil || !claimed {
		return
	}
	task, err := s.taskRepo.GetByID(ctx, 0, taskID)
	if err != nil {
		return
	}

	var image *ImageStudioImageRecord
	finalQuality := task.Quality
	finalEstimatedCost := task.EstimatedCost
	switch task.Mode {
	case ImageStudioModeGeneration:
		input := ImageStudioGenerateInput{
			UserID:       task.UserID,
			APIKeyID:     task.APIKeyID,
			GroupID:      task.GroupID,
			Model:        task.Model,
			Prompt:       task.Prompt,
			AspectRatio:  task.AspectRatio,
			Quality:      task.Quality,
			OutputFormat: task.OutputFormat,
			Background:   task.Background,
			Size:         task.Size,
		}
		image, finalQuality, finalEstimatedCost, err = s.generateTaskWithRecovery(ctx, input, task.EstimatedCost)
	default:
		err = infraerrors.BadRequest("IMAGE_TASK_MODE_UNSUPPORTED", "image edit tasks are not available yet")
	}

	completedAt := s.now()
	if err != nil {
		_, _ = s.taskRepo.MarkFailed(ctx, taskID, infraerrors.Reason(err), infraerrors.Message(err), completedAt)
		return
	}
	_, _ = s.taskRepo.MarkSucceeded(ctx, taskID, image, finalQuality, finalEstimatedCost, completedAt)
}

func (s *ImageStudioService) generateTaskWithRecovery(ctx context.Context, input ImageStudioGenerateInput, initialEstimatedCost float64) (*ImageStudioImageRecord, string, float64, error) {
	attempts := imageStudioGenerationRecoveryAttempts(input)
	if len(attempts) == 0 {
		attempts = []ImageStudioGenerateInput{input}
	}
	var lastErr error
	for _, attempt := range attempts {
		image, err := s.Generate(ctx, attempt)
		if err == nil {
			return image, attempt.Quality, imageStudioEstimatedCostForAttempt(attempt, initialEstimatedCost), nil
		}
		lastErr = err
		if !imageStudioProviderTimedOutOrDisconnected(err) {
			break
		}
	}
	return nil, input.Quality, initialEstimatedCost, lastErr
}

func imageStudioGenerationRecoveryAttempts(input ImageStudioGenerateInput) []ImageStudioGenerateInput {
	base := normalizeImageStudioTaskAttempt(input)
	attempts := []ImageStudioGenerateInput{base}
	if NormalizeImageStudioQuality(base.Quality) == ImageBillingSize4K {
		attempts = append(attempts, base)
		for _, quality := range []string{ImageBillingSize2K, ImageBillingSize1K} {
			downgraded, ok := imageStudioTaskAttemptWithQuality(base, quality)
			if ok {
				attempts = append(attempts, downgraded)
			}
		}
		return attempts
	}
	attempts = append(attempts, base)
	return attempts
}

func normalizeImageStudioTaskAttempt(input ImageStudioGenerateInput) ImageStudioGenerateInput {
	out := input
	quality := NormalizeImageStudioQuality(out.Quality)
	size, billingTier, err := ResolveImageStudioRenderSpec(out.AspectRatio, quality)
	if err != nil {
		return out
	}
	out.Quality = firstNonEmptyTrimmed(quality, billingTier)
	out.Size = size
	out.BillingTier = billingTier
	return out
}

func imageStudioTaskAttemptWithQuality(input ImageStudioGenerateInput, quality string) (ImageStudioGenerateInput, bool) {
	size, billingTier, err := ResolveImageStudioRenderSpec(input.AspectRatio, quality)
	if err != nil {
		return ImageStudioGenerateInput{}, false
	}
	out := input
	out.Quality = billingTier
	out.Size = size
	out.BillingTier = billingTier
	return out, true
}

func imageStudioEstimatedCostForAttempt(input ImageStudioGenerateInput, fallback float64) float64 {
	cost := estimateImageStudioCost(nil, input.BillingTier)
	if cost > 0 {
		return cost
	}
	return fallback
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func imageStudioExecutionError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if imageStudioProviderTimedOutOrDisconnected(err) {
		return fmt.Errorf("%s: %w", operation, ErrImageProviderTimeoutOrDisconnect.WithCause(err))
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func imageStudioProviderTimedOutOrDisconnected(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"unexpected eof",
		"connection reset",
		"connection refused",
		"connection aborted",
		"server closed idle connection",
		"use of closed network connection",
		"timeout awaiting response headers",
		"client.timeout exceeded",
		"i/o timeout",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
