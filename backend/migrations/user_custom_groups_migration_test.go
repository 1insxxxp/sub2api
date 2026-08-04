package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration194DefersValidationAndHotIndexes(t *testing.T) {
	content, err := FS.ReadFile("194_user_custom_model_groups.sql")
	require.NoError(t, err)

	sql := string(content)
	require.NotContains(t, sql, "CREATE INDEX")
	require.NotContains(t, sql, "CREATE UNIQUE INDEX")
	require.Contains(t, sql, "api_keys_custom_group_id_fkey")
	require.Contains(t, sql, "user_custom_groups(id) ON DELETE RESTRICT NOT VALID")
	require.Contains(t, sql, "api_keys_single_group_binding")
	require.Contains(t, sql, "CHECK (NOT (group_id IS NOT NULL AND custom_group_id IS NOT NULL)) NOT VALID")
	require.Contains(t, sql, "usage_logs_custom_group_id_fkey")
	require.Contains(t, sql, "user_custom_groups(id) ON DELETE SET NULL NOT VALID")
}

func TestMigration194aCreatesIndexesConcurrently(t *testing.T) {
	content, err := FS.ReadFile("194a_user_custom_model_group_indexes_notx.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Equal(t, 7, strings.Count(sql, "CREATE INDEX CONCURRENTLY IF NOT EXISTS")+strings.Count(sql, "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS"))
	require.Contains(t, sql, "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_custom_group_id")
	require.Contains(t, sql, "ON usage_logs(custom_group_id)")
	require.NotContains(t, sql, "BEGIN")
	require.NotContains(t, sql, "COMMIT")
}
