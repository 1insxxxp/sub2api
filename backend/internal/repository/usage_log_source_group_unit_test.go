//go:build unit

package repository

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareUsageLogInsert_SourceGroupIDArgWiring(t *testing.T) {
	sourceGroupID := int64(202)
	log := newSessionIDUsageLog(nil)
	log.SourceGroupID = &sourceGroupID

	prepared := prepareUsageLogInsert(log)
	require.Len(t, prepared.args, len(usageLogInsertArgTypes))

	const sourceGroupArgIndex = 10
	sourceGroupArg := prepared.args[sourceGroupArgIndex]
	nullID, ok := sourceGroupArg.(sql.NullInt64)
	require.True(t, ok, "source_group_id arg should be sql.NullInt64, got %T", sourceGroupArg)
	require.True(t, nullID.Valid)
	require.Equal(t, sourceGroupID, nullID.Int64)
	require.Equal(t, "bigint", usageLogInsertArgTypes[sourceGroupArgIndex])

	key := usageLogBatchKey(log.RequestID, log.APIKeyID)
	batchQuery, _ := buildUsageLogBatchInsertQuery([]string{key}, map[string]usageLogInsertPrepared{key: prepared})
	bestEffortQuery, _ := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})
	require.GreaterOrEqual(t, strings.Count(batchQuery, "source_group_id"), 3)
	require.GreaterOrEqual(t, strings.Count(bestEffortQuery, "source_group_id"), 3)
	require.Contains(t, usageLogSelectColumns, "source_group_id")
}
