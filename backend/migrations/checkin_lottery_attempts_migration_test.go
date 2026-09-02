package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckinLotteryAttemptsMigrationContract(t *testing.T) {
	raw, err := FS.ReadFile("238_checkin_lottery_attempts.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(raw)), " ")
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS lottery_attempt_wallets",
		"balance INTEGER NOT NULL DEFAULT 0",
		"CHECK (balance >= 0)",
		"CREATE TABLE IF NOT EXISTS lottery_attempt_ledger",
		"delta INTEGER NOT NULL",
		"balance_after INTEGER NOT NULL",
		"source_type VARCHAR(32) NOT NULL",
		"source_id BIGINT NOT NULL",
		"lottery_attempt_ledger_source_uq",
		"ON lottery_attempt_ledger (source_type, source_id)",
		"ADD COLUMN IF NOT EXISTS lottery_attempts_reward INTEGER NOT NULL DEFAULT 0",
		"ADD COLUMN IF NOT EXISTS attempt_source VARCHAR(20) NOT NULL DEFAULT 'activity'",
		"UPDATE lottery_draws SET attempt_source = 'activity'",
		"CHECK (attempt_source IN ('activity', 'wallet'))",
		"CHECK (attempt_limit >= 0)",
	} {
		require.Contains(t, sql, fragment)
	}
}
