package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLotteryAttemptGrantIdempotencyMigrationContract(t *testing.T) {
	raw, err := FS.ReadFile("240_lottery_attempt_grant_idempotency.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(raw)), " ")
	for _, fragment := range []string{
		"ALTER TABLE lottery_attempt_grants ADD COLUMN IF NOT EXISTS request_key VARCHAR(128)",
		"ADD COLUMN IF NOT EXISTS target_all BOOLEAN NOT NULL DEFAULT FALSE",
		"SET request_key = 'legacy-' || id",
		"ALTER COLUMN request_key SET NOT NULL",
		"lottery_attempt_grants_request_user_uq",
	} {
		require.Contains(t, sql, fragment)
	}
}
