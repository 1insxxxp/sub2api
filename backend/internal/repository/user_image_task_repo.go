package repository

import (
	"context"
	"encoding/json"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/userimagetask"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userImageTaskRepository struct {
	client *dbent.Client
}

func NewUserImageTaskRepository(client *dbent.Client) service.UserImageTaskRepository {
	return &userImageTaskRepository{client: client}
}

func (r *userImageTaskRepository) Create(ctx context.Context, task *service.ImageStudioTask) error {
	if task == nil {
		return nil
	}
	client := clientFromContext(ctx, r.client)
	builder := client.UserImageTask.Create().
		SetUserID(task.UserID).
		SetNillableAPIKeyID(task.APIKeyID).
		SetNillableGroupID(task.GroupID).
		SetNillableImageID(task.ImageID).
		SetMode(task.Mode).
		SetStatus(task.Status).
		SetModel(task.Model).
		SetPrompt(task.Prompt).
		SetAspectRatio(task.AspectRatio).
		SetQuality(task.Quality).
		SetSize(task.Size).
		SetEstimatedCost(task.EstimatedCost).
		SetSourceImageCount(task.SourceImageCount).
		SetNillableStartedAt(task.StartedAt).
		SetNillableCompletedAt(task.CompletedAt)
	if len(task.ReferenceObjectKeys) > 0 {
		raw, err := json.Marshal(task.ReferenceObjectKeys)
		if err != nil {
			return err
		}
		builder.SetReferenceObjectKeys(string(raw))
	}
	if task.ErrorReason != "" {
		builder.SetErrorReason(task.ErrorReason)
	}
	if task.ErrorMessage != "" {
		builder.SetErrorMessage(task.ErrorMessage)
	}
	if !task.CreatedAt.IsZero() {
		builder.SetCreatedAt(task.CreatedAt)
	}
	if !task.UpdatedAt.IsZero() {
		builder.SetUpdatedAt(task.UpdatedAt)
	}

	row, err := builder.Save(ctx)
	if err != nil {
		return err
	}
	*task = *userImageTaskEntityToService(row)
	return nil
}

func (r *userImageTaskRepository) GetByID(ctx context.Context, userID int64, taskID int64) (*service.ImageStudioTask, error) {
	client := clientFromContext(ctx, r.client)
	query := client.UserImageTask.Query().
		Where(userimagetask.IDEQ(taskID)).
		WithImage()
	if userID > 0 {
		query.Where(userimagetask.UserIDEQ(userID))
	}
	row, err := query.Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrImageStudioTaskNotFound
		}
		return nil, err
	}
	return userImageTaskEntityToService(row), nil
}

func (r *userImageTaskRepository) ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.ImageStudioTask, *pagination.PaginationResult, error) {
	client := clientFromContext(ctx, r.client)
	query := client.UserImageTask.Query().
		Where(userimagetask.UserIDEQ(userID)).
		WithImage()
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	if total == 0 {
		return []service.ImageStudioTask{}, paginationResultFromTotal(0, params), nil
	}
	rows, err := query.
		Order(dbent.Desc(userimagetask.FieldCreatedAt), dbent.Desc(userimagetask.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	return userImageTaskEntitiesToService(rows), paginationResultFromTotal(int64(total), params), nil
}

func (r *userImageTaskRepository) MarkRunning(ctx context.Context, taskID int64, startedAt time.Time) (bool, error) {
	client := clientFromContext(ctx, r.client)
	affected, err := client.UserImageTask.Update().
		Where(
			userimagetask.IDEQ(taskID),
			userimagetask.StatusEQ(service.ImageStudioTaskStatusQueued),
		).
		SetStatus(service.ImageStudioTaskStatusRunning).
		SetStartedAt(startedAt).
		SetUpdatedAt(startedAt).
		Save(ctx)
	return affected > 0, err
}

func (r *userImageTaskRepository) MarkSucceeded(ctx context.Context, taskID int64, image *service.ImageStudioImageRecord, quality string, estimatedCost float64, completedAt time.Time) (*service.ImageStudioTask, error) {
	client := clientFromContext(ctx, r.client)
	builder := client.UserImageTask.UpdateOneID(taskID).
		SetStatus(service.ImageStudioTaskStatusSucceeded).
		SetCompletedAt(completedAt).
		SetUpdatedAt(completedAt).
		ClearErrorReason().
		ClearErrorMessage()
	if image != nil && image.ID > 0 {
		builder.SetImageID(image.ID)
		if image.Size != "" {
			builder.SetSize(image.Size)
		}
	}
	if quality != "" {
		builder.SetQuality(quality)
	}
	if estimatedCost >= 0 {
		builder.SetEstimatedCost(estimatedCost)
	}
	row, err := builder.Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrImageStudioTaskNotFound
		}
		return nil, err
	}
	if image != nil {
		row.Edges.Image = userImageRecordToEnt(image)
	}
	return userImageTaskEntityToService(row), nil
}

func (r *userImageTaskRepository) MarkFailed(ctx context.Context, taskID int64, reason string, message string, completedAt time.Time) (*service.ImageStudioTask, error) {
	client := clientFromContext(ctx, r.client)
	row, err := client.UserImageTask.UpdateOneID(taskID).
		SetStatus(service.ImageStudioTaskStatusFailed).
		SetErrorReason(reason).
		SetErrorMessage(message).
		SetCompletedAt(completedAt).
		SetUpdatedAt(completedAt).
		Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrImageStudioTaskNotFound
		}
		return nil, err
	}
	return userImageTaskEntityToService(row), nil
}

func userImageTaskEntitiesToService(rows []*dbent.UserImageTask) []service.ImageStudioTask {
	out := make([]service.ImageStudioTask, 0, len(rows))
	for _, row := range rows {
		out = append(out, *userImageTaskEntityToService(row))
	}
	return out
}

func userImageTaskEntityToService(row *dbent.UserImageTask) *service.ImageStudioTask {
	if row == nil {
		return nil
	}
	out := &service.ImageStudioTask{
		ID:               row.ID,
		UserID:           row.UserID,
		APIKeyID:         row.APIKeyID,
		GroupID:          row.GroupID,
		ImageID:          row.ImageID,
		Mode:             row.Mode,
		Status:           row.Status,
		Model:            row.Model,
		AspectRatio:      row.AspectRatio,
		Quality:          row.Quality,
		Size:             row.Size,
		EstimatedCost:    row.EstimatedCost,
		SourceImageCount: row.SourceImageCount,
		StartedAt:        row.StartedAt,
		CompletedAt:      row.CompletedAt,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
	if row.Prompt != nil {
		out.Prompt = *row.Prompt
	}
	if row.ReferenceObjectKeys != nil && *row.ReferenceObjectKeys != "" {
		_ = json.Unmarshal([]byte(*row.ReferenceObjectKeys), &out.ReferenceObjectKeys)
	}
	if row.ErrorReason != nil {
		out.ErrorReason = *row.ErrorReason
	}
	if row.ErrorMessage != nil {
		out.ErrorMessage = *row.ErrorMessage
	}
	if row.Edges.Image != nil {
		out.Image = userImageEntityToService(row.Edges.Image)
	}
	return out
}

func userImageRecordToEnt(record *service.ImageStudioImageRecord) *dbent.UserImage {
	if record == nil {
		return nil
	}
	prompt := record.Prompt
	return &dbent.UserImage{
		ID:               record.ID,
		UserID:           record.UserID,
		Mode:             record.Mode,
		Model:            record.Model,
		Prompt:           &prompt,
		AspectRatio:      record.AspectRatio,
		Size:             record.Size,
		ImageURL:         record.ImageURL,
		StorageDriver:    record.StorageDriver,
		StorageObjectKey: record.StorageObjectKey,
		MimeType:         record.MimeType,
		Bytes:            record.Bytes,
		Cost:             record.Cost,
		UsageLogID:       record.UsageLogID,
		SourceImageCount: record.SourceImageCount,
		ExpiresAt:        record.ExpiresAt,
		DeletedAt:        record.DeletedAt,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}
}
