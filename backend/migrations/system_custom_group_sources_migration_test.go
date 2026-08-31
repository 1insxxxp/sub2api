package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystemCustomGroupSourcesMigration(t *testing.T) {
	content, err := FS.ReadFile("236_system_custom_group_sources.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS system_custom_group_sources")
	require.Contains(t, sql, "UNIQUE (group_id, source_group_id)")
	require.Contains(t, sql, "UNIQUE (group_id, priority)")
	require.Contains(t, sql, "ROW_NUMBER() OVER")
	require.Contains(t, sql, "FROM system_custom_group_models")
	require.Contains(t, sql, "WHERE NOT EXISTS")
	require.Contains(t, sql, "configured_sources.group_id = existing_sources.group_id")
	require.Contains(t, sql, "ON CONFLICT DO NOTHING")
	require.NotContains(t, sql, "ON CONFLICT (group_id, source_group_id) DO NOTHING")
}
