//go:build unit

package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSafeDateFormat(t *testing.T) {
	tests := []struct {
		name        string
		granularity string
		expected    string
	}{
		// 合法值
		{"hour", "hour", "YYYY-MM-DD HH24:00"},
		{"day", "day", "YYYY-MM-DD"},
		{"week", "week", "IYYY-IW"},
		{"month", "month", "YYYY-MM"},

		// 非法值回退到默认
		{"空字符串", "", "YYYY-MM-DD"},
		{"未知粒度 year", "year", "YYYY-MM-DD"},
		{"未知粒度 minute", "minute", "YYYY-MM-DD"},

		// 恶意字符串
		{"SQL 注入尝试", "'; DROP TABLE users; --", "YYYY-MM-DD"},
		{"带引号", "day'", "YYYY-MM-DD"},
		{"带括号", "day)", "YYYY-MM-DD"},
		{"Unicode", "日", "YYYY-MM-DD"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := safeDateFormat(tc.granularity)
			require.Equal(t, tc.expected, got, "safeDateFormat(%q)", tc.granularity)
		})
	}
}

func TestBuildUsageLogBatchInsertQuery_UsesConflictDoNothing(t *testing.T) {
	log := &service.UsageLog{
		UserID:       1,
		APIKeyID:     2,
		AccountID:    3,
		RequestID:    "req-batch-no-update",
		Model:        "gpt-5",
		InputTokens:  10,
		OutputTokens: 5,
		TotalCost:    1.2,
		ActualCost:   1.2,
		CreatedAt:    time.Now().UTC(),
	}
	prepared := prepareUsageLogInsert(log)

	query, _ := buildUsageLogBatchInsertQuery([]string{usageLogBatchKey(log.RequestID, log.APIKeyID)}, map[string]usageLogInsertPrepared{
		usageLogBatchKey(log.RequestID, log.APIKeyID): prepared,
	})

	require.Contains(t, query, "ON CONFLICT (request_id, api_key_id) DO NOTHING")
	require.NotContains(t, strings.ToUpper(query), "DO UPDATE")
}

func TestUsageLogInsert_ThresholdExemptCostWiring(t *testing.T) {
	const thresholdExemptCostArgIndex = 30
	createdAt := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	log := &service.UsageLog{
		UserID:              1,
		APIKeyID:            2,
		AccountID:           3,
		RequestID:           "req-threshold-exempt-cost",
		Model:               "gpt-5",
		ActualCost:          1.25,
		ThresholdExemptCost: 0.375,
		CreatedAt:           createdAt,
	}
	prepared := prepareUsageLogInsert(log)

	require.Len(t, prepared.args, len(usageLogInsertArgTypes))
	require.Equal(t, "numeric", usageLogInsertArgTypes[thresholdExemptCostArgIndex])
	require.Equal(t, 0.375, prepared.args[thresholdExemptCostArgIndex])
	require.Contains(t, usageLogSelectColumns, "threshold_exempt_cost")

	db, mock := newSQLMock(t)
	mock.ExpectQuery("(?s)INSERT INTO usage_logs.*threshold_exempt_cost").
		WithArgs(anySliceToDriverValues(prepared.args)...).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(91), createdAt))
	inserted, err := (&usageLogRepository{sql: db}).createSingle(context.Background(), db, log)
	require.NoError(t, err)
	require.True(t, inserted)
	require.NoError(t, mock.ExpectationsWereMet())

	key := usageLogBatchKey(log.RequestID, log.APIKeyID)
	batchQuery, batchArgs := buildUsageLogBatchInsertQuery(
		[]string{key},
		map[string]usageLogInsertPrepared{key: prepared},
	)
	require.GreaterOrEqual(t, strings.Count(batchQuery, "threshold_exempt_cost"), 3)
	require.Equal(t, 0.375, batchArgs[thresholdExemptCostArgIndex+1])

	bestEffortQuery, bestEffortArgs := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})
	require.GreaterOrEqual(t, strings.Count(bestEffortQuery, "threshold_exempt_cost"), 3)
	require.Equal(t, 0.375, bestEffortArgs[thresholdExemptCostArgIndex])
}
