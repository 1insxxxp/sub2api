package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/groupref"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type categoryDefaultGroupRepository struct{ client *dbent.Client }

func NewCategoryDefaultGroupRepository(client *dbent.Client, _ *sql.DB) service.CategoryDefaultGroupRepository {
	return &categoryDefaultGroupRepository{client: client}
}

func (r *categoryDefaultGroupRepository) List(ctx context.Context) ([]service.CategoryDefaultGroupConfig, error) {
	out := make([]service.CategoryDefaultGroupConfig, 0)
	for _, category := range service.CategoryDefaultCategories {
		config, err := categoryDefaultLoadConfig(ctx, r.client, category, 0, false)
		if errors.Is(err, service.ErrCategoryDefaultNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, *config)
	}
	return out, nil
}

func (r *categoryDefaultGroupRepository) Get(ctx context.Context, category string) (*service.CategoryDefaultGroupConfig, error) {
	return categoryDefaultLoadConfig(ctx, r.client, category, 0, false)
}
func (r *categoryDefaultGroupRepository) GetByGroupID(ctx context.Context, id int64) (*service.CategoryDefaultGroupConfig, error) {
	return categoryDefaultLoadConfig(ctx, r.client, "", id, false)
}

func categoryDefaultLoadConfig(ctx context.Context, client *dbent.Client, category string, id int64, lock bool) (*service.CategoryDefaultGroupConfig, error) {
	query := `SELECT c.group_id,c.category,g.name,g.status,c.revision FROM category_default_groups c JOIN groups g ON g.id=c.group_id WHERE g.deleted_at IS NULL AND g.is_category_default=TRUE AND (c.category=$1 OR ($2>0 AND c.group_id=$2))`
	if lock {
		query += ` FOR UPDATE OF c,g`
	}
	rows, err := client.QueryContext(ctx, query, category, id)
	if err != nil {
		return nil, err
	}
	config := &service.CategoryDefaultGroupConfig{SourceGroupIDs: []int64{}, Models: []service.CategoryDefaultPublishedModel{}}
	status := ""
	if !rows.Next() {
		rowErr := rows.Err()
		_ = rows.Close()
		if rowErr != nil {
			return nil, rowErr
		}
		return nil, service.ErrCategoryDefaultNotFound
	}
	err = rows.Scan(&config.GroupID, &config.Category, &config.Name, &status, &config.Revision)
	closeErr := rows.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	config.Enabled = status == service.StatusActive
	rows, err = client.QueryContext(ctx, `SELECT source_group_id FROM category_default_group_sources WHERE group_id=$1 ORDER BY source_group_id`, config.GroupID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sourceID int64
		if err := rows.Scan(&sourceID); err != nil {
			_ = rows.Close()
			return nil, err
		}
		config.SourceGroupIDs = append(config.SourceGroupIDs, sourceID)
	}
	err = rows.Err()
	closeErr = rows.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	rows, err = client.QueryContext(ctx, `SELECT public_model,source_group_id,source_model,fingerprint FROM category_default_group_models WHERE group_id=$1 ORDER BY lower(public_model),public_model`, config.GroupID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var model service.CategoryDefaultPublishedModel
		if err := rows.Scan(&model.PublicModel, &model.SourceGroupID, &model.SourceModel, &model.Fingerprint); err != nil {
			_ = rows.Close()
			return nil, err
		}
		config.Models = append(config.Models, model)
	}
	err = rows.Err()
	closeErr = rows.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	config.ModelsCount = len(config.Models)
	if err := categoryDefaultLoadInvalidations(ctx, client, config); err != nil {
		return nil, err
	}
	return config, nil
}

func (r *categoryDefaultGroupRepository) Sources(ctx context.Context, ids []int64) ([]service.CategoryDefaultSourceSnapshot, error) {
	return categoryDefaultLoadSources(ctx, r.client, ids)
}

func (r *categoryDefaultGroupRepository) CandidateSources(ctx context.Context, category string) ([]service.CategoryDefaultSourceSnapshot, error) {
	groups, err := r.client.Group.Query().Where(group.StatusEQ(service.StatusActive), group.SubscriptionTypeEQ(service.SubscriptionTypeStandard), group.IsCategoryDefaultEQ(false), group.SystemCustomRoutingEnabledEQ(false)).Order(dbent.Asc(group.FieldSortOrder), dbent.Asc(group.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(groups))
	for _, item := range groups {
		if service.EffectiveKeyDisplayCategory(groupEntityToService(item)) == category {
			ids = append(ids, item.ID)
		}
	}
	return categoryDefaultLoadSources(ctx, r.client, ids)
}

func categoryDefaultLoadSources(ctx context.Context, client *dbent.Client, ids []int64) ([]service.CategoryDefaultSourceSnapshot, error) {
	out := make([]service.CategoryDefaultSourceSnapshot, 0, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	groups, err := client.Group.Query().Where(group.IDIn(ids...)).Order(dbent.Asc(group.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := (&settingRepository{client: client}).GetMultiple(ctx, []string{service.SettingKeyGrokDefaultTextModel, service.SettingKeyGrokCrossClientModelMapEnabled})
	if err != nil {
		return nil, err
	}
	grokOptions := service.CategoryDefaultGrokMappingOptionsFromSettings(settings)
	ar := newAccountRepositoryWithSQL(client, client, nil)
	cr := NewCompositeModelRouteRepository(client)
	for _, entity := range groups {
		g := groupEntityToService(entity)
		platforms := []string{g.Platform}
		if g.Platform == service.PlatformComposite {
			platforms = domain.ConcretePlatformIDs()
		} else if g.Platform == service.PlatformAnthropic || g.Platform == service.PlatformGemini {
			platforms = append(platforms, service.PlatformAntigravity)
		}
		accounts, err := ar.ListModelAvailabilityCandidates(ctx, &g.ID, platforms, false)
		if err != nil {
			return nil, err
		}
		snapshot := service.CategoryDefaultSourceSnapshot{Group: g, Accounts: accounts, Routes: []service.CompositeModelRoute{}, GrokMappingOptions: &grokOptions}
		if g.Platform == service.PlatformComposite {
			snapshot.Routes, err = cr.ListByGroup(ctx, g.ID, false)
			if err != nil {
				return nil, err
			}
		}
		snapshot.Channel, err = categoryDefaultLoadSourceChannel(ctx, client, g.ID)
		if err != nil {
			return nil, err
		}
		snapshot.DeclarationEpoch, err = categoryDefaultSourceEpoch(ctx, client, g.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, snapshot)
	}
	return out, nil
}

func categoryDefaultLoadSourceChannel(ctx context.Context, client *dbent.Client, groupID int64) (*service.Channel, error) {
	rows, err := client.QueryContext(ctx, `SELECT c.id,c.status,c.model_mapping FROM channels c JOIN channel_groups cg ON cg.channel_id=c.id WHERE cg.group_id=$1 AND c.status='active'`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	channel := &service.Channel{GroupIDs: []int64{groupID}}
	var rawMapping []byte
	if err := rows.Scan(&channel.ID, &channel.Status, &rawMapping); err != nil {
		return nil, err
	}
	channel.ModelMapping = unmarshalModelMapping(rawMapping)
	return channel, rows.Err()
}

func (r *categoryDefaultGroupRepository) Publish(ctx context.Context, category string, ids []int64, expected int64, validate service.CategoryDefaultPublishValidator) (*service.CategoryDefaultGroupConfig, error) {
	tx, err := r.client.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, categoryDefaultTransactionError(err)
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	// This lock also serializes the first publication, before an entry row exists.
	rows, err := client.QueryContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "sub2api-category-default:"+category)
	if err != nil {
		return nil, categoryDefaultTransactionError(err)
	}
	for rows.Next() {
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	// Alias/binding writes already acquire source reference locks before their
	// declaration triggers take the config lock. Use that same ordering here.
	if err := groupref.LockGroupReferenceWrites(ctx, tx, ids...); err != nil {
		return nil, err
	}
	current, err := categoryDefaultLoadConfig(ctx, client, category, 0, true)
	if errors.Is(err, service.ErrCategoryDefaultNotFound) {
		current = &service.CategoryDefaultGroupConfig{Category: category, SourceGroupIDs: []int64{}, Models: []service.CategoryDefaultPublishedModel{}}
	} else if err != nil {
		return nil, categoryDefaultTransactionError(err)
	}
	if current.Revision != expected {
		return nil, service.ErrCategoryDefaultStalePreview
	}
	// Do not take source group row locks after the config lock: source writers
	// already hold their group row before the declaration trigger locks config.
	// The source clock serializes both paths without that opposite lock order;
	// SERIALIZABLE plus the fresh eligibility validation rejects stale sources.
	if err := categoryDefaultLockSourceClocks(ctx, client, ids); err != nil {
		return nil, categoryDefaultTransactionError(err)
	}
	sources, err := categoryDefaultLoadSources(ctx, client, ids)
	if err != nil {
		return nil, err
	}
	models, err := validate(current, sources)
	if err != nil {
		return nil, err
	}
	if current.GroupID == 0 {
		g := &service.Group{Name: categoryDefaultName(category), Description: "按已发布模型关联来源分组，沿来源分组计费", Platform: service.PlatformComposite, KeyDisplayCategory: category, IsCategoryDefault: true, RateMultiplier: 1, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, SortOrder: -1000}
		if err := createGroupRecord(ctx, client, g); err != nil {
			return nil, categoryDefaultTransactionError(err)
		}
		current.GroupID = g.ID
		current.Enabled = true
		current.Name = g.Name
		if _, err := client.ExecContext(ctx, `INSERT INTO category_default_groups(category,group_id,revision) VALUES($1,$2,1)`, category, g.ID); err != nil {
			return nil, categoryDefaultTransactionError(err)
		}
	} else {
		if _, err := client.ExecContext(ctx, `UPDATE category_default_groups SET revision=revision+1,publication_revision=publication_revision+1,updated_at=NOW() WHERE category=$1`, category); err != nil {
			return nil, err
		}
	}
	// Confirmation supersedes the old declaration gate. Keep only the current
	// publication's history; source writers hold the config lock and therefore
	// cannot append to an old publication after this transaction commits.
	if _, err := client.ExecContext(ctx, `DELETE FROM category_default_declaration_history WHERE entry_group_id=$1 AND publication_revision < (SELECT publication_revision FROM category_default_groups WHERE group_id=$1)`, current.GroupID); err != nil {
		return nil, err
	}
	if _, err := client.ExecContext(ctx, `DELETE FROM category_default_group_models WHERE group_id=$1`, current.GroupID); err != nil {
		return nil, err
	}
	if _, err := client.ExecContext(ctx, `DELETE FROM category_default_group_sources WHERE group_id=$1`, current.GroupID); err != nil {
		return nil, err
	}
	for _, sourceID := range ids {
		if _, err := client.ExecContext(ctx, `INSERT INTO category_default_group_sources(group_id,source_group_id) VALUES($1,$2)`, current.GroupID, sourceID); err != nil {
			return nil, err
		}
	}
	for _, model := range models {
		if _, err := client.ExecContext(ctx, `INSERT INTO category_default_group_models(group_id,public_model,source_group_id,source_model,fingerprint) VALUES($1,$2,$3,$4,$5)`, current.GroupID, model.PublicModel, model.SourceGroupID, model.SourceModel, model.Fingerprint); err != nil {
			return nil, categoryDefaultTransactionError(err)
		}
	}
	if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventGroupChanged, nil, &current.GroupID, nil); err != nil {
		return nil, err
	}
	result, err := categoryDefaultLoadConfig(ctx, client, category, 0, false)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, categoryDefaultTransactionError(err)
	}
	return result, nil
}

func (r *categoryDefaultGroupRepository) SetStatus(ctx context.Context, category string, enabled bool) (*service.CategoryDefaultGroupConfig, error) {
	tx, err := r.client.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	current, err := categoryDefaultLoadConfig(ctx, client, category, 0, true)
	if err != nil {
		return nil, err
	}
	if enabled && len(current.Models) == 0 {
		return nil, service.ErrCategoryDefaultInvalidSource
	}
	status := "inactive"
	if enabled {
		status = service.StatusActive
	}
	if _, err := client.Group.UpdateOneID(current.GroupID).SetStatus(status).Save(ctx); err != nil {
		return nil, err
	}
	if _, err := client.ExecContext(ctx, `UPDATE category_default_groups SET revision=revision+1,updated_at=NOW() WHERE category=$1`, category); err != nil {
		return nil, err
	}
	if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventGroupChanged, nil, &current.GroupID, nil); err != nil {
		return nil, err
	}
	result, err := categoryDefaultLoadConfig(ctx, client, category, 0, false)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, categoryDefaultTransactionError(err)
	}
	return result, nil
}

func categoryDefaultTransactionError(err error) error {
	var pgErr *pq.Error
	if errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01" || pgErr.Code == "23505") {
		return service.ErrCategoryDefaultStalePreview.WithCause(err)
	}
	return err
}

func categoryDefaultName(category string) string {
	names := map[string]string{"anthropic": "Anthropic", "openai": "OpenAI", "gemini": "Gemini", "grok": "Grok", "domestic": "国产模型", "other": "其他"}
	name, ok := names[category]
	if !ok {
		name = fmt.Sprint(category)
	}
	return name + " 默认分组"
}
