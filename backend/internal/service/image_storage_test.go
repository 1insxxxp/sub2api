package service

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLocalImageStoragePutAndDelete(t *testing.T) {
	dir := t.TempDir()
	storage, err := NewLocalImageStorage(LocalImageStorageConfig{
		RootDir:       dir,
		PublicBaseURL: "https://assets.example.com/images/",
	})
	require.NoError(t, err)

	url, err := storage.Put(context.Background(), "images/user-1/2026/06/example.png", "image/png", []byte("png-bytes"))
	require.NoError(t, err)
	require.Equal(t, "https://assets.example.com/images/images/user-1/2026/06/example.png", url)

	got, err := os.ReadFile(filepath.Join(dir, "images", "user-1", "2026", "06", "example.png"))
	require.NoError(t, err)
	require.Equal(t, []byte("png-bytes"), got)

	err = storage.Delete(context.Background(), "images/user-1/2026/06/example.png")
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, "images", "user-1", "2026", "06", "example.png"))
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestImageStorageObjectKeyGeneration(t *testing.T) {
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	key, err := GenerateImageStorageObjectKey(123, "image/png", now)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(key, "images/user-123/2026/06/"))
	require.True(t, strings.HasSuffix(key, ".png"))
	require.NotContains(t, key, "..")
	require.NotContains(t, key, "\\")

	other, err := GenerateImageStorageObjectKey(123, "image/png", now)
	require.NoError(t, err)
	require.NotEqual(t, key, other)
}

func TestLocalImageStorageRejectsUnsafeObjectKeys(t *testing.T) {
	storage, err := NewLocalImageStorage(LocalImageStorageConfig{
		RootDir:       t.TempDir(),
		PublicBaseURL: "https://assets.example.com",
	})
	require.NoError(t, err)

	_, err = storage.Put(context.Background(), "../escape.png", "image/png", []byte("bad"))
	require.Error(t, err)

	_, err = storage.Put(context.Background(), `images\bad.png`, "image/png", []byte("bad"))
	require.Error(t, err)
}

func TestNewR2ImageStorageValidatesConfig(t *testing.T) {
	_, err := NewR2ImageStorage(R2ImageStorageConfig{})
	require.Error(t, err)

	_, err = NewR2ImageStorage(R2ImageStorageConfig{
		AccountID:     "account",
		AccessKeyID:   "key",
		SecretKey:     "secret",
		Bucket:        "bucket",
		PublicBaseURL: "https://assets.example.com",
	})
	require.NoError(t, err)
}

func TestDefaultImageStudioStorageFactoryCreatesR2FromEnvironment(t *testing.T) {
	t.Setenv("R2_ACCOUNT_ID", "account")
	t.Setenv("R2_ACCESS_KEY_ID", "key")
	t.Setenv("R2_SECRET_ACCESS_KEY", "secret")
	t.Setenv("R2_BUCKET", "bucket")

	storage, err := defaultImageStudioStorageFactory(context.Background(), &ImageStudioSettings{
		StorageDriver:   ImageStorageDriverR2,
		R2PublicBaseURL: "https://assets.example.com/images",
	})

	require.NoError(t, err)
	r2Storage, ok := storage.(*R2ImageStorage)
	require.True(t, ok)
	require.Equal(t, "bucket", r2Storage.bucket)
	require.Equal(t, "https://assets.example.com/images", r2Storage.publicBaseURL)
}
