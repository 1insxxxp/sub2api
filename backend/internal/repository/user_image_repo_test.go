package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/userimage"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newUserImageEntRepo(t *testing.T) (*userImageRepository, *dbent.Client) {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(10)

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	return &userImageRepository{client: client}, client
}

func createUserImageTestUser(t *testing.T, client *dbent.Client, email string) int64 {
	t.Helper()

	user, err := client.User.Create().
		SetEmail(email).
		SetUsername(email).
		SetPasswordHash("hash").
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		Save(context.Background())
	require.NoError(t, err)
	return user.ID
}

func createUserImageTestRecord(t *testing.T, repo service.UserImageRepository, record service.ImageStudioImageRecord) service.ImageStudioImageRecord {
	t.Helper()

	require.NoError(t, repo.Create(context.Background(), &record))
	require.NotZero(t, record.ID)
	return record
}

func TestUserImageRepositoryCreateListAndCountNonDeletedByUser(t *testing.T) {
	repo, client := newUserImageEntRepo(t)
	ctx := context.Background()
	userID := createUserImageTestUser(t, client, "image-user@example.com")
	otherUserID := createUserImageTestUser(t, client, "other-image-user@example.com")
	oldTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	newTime := oldTime.Add(time.Hour)

	old := createUserImageTestRecord(t, repo, service.ImageStudioImageRecord{
		UserID:           userID,
		Mode:             service.ImageStudioModeGeneration,
		Model:            "gpt-image-1",
		Prompt:           "old image",
		AspectRatio:      "1:1",
		Size:             "1024x1024",
		ImageURL:         "https://assets.example.com/old.png",
		StorageDriver:    service.ImageStorageDriverLocal,
		StorageObjectKey: "images/user-1/2026/01/old.png",
		MimeType:         "image/png",
		Bytes:            12,
		Cost:             0.02,
		CreatedAt:        oldTime,
		UpdatedAt:        oldTime,
	})
	newer := createUserImageTestRecord(t, repo, service.ImageStudioImageRecord{
		UserID:           userID,
		Mode:             service.ImageStudioModeEdit,
		Model:            "gpt-image-1",
		Prompt:           "new image",
		AspectRatio:      "16:9",
		Size:             "1536x864",
		ImageURL:         "https://assets.example.com/new.png",
		StorageDriver:    service.ImageStorageDriverLocal,
		StorageObjectKey: "images/user-1/2026/01/new.png",
		MimeType:         "image/png",
		Bytes:            34,
		Cost:             0.04,
		UsageLogID:       ptrInt64(99),
		SourceImageCount: 1,
		CreatedAt:        newTime,
		UpdatedAt:        newTime,
	})
	_ = createUserImageTestRecord(t, repo, service.ImageStudioImageRecord{
		UserID:           otherUserID,
		Mode:             service.ImageStudioModeGeneration,
		Model:            "gpt-image-1",
		Prompt:           "other image",
		AspectRatio:      "1:1",
		Size:             "1024x1024",
		ImageURL:         "https://assets.example.com/other.png",
		StorageDriver:    service.ImageStorageDriverLocal,
		StorageObjectKey: "images/user-2/2026/01/other.png",
		MimeType:         "image/png",
		CreatedAt:        newTime.Add(time.Minute),
		UpdatedAt:        newTime.Add(time.Minute),
	})

	require.NoError(t, repo.SoftDelete(ctx, old.ID, userID))

	items, page, err := repo.ListByUser(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(1), page.Total)
	require.Equal(t, newer.ID, items[0].ID)
	require.Equal(t, int64(99), *items[0].UsageLogID)
	require.Equal(t, 1, items[0].SourceImageCount)

	count, err := repo.CountSavedByUser(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

func TestUserImageRepositorySoftDeleteChecksOwnership(t *testing.T) {
	repo, client := newUserImageEntRepo(t)
	ctx := context.Background()
	ownerID := createUserImageTestUser(t, client, "owner@example.com")
	otherID := createUserImageTestUser(t, client, "not-owner@example.com")

	record := createUserImageTestRecord(t, repo, service.ImageStudioImageRecord{
		UserID:           ownerID,
		Mode:             service.ImageStudioModeGeneration,
		Model:            "gpt-image-1",
		Prompt:           "owned image",
		AspectRatio:      "1:1",
		Size:             "1024x1024",
		ImageURL:         "https://assets.example.com/owned.png",
		StorageDriver:    service.ImageStorageDriverLocal,
		StorageObjectKey: "images/user-1/2026/01/owned.png",
		MimeType:         "image/png",
	})

	require.ErrorIs(t, repo.SoftDelete(ctx, record.ID, otherID), service.ErrUserImageNotFound)

	loaded, err := client.UserImage.Query().Where(userimage.IDEQ(record.ID)).Only(ctx)
	require.NoError(t, err)
	require.Nil(t, loaded.DeletedAt)

	require.NoError(t, repo.SoftDelete(ctx, record.ID, ownerID))
	loaded, err = client.UserImage.Query().Where(userimage.IDEQ(record.ID)).Only(ctx)
	require.NoError(t, err)
	require.NotNil(t, loaded.DeletedAt)
}

func TestUserImageRepositoryDeleteOldestOverLimitAndListExpired(t *testing.T) {
	repo, client := newUserImageEntRepo(t)
	ctx := context.Background()
	userID := createUserImageTestUser(t, client, "cleanup@example.com")
	base := time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC)

	var records []service.ImageStudioImageRecord
	for i := 0; i < 3; i++ {
		expiresAt := base.Add(time.Duration(i-1) * time.Hour)
		records = append(records, createUserImageTestRecord(t, repo, service.ImageStudioImageRecord{
			UserID:           userID,
			Mode:             service.ImageStudioModeGeneration,
			Model:            "gpt-image-1",
			Prompt:           fmt.Sprintf("image %d", i),
			AspectRatio:      "1:1",
			Size:             "1024x1024",
			ImageURL:         fmt.Sprintf("https://assets.example.com/%d.png", i),
			StorageDriver:    service.ImageStorageDriverLocal,
			StorageObjectKey: fmt.Sprintf("images/user-1/2026/02/%d.png", i),
			MimeType:         "image/png",
			ExpiresAt:        &expiresAt,
			CreatedAt:        base.Add(time.Duration(i) * time.Minute),
			UpdatedAt:        base.Add(time.Duration(i) * time.Minute),
		}))
	}

	deleted, err := repo.DeleteOldestOverLimit(ctx, userID, 1)
	require.NoError(t, err)
	require.Len(t, deleted, 2)
	require.Equal(t, []int64{records[0].ID, records[1].ID}, []int64{deleted[0].ID, deleted[1].ID})

	count, err := repo.CountSavedByUser(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	expired, err := repo.ListExpired(ctx, base.Add(3*time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, expired, 1)
	require.Equal(t, records[2].ID, expired[0].ID)
}

func ptrInt64(v int64) *int64 {
	return &v
}
