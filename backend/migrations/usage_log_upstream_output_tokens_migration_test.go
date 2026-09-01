package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageLogUpstreamOutputTokensMigration(t *testing.T) {
	content, err := FS.ReadFile("237_add_usage_log_upstream_output_tokens.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS upstream_output_tokens INTEGER NOT NULL DEFAULT 0")
	require.Contains(t, sql, "COMMENT ON COLUMN usage_logs.upstream_output_tokens")
}
