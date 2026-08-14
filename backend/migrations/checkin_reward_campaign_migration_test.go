package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckinRewardCampaignMigrationContract(t *testing.T) {
	raw, err := FS.ReadFile("222_checkin_reward_campaigns.sql")
	require.NoError(t, err)
	sql := string(raw)

	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS checkin_reward_campaigns")
	require.Contains(t, sql, "reward_tiers JSONB")
	require.Contains(t, sql, "CHECK (start_date <= end_date)")
	require.Contains(t, sql, "EXCLUDE USING gist")
	require.Contains(t, sql, "WHERE (status = 'enabled')")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS reward_campaign_id")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS reward_campaign_name")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS reward_campaign_tiers_snapshot")

	normalizedSQL := strings.Join(strings.Fields(sql), " ")
	require.Contains(t, normalizedSQL,
		"CREATE INDEX IF NOT EXISTS user_checkins_reward_campaign_id_idx ON user_checkins (reward_campaign_id)")
}
