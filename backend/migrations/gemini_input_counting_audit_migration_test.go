package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiInputCountingAuditMigrationIsAdditiveAndBounded(t *testing.T) {
	content, err := FS.ReadFile("257_gemini_input_counting_audit.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	ddlAt := strings.Index(sql, "ALTER TABLE usage_response_outcomes")
	require.GreaterOrEqual(t, ddlAt, 0)
	for _, setting := range []string{"SET LOCAL lock_timeout = '3s';", "SET LOCAL statement_timeout = '30s';"} {
		at := strings.Index(sql, setting)
		require.GreaterOrEqual(t, at, 0)
		require.Less(t, at, ddlAt)
	}
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS upstream_input_tokens INTEGER")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS input_token_policy VARCHAR(64) NOT NULL DEFAULT ''")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS input_token_policy_count INTEGER")
	for _, field := range []string{"upstream_input_tokens", "input_token_policy_count"} {
		require.NotContains(t, sql, field+" INTEGER NOT NULL", "historical unknown must remain distinct from explicit zero")
		require.NotContains(t, sql, field+" INTEGER DEFAULT", "historical unknown must not become an invented count")
	}
	require.NotContains(t, strings.ToUpper(sql), "UPDATE ")
	require.NotContains(t, sql, "ALTER TABLE usage_logs")
}
