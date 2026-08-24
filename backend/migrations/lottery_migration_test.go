package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLotteryMigrationContract(t *testing.T) {
	raw, err := FS.ReadFile("231_lottery.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(raw)), " ")
	for _, table := range []string{
		"lottery_activities",
		"lottery_prizes",
		"lottery_prize_items",
		"lottery_draws",
	} {
		require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS "+table)
	}

	for _, fragment := range []string{
		"lottery_activities_one_active_uq",
		"ON lottery_activities (status)",
		"WHERE (status = 'active')",
		"lottery_activities_status_idx",
		"lottery_activities_attempt_mode_idx",
		"lottery_prizes_activity_enabled_sort_idx",
		"lottery_prize_items_prize_status_idx",
		"lottery_draws_user_created_at_idx",
		"lottery_draws_attempt_key_uq",
		"attempt_key VARCHAR(128) NOT NULL",
		"prize_name VARCHAR(120) NOT NULL",
		"product_content TEXT",
	} {
		require.Contains(t, sql, fragment)
	}
}
