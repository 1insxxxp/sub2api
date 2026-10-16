package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageResponseOutcomesAntiTruncationRoundsMigration(t *testing.T) {
	content, err := FS.ReadFile("247_usage_response_outcomes_anti_truncation_rounds.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ALTER TABLE usage_response_outcomes")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS anti_truncation_rounds INTEGER NOT NULL DEFAULT 0")
	require.Contains(t, sql, "CHECK (anti_truncation_rounds >= 0)")
}
