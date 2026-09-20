package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupModelStatusVisibilityMigration(t *testing.T) {
	body, err := FS.ReadFile("244_group_model_status_visibility.sql")
	require.NoError(t, err)
	sql := string(body)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS model_status_visibility JSONB NOT NULL DEFAULT '{}'::jsonb")
	require.Contains(t, sql, "COMMENT ON COLUMN groups.model_status_visibility")
}
