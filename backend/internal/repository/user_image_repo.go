package repository

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/userimage"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userImageRepository struct {
	client *dbent.Client
}

func NewUserImageRepository(client *dbent.Client) service.UserImageRepository {
	return &userImageRepository{client: client}
}

func (r *userImageRepository) Create(ctx context.Context, record *service.ImageStudioImageRecord) error {
	if record == nil {
		return nil
	}
	client := clientFromContext(ctx, r.client)
	builder := client.UserImage.Create().
		SetUserID(record.UserID).
		SetMode(record.Mode).
		SetModel(record.Model).
		SetPrompt(record.Prompt).
		SetAspectRatio(record.AspectRatio).
		SetSize(record.Size).
		SetImageURL(record.ImageURL).
		SetStorageDriver(record.StorageDriver).
		SetStorageObjectKey(record.StorageObjectKey).
		SetMimeType(record.MimeType).
		SetBytes(record.Bytes).
		SetCost(record.Cost).
		SetSourceImageCount(record.SourceImageCount).
		SetNillableUsageLogID(record.UsageLogID).
		SetNillableExpiresAt(record.ExpiresAt).
		SetNillableDeletedAt(record.DeletedAt)
	if !record.CreatedAt.IsZero() {
		builder.SetCreatedAt(record.CreatedAt)
	}
	if !record.UpdatedAt.IsZero() {
		builder.SetUpdatedAt(record.UpdatedAt)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return err
	}
	*record = *userImageEntityToService(created)
	return nil
}

func (r *userImageRepository) ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.ImageStudioImageRecord, *pagination.PaginationResult, error) {
	client := clientFromContext(ctx, r.client)
	query := client.UserImage.Query().
		Where(userimage.UserIDEQ(userID), userimage.DeletedAtIsNil())
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	if total == 0 {
		return []service.ImageStudioImageRecord{}, paginationResultFromTotal(0, params), nil
	}
	rows, err := query.
		Order(dbent.Desc(userimage.FieldCreatedAt), dbent.Desc(userimage.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	return userImageEntitiesToService(rows), paginationResultFromTotal(int64(total), params), nil
}

func (r *userImageRepository) GetByID(ctx context.Context, id int64) (*service.ImageStudioImageRecord, error) {
	client := clientFromContext(ctx, r.client)
	row, err := client.UserImage.Query().
		Where(userimage.IDEQ(id), userimage.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrUserImageNotFound
		}
		return nil, err
	}
	return userImageEntityToService(row), nil
}

func (r *userImageRepository) SoftDelete(ctx context.Context, id int64, userID int64) error {
	client := clientFromContext(ctx, r.client)
	affected, err := client.UserImage.Update().
		Where(userimage.IDEQ(id), userimage.UserIDEQ(userID), userimage.DeletedAtIsNil()).
		SetDeletedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrUserImageNotFound
	}
	return nil
}

func (r *userImageRepository) CountSavedByUser(ctx context.Context, userID int64) (int64, error) {
	client := clientFromContext(ctx, r.client)
	count, err := client.UserImage.Query().
		Where(userimage.UserIDEQ(userID), userimage.DeletedAtIsNil()).
		Count(ctx)
	return int64(count), err
}

func (r *userImageRepository) DeleteOldestOverLimit(ctx context.Context, userID int64, limit int) ([]service.ImageStudioImageRecord, error) {
	if limit < 0 {
		limit = 0
	}
	client := clientFromContext(ctx, r.client)
	total, err := client.UserImage.Query().
		Where(userimage.UserIDEQ(userID), userimage.DeletedAtIsNil()).
		Count(ctx)
	if err != nil {
		return nil, err
	}
	over := total - limit
	if over <= 0 {
		return []service.ImageStudioImageRecord{}, nil
	}

	rows, err := client.UserImage.Query().
		Where(userimage.UserIDEQ(userID), userimage.DeletedAtIsNil()).
		Order(dbent.Asc(userimage.FieldCreatedAt), dbent.Asc(userimage.FieldID)).
		Limit(over).
		All(ctx)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []service.ImageStudioImageRecord{}, nil
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	_, err = client.UserImage.Update().
		Where(userimage.IDIn(ids...), userimage.DeletedAtIsNil()).
		SetDeletedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return userImageEntitiesToService(rows), nil
}

func (r *userImageRepository) ListExpired(ctx context.Context, now time.Time, limit int) ([]service.ImageStudioImageRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.UserImage.Query().
		Where(
			userimage.DeletedAtIsNil(),
			userimage.ExpiresAtNotNil(),
			userimage.ExpiresAtLTE(now),
		).
		Order(dbent.Asc(userimage.FieldExpiresAt), dbent.Asc(userimage.FieldID)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return userImageEntitiesToService(rows), nil
}

func userImageEntitiesToService(rows []*dbent.UserImage) []service.ImageStudioImageRecord {
	out := make([]service.ImageStudioImageRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, *userImageEntityToService(row))
	}
	return out
}

func userImageEntityToService(row *dbent.UserImage) *service.ImageStudioImageRecord {
	if row == nil {
		return nil
	}
	out := &service.ImageStudioImageRecord{
		ID:               row.ID,
		UserID:           row.UserID,
		Mode:             row.Mode,
		Model:            row.Model,
		AspectRatio:      row.AspectRatio,
		Size:             row.Size,
		ImageURL:         row.ImageURL,
		StorageDriver:    row.StorageDriver,
		StorageObjectKey: row.StorageObjectKey,
		MimeType:         row.MimeType,
		Bytes:            row.Bytes,
		Cost:             row.Cost,
		UsageLogID:       row.UsageLogID,
		SourceImageCount: row.SourceImageCount,
		ExpiresAt:        row.ExpiresAt,
		DeletedAt:        row.DeletedAt,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
	if row.Prompt != nil {
		out.Prompt = *row.Prompt
	}
	return out
}
