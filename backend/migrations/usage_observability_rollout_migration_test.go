package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageObservabilityMigrationsBoundProductionLockWait(t *testing.T) {
	for _, name := range []string{"253_usage_token_provenance.sql", "254_usage_pending_records.sql"} {
		t.Run(name, func(t *testing.T) {
			content, err := FS.ReadFile(name)
			require.NoError(t, err)
			sql := strings.Join(strings.Fields(string(content)), " ")
			lockAt := strings.Index(sql, "SET LOCAL lock_timeout = '3s';")
			statementAt := strings.Index(sql, "SET LOCAL statement_timeout = '30s';")
			require.GreaterOrEqual(t, lockAt, 0, "a waiting migration must not queue production writes for the 10-minute bootstrap timeout")
			require.GreaterOrEqual(t, statementAt, 0, "migration execution must remain bounded after it acquires locks")
			for _, ddl := range []string{"ALTER TABLE", "CREATE TABLE", "CREATE OR REPLACE FUNCTION", "DROP TRIGGER"} {
				if firstDDL := strings.Index(sql, ddl); firstDDL >= 0 {
					require.Less(t, lockAt, firstDDL)
					require.Less(t, statementAt, firstDDL)
				}
			}
		})
	}
}

func TestUsageTokenProvenanceMigrationAvoidsBlockingHistoricalValidation(t *testing.T) {
	content, err := FS.ReadFile("253_usage_token_provenance.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	for _, field := range []string{"input_tokens_source", "output_tokens_source"} {
		constraint := "usage_response_outcomes_" + field + "_check"
		require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS "+field+" VARCHAR(20) NOT NULL DEFAULT 'unknown'")
		require.Contains(t, sql, "conrelid = 'usage_response_outcomes'::regclass AND conname = '"+constraint+"'", "constraint existence must be scoped to this table")
		require.Contains(t, sql, "ADD CONSTRAINT "+constraint+" CHECK ("+field+" IN ('unknown', 'upstream', 'unreported', 'estimated')) NOT VALID", "new writes stay constrained without scanning all historical outcomes under the DDL lock")
	}
	require.Equal(t, 2, strings.Count(sql, "IF NOT EXISTS ( SELECT 1 FROM pg_constraint"))
	require.NotContains(t, sql, "VALIDATE CONSTRAINT", "historical validation belongs outside the short schema-change transaction")
	require.Contains(t, sql, "locally counted received output")
}
