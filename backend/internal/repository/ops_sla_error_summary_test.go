package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGetSLAErrorSummaryAggregatesFinalFailuresAndUsesSLAExclusions(t *testing.T) {
	captured := ""
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(summaryQueryMatcher{captured: &captured}))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	start := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	latest := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	// The filter deliberately opts into recovered upstream rows and uses view=all.
	// SLA aggregation must still force final status >= 400 and exclude business limits.
	mock.ExpectQuery("FROM ops_error_logs e").WithArgs(start, "openai", int64(7), "upstream", "%needle%").WillReturnRows(
		sqlmock.NewRows([]string{"group_id", "group_name", "model", "account_id", "account_name", "status_code", "reason", "error_type", "count", "latest_at", "representative_error_id"}).
			AddRow(int64(7), "production", "gpt-5", int64(11), "primary", 503, "upstream request failed", "upstream_error", int64(3), latest, int64(103)).
			AddRow(int64(7), "production", "gpt-5", int64(11), "primary", 400, "invalid request", "client_error", int64(1), latest.Add(-time.Minute), int64(102)).
			AddRow(int64(7), "production", "claude", int64(12), "backup", 500, "provider failed", "upstream_error", int64(2), latest.Add(-2*time.Minute), int64(101)).
			AddRow(nil, "未分组", "gpt-5", nil, "未知账号", 502, "gateway failed", "gateway_error", int64(1), latest.Add(-3*time.Minute), int64(100)),
	)

	result, err := (&opsRepository{db: db}).GetSLAErrorSummary(context.Background(), &service.OpsErrorLogFilter{
		StartTime:                &start,
		Platform:                 "openai",
		GroupID:                  summaryPtrInt64(7),
		Phase:                    "upstream",
		IncludeRecoveredUpstream: true,
		View:                     "all",
		Query:                    "needle",
	})
	require.NoError(t, err)
	require.Equal(t, int64(7), result.TotalErrors)
	require.Equal(t, 2, result.GroupCount)
	require.Equal(t, "production", result.Groups[0].GroupName)
	require.Equal(t, int64(6), result.Groups[0].ErrorCount)
	require.Equal(t, "gpt-5", result.Groups[0].Models[0].Model)
	require.Equal(t, int64(4), result.Groups[0].Models[0].ErrorCount)
	require.Equal(t, int64(4), result.Groups[0].Models[0].Accounts[0].ErrorCount)
	require.Equal(t, int64(103), result.Groups[0].Models[0].Accounts[0].Reasons[0].RepresentativeErrorID)
	require.Nil(t, result.Groups[1].GroupID)
	require.Nil(t, result.Groups[1].Models[0].Accounts[0].AccountID)
	require.NoError(t, mock.ExpectationsWereMet())

	// SLA summary must not inherit provider-health recovery semantics, and must
	// always exclude business-limited rows even when the caller asks for all rows.
	require.Contains(t, captured, "COALESCE(e.status_code, 0) >= 400")
	require.Contains(t, captured, "COALESCE(e.is_business_limited,false) = false")
	require.NotContains(t, captured, "IncludeRecoveredUpstream")
	require.Contains(t, captured, "e.created_at >= $1")
	require.Contains(t, captured, "e.platform = $2")
	require.Contains(t, captured, "e.group_id = $3")
	require.Contains(t, captured, "e.error_phase = $4")
	require.Contains(t, captured, "e.error_message ILIKE $5")
	require.NotContains(t, captured, "upstream_status_code, e.status_code, 0) = ANY")
}

func TestGetSLAErrorSummarySortsGroupsModelsAccountsReasonsAndKeepsEmptyRowsNormalized(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	base := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{"group_id", "group_name", "model", "account_id", "account_name", "status_code", "reason", "error_type", "count", "latest_at", "representative_error_id"}).
		AddRow(int64(2), "same-name", "z-model", int64(20), "same-account", 500, "z reason", "upstream_error", int64(1), base, int64(20)).
		AddRow(int64(1), "same-name", "a-model", int64(10), "same-account", 502, "b reason", "gateway_error", int64(1), base, int64(10)).
		AddRow(int64(1), "same-name", "a-model", int64(10), "same-account", 500, "a reason", "gateway_error", int64(2), base.Add(-time.Minute), int64(9)).
		AddRow(int64(1), "same-name", "a-model", int64(10), "same-account", 500, "a reason", "gateway_error", int64(1), base.Add(2*time.Minute), int64(11))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	result, err := (&opsRepository{db: db}).GetSLAErrorSummary(context.Background(), &service.OpsErrorLogFilter{})
	require.NoError(t, err)
	require.Equal(t, 2, result.GroupCount)
	require.EqualValues(t, 1, *result.Groups[0].GroupID)
	require.Equal(t, "a-model", result.Groups[0].Models[0].Model)
	require.Equal(t, int64(4), result.Groups[0].Models[0].ErrorCount)
	require.Equal(t, int64(4), result.Groups[0].Models[0].Accounts[0].ErrorCount)
	require.Equal(t, int64(3), result.Groups[0].Models[0].Accounts[0].Reasons[0].Count)
	require.Equal(t, int64(11), result.Groups[0].Models[0].Accounts[0].Reasons[0].RepresentativeErrorID, "latest reason row should win representative id")
	require.NoError(t, mock.ExpectationsWereMet())

	emptyDB, emptyMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = emptyDB.Close() }()
	emptyMock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"group_id", "group_name", "model", "account_id", "account_name", "status_code", "reason", "error_type", "count", "latest_at", "representative_error_id"}))
	empty, err := (&opsRepository{db: emptyDB}).GetSLAErrorSummary(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, empty.Groups)
	require.Empty(t, empty.Groups)
	require.NoError(t, emptyMock.ExpectationsWereMet())
}

func TestGetSLAErrorSummaryTruncatesTopLevelGroupsAfterSorting(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	rows := sqlmock.NewRows([]string{"group_id", "group_name", "model", "account_id", "account_name", "status_code", "reason", "error_type", "count", "latest_at", "representative_error_id"})
	base := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= maxSummaryGroups+1; i++ {
		rows.AddRow(int64(i), fmt.Sprintf("group-%d", i), "model", int64(i), "account", 500, "reason", "gateway_error", int64(1), base.Add(time.Duration(i)*time.Minute), int64(i))
	}
	mock.ExpectQuery("SELECT").WillReturnRows(rows)
	result, err := (&opsRepository{db: db}).GetSLAErrorSummary(context.Background(), &service.OpsErrorLogFilter{})
	require.NoError(t, err)
	require.Equal(t, maxSummaryGroups+1, result.GroupCount)
	require.Len(t, result.Groups, maxSummaryGroups)
	require.True(t, result.GroupsTruncated)
	require.Equal(t, "group-101", result.Groups[0].GroupName)
	require.NoError(t, mock.ExpectationsWereMet())
}
