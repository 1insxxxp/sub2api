package migrations

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestContinuationUsageAuditMigration(t *testing.T) {
	data, err := FS.ReadFile("262_continuation_usage_audit.sql")
	require.NoError(t, err)
	require.Contains(t, string(data), "ADD COLUMN IF NOT EXISTS continuation_usage JSONB")
	require.NotContains(t, string(data), "UPDATE usage_logs")
}
