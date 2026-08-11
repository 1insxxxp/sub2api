package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/ent/systemcustomgroupmodel"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type systemCustomGroupRepository struct {
	client *dbent.Client
}

func NewSystemCustomGroupRepository(client *dbent.Client) service.SystemCustomGroupRepository {
	return &systemCustomGroupRepository{client: client}
}

func (r *systemCustomGroupRepository) Create(ctx context.Context, groupIn *service.Group, models []service.SystemCustomGroupModelInput) error {
	if r == nil || r.client == nil {
		return errors.New("system custom group repository is not configured")
	}
	if groupIn == nil {
		return errors.New("group is nil")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	if err := createGroupRecord(ctx, client, groupIn); err != nil {
		return err
	}
	if err := createSystemCustomRouteSnapshot(ctx, client, groupIn.ID, models); err != nil {
		return err
	}
	if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventGroupChanged, nil, &groupIn.ID, nil); err != nil {
		return fmt.Errorf("enqueue system custom group scheduler event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *systemCustomGroupRepository) Update(ctx context.Context, groupIn *service.Group, models []service.SystemCustomGroupModelInput) error {
	if r == nil || r.client == nil {
		return errors.New("system custom group repository is not configured")
	}
	if groupIn == nil {
		return errors.New("group is nil")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	if err := updateSystemCustomGroupRecord(ctx, client, groupIn); err != nil {
		return err
	}
	if _, err := client.SystemCustomGroupModel.Delete().Where(systemcustomgroupmodel.GroupIDEQ(groupIn.ID)).Exec(ctx); err != nil {
		return fmt.Errorf("delete system custom group route snapshot: %w", err)
	}
	if err := createSystemCustomRouteSnapshot(ctx, client, groupIn.ID, models); err != nil {
		return err
	}
	if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventGroupChanged, nil, &groupIn.ID, nil); err != nil {
		return fmt.Errorf("enqueue system custom group scheduler event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *systemCustomGroupRepository) Delete(ctx context.Context, groupID int64) error {
	if r == nil || r.client == nil {
		return errors.New("system custom group repository is not configured")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()

	// Plans have no foreign key to groups, and an existing subscription can be
	// reactivated without changing group_id. Lock both tables so the reference
	// check and deletion form one atomic operation instead of a TOCTOU window.
	if _, err := client.ExecContext(ctx, "LOCK TABLE subscription_plans, user_subscriptions IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return fmt.Errorf("lock system custom group references: %w", err)
	}
	var isSystemCustom bool
	rows, err := client.QueryContext(ctx, `
		SELECT system_custom_routing_enabled
		FROM groups
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, groupID)
	if err != nil {
		return fmt.Errorf("lock system custom group: %w", err)
	}
	if rows.Next() {
		err = rows.Scan(&isSystemCustom)
	}
	closeErr := rows.Close()
	if err != nil {
		return fmt.Errorf("scan system custom group: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close system custom group row: %w", closeErr)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read system custom group: %w", err)
	}
	if !isSystemCustom {
		return service.ErrSystemCustomGroupNotFound
	}

	inUse, err := systemCustomGroupReferencesExist(ctx, client, groupID)
	if err != nil {
		return err
	}
	if inUse {
		return service.ErrSystemCustomGroupInUse
	}

	// Retired plans cannot outlive their group because subscription_plans has no
	// FK. Expired/cancelled subscriptions are removed by the group FK cascade.
	if _, err := client.ExecContext(ctx, "DELETE FROM subscription_plans WHERE group_id = $1", groupID); err != nil {
		return fmt.Errorf("delete retired system custom group plans: %w", err)
	}
	deleted, err := client.Group.Delete().
		Where(group.IDEQ(groupID), group.SystemCustomRoutingEnabledEQ(true)).
		Exec(mixins.SkipSoftDelete(ctx))
	if err != nil {
		return fmt.Errorf("delete system custom group: %w", err)
	}
	if deleted != 1 {
		return service.ErrSystemCustomGroupNotFound
	}
	if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventGroupChanged, nil, &groupID, nil); err != nil {
		return fmt.Errorf("enqueue system custom group scheduler event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func systemCustomGroupReferencesExist(ctx context.Context, client *dbent.Client, groupID int64) (bool, error) {
	queries := []struct {
		label string
		sql   string
	}{
		{label: "active plan", sql: "SELECT EXISTS (SELECT 1 FROM subscription_plans WHERE group_id = $1 AND for_sale = TRUE)"},
		{label: "active subscription", sql: "SELECT EXISTS (SELECT 1 FROM user_subscriptions WHERE group_id = $1 AND deleted_at IS NULL AND status = 'active' AND starts_at <= NOW() AND expires_at > NOW())"},
	}
	for _, query := range queries {
		rows, err := client.QueryContext(ctx, query.sql, groupID)
		if err != nil {
			return false, fmt.Errorf("check system custom group %s reference: %w", query.label, err)
		}
		var exists bool
		if rows.Next() {
			err = rows.Scan(&exists)
		}
		closeErr := rows.Close()
		if err != nil {
			return false, fmt.Errorf("scan system custom group %s reference: %w", query.label, err)
		}
		if closeErr != nil {
			return false, fmt.Errorf("close system custom group %s reference: %w", query.label, closeErr)
		}
		if err := rows.Err(); err != nil {
			return false, fmt.Errorf("read system custom group %s reference: %w", query.label, err)
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

func (r *systemCustomGroupRepository) Get(ctx context.Context, groupID int64) (*service.SystemCustomGroup, error) {
	row, err := r.client.Group.Query().
		Where(group.IDEQ(groupID), group.SystemCustomRoutingEnabledEQ(true)).
		WithSystemCustomRoutes(func(query *dbent.SystemCustomGroupModelQuery) {
			query.WithSourceGroup().Order(dbent.Asc(systemcustomgroupmodel.FieldID))
		}).
		Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, service.ErrSystemCustomGroupNotFound
	}
	if err != nil {
		return nil, err
	}
	out := &service.SystemCustomGroup{Group: *groupEntityToService(row), Models: make([]service.SystemCustomGroupModel, 0, len(row.Edges.SystemCustomRoutes))}
	for _, route := range row.Edges.SystemCustomRoutes {
		out.Models = append(out.Models, mapSystemCustomGroupModel(route))
	}
	return out, nil
}

func (r *systemCustomGroupRepository) ListModels(ctx context.Context, groupID int64, enabledOnly bool) ([]service.SystemCustomGroupModel, error) {
	query := r.client.SystemCustomGroupModel.Query().
		Where(systemcustomgroupmodel.GroupIDEQ(groupID)).
		WithSourceGroup().
		Order(dbent.Asc(systemcustomgroupmodel.FieldID))
	if enabledOnly {
		query = query.Where(systemcustomgroupmodel.EnabledEQ(true))
	}
	rows, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.SystemCustomGroupModel, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapSystemCustomGroupModel(row))
	}
	return out, nil
}

func (r *systemCustomGroupRepository) ResolveModel(ctx context.Context, groupID int64, publicModel string) (*service.SystemCustomGroupModel, error) {
	publicModel = strings.TrimSpace(publicModel)
	if publicModel == "" {
		return nil, service.ErrSystemCustomGroupRouteNotFound
	}
	row, err := r.client.SystemCustomGroupModel.Query().
		Where(
			systemcustomgroupmodel.GroupIDEQ(groupID),
			systemcustomgroupmodel.EnabledEQ(true),
			systemcustomgroupmodel.PublicModelEqualFold(publicModel),
			systemcustomgroupmodel.HasGroupWith(group.SystemCustomRoutingEnabledEQ(true)),
		).
		WithSourceGroup().
		Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, service.ErrSystemCustomGroupRouteNotFound
	}
	if err != nil {
		return nil, err
	}
	out := mapSystemCustomGroupModel(row)
	return &out, nil
}

func createSystemCustomRouteSnapshot(ctx context.Context, client *dbent.Client, groupID int64, models []service.SystemCustomGroupModelInput) error {
	for _, model := range models {
		_, err := client.SystemCustomGroupModel.Create().
			SetGroupID(groupID).
			SetPublicModel(model.PublicModel).
			SetSourceGroupID(model.SourceGroupID).
			SetSourceModel(model.SourceModel).
			SetEnabled(model.Enabled).
			Save(ctx)
		if err != nil {
			return translateSystemCustomRoutePersistenceError(err, model)
		}
	}
	return nil
}

func updateSystemCustomGroupRecord(ctx context.Context, client *dbent.Client, groupIn *service.Group) error {
	builder := client.Group.Update().
		Where(group.IDEQ(groupIn.ID), group.SystemCustomRoutingEnabledEQ(true)).
		SetName(groupIn.Name).
		SetDescription(groupIn.Description).
		SetPlatform(service.PlatformComposite).
		SetRateMultiplier(1).
		SetIsExclusive(true).
		SetSubscriptionType(service.SubscriptionTypeSubscription).
		SetSystemCustomRoutingEnabled(true).
		SetDefaultValidityDays(groupIn.DefaultValidityDays)
	if groupIn.DailyLimitUSD != nil {
		builder = builder.SetDailyLimitUsd(*groupIn.DailyLimitUSD)
	} else {
		builder = builder.ClearDailyLimitUsd()
	}
	if groupIn.WeeklyLimitUSD != nil {
		builder = builder.SetWeeklyLimitUsd(*groupIn.WeeklyLimitUSD)
	} else {
		builder = builder.ClearWeeklyLimitUsd()
	}
	if groupIn.MonthlyLimitUSD != nil {
		builder = builder.SetMonthlyLimitUsd(*groupIn.MonthlyLimitUSD)
	} else {
		builder = builder.ClearMonthlyLimitUsd()
	}
	affected, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrSystemCustomGroupNotFound, service.ErrGroupExists)
	}
	if affected == 0 {
		return service.ErrSystemCustomGroupNotFound
	}
	return nil
}

func mapSystemCustomGroupModel(row *dbent.SystemCustomGroupModel) service.SystemCustomGroupModel {
	out := service.SystemCustomGroupModel{
		ID: row.ID, GroupID: row.GroupID, PublicModel: row.PublicModel,
		SourceGroupID: row.SourceGroupID, SourceModel: row.SourceModel, Enabled: row.Enabled,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.Edges.SourceGroup != nil {
		out.SourceGroup = groupEntityToService(row.Edges.SourceGroup)
	}
	return out
}

func translateSystemCustomRoutePersistenceError(err error, model service.SystemCustomGroupModelInput) error {
	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		switch pgErr.Constraint {
		case "uq_system_custom_group_public_model_ci":
			return &service.SystemCustomGroupRouteError{Kind: service.ErrSystemCustomGroupDuplicatePublicModel, PublicModel: model.PublicModel, SourceGroupID: model.SourceGroupID, SourceModel: model.SourceModel}
		case "uq_system_custom_group_source_model_ci":
			return &service.SystemCustomGroupRouteError{Kind: service.ErrSystemCustomGroupDuplicateSourceModel, PublicModel: model.PublicModel, SourceGroupID: model.SourceGroupID, SourceModel: model.SourceModel}
		case "system_custom_group_models_no_self_reference":
			return &service.SystemCustomGroupRouteError{Kind: service.ErrSystemCustomGroupSelfReference, PublicModel: model.PublicModel, SourceGroupID: model.SourceGroupID, SourceModel: model.SourceModel}
		}
	}
	return fmt.Errorf("persist system custom group route public_model=%q source_group_id=%d source_model=%q: %w", model.PublicModel, model.SourceGroupID, model.SourceModel, err)
}
