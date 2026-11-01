package migrations

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestUserSupportMigrationDurableClaims(t *testing.T) {
	data, err := FS.ReadFile("255_user_support.sql")
	require.NoError(t, err)
	sql := string(data)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS user_support_access")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS support_trial_grants")
	require.Contains(t, sql, "user_id BIGINT PRIMARY KEY")
	require.Contains(t, sql, "CHECK (amount = 15)")
	require.NotContains(t, sql, "REFERENCES redeem_codes")
	require.NotContains(t, sql, "ON DELETE CASCADE")
}
