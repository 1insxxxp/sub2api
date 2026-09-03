package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLotteryAttemptGrantsMigrationContract(t *testing.T) {
	raw, err := FS.ReadFile("239_lottery_attempt_grants.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(raw)), " ")
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS lottery_attempt_grants",
		"user_id BIGINT NOT NULL",
		"amount INTEGER NOT NULL",
		"description TEXT NOT NULL DEFAULT ''",
		"created_by BIGINT NOT NULL",
		"lottery_attempt_grants_user_created_at_idx",
		"DROP CONSTRAINT IF EXISTS lottery_attempt_ledger_source_type_check",
		"CHECK (source_type IN ('checkin_streak', 'lottery_draw', 'admin_grant'))",
	} {
		require.Contains(t, sql, fragment)
	}
}
