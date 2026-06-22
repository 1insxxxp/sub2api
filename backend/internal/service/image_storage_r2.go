package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type R2ImageStorageConfig struct {
	AccountID     string
	AccessKeyID   string
	SecretKey     string
	Bucket        string
	PublicBaseURL string
}

type r2S3Client interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type R2ImageStorage struct {
	bucket        string
	publicBaseURL string
	client        r2S3Client
}

func NewR2ImageStorage(cfg R2ImageStorageConfig) (*R2ImageStorage, error) {
	cfg.AccountID = strings.TrimSpace(cfg.AccountID)
	cfg.AccessKeyID = strings.TrimSpace(cfg.AccessKeyID)
	cfg.SecretKey = strings.TrimSpace(cfg.SecretKey)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.PublicBaseURL = strings.TrimSpace(cfg.PublicBaseURL)
	if cfg.AccountID == "" || cfg.AccessKeyID == "" || cfg.SecretKey == "" || cfg.Bucket == "" || cfg.PublicBaseURL == "" {
		return nil, fmt.Errorf("r2 image storage config is incomplete")
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(endpoint),
		Credentials:  aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretKey, "")),
	})
	return &R2ImageStorage{
		bucket:        cfg.Bucket,
		publicBaseURL: cfg.PublicBaseURL,
		client:        client,
	}, nil
}

func (s *R2ImageStorage) Put(ctx context.Context, objectKey string, contentType string, data []byte) (string, error) {
	if err := validateImageStorageObjectKey(objectKey); err != nil {
		return "", err
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("put r2 image object: %w", err)
	}
	return joinPublicImageURL(s.publicBaseURL, objectKey), nil
}

func (s *R2ImageStorage) Delete(ctx context.Context, objectKey string) error {
	if err := validateImageStorageObjectKey(objectKey); err != nil {
		return err
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("delete r2 image object: %w", err)
	}
	return nil
}
