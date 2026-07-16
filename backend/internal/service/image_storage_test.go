package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
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

var pngBytes = []byte("\x89PNG\r\n\x1a\nfake-png-payload")

type savedImage struct {
	key         string
	contentType string
	data        []byte
}

type fakeImageResultStorage struct {
	saved []savedImage
	url   string
	err   error
}

func (f *fakeImageResultStorage) Save(_ context.Context, key, contentType string, data []byte) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.saved = append(f.saved, savedImage{key: key, contentType: contentType, data: append([]byte(nil), data...)})
	if f.url != "" {
		return f.url, nil
	}
	return "https://cdn.test/" + key, nil
}

func TestImageResultUploaderRewritesB64JSON(t *testing.T) {
	storage := &fakeImageResultStorage{}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)

	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	result := json.RawMessage(`{"created":1,"data":[{"b64_json":"` + b64 + `","revised_prompt":"a cat"}]}`)

	out, err := uploader.Rewrite(context.Background(), "imgtask_abc", result)
	require.NoError(t, err)

	require.Len(t, storage.saved, 1)
	require.Equal(t, "images/imgtask_abc-0.png", storage.saved[0].key)
	require.Equal(t, "image/png", storage.saved[0].contentType)
	require.Equal(t, pngBytes, storage.saved[0].data)

	var parsed struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out, &parsed))
	require.Len(t, parsed.Data, 1)
	require.JSONEq(t, `"https://cdn.test/images/imgtask_abc-0.png"`, string(parsed.Data[0]["url"]))
	_, hasB64 := parsed.Data[0]["b64_json"]
	require.False(t, hasB64, "b64_json must be stripped after offload")
	require.JSONEq(t, `"a cat"`, string(parsed.Data[0]["revised_prompt"]), "unrelated fields preserved")
}

func TestImageResultUploaderRewritesURL(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer upstream.Close()

	storage := &fakeImageResultStorage{}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)

	result := json.RawMessage(`{"created":1,"data":[{"url":"` + upstream.URL + `/pic.png"}]}`)
	out, err := uploader.Rewrite(context.Background(), "imgtask_xyz", result)
	require.NoError(t, err)

	require.Len(t, storage.saved, 1)
	require.Equal(t, pngBytes, storage.saved[0].data)
	require.Equal(t, "image/png", storage.saved[0].contentType)

	var parsed struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out, &parsed))
	require.JSONEq(t, `"https://cdn.test/images/imgtask_xyz-0.png"`, string(parsed.Data[0]["url"]))
}

func TestImageResultUploaderPropagatesStorageError(t *testing.T) {
	storage := &fakeImageResultStorage{err: errors.New("bucket unreachable")}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)

	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	result := json.RawMessage(`{"data":[{"b64_json":"` + b64 + `"}]}`)

	_, err := uploader.Rewrite(context.Background(), "imgtask_err", result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "bucket unreachable")
}

func TestImageResultUploaderNilStoragePassthrough(t *testing.T) {
	var uploader *ImageResultUploader
	result := json.RawMessage(`{"data":[{"url":"https://example.test/x.png"}]}`)
	out, err := uploader.Rewrite(context.Background(), "imgtask_nil", result)
	require.NoError(t, err)
	require.JSONEq(t, string(result), string(out))
}

func TestImageTaskServiceCompleteOffloadsToStorage(t *testing.T) {
	store := &imageTaskMemoryStore{}
	storage := &fakeImageResultStorage{}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)
	svc := NewImageTaskServiceWithUploader(store, uploader, time.Hour, time.Minute)
	require.True(t, svc.Enabled())

	owner := ImageTaskOwner{UserID: 1, APIKeyID: 2}
	created, err := svc.Create(context.Background(), owner)
	require.NoError(t, err)

	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	result := json.RawMessage(`{"created":1,"data":[{"b64_json":"` + b64 + `"}]}`)
	require.NoError(t, svc.Complete(context.Background(), created.ID, http.StatusOK, result))

	got, err := svc.Get(context.Background(), owner, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusCompleted, got.Status)
	require.Equal(t, "https://cdn.test/images/"+created.ID+"-0.png", got.ImageURL)
	require.NotContains(t, string(got.Result), "b64_json", "large base64 must not be persisted to Redis")
	require.Len(t, storage.saved, 1)
}

func TestImageTaskServiceCompleteOffloadFailureMarksFailed(t *testing.T) {
	store := &imageTaskMemoryStore{}
	storage := &fakeImageResultStorage{err: errors.New("bucket unreachable")}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)
	svc := NewImageTaskServiceWithUploader(store, uploader, time.Hour, time.Minute)

	owner := ImageTaskOwner{UserID: 1, APIKeyID: 2}
	created, err := svc.Create(context.Background(), owner)
	require.NoError(t, err)

	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	result := json.RawMessage(`{"data":[{"b64_json":"` + b64 + `"}]}`)
	require.NoError(t, svc.Complete(context.Background(), created.ID, http.StatusOK, result))

	got, err := svc.Get(context.Background(), owner, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusFailed, got.Status)
	require.Equal(t, http.StatusBadGateway, got.HTTPStatus)
	require.Contains(t, string(got.Error), "object storage")
	require.NotContains(t, string(got.Result), "b64_json", "failed offload must not persist base64 to Redis")
}
