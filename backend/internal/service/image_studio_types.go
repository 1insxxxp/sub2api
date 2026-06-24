package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	ImageStudioModeGeneration = "generation"
	ImageStudioModeEdit       = "edit"
)

const (
	ImageStudioTaskStatusQueued    = "queued"
	ImageStudioTaskStatusRunning   = "running"
	ImageStudioTaskStatusSucceeded = "succeeded"
	ImageStudioTaskStatusFailed    = "failed"
)

type ImageStudioAspectRatio struct {
	Ratio       string `json:"ratio"`
	Size        string `json:"size"`
	BillingTier string `json:"billing_tier"`
}

type ImageStudioConfig struct {
	Enabled             bool                     `json:"enabled"`
	AllowedModels       []string                 `json:"allowed_models"`
	DefaultModel        string                   `json:"default_model"`
	AspectRatios        []ImageStudioAspectRatio `json:"aspect_ratios"`
	MaxReferenceImageMB int                      `json:"max_reference_image_mb"`
	RetentionDays       int                      `json:"retention_days"`
	MaxImagesPerUser    int                      `json:"max_images_per_user"`
}

type ImageStudioOptions struct {
	Enabled        bool                     `json:"enabled"`
	DefaultGroupID *int64                   `json:"default_group_id,omitempty"`
	DefaultModel   string                   `json:"default_model"`
	Groups         []ImageStudioGroupOption `json:"groups"`
}

type ImageStudioGroupOption struct {
	ID          int64                         `json:"id"`
	Name        string                        `json:"name"`
	Description string                        `json:"description,omitempty"`
	Platform    string                        `json:"platform"`
	Models      []ImageStudioModelOption      `json:"models"`
	Qualities   []ImageStudioQualityOption    `json:"qualities"`
	Prices      []ImageStudioPricePreviewItem `json:"prices"`
}

type ImageStudioModelOption struct {
	Model        string   `json:"model"`
	Label        string   `json:"label"`
	MappedModel  string   `json:"mapped_model,omitempty"`
	Capabilities []string `json:"capabilities"`
}

type ImageStudioQualityOption struct {
	Quality       string  `json:"quality"`
	Label         string  `json:"label"`
	BillingTier   string  `json:"billing_tier"`
	EstimatedCost float64 `json:"estimated_cost"`
}

type ImageStudioPricePreviewItem struct {
	Ratio         string  `json:"ratio"`
	Quality       string  `json:"quality"`
	Size          string  `json:"size"`
	BillingTier   string  `json:"billing_tier"`
	EstimatedCost float64 `json:"estimated_cost"`
}

type ImageStudioGenerateInput struct {
	UserID       int64  `json:"user_id"`
	APIKeyID     *int64 `json:"api_key_id,omitempty"`
	GroupID      *int64 `json:"group_id,omitempty"`
	Model        string `json:"model"`
	Prompt       string `json:"prompt"`
	AspectRatio  string `json:"aspect_ratio"`
	Quality      string `json:"quality,omitempty"`
	OutputFormat string `json:"output_format,omitempty"`
	Background   string `json:"background,omitempty"`
	Size         string `json:"size,omitempty"`
	BillingTier  string `json:"billing_tier,omitempty"`
	UserAgent    string `json:"-"`
	IPAddress    string `json:"-"`
}

type ImageStudioEditInput struct {
	UserID          int64                       `json:"user_id"`
	APIKeyID        *int64                      `json:"api_key_id,omitempty"`
	GroupID         *int64                      `json:"group_id,omitempty"`
	Model           string                      `json:"model"`
	Prompt          string                      `json:"prompt"`
	AspectRatio     string                      `json:"aspect_ratio"`
	Quality         string                      `json:"quality,omitempty"`
	OutputFormat    string                      `json:"output_format,omitempty"`
	Background      string                      `json:"background,omitempty"`
	Size            string                      `json:"size,omitempty"`
	BillingTier     string                      `json:"billing_tier,omitempty"`
	ReferenceImages []ImageStudioReferenceImage `json:"-"`
	UserAgent       string                      `json:"-"`
	IPAddress       string                      `json:"-"`
}

type ImageStudioReferenceImage struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Data        []byte `json:"-"`
}

type ImageStudioExecutionResult struct {
	ImageBytes       []byte                      `json:"-"`
	MimeType         string                      `json:"mime_type"`
	OutputFormat     string                      `json:"output_format"`
	Background       string                      `json:"background"`
	Cost             float64                     `json:"cost"`
	UsageLogID       *int64                      `json:"usage_log_id,omitempty"`
	RequestID        string                      `json:"request_id,omitempty"`
	SourceImageCount int                         `json:"source_image_count"`
	CommitUsage      func(context.Context) error `json:"-"`
}

type ImageStudioImageRecord struct {
	ID               int64      `json:"id"`
	UserID           int64      `json:"user_id"`
	Mode             string     `json:"mode"`
	Model            string     `json:"model"`
	Prompt           string     `json:"prompt"`
	AspectRatio      string     `json:"aspect_ratio"`
	Size             string     `json:"size"`
	ImageURL         string     `json:"image_url"`
	StorageDriver    string     `json:"storage_driver"`
	StorageObjectKey string     `json:"storage_object_key"`
	MimeType         string     `json:"mime_type"`
	OutputFormat     string     `json:"output_format"`
	Background       string     `json:"background"`
	Bytes            int64      `json:"bytes"`
	Cost             float64    `json:"cost"`
	UsageLogID       *int64     `json:"usage_log_id,omitempty"`
	SourceImageCount int        `json:"source_image_count"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type ImageStudioTask struct {
	ID                  int64                   `json:"id"`
	UserID              int64                   `json:"user_id"`
	APIKeyID            *int64                  `json:"api_key_id,omitempty"`
	GroupID             *int64                  `json:"group_id,omitempty"`
	ImageID             *int64                  `json:"image_id,omitempty"`
	Image               *ImageStudioImageRecord `json:"image,omitempty"`
	Mode                string                  `json:"mode"`
	Status              string                  `json:"status"`
	Model               string                  `json:"model"`
	Prompt              string                  `json:"prompt"`
	AspectRatio         string                  `json:"aspect_ratio"`
	Quality             string                  `json:"quality"`
	OutputFormat        string                  `json:"output_format"`
	Background          string                  `json:"background"`
	Size                string                  `json:"size"`
	EstimatedCost       float64                 `json:"estimated_cost"`
	SourceImageCount    int                     `json:"source_image_count"`
	ReferenceObjectKeys []string                `json:"reference_object_keys,omitempty"`
	ErrorReason         string                  `json:"error_reason,omitempty"`
	ErrorMessage        string                  `json:"error_message,omitempty"`
	StartedAt           *time.Time              `json:"started_at,omitempty"`
	CompletedAt         *time.Time              `json:"completed_at,omitempty"`
	CreatedAt           time.Time               `json:"created_at"`
	UpdatedAt           time.Time               `json:"updated_at"`
}

type ImageStudioTaskCreateInput struct {
	Generate *ImageStudioGenerateInput
	Edit     *ImageStudioEditInput
}

type ImageStudioLocalFile struct {
	Name        string
	ContentType string
	ModTime     time.Time
	Size        int64
	Reader      io.ReadSeeker
	Close       func() error
}

type ImageStudioDownloadFile struct {
	Name        string
	ContentType string
	ModTime     time.Time
	Size        int64
	Reader      io.ReadSeeker
	Close       func() error
}

type ImageStudioPricePreview struct {
	AspectRatio string  `json:"aspect_ratio"`
	Size        string  `json:"size"`
	BillingTier string  `json:"billing_tier"`
	Estimated   float64 `json:"estimated"`
}

func SupportedImageStudioAspectRatios() []ImageStudioAspectRatio {
	return []ImageStudioAspectRatio{
		{Ratio: "1:1", Size: "1024x1024", BillingTier: ImageBillingSize1K},
		{Ratio: "16:9", Size: "1536x864", BillingTier: ImageBillingSize2K},
		{Ratio: "9:16", Size: "864x1536", BillingTier: ImageBillingSize2K},
		{Ratio: "4:3", Size: "1024x768", BillingTier: ImageBillingSize1K},
		{Ratio: "3:4", Size: "768x1024", BillingTier: ImageBillingSize1K},
	}
}

func SupportedImageStudioQualities() []ImageStudioQualityOption {
	return []ImageStudioQualityOption{
		{Quality: ImageBillingSize1K, Label: ImageBillingSize1K, BillingTier: ImageBillingSize1K},
		{Quality: ImageBillingSize2K, Label: ImageBillingSize2K, BillingTier: ImageBillingSize2K},
		{Quality: ImageBillingSize4K, Label: ImageBillingSize4K, BillingTier: ImageBillingSize4K},
	}
}

func ResolveImageStudioAspectRatio(ratio string) (string, string, error) {
	normalized := strings.TrimSpace(ratio)
	for _, item := range SupportedImageStudioAspectRatios() {
		if item.Ratio == normalized {
			return item.Size, item.BillingTier, nil
		}
	}
	return "", "", fmt.Errorf("unsupported image aspect ratio: %s", ratio)
}

func ResolveImageStudioRenderSpec(ratio string, quality string) (string, string, error) {
	normalizedQuality := NormalizeImageStudioQuality(quality)
	if normalizedQuality == "" {
		return ResolveImageStudioAspectRatio(ratio)
	}
	normalizedRatio := firstNonEmptyTrimmed(ratio, "1:1")
	sizeByQuality, ok := map[string]map[string]string{
		ImageBillingSize1K: {
			"1:1":  "1024x1024",
			"16:9": "1024x576",
			"9:16": "576x1024",
			"4:3":  "1024x768",
			"3:4":  "768x1024",
		},
		ImageBillingSize2K: {
			"1:1":  "2048x2048",
			"16:9": "2048x1152",
			"9:16": "1152x2048",
			"4:3":  "2048x1536",
			"3:4":  "1536x2048",
		},
		ImageBillingSize4K: {
			"1:1":  "4096x4096",
			"16:9": "3840x2160",
			"9:16": "2160x3840",
			"4:3":  "4096x3072",
			"3:4":  "3072x4096",
		},
	}[normalizedQuality]
	if !ok {
		return "", "", fmt.Errorf("unsupported image quality: %s", quality)
	}
	size, ok := sizeByQuality[normalizedRatio]
	if !ok {
		return "", "", fmt.Errorf("unsupported image aspect ratio: %s", ratio)
	}
	return size, normalizedQuality, nil
}

func NormalizeImageStudioQuality(quality string) string {
	normalized := strings.ToUpper(strings.TrimSpace(quality))
	switch normalized {
	case "", "AUTO":
		return ""
	case ImageBillingSize1K, ImageBillingSize2K, ImageBillingSize4K:
		return normalized
	default:
		return normalized
	}
}

func NormalizeImageStudioOutputFormat(format string) string {
	normalized := strings.ToLower(strings.TrimSpace(format))
	switch normalized {
	case "", "auto":
		return "png"
	case "jpg":
		return "jpeg"
	case "png", "jpeg", "webp":
		return normalized
	default:
		return normalized
	}
}

func NormalizeImageStudioBackground(background string) string {
	normalized := strings.ToLower(strings.TrimSpace(background))
	switch normalized {
	case "", "auto":
		return "auto"
	case "opaque", "transparent":
		return normalized
	default:
		return normalized
	}
}

func ValidateImageStudioOutputOptions(outputFormat string, background string) error {
	format := NormalizeImageStudioOutputFormat(outputFormat)
	switch format {
	case "png", "jpeg", "webp":
	default:
		return fmt.Errorf("unsupported image output format: %s", outputFormat)
	}
	normalizedBackground := NormalizeImageStudioBackground(background)
	switch normalizedBackground {
	case "auto", "opaque", "transparent":
	default:
		return fmt.Errorf("unsupported image background: %s", background)
	}
	if format == "jpeg" && normalizedBackground == "transparent" {
		return fmt.Errorf("transparent background is not supported for jpeg output")
	}
	return nil
}

func ValidateImageStudioOutputOptionsForModel(model string, outputFormat string, background string) error {
	if err := ValidateImageStudioOutputOptions(outputFormat, background); err != nil {
		return err
	}
	if NormalizeImageStudioBackground(background) == "transparent" && !ImageStudioModelSupportsTransparentBackground(model) {
		return fmt.Errorf("transparent background is not supported for %s", strings.TrimSpace(model))
	}
	return nil
}

func ImageStudioModelSupportsTransparentBackground(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "gpt-image-2":
		return false
	default:
		return true
	}
}

func imageStudioOutputFormatFromMIMEType(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg":
		return "jpeg"
	case "image/webp":
		return "webp"
	case "image/png":
		return "png"
	default:
		return "png"
	}
}

func ValidateImageStudioModel(model string, allowed []string) error {
	normalized := strings.TrimSpace(model)
	if normalized == "" {
		return fmt.Errorf("image model is required")
	}
	for _, candidate := range allowed {
		if normalized == strings.TrimSpace(candidate) {
			return nil
		}
	}
	return fmt.Errorf("image model is not allowed: %s", model)
}

func NormalizeImageStudioPrompt(prompt string) (string, error) {
	normalized := strings.TrimSpace(prompt)
	if normalized == "" {
		return "", fmt.Errorf("image prompt is required")
	}
	return normalized, nil
}

func ImageStudioPromptWithAspectRatioGuidance(prompt string, aspectRatio string) string {
	normalizedPrompt := strings.TrimSpace(prompt)
	normalizedRatio := strings.TrimSpace(aspectRatio)
	if normalizedPrompt == "" || normalizedRatio == "" {
		return normalizedPrompt
	}
	return normalizedPrompt + "\n\n" + imageStudioAspectRatioGuidance(normalizedRatio)
}

func imageStudioAspectRatioGuidance(aspectRatio string) string {
	normalized := strings.TrimSpace(aspectRatio)
	shape := "selected"
	switch normalized {
	case "1:1":
		shape = "square 1:1"
	case "16:9":
		shape = "wide 16:9 landscape"
	case "9:16":
		shape = "vertical 9:16 portrait"
	case "4:3":
		shape = "classic 4:3 landscape"
	case "3:4":
		shape = "vertical 3:4 portrait"
	}
	if normalized == "" {
		normalized = "1:1"
		shape = "square 1:1"
	}
	return fmt.Sprintf(
		"Composition constraint: generate the image on a %s canvas. The final image canvas must be %s, with the main subject fully composed inside that frame. Do not create a wide, panoramic, letterboxed, or cropped composition unless the requested ratio is wide. Do not place a narrow image on a larger blank canvas.",
		shape,
		normalized,
	)
}
