package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmptyResponseCompensationMigration(t *testing.T) {
	raw, err := FS.ReadFile("196_empty_response_compensation.sql")
	require.NoError(t, err)

	sql := string(raw)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS usage_response_outcomes")
	require.Contains(t, sql, "UNIQUE (request_id, api_key_id)")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS empty_response_claims")
	require.Contains(t, sql, "usage_log_id BIGINT NOT NULL UNIQUE")
	require.Contains(t, sql, "compensated_cost DECIMAL(20,10) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "empty_response_compensation_enabled BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, sql, "CHECK (compensated_cost >= 0 AND compensated_cost <= actual_cost)")

	for _, forbidden := range []string{
		"response_text",
		"response_body",
		"prompt_text",
		"tool_arguments",
	} {
		require.NotContains(t, strings.ToLower(sql), forbidden)
	}
}
