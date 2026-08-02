package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserCheckinUsageRebateMigration(t *testing.T) {
	raw, err := FS.ReadFile("193_user_checkin_usage_rebate.sql")
	require.NoError(t, err)

	sql := string(raw)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS previous_day_usage_amount DECIMAL(20,8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS usage_rebate_amount DECIMAL(20,8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS reward_cap_adjustment DECIMAL(20,8) NOT NULL DEFAULT 0")
}
