package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserImageTaskRepositoryCreateListAndMarkSuccess(t *testing.T) {
	imageRepo, client := newUserImageEntRepo(t)
	taskRepo := NewUserImageTaskRepository(client)
	ctx := context.Background()
	userID := createUserImageTestUser(t, client, "image-task-user@example.com")
	otherUserID := createUserImageTestUser(t, client, "other-image-task-user@example.com")
	createdAt := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)

	task := &service.ImageStudioTask{
		UserID:              userID,
		Mode:                service.ImageStudioModeGeneration,
		Status:              service.ImageStudioTaskStatusQueued,
		Model:               "gpt-image-2",
		Prompt:              "blue portal",
		AspectRatio:         "16:9",
		Quality:             service.ImageBillingSize4K,
		Size:                "3840x2160",
		EstimatedCost:       0.16,
		ReferenceObjectKeys: []string{"refs/one.png"},
		CreatedAt:           createdAt,
		UpdatedAt:           createdAt,
	}
	require.NoError(t, taskRepo.Create(ctx, task))
	require.NotZero(t, task.ID)

	startedAt := createdAt.Add(time.Second)
	claimed, err := taskRepo.MarkRunning(ctx, task.ID, startedAt)
	require.NoError(t, err)
	require.True(t, claimed)
	running, err := taskRepo.GetByID(ctx, userID, task.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioTaskStatusRunning, running.Status)
	require.Equal(t, []string{"refs/one.png"}, running.ReferenceObjectKeys)

	_, err = taskRepo.GetByID(ctx, otherUserID, task.ID)
	require.ErrorIs(t, err, service.ErrImageStudioTaskNotFound)

	image := createUserImageTestRecord(t, imageRepo, service.ImageStudioImageRecord{
		UserID:           userID,
		Mode:             service.ImageStudioModeGeneration,
		Model:            "gpt-image-2",
		Prompt:           "blue portal",
		AspectRatio:      "16:9",
		Size:             "1024x576",
		ImageURL:         "https://assets.example.com/out.png",
		StorageDriver:    service.ImageStorageDriverLocal,
		StorageObjectKey: "images/user-1/2026/06/out.png",
		MimeType:         "image/png",
		Bytes:            12,
		Cost:             0.03,
	})
	completedAt := startedAt.Add(time.Second)
	succeeded, err := taskRepo.MarkSucceeded(ctx, task.ID, &image, service.ImageBillingSize1K, 0.03, completedAt)
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioTaskStatusSucceeded, succeeded.Status)
	require.Equal(t, service.ImageBillingSize1K, succeeded.Quality)
	require.Equal(t, "1024x576", succeeded.Size)
	require.Equal(t, 0.03, succeeded.EstimatedCost)
	require.NotNil(t, succeeded.ImageID)
	require.Equal(t, image.ID, *succeeded.ImageID)
	require.NotNil(t, succeeded.Image)
	require.Equal(t, "https://assets.example.com/out.png", succeeded.Image.ImageURL)

	claimed, err = taskRepo.MarkRunning(ctx, task.ID, completedAt.Add(time.Second))
	require.NoError(t, err)
	require.False(t, claimed)
	stillSucceeded, err := taskRepo.GetByID(ctx, userID, task.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioTaskStatusSucceeded, stillSucceeded.Status)

	items, page, err := taskRepo.ListByUser(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Len(t, items, 1)
	require.Equal(t, task.ID, items[0].ID)
	require.NotNil(t, items[0].Image)
}

func TestUserImageTaskRepositoryMarkFailed(t *testing.T) {
	_, client := newUserImageEntRepo(t)
	taskRepo := NewUserImageTaskRepository(client)
	ctx := context.Background()
	userID := createUserImageTestUser(t, client, "image-task-fail@example.com")

	task := &service.ImageStudioTask{
		UserID:      userID,
		Mode:        service.ImageStudioModeGeneration,
		Status:      service.ImageStudioTaskStatusQueued,
		Model:       "gpt-image-2",
		Prompt:      "blue portal",
		AspectRatio: "1:1",
		Quality:     service.ImageBillingSize1K,
		Size:        "1024x1024",
	}
	require.NoError(t, taskRepo.Create(ctx, task))

	completedAt := time.Date(2026, 6, 23, 10, 0, 2, 0, time.UTC)
	failed, err := taskRepo.MarkFailed(ctx, task.ID, "IMAGE_PROVIDER_TIMEOUT_OR_DISCONNECT", "not charged", completedAt)
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioTaskStatusFailed, failed.Status)
	require.Equal(t, "IMAGE_PROVIDER_TIMEOUT_OR_DISCONNECT", failed.ErrorReason)
	require.Contains(t, failed.ErrorMessage, "not charged")
	require.NotNil(t, failed.CompletedAt)
}
