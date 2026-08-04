package repository

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/ent/usercustomgroup"
	"github.com/Wei-Shaw/sub2api/ent/usercustomgroupmodel"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userCustomGroupRepository struct{ client *dbent.Client }

func NewUserCustomGroupRepository(client *dbent.Client) service.UserCustomGroupRepository {
	return &userCustomGroupRepository{client: client}
}

func (r *userCustomGroupRepository) ListByUserID(ctx context.Context, userID int64) ([]service.UserCustomGroup, error) {
	rows, err := r.client.UserCustomGroup.Query().Where(usercustomgroup.UserIDEQ(userID), usercustomgroup.DeletedAtIsNil()).WithModels(func(q *dbent.UserCustomGroupModelQuery) { q.WithSourceGroup() }).Order(dbent.Desc(usercustomgroup.FieldCreatedAt)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.UserCustomGroup, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapUserCustomGroup(row))
	}
	return out, nil
}

func (r *userCustomGroupRepository) GetOwned(ctx context.Context, id, userID int64) (*service.UserCustomGroup, error) {
	row, err := r.client.UserCustomGroup.Query().Where(usercustomgroup.IDEQ(id), usercustomgroup.UserIDEQ(userID), usercustomgroup.DeletedAtIsNil()).WithModels(func(q *dbent.UserCustomGroupModelQuery) { q.WithSourceGroup() }).Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, service.ErrUserCustomGroupNotFound
	}
	if err != nil {
		return nil, err
	}
	out := mapUserCustomGroup(row)
	return &out, nil
}

func (r *userCustomGroupRepository) Create(ctx context.Context, group *service.UserCustomGroup, models []service.UserCustomGroupModelInput) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	row, err := tx.UserCustomGroup.Create().SetUserID(group.UserID).SetName(group.Name).SetStatus(group.Status).Save(ctx)
	if err != nil {
		return err
	}
	for _, m := range models {
		if _, err = tx.UserCustomGroupModel.Create().SetCustomGroupID(row.ID).SetPublicModel(m.PublicModel).SetSourceGroupID(m.SourceGroupID).SetSourceModel(m.SourceModel).Save(ctx); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	group.ID, group.CreatedAt, group.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
	return nil
}

func (r *userCustomGroupRepository) Update(ctx context.Context, group *service.UserCustomGroup, models *[]service.UserCustomGroupModelInput) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	n, err := tx.UserCustomGroup.Update().Where(usercustomgroup.IDEQ(group.ID), usercustomgroup.UserIDEQ(group.UserID), usercustomgroup.DeletedAtIsNil()).SetName(group.Name).SetStatus(group.Status).Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrUserCustomGroupNotFound
	}
	if models != nil {
		if _, err = tx.UserCustomGroupModel.Delete().Where(usercustomgroupmodel.CustomGroupIDEQ(group.ID)).Exec(ctx); err != nil {
			return err
		}
		for _, m := range *models {
			if _, err = tx.UserCustomGroupModel.Create().SetCustomGroupID(group.ID).SetPublicModel(m.PublicModel).SetSourceGroupID(m.SourceGroupID).SetSourceModel(m.SourceModel).Save(ctx); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (r *userCustomGroupRepository) Delete(ctx context.Context, id, userID int64) error {
	n, err := r.client.UserCustomGroup.Update().Where(usercustomgroup.IDEQ(id), usercustomgroup.UserIDEQ(userID), usercustomgroup.DeletedAtIsNil()).SetDeletedAt(time.Now()).SetStatus(service.StatusDisabled).Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrUserCustomGroupNotFound
	}
	return nil
}

func (r *userCustomGroupRepository) CountByUserID(ctx context.Context, userID int64) (int, error) {
	return r.client.UserCustomGroup.Query().Where(usercustomgroup.UserIDEQ(userID), usercustomgroup.DeletedAtIsNil()).Count(ctx)
}
func (r *userCustomGroupRepository) CountBoundAPIKeys(ctx context.Context, id int64) (int, error) {
	return r.client.APIKey.Query().Where(apikey.CustomGroupIDEQ(id), apikey.DeletedAtIsNil()).Count(ctx)
}

func (r *userCustomGroupRepository) ResolveModel(ctx context.Context, id, userID int64, model string) (*service.UserCustomGroupModel, error) {
	row, err := r.client.UserCustomGroupModel.Query().Where(usercustomgroupmodel.CustomGroupIDEQ(id), usercustomgroupmodel.PublicModelEqualFold(model), usercustomgroupmodel.HasCustomGroupWith(usercustomgroup.UserIDEQ(userID), usercustomgroup.DeletedAtIsNil(), usercustomgroup.StatusEQ(service.StatusActive))).WithSourceGroup().Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, service.ErrUserCustomGroupInvalidModel
	}
	if err != nil {
		return nil, err
	}
	out := mapUserCustomGroupModel(row)
	return &out, nil
}

func mapUserCustomGroup(row *dbent.UserCustomGroup) service.UserCustomGroup {
	out := service.UserCustomGroup{ID: row.ID, UserID: row.UserID, Name: row.Name, Status: row.Status, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Models: []service.UserCustomGroupModel{}}
	for _, m := range row.Edges.Models {
		out.Models = append(out.Models, mapUserCustomGroupModel(m))
	}
	return out
}

func mapUserCustomGroupModel(row *dbent.UserCustomGroupModel) service.UserCustomGroupModel {
	out := service.UserCustomGroupModel{ID: row.ID, CustomGroupID: row.CustomGroupID, PublicModel: row.PublicModel, SourceGroupID: row.SourceGroupID, SourceModel: row.SourceModel, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	if row.Edges.SourceGroup != nil {
		out.SourceGroup = groupEntityToService(row.Edges.SourceGroup)
	}
	return out
}
