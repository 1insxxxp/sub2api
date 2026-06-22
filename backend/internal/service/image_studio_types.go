package service

import (
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

type ImageStudioGenerateInput struct {
	UserID      int64  `json:"user_id"`
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	AspectRatio string `json:"aspect_ratio"`
}

type ImageStudioEditInput struct {
	UserID      int64  `json:"user_id"`
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	AspectRatio string `json:"aspect_ratio"`
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

func ResolveImageStudioAspectRatio(ratio string) (string, string, error) {
	normalized := strings.TrimSpace(ratio)
	for _, item := range SupportedImageStudioAspectRatios() {
		if item.Ratio == normalized {
			return item.Size, item.BillingTier, nil
		}
	}
	return "", "", fmt.Errorf("unsupported image aspect ratio: %s", ratio)
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
