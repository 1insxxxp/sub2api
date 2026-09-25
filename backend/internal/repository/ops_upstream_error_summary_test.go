package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type summaryQueryMatcher struct{ captured *string }

func (m summaryQueryMatcher) Match(expected string, actual string) error {
	if !strings.Contains(actual, expected) {
		return fmt.Errorf("query missing expected fragment %q", expected)
	}
	*m.captured = actual
	return nil
}

func TestGetUpstreamErrorSummaryAggregatesAndSortsStoredReasons(t *testing.T) {
	captured := ""
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(summaryQueryMatcher{captured: &captured}))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	start := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	prefix := strings.Repeat("x", 512)
	mock.ExpectQuery("FROM ops_error_logs e").WithArgs(start, "openai", int64(7), "upstream", "{429}", "%needle%").WillReturnRows(
		sqlmock.NewRows([]string{"group_id", "group_name", "model", "account_id", "account_name", "status_code", "reason", "error_type", "count", "latest_at", "representative_error_id"}).
			AddRow(int64(7), "production", "gpt-5", int64(11), "primary", 429, "rate limited", "upstream_error", int64(2), time.Date(2026, 9, 25, 10, 2, 0, 0, time.UTC), int64(102)).
			AddRow(int64(7), "production", "gpt-5", int64(11), "primary", 500, "overloaded", "upstream_error", int64(1), time.Date(2026, 9, 25, 10, 1, 0, 0, time.UTC), int64(101)).
			AddRow(int64(7), "production", "claude", int64(12), "backup", 503, "overloaded", "upstream_error", int64(3), time.Date(2026, 9, 25, 10, 3, 0, 0, time.UTC), int64(103)).
			AddRow(int64(7), "production", "gpt-5", int64(11), "primary", 500, prefix+"A", "upstream_error", int64(1), time.Date(2026, 9, 25, 10, 4, 0, 0, time.UTC), int64(104)).
			AddRow(int64(7), "production", "gpt-5", int64(11), "primary", 500, prefix+"B", "upstream_error", int64(1), time.Date(2026, 9, 25, 10, 5, 0, 0, time.UTC), int64(105)).
			AddRow(nil, "未分组", "gpt-5", nil, "未知账号", 502, "gateway", "provider", int64(4), time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC), int64(90)),
	)
	repo := &opsRepository{db: db}
	result, err := repo.GetUpstreamErrorSummary(context.Background(), &service.OpsErrorLogFilter{StartTime: &start, Platform: "openai", GroupID: summaryPtrInt64(7), Phase: "upstream", IncludeRecoveredUpstream: true, StatusCodes: []int{429}, Query: "needle", View: "all"})
	require.NoError(t, err)
	require.Equal(t, int64(12), result.TotalErrors)
	require.Equal(t, 2, result.GroupCount)
	require.Equal(t, "production", result.Groups[0].GroupName)
	require.Equal(t, int64(8), result.Groups[0].ErrorCount)
	require.Equal(t, "gpt-5", result.Groups[0].Models[0].Model)
	require.Equal(t, int64(5), result.Groups[0].Models[0].Accounts[0].ErrorCount)
	require.Equal(t, "未分组", result.Groups[1].GroupName)
	require.Nil(t, result.Groups[1].GroupID)
	require.Nil(t, result.Groups[1].Models[0].Accounts[0].AccountID)
	require.Equal(t, int64(103), result.Groups[0].Models[1].Accounts[0].Reasons[0].RepresentativeErrorID)
	primaryReasons := result.Groups[0].Models[0].Accounts[0].Reasons
	require.Len(t, primaryReasons, 4)
	// The two stored messages differ only after the response limit. They remain
	// separate aggregates even though their returned, sanitized text is equal.
	var truncatedPrefixReasons []*service.OpsUpstreamErrorSummaryReason
	for _, reason := range primaryReasons {
		if reason.Message == prefix {
			truncatedPrefixReasons = append(truncatedPrefixReasons, reason)
		}
	}
	require.Len(t, truncatedPrefixReasons, 2)
	require.Equal(t, int64(1), truncatedPrefixReasons[0].Count)
	require.Equal(t, int64(1), truncatedPrefixReasons[1].Count)
	require.NoError(t, mock.ExpectationsWereMet())

	// The query is constrained by the shared list predicate builder and only
	// selects persisted reason fields; request bodies/endpoints/credentials are
	// deliberately absent from the summary projection.
	require.Contains(t, captured, "e.created_at >= $1")
	require.Contains(t, captured, "e.platform = $2")
	require.Contains(t, captured, "e.group_id = $3")
	require.Contains(t, captured, "e.error_phase = $4")
	require.Contains(t, captured, "upstream_status_code")
	require.Contains(t, captured, "e.error_message ILIKE $6")
	require.NotContains(t, captured, "error_body")
	require.NotContains(t, captured, "request_path")
	require.True(t, strings.Contains(captured, "upstream_error_message"))
}

func TestGetUpstreamErrorSummarySortsEqualGroupNamesByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	latest := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{"group_id", "group_name", "model", "account_id", "account_name", "status_code", "reason", "error_type", "count", "latest_at", "representative_error_id"}).
		AddRow(int64(2), "same-name", "model", int64(2), "same-account", 500, "reason", "upstream_error", int64(1), latest, int64(2)).
		AddRow(int64(1), "same-name", "model", int64(1), "same-account", 500, "reason", "upstream_error", int64(1), latest, int64(1))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	result, err := (&opsRepository{db: db}).GetUpstreamErrorSummary(context.Background(), &service.OpsErrorLogFilter{})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)
	require.EqualValues(t, 1, *result.Groups[0].GroupID)
	require.EqualValues(t, 2, *result.Groups[1].GroupID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUpstreamErrorSummaryTruncatesTopLevelGroupsAfterSorting(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	rows := sqlmock.NewRows([]string{"group_id", "group_name", "model", "account_id", "account_name", "status_code", "reason", "error_type", "count", "latest_at", "representative_error_id"})
	base := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 101; i++ {
		rows.AddRow(int64(i), fmt.Sprintf("group-%d", i), "model", int64(i), "account", 500, "reason", "upstream_error", int64(1), base.Add(time.Duration(i)*time.Minute), int64(i))
	}
	mock.ExpectQuery("SELECT").WillReturnRows(rows)
	result, err := (&opsRepository{db: db}).GetUpstreamErrorSummary(context.Background(), &service.OpsErrorLogFilter{})
	require.NoError(t, err)
	require.Equal(t, 101, result.GroupCount)
	require.Len(t, result.Groups, 100)
	require.True(t, result.GroupsTruncated)
	require.Equal(t, "group-101", result.Groups[0].GroupName)
	require.Equal(t, base.Add(101*time.Minute), *result.LatestAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func summaryPtrInt64(v int64) *int64 { return &v }

func TestGetUpstreamErrorSummaryNilDBAndEmptyRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
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
