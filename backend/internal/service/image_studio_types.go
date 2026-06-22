package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	ImageStudioModeGeneration = "generation"
	ImageStudioModeEdit       = "edit"
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
	UserID      int64  `json:"user_id"`
	GroupID     *int64 `json:"group_id,omitempty"`
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	AspectRatio string `json:"aspect_ratio"`
	Quality     string `json:"quality,omitempty"`
	Size        string `json:"size,omitempty"`
	BillingTier string `json:"billing_tier,omitempty"`
	UserAgent   string `json:"-"`
	IPAddress   string `json:"-"`
}

type ImageStudioEditInput struct {
	UserID          int64                       `json:"user_id"`
	GroupID         *int64                      `json:"group_id,omitempty"`
	Model           string                      `json:"model"`
	Prompt          string                      `json:"prompt"`
	AspectRatio     string                      `json:"aspect_ratio"`
	Quality         string                      `json:"quality,omitempty"`
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
	Bytes            int64      `json:"bytes"`
	Cost             float64    `json:"cost"`
	UsageLogID       *int64     `json:"usage_log_id,omitempty"`
	SourceImageCount int        `json:"source_image_count"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
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
