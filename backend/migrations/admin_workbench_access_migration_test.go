package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminWorkbenchAccessMigrationPreservesExistingChoices(t *testing.T) {
	data, err := FS.ReadFile("256_admin_workbench_access.sql")
	require.NoError(t, err)
	sql := string(data)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS sub_admin_workbench_access")
	require.Contains(t, sql, "commission_enabled BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, sql, "affiliate_leaderboard_enabled BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, sql, "role = 'sub_admin'")
	require.Contains(t, sql, "deleted_at IS NULL")
	require.Contains(t, sql, "ON CONFLICT (user_id) DO NOTHING")
	require.NotContains(t, sql, "ON DELETE CASCADE")
	require.NotContains(t, sql, "global_grants")
}
