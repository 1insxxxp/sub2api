package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystemCustomGroupRoutesMigration(t *testing.T) {
	content, err := FS.ReadFile("221_system_custom_group_routes.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS system_custom_routing_enabled BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS system_custom_group_models")
	require.Contains(t, sql, "group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE")
	require.Contains(t, sql, "source_group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS source_group_id BIGINT NULL")
	require.Contains(t, sql, "CHECK (group_id <> source_group_id)")

	indexContent, err := FS.ReadFile("221a_system_custom_group_route_indexes_notx.sql")
	require.NoError(t, err)

	indexSQL := strings.Join(strings.Fields(string(indexContent)), " ")
	require.Contains(t, indexSQL, "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_system_custom_group_public_model_ci ON system_custom_group_models(group_id, LOWER(public_model))")
	require.Contains(t, indexSQL, "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_system_custom_group_source_model_ci ON system_custom_group_models(group_id, source_group_id, LOWER(source_model))")
	require.NotContains(t, indexSQL, "BEGIN")
	require.NotContains(t, indexSQL, "COMMIT")
}
