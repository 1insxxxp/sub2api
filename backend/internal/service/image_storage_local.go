package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type LocalImageStorageConfig struct {
	RootDir       string
	PublicBaseURL string
}

type LocalImageStorage struct {
	rootDir       string
	publicBaseURL string
}

func NewLocalImageStorage(cfg LocalImageStorageConfig) (*LocalImageStorage, error) {
	rootDir := strings.TrimSpace(cfg.RootDir)
	if rootDir == "" {
		return nil, fmt.Errorf("local image storage root dir is required")
	}
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve local image storage root: %w", err)
	}
	return &LocalImageStorage{
		rootDir:       absRoot,
		publicBaseURL: strings.TrimSpace(cfg.PublicBaseURL),
	}, nil
}

func (s *LocalImageStorage) Put(ctx context.Context, objectKey string, contentType string, data []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := validateImageStorageObjectKey(objectKey); err != nil {
		return "", err
	}
	target, err := s.resolvePath(objectKey)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", fmt.Errorf("create image storage directory: %w", err)
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return "", fmt.Errorf("write image storage object: %w", err)
	}
	return joinPublicImageURL(s.publicBaseURL, objectKey), nil
}

func (s *LocalImageStorage) Delete(ctx context.Context, objectKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateImageStorageObjectKey(objectKey); err != nil {
		return err
	}
	target, err := s.resolvePath(objectKey)
	if err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete image storage object: %w", err)
	}
	return nil
}

func (s *LocalImageStorage) resolvePath(objectKey string) (string, error) {
	target := filepath.Join(s.rootDir, filepath.FromSlash(objectKey))
	rel, err := filepath.Rel(s.rootDir, target)
	if err != nil {
		return "", fmt.Errorf("resolve image storage path: %w", err)
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("object key escapes storage root")
	}
	return target, nil
}
