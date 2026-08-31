package migrate

import (
	"testing"

	entschema "entgo.io/ent/dialect/sql/schema"
	"github.com/stretchr/testify/require"
)

func TestSystemCustomGroupSourcesIndexNamesMatchMigration(t *testing.T) {
	sourceUnique := findIndexByName(
		t,
		SystemCustomGroupSourcesTable,
		"system_custom_group_sources_group_id_source_group_id_key",
	)
	require.True(t, sourceUnique.Unique)
	require.Equal(t, []string{"group_id", "source_group_id"}, indexColumnNames(sourceUnique))

	priorityUnique := findIndexByName(
		t,
		SystemCustomGroupSourcesTable,
		"system_custom_group_sources_group_id_priority_key",
	)
	require.True(t, priorityUnique.Unique)
	require.Equal(t, []string{"group_id", "priority"}, indexColumnNames(priorityUnique))

	sourceLookup := findIndexByName(
		t,
		SystemCustomGroupSourcesTable,
		"idx_system_custom_group_sources_source_group_id",
	)
	require.False(t, sourceLookup.Unique)
	require.Equal(t, []string{"source_group_id"}, indexColumnNames(sourceLookup))
}

func indexColumnNames(index *entschema.Index) []string {
	names := make([]string, 0, len(index.Columns))
	for _, column := range index.Columns {
		names = append(names, column.Name)
	}
	return names
}
