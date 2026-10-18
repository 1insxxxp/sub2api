//go:build unit

package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration249CreatesCommissionExclusionIntervals(t *testing.T) {
	content, err := FS.ReadFile("249_sub_admin_commission_exclusions.sql")
	require.NoError(t, err)
	sql := string(content)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS sub_admin_commission_exclusions")
	require.Contains(t, sql, "DEFAULT clock_timestamp()")
	require.Contains(t, sql, "CHECK (ended_at IS NULL OR ended_at >= started_at)")
	require.Contains(t, sql, "WHERE ended_at IS NULL")
	require.Contains(t, sql, "ON sub_admin_commission_exclusions (user_id, started_at)")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS commission_excluded_cost DECIMAL(20, 10) NOT NULL DEFAULT 0")
	require.NotContains(t, sql, "UPDATE usage_group_daily_rollups")
}
