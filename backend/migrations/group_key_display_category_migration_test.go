package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupKeyDisplayCategoryMigrationIsAdditive(t *testing.T) {
	content, err := FS.ReadFile("245_group_key_display_category.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "SET LOCAL lock_timeout = '5s';")
	require.Contains(t, sql, "SET LOCAL statement_timeout = '30s';")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS key_display_category VARCHAR(20) NOT NULL DEFAULT ''")
	require.Contains(t, sql, "CHECK (key_display_category IN ('', 'anthropic', 'openai', 'domestic', 'other'))")
	require.NotContains(t, strings.ToUpper(sql), "UPDATE GROUPS")
	require.NotContains(t, strings.ToUpper(sql), "DROP COLUMN")
}
