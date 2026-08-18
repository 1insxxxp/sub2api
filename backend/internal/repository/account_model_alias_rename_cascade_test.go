package repository

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestChannelRepositoryModelAliasRenameCopiesPricingAndMappingPreservingOld(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &channelRepository{db: db}
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT c\.id, c\.model_mapping.*FROM channels AS c.*channel_groups.*group_id = ANY\(\$1\).*FOR UPDATE`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "model_mapping"}).
			AddRow(int64(77), []byte(`{"antigravity":{"old-model":"upstream-old","other":"upstream-other"},"openai":{"old-model":"openai-old"}}`)))
	mock.ExpectQuery(`(?s)SELECT id, channel_id, models.*FROM channel_model_pricing.*platform = \$2.*FOR UPDATE`).
		WithArgs(sqlmock.AnyArg(), service.PlatformAntigravity).
		WillReturnRows(sqlmock.NewRows([]string{"id", "channel_id", "models"}).
			AddRow(int64(900), int64(77), []byte(`["old-model","sibling"]`)))
	mock.ExpectExec(`UPDATE channel_model_pricing SET models = \$1, updated_at = NOW\(\) WHERE id = \$2`).
		WithArgs(jsonStringSliceArg{want: []string{"old-model", "sibling", "new-model"}}, int64(900)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE channels SET model_mapping = \$1, updated_at = NOW\(\) WHERE id = \$2`).
		WithArgs(channelMappingArg{
			platform: "antigravity",
			want: map[string]string{
				"old-model": "upstream-old",
				"new-model": "upstream-old",
				"other":     "upstream-other",
			},
			otherPlatforms: map[string]map[string]string{
				"openai": {"old-model": "openai-old"},
			},
		}, int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, err := repo.CascadeAccountModelAliasRenames(ctx, 12, []int64{101, 102}, []service.AccountModelAliasRename{
		{OldModel: "old-model", NewModel: "new-model"},
	})

	require.NoError(t, err)
	require.Equal(t, 1, got.ChannelPricingUpdated)
	require.Equal(t, 1, got.ChannelMappingsUpdated)
	require.Empty(t, got.Skipped)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelRepositoryModelAliasRenameSkipsPricingAndMappingConflicts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &channelRepository{db: db}
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT c\.id, c\.model_mapping.*FROM channels AS c.*channel_groups.*group_id = ANY\(\$1\).*FOR UPDATE`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "model_mapping"}).
			AddRow(int64(77), []byte(`{"antigravity":{"old-model":"upstream-old","new-model":"upstream-new"}}`)))
	mock.ExpectQuery(`(?s)SELECT id, channel_id, models.*FROM channel_model_pricing.*platform = \$2.*FOR UPDATE`).
		WithArgs(sqlmock.AnyArg(), service.PlatformAntigravity).
		WillReturnRows(sqlmock.NewRows([]string{"id", "channel_id", "models"}).
			AddRow(int64(901), int64(77), []byte(`["old-model"]`)).
			AddRow(int64(902), int64(77), []byte(`["new-model"]`)))
	mock.ExpectCommit()

	got, err := repo.CascadeAccountModelAliasRenames(ctx, 12, []int64{101}, []service.AccountModelAliasRename{
		{OldModel: "old-model", NewModel: "new-model"},
	})

	require.NoError(t, err)
	require.Zero(t, got.ChannelPricingUpdated)
	require.Zero(t, got.ChannelMappingsUpdated)
	require.ElementsMatch(t, []string{"channel_pricing", "channel_mapping"}, skipScopes(got.Skipped))
	for _, skipped := range got.Skipped {
		require.Equal(t, int64(77), skipped.OwnerID)
		require.Equal(t, "old-model", skipped.OldModel)
		require.Equal(t, "new-model", skipped.NewModel)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelRepositoryModelAliasRenameSkipsPricingSemanticConflicts(t *testing.T) {
	tests := []struct {
		name     string
		models   []string
		newModel string
	}{
		{
			name:     "case insensitive new model",
			models:   []string{"old-model", "NEW-MODEL"},
			newModel: "new-model",
		},
		{
			name:     "wildcard new model",
			models:   []string{"old-model", "new-*"},
			newModel: "new-model",
		},
		{
			name:     "claude dot hyphen normalized",
			models:   []string{"old-model", "claude-sonnet-4.5"},
			newModel: "claude-sonnet-4-5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			repo := &channelRepository{db: db}
			ctx := context.Background()
			modelsJSON, err := json.Marshal(tt.models)
			require.NoError(t, err)

			mock.ExpectBegin()
			expectChannelAliasRenameCandidates(mock, []byte(`{}`))
			mock.ExpectQuery(`(?s)SELECT id, channel_id, models.*FROM channel_model_pricing.*platform = \$2.*FOR UPDATE`).
				WithArgs(sqlmock.AnyArg(), service.PlatformAntigravity).
				WillReturnRows(sqlmock.NewRows([]string{"id", "channel_id", "models"}).
					AddRow(int64(901), int64(77), modelsJSON))
			mock.ExpectCommit()

			got, err := repo.CascadeAccountModelAliasRenames(ctx, 12, []int64{101}, []service.AccountModelAliasRename{
				{OldModel: "old-model", NewModel: tt.newModel},
			})

			require.NoError(t, err)
			require.Zero(t, got.ChannelPricingUpdated)
			skipped := skipByScope(t, got.Skipped, "channel_pricing")
			require.Equal(t, int64(77), skipped.OwnerID)
			require.Equal(t, tt.newModel, skipped.NewModel)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestChannelRepositoryModelAliasRenameSkipsMappingSemanticConflicts(t *testing.T) {
	tests := []struct {
		name       string
		mappingKey string
	}{
		{name: "wildcard new model", mappingKey: "new-*"},
		{name: "case insensitive new model", mappingKey: "NEW-MODEL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			repo := &channelRepository{db: db}
			ctx := context.Background()
			modelMapping := map[string]map[string]string{
				service.PlatformAntigravity: {
					"old-model":   "upstream-old",
					tt.mappingKey: "upstream-new",
				},
			}
			modelMappingJSON, err := json.Marshal(modelMapping)
			require.NoError(t, err)

			mock.ExpectBegin()
			expectChannelAliasRenameCandidates(mock, modelMappingJSON)
			mock.ExpectQuery(`(?s)SELECT id, channel_id, models.*FROM channel_model_pricing.*platform = \$2.*FOR UPDATE`).
				WithArgs(sqlmock.AnyArg(), service.PlatformAntigravity).
				WillReturnRows(sqlmock.NewRows([]string{"id", "channel_id", "models"}))
			mock.ExpectCommit()

			got, err := repo.CascadeAccountModelAliasRenames(ctx, 12, []int64{101}, []service.AccountModelAliasRename{
				{OldModel: "old-model", NewModel: "new-model"},
			})

			require.NoError(t, err)
			require.Zero(t, got.ChannelMappingsUpdated)
			skipped := skipByScope(t, got.Skipped, "channel_mapping")
			require.Equal(t, int64(77), skipped.OwnerID)
			require.Equal(t, "new-model", skipped.NewModel)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUserCustomGroupRepositoryModelAliasRenameUpdatesRoutesAndSkipsCollisions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &userCustomGroupRepository{db: db}
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT m\.id, m\.custom_group_id, m\.public_model, m\.source_group_id, m\.source_model.*FROM user_custom_group_models AS m.*user_custom_groups AS cg.*m\.source_group_id = ANY\(\$1\).*LOWER\(m\.source_model\) = LOWER\(\$2\).*FOR UPDATE`).
		WithArgs(sqlmock.AnyArg(), "old-model").
		WillReturnRows(sqlmock.NewRows([]string{"id", "custom_group_id", "public_model", "source_group_id", "source_model"}).
			AddRow(int64(101), int64(201), "old-model", int64(10), "old-model").
			AddRow(int64(102), int64(202), "OLD-MODEL", int64(20), "old-model").
			AddRow(int64(103), int64(203), "old-model", int64(10), "old-model"))
	expectUserAliasSourceConflict(mock, 201, 10, "new-model", 101, false)
	expectUserAliasPublicConflict(mock, 201, "new-model", 101, false)
	expectUserAliasUpdate(mock, 101, "new-model", true)
	expectUserAliasSourceConflict(mock, 202, 20, "new-model", 102, false)
	expectUserAliasPublicConflict(mock, 202, "new-model", 102, true)
	expectUserAliasUpdate(mock, 102, "new-model", false)
	expectUserAliasSourceConflict(mock, 203, 10, "new-model", 103, true)
	mock.ExpectExec(`(?s)INSERT INTO auth_cache_invalidation_outbox \(cache_key\).*FROM api_keys AS k.*k\.custom_group_id = ANY\(\$1\)`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	got, err := repo.CascadeAccountModelAliasRenames(ctx, 12, []int64{10, 20}, []service.AccountModelAliasRename{
		{OldModel: "old-model", NewModel: "new-model"},
	})

	require.NoError(t, err)
	require.Equal(t, 2, got.UserCustomRoutesUpdated)
	require.ElementsMatch(t, []string{"user_custom_route_public_model", "user_custom_route_source_model"}, skipScopes(got.Skipped))
	require.Equal(t, int64(202), skipByScope(t, got.Skipped, "user_custom_route_public_model").OwnerID)
	require.Equal(t, int64(203), skipByScope(t, got.Skipped, "user_custom_route_source_model").OwnerID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSystemCustomGroupRepositoryModelAliasRenameUpdatesRoutesAndSkipsCollisions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	repo := &systemCustomGroupRepository{client: client}
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT DISTINCT m\.group_id.*FROM system_custom_group_models AS m.*groups AS g.*g\.system_custom_routing_enabled = TRUE.*m\.source_group_id = ANY\(\$1\).*LOWER\(m\.source_model\) = LOWER\(\$2\).*ORDER BY m\.group_id`).
		WithArgs(sqlmock.AnyArg(), "old-model").
		WillReturnRows(sqlmock.NewRows([]string{"group_id"}).
			AddRow(int64(501)).
			AddRow(int64(502)).
			AddRow(int64(503)))
	expectGroupReferenceAliasRenameLock(mock, 10)
	expectGroupReferenceAliasRenameLock(mock, 20)
	expectGroupReferenceAliasRenameLock(mock, 501)
	expectGroupReferenceAliasRenameLock(mock, 502)
	expectGroupReferenceAliasRenameLock(mock, 503)
	mock.ExpectQuery(`(?s)SELECT m\.id, m\.group_id, m\.public_model, m\.source_group_id, m\.source_model.*FROM system_custom_group_models AS m.*groups AS g.*g\.system_custom_routing_enabled = TRUE.*m\.source_group_id = ANY\(\$1\).*LOWER\(m\.source_model\) = LOWER\(\$2\).*FOR UPDATE`).
		WithArgs(sqlmock.AnyArg(), "old-model").
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_id", "public_model", "source_group_id", "source_model"}).
			AddRow(int64(301), int64(501), "old-model", int64(10), "old-model").
			AddRow(int64(302), int64(502), "OLD-MODEL", int64(20), "old-model").
			AddRow(int64(303), int64(503), "old-model", int64(10), "old-model"))
	expectSystemAliasSourceConflict(mock, 501, 10, "new-model", 301, false)
	expectSystemAliasPublicConflict(mock, 501, "new-model", 301, false)
	expectSystemAliasUpdate(mock, 301, "new-model", true)
	expectSystemAliasSourceConflict(mock, 502, 20, "new-model", 302, false)
	expectSystemAliasPublicConflict(mock, 502, "new-model", 302, true)
	expectSystemAliasUpdate(mock, 302, "new-model", false)
	expectSystemAliasSourceConflict(mock, 503, 10, "new-model", 303, true)
	expectSchedulerGroupChanged(mock, 501)
	expectSchedulerGroupChanged(mock, 502)
	mock.ExpectCommit()

	got, err := repo.CascadeAccountModelAliasRenames(ctx, 12, []int64{10, 20}, []service.AccountModelAliasRename{
		{OldModel: "old-model", NewModel: "new-model"},
	})

	require.NoError(t, err)
	require.Equal(t, 2, got.SystemCustomRoutesUpdated)
	require.ElementsMatch(t, []string{"system_custom_route_public_model", "system_custom_route_source_model"}, skipScopes(got.Skipped))
	require.Equal(t, int64(502), skipByScope(t, got.Skipped, "system_custom_route_public_model").OwnerID)
	require.Equal(t, int64(503), skipByScope(t, got.Skipped, "system_custom_route_source_model").OwnerID)
	require.NoError(t, mock.ExpectationsWereMet())
}

type jsonStringSliceArg struct {
	want []string
}

func (a jsonStringSliceArg) Match(value driver.Value) bool {
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return false
	}
	var got []string
	if err := json.Unmarshal(raw, &got); err != nil {
		return false
	}
	if len(got) != len(a.want) {
		return false
	}
	for i := range got {
		if got[i] != a.want[i] {
			return false
		}
	}
	return true
}

type channelMappingArg struct {
	platform       string
	want           map[string]string
	otherPlatforms map[string]map[string]string
}

func (a channelMappingArg) Match(value driver.Value) bool {
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return false
	}
	var got map[string]map[string]string
	if err := json.Unmarshal(raw, &got); err != nil {
		return false
	}
	if !stringMapEqual(got[a.platform], a.want) {
		return false
	}
	for platform, want := range a.otherPlatforms {
		if !stringMapEqual(got[platform], want) {
			return false
		}
	}
	return true
}

func stringMapEqual(got, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for key, wantValue := range want {
		if got[key] != wantValue {
			return false
		}
	}
	return true
}

func expectUserAliasSourceConflict(mock sqlmock.Sqlmock, customGroupID, sourceGroupID int64, newModel string, routeID int64, exists bool) {
	mock.ExpectQuery(`(?s)SELECT EXISTS.*FROM user_custom_group_models AS conflict.*conflict\.custom_group_id = \$1.*conflict\.source_group_id = \$2.*LOWER\(conflict\.source_model\) = LOWER\(\$3\).*conflict\.id <> \$4`).
		WithArgs(customGroupID, sourceGroupID, newModel, routeID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(exists))
}

func expectChannelAliasRenameCandidates(mock sqlmock.Sqlmock, modelMapping []byte) {
	mock.ExpectQuery(`(?s)SELECT c\.id, c\.model_mapping.*FROM channels AS c.*channel_groups.*group_id = ANY\(\$1\).*FOR UPDATE`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "model_mapping"}).
			AddRow(int64(77), modelMapping))
}

func expectUserAliasPublicConflict(mock sqlmock.Sqlmock, customGroupID int64, newModel string, routeID int64, exists bool) {
	mock.ExpectQuery(`(?s)SELECT EXISTS.*FROM user_custom_group_models AS conflict.*conflict\.custom_group_id = \$1.*LOWER\(conflict\.public_model\) = LOWER\(\$2\).*conflict\.id <> \$3`).
		WithArgs(customGroupID, newModel, routeID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(exists))
}

func expectUserAliasUpdate(mock sqlmock.Sqlmock, routeID int64, newModel string, updatePublic bool) {
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE user_custom_group_models
SET source_model = $1,
    public_model = CASE WHEN $2 THEN $1 ELSE public_model END,
    updated_at = NOW()
WHERE id = $3`)).
		WithArgs(newModel, updatePublic, routeID).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func expectSystemAliasSourceConflict(mock sqlmock.Sqlmock, groupID, sourceGroupID int64, newModel string, routeID int64, exists bool) {
	mock.ExpectQuery(`(?s)SELECT EXISTS.*FROM system_custom_group_models AS conflict.*conflict\.group_id = \$1.*conflict\.source_group_id = \$2.*LOWER\(conflict\.source_model\) = LOWER\(\$3\).*conflict\.id <> \$4`).
		WithArgs(groupID, sourceGroupID, newModel, routeID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(exists))
}

func expectSystemAliasPublicConflict(mock sqlmock.Sqlmock, groupID int64, newModel string, routeID int64, exists bool) {
	mock.ExpectQuery(`(?s)SELECT EXISTS.*FROM system_custom_group_models AS conflict.*conflict\.group_id = \$1.*LOWER\(conflict\.public_model\) = LOWER\(\$2\).*conflict\.id <> \$3`).
		WithArgs(groupID, newModel, routeID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(exists))
}

func expectSystemAliasUpdate(mock sqlmock.Sqlmock, routeID int64, newModel string, updatePublic bool) {
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE system_custom_group_models
SET source_model = $1,
    public_model = CASE WHEN $2 THEN $1 ELSE public_model END,
    updated_at = NOW()
WHERE id = $3`)).
		WithArgs(newModel, updatePublic, routeID).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func expectGroupReferenceAliasRenameLock(mock sqlmock.Sqlmock, groupID int64) {
	mock.ExpectQuery("SELECT pg_advisory_xact_lock").
		WithArgs(int32(0x53554232), accountAliasRenameGroupReferenceLockKey(groupID)).
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_xact_lock"}).AddRow(nil))
}

func accountAliasRenameGroupReferenceLockKey(groupID int64) int32 {
	unsigned := uint64(groupID)
	return int32(uint32(unsigned) ^ uint32(unsigned>>32))
}

func expectSchedulerGroupChanged(mock sqlmock.Sqlmock, groupID int64) {
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).
		WithArgs(service.SchedulerOutboxEventGroupChanged, nil, groupID, nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func skipScopes(skipped []service.AccountModelAliasRenameSkipItem) []string {
	scopes := make([]string, 0, len(skipped))
	for _, item := range skipped {
		scopes = append(scopes, item.Scope)
	}
	return scopes
}

func skipByScope(t *testing.T, skipped []service.AccountModelAliasRenameSkipItem, scope string) service.AccountModelAliasRenameSkipItem {
	t.Helper()
	for _, item := range skipped {
		if item.Scope == scope {
			return item
		}
	}
	require.Failf(t, "missing skipped scope", "scope %q not found in %+v", scope, skipped)
	return service.AccountModelAliasRenameSkipItem{}
}
