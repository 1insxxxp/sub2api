package service

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	ImageStorageDriverLocal = "local"
	ImageStorageDriverR2    = "r2"
)

type ImageStorage interface {
	Put(ctx context.Context, objectKey string, contentType string, data []byte) (string, error)
	Open(ctx context.Context, objectKey string) (*ImageStudioStoredFile, error)
	Delete(ctx context.Context, objectKey string) error
}

type ImageStudioStoredFile struct {
	Name        string
	ContentType string
	ModTime     time.Time
	Size        int64
	Reader      io.ReadSeeker
	Close       func() error
}

func GenerateImageStorageObjectKey(userID int64, contentType string, now time.Time) (string, error) {
	if userID <= 0 {
		return "", fmt.Errorf("user id is required")
	}
	ext := imageStorageExtension(contentType)
	return fmt.Sprintf(
		"images/user-%d/%04d/%02d/%s%s",
		userID,
		now.UTC().Year(),
		int(now.UTC().Month()),
		uuid.NewString(),
		ext,
	), nil
}

func imageStorageExtension(contentType string) string {
	contentType = strings.TrimSpace(strings.ToLower(contentType))
	switch contentType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	}
	if exts, err := mime.ExtensionsByType(contentType); err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ".png"
}

func validateImageStorageObjectKey(objectKey string) error {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return fmt.Errorf("object key is required")
	}
	if strings.Contains(objectKey, "\\") {
		return fmt.Errorf("object key must use forward slashes")
	}
	if strings.HasPrefix(objectKey, "/") || filepath.IsAbs(objectKey) {
		return fmt.Errorf("object key must be relative")
	}
	cleaned := filepath.ToSlash(filepath.Clean(objectKey))
	if cleaned == "." || cleaned != objectKey || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return fmt.Errorf("unsafe object key")
	}
	return nil
}

func joinPublicImageURL(baseURL string, objectKey string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	objectKey = strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if baseURL == "" {
		return "/" + objectKey
	}
	return baseURL + "/" + objectKey
}
