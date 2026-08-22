package migrations

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubAdminCommissionGrantsMigration(t *testing.T) {
	sql, err := os.ReadFile("224_sub_admin_commission_grants.sql")
	require.NoError(t, err)
	body := string(sql)

	require.Contains(t, body, "CREATE TABLE IF NOT EXISTS sub_admin_commission_grants")
	require.Contains(t, body, "sub_admin_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE")
	require.Contains(t, body, "group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE")
	require.Contains(t, body, "granted_date DATE NOT NULL")
	require.Contains(t, body, "enabled BOOLEAN NOT NULL DEFAULT TRUE")
	require.Contains(t, body, "created_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL")
	require.Contains(t, body, "CREATE UNIQUE INDEX IF NOT EXISTS idx_sub_admin_commission_grants_active_unique")
	require.Contains(t, body, "ON sub_admin_commission_grants (sub_admin_user_id, group_id)")
	require.Contains(t, body, "WHERE enabled = TRUE")
	require.Contains(t, body, "CREATE INDEX IF NOT EXISTS idx_sub_admin_commission_grants_sub_admin_enabled")
	require.Contains(t, body, "ON sub_admin_commission_grants (sub_admin_user_id, enabled, granted_date)")
	require.Contains(t, body, "CREATE INDEX IF NOT EXISTS idx_sub_admin_commission_grants_group_enabled")
	require.Contains(t, body, "ON sub_admin_commission_grants (group_id, enabled)")
}
