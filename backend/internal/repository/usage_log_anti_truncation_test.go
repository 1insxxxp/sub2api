package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestHydrateUsageLogCompensationLoadsAntiTruncationRounds(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := newUsageLogRepositoryWithSQL(nil, db)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT ul.id, erc.id, erc.status, erc.reason_code")).
		WithArgs("{42}").
		WillReturnRows(sqlmock.NewRows([]string{
			"usage_id", "claim_id", "claim_status", "reason_code", "http_status", "upstream_status",
			"has_text", "has_tool_call", "has_reasoning", "has_media", "output_bytes", "event_count",
			"stream_completed", "finish_reason", "disconnect_source", "upstream_error_kind", "collector_version", "anti_truncation_rounds", "input_tokens_source", "output_tokens_source",
		}).AddRow(42, nil, nil, nil, 200, 200, true, false, false, false, 10, 2, true, "stop", "none", "none", 1, 3, "unknown", "unknown"))
	logs := []service.UsageLog{{ID: 42}}
	require.NoError(t, repo.hydrateUsageLogCompensation(context.Background(), logs))
	require.Equal(t, 3, logs[0].AntiTruncationRounds)
	require.NotNil(t, logs[0].Outcome)
	require.NoError(t, mock.ExpectationsWereMet())
}
