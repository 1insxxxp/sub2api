//go:build unit

package repository

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareUsageLogInsert_CustomGroupIDArgWiring(t *testing.T) {
	customGroupID := int64(17)
	log := newSessionIDUsageLog(nil)
	log.CustomGroupID = &customGroupID

	prepared := prepareUsageLogInsert(log)
	require.Len(t, prepared.args, len(usageLogInsertArgTypes))

	const customGroupArgIndex = 11
	customGroupArg := prepared.args[customGroupArgIndex]
	nullID, ok := customGroupArg.(sql.NullInt64)
	require.True(t, ok, "custom_group_id arg should be sql.NullInt64, got %T", customGroupArg)
	require.True(t, nullID.Valid)
	require.Equal(t, customGroupID, nullID.Int64)
	require.Equal(t, "bigint", usageLogInsertArgTypes[customGroupArgIndex])

	key := usageLogBatchKey(log.RequestID, log.APIKeyID)
	batchQuery, _ := buildUsageLogBatchInsertQuery([]string{key}, map[string]usageLogInsertPrepared{key: prepared})
	bestEffortQuery, _ := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})
	require.GreaterOrEqual(t, strings.Count(batchQuery, "custom_group_id"), 3)
	require.GreaterOrEqual(t, strings.Count(bestEffortQuery, "custom_group_id"), 3)
	require.Contains(t, usageLogSelectColumns, "custom_group_id")
}
