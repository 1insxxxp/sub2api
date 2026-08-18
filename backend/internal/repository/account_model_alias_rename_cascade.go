package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/groupref"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const (
	accountModelAliasRenameChannelPricingScope           = "channel_pricing"
	accountModelAliasRenameChannelMappingScope           = "channel_mapping"
	accountModelAliasRenameUserRouteSourceScope          = "user_custom_route_source_model"
	accountModelAliasRenameUserRoutePublicScope          = "user_custom_route_public_model"
	accountModelAliasRenameSystemRouteSourceScope        = "system_custom_route_source_model"
	accountModelAliasRenameSystemRoutePublicScope        = "system_custom_route_public_model"
	accountModelAliasRenameReasonNewModelExists          = "new_model_already_configured"
	accountModelAliasRenameReasonSourceModelConflict     = "source_model_conflict"
	accountModelAliasRenameReasonPublicModelConflict     = "public_model_conflict"
	accountModelAliasRenameReasonUpdateAffectedNoRows    = "update_affected_no_rows"
	accountModelAliasRenameReasonRepositoryNotConfigured = "repository_not_configured"
)

type accountModelAliasRenameChannel struct {
	id           int64
	modelMapping map[string]map[string]string
}

type accountModelAliasRenamePricingRow struct {
	id        int64
	channelID int64
	models    []string
}

type accountModelAliasRenameRoute struct {
	id            int64
	ownerID       int64
	publicModel   string
	sourceGroupID int64
	sourceModel   string
}

type accountModelAliasRenameRouteQueries struct {
	lockCandidatesSQL string
	candidatesSQL     string
	sourceConflictSQL string
	publicConflictSQL string
	updateSQL         string
	sourceSkipScope   string
	publicSkipScope   string
}

func (r *channelRepository) CascadeAccountModelAliasRenames(ctx context.Context, _ int64, groupIDs []int64, renames []service.AccountModelAliasRename) (*service.AccountModelAliasRenameCascadeResult, error) {
	result := &service.AccountModelAliasRenameCascadeResult{}
	groupIDs, renames = normalizeRepositoryAccountModelAliasRenameInputs(groupIDs, renames)
	if len(groupIDs) == 0 || len(renames) == 0 {
		return result, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New(accountModelAliasRenameReasonRepositoryNotConfigured)
	}

	err := r.runInTx(ctx, func(tx *sql.Tx) error {
		channels, channelIDs, err := loadAccountModelAliasRenameChannels(ctx, tx, groupIDs)
		if err != nil {
			return err
		}
		if len(channelIDs) == 0 {
			return nil
		}
		pricingRows, err := loadAccountModelAliasRenamePricingRows(ctx, tx, channelIDs)
		if err != nil {
			return err
		}

		for _, rename := range renames {
			updated, skipped, err := cascadeAccountModelAliasRenameChannelPricing(ctx, tx, pricingRows, rename)
			if err != nil {
				return err
			}
			result.ChannelPricingUpdated += updated
			result.Skipped = append(result.Skipped, skipped...)
		}
		for _, rename := range renames {
			updated, skipped, err := cascadeAccountModelAliasRenameChannelMappings(ctx, tx, channels, rename)
			if err != nil {
				return err
			}
			result.ChannelMappingsUpdated += updated
			result.Skipped = append(result.Skipped, skipped...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *userCustomGroupRepository) CascadeAccountModelAliasRenames(ctx context.Context, _ int64, groupIDs []int64, renames []service.AccountModelAliasRename) (*service.AccountModelAliasRenameCascadeResult, error) {
	result := &service.AccountModelAliasRenameCascadeResult{}
	groupIDs, renames = normalizeRepositoryAccountModelAliasRenameInputs(groupIDs, renames)
	if len(groupIDs) == 0 || len(renames) == 0 {
		return result, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New(accountModelAliasRenameReasonRepositoryNotConfigured)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin user custom model alias rename cascade: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	changedOwners := make(map[int64]struct{})
	for _, rename := range renames {
		updated, skipped, err := cascadeAccountModelAliasRenameRoutes(ctx, tx, accountModelAliasRenameUserRouteQueries(), groupIDs, rename, changedOwners)
		if err != nil {
			return nil, err
		}
		result.UserCustomRoutesUpdated += updated
		result.Skipped = append(result.Skipped, skipped...)
	}
	if len(changedOwners) > 0 {
		if err := enqueueUserCustomGroupAuthCacheInvalidations(ctx, tx, sortedInt64Keys(changedOwners)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *systemCustomGroupRepository) CascadeAccountModelAliasRenames(ctx context.Context, _ int64, groupIDs []int64, renames []service.AccountModelAliasRename) (*service.AccountModelAliasRenameCascadeResult, error) {
	result := &service.AccountModelAliasRenameCascadeResult{}
	groupIDs, renames = normalizeRepositoryAccountModelAliasRenameInputs(groupIDs, renames)
	if len(groupIDs) == 0 || len(renames) == 0 {
		return result, nil
	}
	if r == nil || r.client == nil {
		return nil, errors.New("system custom group repository is not configured")
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin system custom model alias rename cascade: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()

	queries := accountModelAliasRenameSystemRouteQueries()
	lockGroupIDs := make(map[int64]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		lockGroupIDs[groupID] = struct{}{}
	}
	for _, rename := range renames {
		ownerIDs, err := loadAccountModelAliasRenameSystemRouteOwnerIDs(ctx, client, queries.lockCandidatesSQL, groupIDs, rename.OldModel)
		if err != nil {
			return nil, err
		}
		for _, ownerID := range ownerIDs {
			lockGroupIDs[ownerID] = struct{}{}
		}
	}
	if err := groupref.LockGroupReferenceWrites(ctx, tx, sortedInt64Keys(lockGroupIDs)...); err != nil {
		return nil, fmt.Errorf("lock system custom group alias rename references: %w", err)
	}

	changedGroups := make(map[int64]struct{})
	for _, rename := range renames {
		routes, err := loadAccountModelAliasRenameRoutes(ctx, client, queries.candidatesSQL, groupIDs, rename.OldModel)
		if err != nil {
			return nil, err
		}
		updated, skipped, err := processAccountModelAliasRenameRoutes(ctx, client, queries, routes, rename, changedGroups)
		if err != nil {
			return nil, err
		}
		result.SystemCustomRoutesUpdated += updated
		result.Skipped = append(result.Skipped, skipped...)
	}
	for _, groupID := range sortedInt64Keys(changedGroups) {
		id := groupID
		if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventGroupChanged, nil, &id, nil); err != nil {
			return nil, fmt.Errorf("enqueue system custom group alias rename scheduler event: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func loadAccountModelAliasRenameChannels(ctx context.Context, exec dbExec, groupIDs []int64) ([]accountModelAliasRenameChannel, []int64, error) {
	rows, err := exec.QueryContext(ctx, `
SELECT c.id, c.model_mapping
FROM channels AS c
WHERE EXISTS (
    SELECT 1
    FROM channel_groups AS cg
    WHERE cg.channel_id = c.id
      AND cg.group_id = ANY($1)
)
ORDER BY c.id
FOR UPDATE
`, pq.Array(groupIDs))
	if err != nil {
		return nil, nil, fmt.Errorf("load channel alias rename candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	channels := []accountModelAliasRenameChannel{}
	channelIDs := []int64{}
	for rows.Next() {
		var (
			channel accountModelAliasRenameChannel
			raw     []byte
		)
		if err := rows.Scan(&channel.id, &raw); err != nil {
			return nil, nil, fmt.Errorf("scan channel alias rename candidate: %w", err)
		}
		channel.modelMapping = unmarshalModelMapping(raw)
		channels = append(channels, channel)
		channelIDs = append(channelIDs, channel.id)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate channel alias rename candidates: %w", err)
	}
	return channels, channelIDs, nil
}

func loadAccountModelAliasRenamePricingRows(ctx context.Context, exec dbExec, channelIDs []int64) ([]accountModelAliasRenamePricingRow, error) {
	rows, err := exec.QueryContext(ctx, `
SELECT id, channel_id, models
FROM channel_model_pricing
WHERE channel_id = ANY($1)
  AND platform = $2
ORDER BY channel_id, id
FOR UPDATE
`, pq.Array(channelIDs), service.PlatformAntigravity)
	if err != nil {
		return nil, fmt.Errorf("load channel pricing alias rename candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	pricingRows := []accountModelAliasRenamePricingRow{}
	for rows.Next() {
		var (
			row accountModelAliasRenamePricingRow
			raw []byte
		)
		if err := rows.Scan(&row.id, &row.channelID, &raw); err != nil {
			return nil, fmt.Errorf("scan channel pricing alias rename candidate: %w", err)
		}
		if err := json.Unmarshal(raw, &row.models); err != nil {
			return nil, fmt.Errorf("unmarshal channel pricing models for alias rename: %w", err)
		}
		pricingRows = append(pricingRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channel pricing alias rename candidates: %w", err)
	}
	return pricingRows, nil
}

func cascadeAccountModelAliasRenameChannelPricing(ctx context.Context, exec dbExec, pricingRows []accountModelAliasRenamePricingRow, rename service.AccountModelAliasRename) (int, []service.AccountModelAliasRenameSkipItem, error) {
	byChannel := make(map[int64][]*accountModelAliasRenamePricingRow)
	for i := range pricingRows {
		row := &pricingRows[i]
		byChannel[row.channelID] = append(byChannel[row.channelID], row)
	}

	updated := 0
	skipped := []service.AccountModelAliasRenameSkipItem{}
	channelIDs := sortedInt64KeysFromPricingRows(byChannel)
	for _, channelID := range channelIDs {
		rows := byChannel[channelID]
		oldRows := []*accountModelAliasRenamePricingRow{}
		newExists := false
		for _, row := range rows {
			if accountModelAliasRenamePricingModelsConflict(row.models, rename.NewModel) {
				newExists = true
			}
			if containsExactString(row.models, rename.OldModel) {
				oldRows = append(oldRows, row)
			}
		}
		if len(oldRows) == 0 {
			continue
		}
		if newExists {
			skipped = append(skipped, service.AccountModelAliasRenameSkipItem{
				Scope:    accountModelAliasRenameChannelPricingScope,
				OwnerID:  channelID,
				OldModel: rename.OldModel,
				NewModel: rename.NewModel,
				Reason:   accountModelAliasRenameReasonNewModelExists,
			})
			continue
		}
		for _, row := range oldRows {
			row.models = append(row.models, rename.NewModel)
			if err := updateAccountModelAliasRenameChannelPricingModels(ctx, exec, row.id, row.models); err != nil {
				return 0, nil, err
			}
			updated++
		}
	}
	return updated, skipped, nil
}

func cascadeAccountModelAliasRenameChannelMappings(ctx context.Context, exec dbExec, channels []accountModelAliasRenameChannel, rename service.AccountModelAliasRename) (int, []service.AccountModelAliasRenameSkipItem, error) {
	updated := 0
	skipped := []service.AccountModelAliasRenameSkipItem{}
	for i := range channels {
		platformMapping := channels[i].modelMapping[service.PlatformAntigravity]
		if len(platformMapping) == 0 {
			continue
		}
		target, ok := platformMapping[rename.OldModel]
		if !ok {
			continue
		}
		if accountModelAliasRenameMappingModelsConflict(platformMapping, rename.NewModel) {
			skipped = append(skipped, service.AccountModelAliasRenameSkipItem{
				Scope:    accountModelAliasRenameChannelMappingScope,
				OwnerID:  channels[i].id,
				OldModel: rename.OldModel,
				NewModel: rename.NewModel,
				Reason:   accountModelAliasRenameReasonNewModelExists,
			})
			continue
		}
		platformMapping[rename.NewModel] = target
		if err := updateAccountModelAliasRenameChannelMapping(ctx, exec, channels[i].id, channels[i].modelMapping); err != nil {
			return 0, nil, err
		}
		updated++
	}
	return updated, skipped, nil
}

func updateAccountModelAliasRenameChannelPricingModels(ctx context.Context, exec dbExec, pricingID int64, models []string) error {
	raw, err := json.Marshal(models)
	if err != nil {
		return fmt.Errorf("marshal channel pricing alias rename models: %w", err)
	}
	result, err := exec.ExecContext(ctx, `UPDATE channel_model_pricing SET models = $1, updated_at = NOW() WHERE id = $2`, raw, pricingID)
	if err != nil {
		return fmt.Errorf("update channel pricing alias rename models: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("%s: channel_model_pricing id=%d", accountModelAliasRenameReasonUpdateAffectedNoRows, pricingID)
	}
	return nil
}

func updateAccountModelAliasRenameChannelMapping(ctx context.Context, exec dbExec, channelID int64, mapping map[string]map[string]string) error {
	raw, err := marshalModelMapping(mapping)
	if err != nil {
		return err
	}
	result, err := exec.ExecContext(ctx, `UPDATE channels SET model_mapping = $1, updated_at = NOW() WHERE id = $2`, raw, channelID)
	if err != nil {
		return fmt.Errorf("update channel alias rename mapping: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("%s: channel id=%d", accountModelAliasRenameReasonUpdateAffectedNoRows, channelID)
	}
	return nil
}

func cascadeAccountModelAliasRenameRoutes(
	ctx context.Context,
	exec sqlExecutor,
	queries accountModelAliasRenameRouteQueries,
	groupIDs []int64,
	rename service.AccountModelAliasRename,
	changedOwners map[int64]struct{},
) (int, []service.AccountModelAliasRenameSkipItem, error) {
	routes, err := loadAccountModelAliasRenameRoutes(ctx, exec, queries.candidatesSQL, groupIDs, rename.OldModel)
	if err != nil {
		return 0, nil, err
	}
	return processAccountModelAliasRenameRoutes(ctx, exec, queries, routes, rename, changedOwners)
}

func processAccountModelAliasRenameRoutes(
	ctx context.Context,
	exec sqlExecutor,
	queries accountModelAliasRenameRouteQueries,
	routes []accountModelAliasRenameRoute,
	rename service.AccountModelAliasRename,
	changedOwners map[int64]struct{},
) (int, []service.AccountModelAliasRenameSkipItem, error) {
	updated := 0
	skipped := []service.AccountModelAliasRenameSkipItem{}
	for _, route := range routes {
		sourceConflict, err := accountModelAliasRenameExists(ctx, exec, queries.sourceConflictSQL, route.ownerID, route.sourceGroupID, rename.NewModel, route.id)
		if err != nil {
			return 0, nil, err
		}
		if sourceConflict {
			skipped = append(skipped, service.AccountModelAliasRenameSkipItem{
				Scope:    queries.sourceSkipScope,
				OwnerID:  route.ownerID,
				OldModel: rename.OldModel,
				NewModel: rename.NewModel,
				Reason:   accountModelAliasRenameReasonSourceModelConflict,
			})
			continue
		}

		updatePublicModel := false
		if strings.EqualFold(route.publicModel, rename.OldModel) {
			publicConflict, err := accountModelAliasRenameExists(ctx, exec, queries.publicConflictSQL, route.ownerID, rename.NewModel, route.id)
			if err != nil {
				return 0, nil, err
			}
			if publicConflict {
				skipped = append(skipped, service.AccountModelAliasRenameSkipItem{
					Scope:    queries.publicSkipScope,
					OwnerID:  route.ownerID,
					OldModel: rename.OldModel,
					NewModel: rename.NewModel,
					Reason:   accountModelAliasRenameReasonPublicModelConflict,
				})
			} else {
				updatePublicModel = true
			}
		}

		result, err := exec.ExecContext(ctx, queries.updateSQL, rename.NewModel, updatePublicModel, route.id)
		if err != nil {
			return 0, nil, fmt.Errorf("update alias rename route: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			return 0, nil, fmt.Errorf("%s: route id=%d", accountModelAliasRenameReasonUpdateAffectedNoRows, route.id)
		}
		updated++
		changedOwners[route.ownerID] = struct{}{}
	}
	return updated, skipped, nil
}

func loadAccountModelAliasRenameRoutes(ctx context.Context, exec sqlExecutor, query string, groupIDs []int64, oldModel string) ([]accountModelAliasRenameRoute, error) {
	rows, err := exec.QueryContext(ctx, query, pq.Array(groupIDs), oldModel)
	if err != nil {
		return nil, fmt.Errorf("load custom route alias rename candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	routes := []accountModelAliasRenameRoute{}
	for rows.Next() {
		var route accountModelAliasRenameRoute
		if err := rows.Scan(&route.id, &route.ownerID, &route.publicModel, &route.sourceGroupID, &route.sourceModel); err != nil {
			return nil, fmt.Errorf("scan custom route alias rename candidate: %w", err)
		}
		routes = append(routes, route)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate custom route alias rename candidates: %w", err)
	}
	return routes, nil
}

func loadAccountModelAliasRenameSystemRouteOwnerIDs(ctx context.Context, exec sqlExecutor, query string, groupIDs []int64, oldModel string) ([]int64, error) {
	rows, err := exec.QueryContext(ctx, query, pq.Array(groupIDs), oldModel)
	if err != nil {
		return nil, fmt.Errorf("load system custom route alias rename lock candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	ownerIDs := []int64{}
	for rows.Next() {
		var ownerID int64
		if err := rows.Scan(&ownerID); err != nil {
			return nil, fmt.Errorf("scan system custom route alias rename lock candidate: %w", err)
		}
		ownerIDs = append(ownerIDs, ownerID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate system custom route alias rename lock candidates: %w", err)
	}
	return ownerIDs, nil
}

func accountModelAliasRenameExists(ctx context.Context, exec sqlExecutor, query string, args ...any) (bool, error) {
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	exists := false
	if rows.Next() {
		if err := rows.Scan(&exists); err != nil {
			return false, err
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return exists, nil
}

func enqueueUserCustomGroupAuthCacheInvalidations(ctx context.Context, exec sqlExecutor, customGroupIDs []int64) error {
	_, err := exec.ExecContext(ctx, `
INSERT INTO auth_cache_invalidation_outbox (cache_key)
SELECT encode(sha256(convert_to(k.key, 'UTF8')), 'hex')
FROM api_keys AS k
WHERE k.custom_group_id = ANY($1)
  AND k.deleted_at IS NULL
  AND k.key <> ''
`, pq.Array(customGroupIDs))
	if err != nil {
		return fmt.Errorf("enqueue user custom group auth cache invalidations: %w", err)
	}
	return nil
}

func accountModelAliasRenameUserRouteQueries() accountModelAliasRenameRouteQueries {
	return accountModelAliasRenameRouteQueries{
		candidatesSQL: `
SELECT m.id, m.custom_group_id, m.public_model, m.source_group_id, m.source_model
FROM user_custom_group_models AS m
JOIN user_custom_groups AS cg ON cg.id = m.custom_group_id
WHERE cg.deleted_at IS NULL
  AND m.source_group_id = ANY($1)
  AND LOWER(m.source_model) = LOWER($2)
ORDER BY m.custom_group_id, m.id
FOR UPDATE
`,
		sourceConflictSQL: `
SELECT EXISTS (
    SELECT 1
    FROM user_custom_group_models AS conflict
    WHERE conflict.custom_group_id = $1
      AND conflict.source_group_id = $2
      AND LOWER(conflict.source_model) = LOWER($3)
      AND conflict.id <> $4
)
`,
		publicConflictSQL: `
SELECT EXISTS (
    SELECT 1
    FROM user_custom_group_models AS conflict
    WHERE conflict.custom_group_id = $1
      AND LOWER(conflict.public_model) = LOWER($2)
      AND conflict.id <> $3
)
`,
		updateSQL: `UPDATE user_custom_group_models
SET source_model = $1,
    public_model = CASE WHEN $2 THEN $1 ELSE public_model END,
    updated_at = NOW()
WHERE id = $3`,
		sourceSkipScope: accountModelAliasRenameUserRouteSourceScope,
		publicSkipScope: accountModelAliasRenameUserRoutePublicScope,
	}
}

func accountModelAliasRenameSystemRouteQueries() accountModelAliasRenameRouteQueries {
	return accountModelAliasRenameRouteQueries{
		lockCandidatesSQL: `
SELECT DISTINCT m.group_id
FROM system_custom_group_models AS m
JOIN groups AS g ON g.id = m.group_id
WHERE g.deleted_at IS NULL
  AND g.system_custom_routing_enabled = TRUE
  AND m.source_group_id = ANY($1)
  AND LOWER(m.source_model) = LOWER($2)
ORDER BY m.group_id
`,
		candidatesSQL: `
SELECT m.id, m.group_id, m.public_model, m.source_group_id, m.source_model
FROM system_custom_group_models AS m
JOIN groups AS g ON g.id = m.group_id
WHERE g.deleted_at IS NULL
  AND g.system_custom_routing_enabled = TRUE
  AND m.source_group_id = ANY($1)
  AND LOWER(m.source_model) = LOWER($2)
ORDER BY m.group_id, m.id
FOR UPDATE
`,
		sourceConflictSQL: `
SELECT EXISTS (
    SELECT 1
    FROM system_custom_group_models AS conflict
    WHERE conflict.group_id = $1
      AND conflict.source_group_id = $2
      AND LOWER(conflict.source_model) = LOWER($3)
      AND conflict.id <> $4
)
`,
		publicConflictSQL: `
SELECT EXISTS (
    SELECT 1
    FROM system_custom_group_models AS conflict
    WHERE conflict.group_id = $1
      AND LOWER(conflict.public_model) = LOWER($2)
      AND conflict.id <> $3
)
`,
		updateSQL: `UPDATE system_custom_group_models
SET source_model = $1,
    public_model = CASE WHEN $2 THEN $1 ELSE public_model END,
    updated_at = NOW()
WHERE id = $3`,
		sourceSkipScope: accountModelAliasRenameSystemRouteSourceScope,
		publicSkipScope: accountModelAliasRenameSystemRoutePublicScope,
	}
}

func normalizeRepositoryAccountModelAliasRenameInputs(groupIDs []int64, renames []service.AccountModelAliasRename) ([]int64, []service.AccountModelAliasRename) {
	normalizedGroupIDs := make([]int64, 0, len(groupIDs))
	seenGroupIDs := make(map[int64]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		if groupID <= 0 {
			continue
		}
		if _, ok := seenGroupIDs[groupID]; ok {
			continue
		}
		seenGroupIDs[groupID] = struct{}{}
		normalizedGroupIDs = append(normalizedGroupIDs, groupID)
	}

	normalizedRenames := make([]service.AccountModelAliasRename, 0, len(renames))
	seenRenames := make(map[string]struct{}, len(renames))
	for _, rename := range renames {
		oldModel := strings.TrimSpace(rename.OldModel)
		newModel := strings.TrimSpace(rename.NewModel)
		if oldModel == "" || newModel == "" || strings.EqualFold(oldModel, newModel) {
			continue
		}
		key := strings.ToLower(oldModel) + "\x00" + strings.ToLower(newModel)
		if _, ok := seenRenames[key]; ok {
			continue
		}
		seenRenames[key] = struct{}{}
		normalizedRenames = append(normalizedRenames, service.AccountModelAliasRename{
			OldModel: oldModel,
			NewModel: newModel,
		})
	}
	return normalizedGroupIDs, normalizedRenames
}

func containsExactString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type accountModelAliasRenameModelEntry struct {
	prefix   string
	wildcard bool
}

func accountModelAliasRenamePricingModelsConflict(models []string, model string) bool {
	target := accountModelAliasRenamePricingModelEntry(model)
	for _, existing := range models {
		if accountModelAliasRenameModelsConflict(accountModelAliasRenamePricingModelEntry(existing), target) {
			return true
		}
	}
	return false
}

func accountModelAliasRenameMappingModelsConflict(mapping map[string]string, model string) bool {
	target := accountModelAliasRenameMappingModelEntry(model)
	for existing := range mapping {
		if accountModelAliasRenameModelsConflict(accountModelAliasRenameMappingModelEntry(existing), target) {
			return true
		}
	}
	return false
}

func accountModelAliasRenamePricingModelEntry(pattern string) accountModelAliasRenameModelEntry {
	prefix, wildcard := accountModelAliasRenameSplitWildcardSuffix(pattern)
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if strings.HasPrefix(prefix, "claude-") {
		prefix = strings.ReplaceAll(prefix, ".", "-")
	}
	return accountModelAliasRenameModelEntry{prefix: prefix, wildcard: wildcard}
}

func accountModelAliasRenameMappingModelEntry(pattern string) accountModelAliasRenameModelEntry {
	prefix, wildcard := accountModelAliasRenameSplitWildcardSuffix(strings.ToLower(pattern))
	return accountModelAliasRenameModelEntry{prefix: prefix, wildcard: wildcard}
}

func accountModelAliasRenameSplitWildcardSuffix(pattern string) (string, bool) {
	if strings.HasSuffix(pattern, "*") {
		return strings.TrimSuffix(pattern, "*"), true
	}
	return pattern, false
}

func accountModelAliasRenameModelsConflict(a, b accountModelAliasRenameModelEntry) bool {
	switch {
	case !a.wildcard && !b.wildcard:
		return a.prefix == b.prefix
	case a.wildcard && !b.wildcard:
		return strings.HasPrefix(b.prefix, a.prefix)
	case !a.wildcard && b.wildcard:
		return strings.HasPrefix(a.prefix, b.prefix)
	default:
		return strings.HasPrefix(a.prefix, b.prefix) ||
			strings.HasPrefix(b.prefix, a.prefix)
	}
}

func sortedInt64Keys(values map[int64]struct{}) []int64 {
	keys := make([]int64, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func sortedInt64KeysFromPricingRows(values map[int64][]*accountModelAliasRenamePricingRow) []int64 {
	keys := make([]int64, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
