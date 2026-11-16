//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRequestLogRetentionSkipsUsageLogsReferencedByEmptyResponseClaims(t *testing.T) {
	ctx := context.Background()
	schema := createGroupUsageRollupTriggerTestSchema(t, ctx, false)
	db := openRetentionTestDB(t, ctx, schema)

	_, err := db.ExecContext(ctx, `
		ALTER TABLE empty_response_claims
		ADD CONSTRAINT empty_response_claims_usage_log_id_fkey
		FOREIGN KEY (usage_log_id) REFERENCES usage_logs(id) ON DELETE RESTRICT;
		INSERT INTO groups (id) VALUES (10);
		INSERT INTO users (id) VALUES (1);
		INSERT INTO usage_logs (id, user_id, group_id, actual_cost, created_at)
		VALUES (1, 1, 10, 1, TIMESTAMPTZ '2020-01-02'),
		       (2, 1, 10, 1, TIMESTAMPTZ '2020-01-03');
		INSERT INTO empty_response_claims (usage_log_id) VALUES (1);
	`)
	require.NoError(t, err)

	affected, err := cleanupUsageLogsBatchWithRollupInvalidation(ctx, db, time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.EqualValues(t, 1, affected)

	var usageLogs, claims int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_logs").Scan(&usageLogs))
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM empty_response_claims").Scan(&claims))
	require.Equal(t, 1, usageLogs, "the claimed usage log remains available for compensation evidence")
	require.Equal(t, 1, claims)
}
