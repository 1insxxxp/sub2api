package repository

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type summaryQueryMatcher struct{ captured *string }

func (m summaryQueryMatcher) Match(_ string, actual string) error {
	*m.captured = actual
	return nil
}

func TestGetUpstreamErrorSummaryAggregatesAndSortsStoredReasons(t *testing.T) {
	captured := ""
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(summaryQueryMatcher{captured: &captured}))
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta("ignored")).WillReturnRows(
		sqlmock.NewRows([]string{"group_id", "group_name", "model", "account_id", "account_name", "status_code", "reason", "error_type", "count", "latest_at", "representative_error_id"}).
			AddRow(int64(7), "production", "gpt-5", int64(11), "primary", 429, "rate limited", "upstream_error", int64(2), time.Date(2026, 9, 25, 10, 2, 0, 0, time.UTC), int64(102)).
			AddRow(int64(7), "production", "gpt-5", int64(11), "primary", 500, "overloaded", "upstream_error", int64(1), time.Date(2026, 9, 25, 10, 1, 0, 0, time.UTC), int64(101)).
			AddRow(int64(7), "production", "claude", int64(12), "backup", 503, "overloaded", "upstream_error", int64(3), time.Date(2026, 9, 25, 10, 3, 0, 0, time.UTC), int64(103)).
			AddRow(nil, "未分组", "gpt-5", nil, "未知账号", 502, "gateway", "provider", int64(4), time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC), int64(90)),
	)
	repo := &opsRepository{db: db}
	start := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	result, err := repo.GetUpstreamErrorSummary(context.Background(), &service.OpsErrorLogFilter{StartTime: &start, Platform: "openai", View: "all"})
	require.NoError(t, err)
	require.Equal(t, int64(10), result.TotalErrors)
	require.Equal(t, 2, result.GroupCount)
	require.Equal(t, "production", result.Groups[0].GroupName)
	require.Equal(t, int64(6), result.Groups[0].ErrorCount)
	require.Equal(t, "claude", result.Groups[0].Models[0].Model)
	require.Equal(t, int64(3), result.Groups[0].Models[0].Accounts[0].ErrorCount)
	require.Equal(t, "未分组", result.Groups[1].GroupName)
	require.Nil(t, result.Groups[1].GroupID)
	require.Nil(t, result.Groups[1].Models[0].Accounts[0].AccountID)
	require.Equal(t, int64(103), result.Groups[0].Models[0].Accounts[0].Reasons[0].RepresentativeErrorID)
	require.NoError(t, mock.ExpectationsWereMet())

	// The query is constrained by the shared list predicate builder and only
	// selects persisted reason fields; request bodies/endpoints/credentials are
	// deliberately absent from the summary projection.
	require.Contains(t, captured, "e.created_at >= $1")
	require.Contains(t, captured, "e.platform = $2")
	require.NotContains(t, captured, "error_body")
	require.NotContains(t, captured, "request_path")
	require.True(t, strings.Contains(captured, "upstream_error_message"))
}

func TestGetUpstreamErrorSummaryNilDBAndEmptyRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"group_id", "group_name", "model", "account_id", "account_name", "status_code", "reason", "error_type", "count", "latest_at", "representative_error_id"}))
	result, err := (&opsRepository{db: db}).GetUpstreamErrorSummary(context.Background(), &service.OpsErrorLogFilter{})
	require.NoError(t, err)
	require.Empty(t, result.Groups)
	require.Equal(t, int64(0), result.TotalErrors)
	require.NoError(t, mock.ExpectationsWereMet())
	var nilRepo *opsRepository
	_, err = nilRepo.GetUpstreamErrorSummary(context.Background(), &service.OpsErrorLogFilter{})
	require.Error(t, err)
}
