package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration174AddsDurableSingleUseRedeemBatches(t *testing.T) {
	content, err := FS.ReadFile("174_add_single_use_redeem_batches.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ALTER TABLE redeem_codes")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS batch_id VARCHAR(64)")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS redeem_batch_claims")
	require.Contains(t, sql, "UNIQUE (batch_id, user_id)")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS redeemcode_batch_id")
	require.NotContains(t, sql, "REFERENCES redeem_codes")
}
