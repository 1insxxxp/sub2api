package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func monthlyReportRepo(t *testing.T) (*subAdminCommissionRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return &subAdminCommissionRepository{db: db, sql: db}, mock
}

const monthlyGrantQuery = `(?s)WITH global_grants AS .*MIN\(cg\.granted_date\).*JOIN users.*u\.deleted_at IS NULL.*u\.role = 'sub_admin'.*cg\.enabled = TRUE.*GROUP BY cg\.group_id.*JOIN groups g.*gg\.granted_date < \$1::date.*ORDER BY g\.name ASC, g\.id ASC`
const monthlyRollupQuery = `(?s)FROM usage_group_daily_rollups r.*JOIN global_grants gg.*r\.bucket_date >= gg\.granted_date.*r\.bucket_date >= \$1::date.*r\.bucket_date < \$2::date.*GROUP BY r\.group_id, r\.bucket_date`
const monthlyLiveQuery = `(?s)FROM usage_logs ul.*JOIN global_grants gg.*ul\.created_at >= \$1 AND ul\.created_at < \$2.*NOT EXISTS.*ce\.started_at <= ul\.created_at.*ce\.ended_at IS NULL OR ul\.created_at < ce\.ended_at.*GROUP BY ul\.group_id, 2`

func TestSubAdminCommissionMonthlyReportBatchesAndFillsAuthorizedGroupDates(t *testing.T) {
	repo, mock := monthlyReportRepo(t)
	today, err := service.ParseGroupUsageDate("2026-08-23")
	require.NoError(t, err)
	mock.ExpectQuery(monthlyGrantQuery).WithArgs("2026-08-24").WillReturnRows(sqlmock.NewRows([]string{"group_id", "group_name", "granted_date", "active"}).
		AddRow(1, "Active", "2026-08-20", true).
		AddRow(2, "New", "2026-08-22", true).
		AddRow(3, "Inactive used", "2026-08-20", false).
		AddRow(4, "Inactive empty", "2026-08-20", false))
	mock.ExpectQuery(monthlyRollupQuery).WithArgs("2026-08-20", "2026-08-22").WillReturnRows(sqlmock.NewRows([]string{"group_id", "date", "actual_cost"}).
		AddRow(1, "2026-08-20", 12.5).AddRow(3, "2026-08-21", 2.5))
	mock.ExpectQuery(monthlyLiveQuery).WithArgs(today.AddDate(0, 0, -1).UTC(), today.AddDate(0, 0, 1).UTC(), service.GroupUsageTimezoneName()).WillReturnRows(sqlmock.NewRows([]string{"group_id", "date", "actual_cost"}).
		AddRow(1, "2026-08-22", 5.0).AddRow(2, "2026-08-23", 7.5))
	report, err := repo.ListMonthlyReport(context.Background(), 17, "2026-08", 0.2, today.Add(10*time.Hour))
	require.NoError(t, err)
	require.Len(t, report.Groups, 3)
	require.Equal(t, []int64{1, 2, 3}, []int64{report.Groups[0].GroupID, report.Groups[1].GroupID, report.Groups[2].GroupID})
	require.Len(t, report.Days, 4)
	require.Len(t, report.Groups[0].Days, 4)
	require.Len(t, report.Groups[1].Days, 2)
	require.Equal(t, "2026-08-22", report.Groups[1].Days[0].Date)
	require.Zero(t, report.Groups[1].Days[0].ActualCost)
	require.Equal(t, 7.5, report.Groups[1].Days[1].ActualCost)
	require.Equal(t, "Inactive used", report.Groups[2].GroupName)
	for _, day := range report.Days {
		var cost, commission float64
		for _, group := range report.Groups {
			for _, groupDay := range group.Days {
				if groupDay.Date == day.Date {
					cost += groupDay.ActualCost
					commission += groupDay.CommissionAmount
				}
			}
		}
		require.Equal(t, cost, day.ActualCost)
		require.Equal(t, commission, day.CommissionAmount)
		require.True(t, day.Enabled)
	}
	require.Equal(t, 12.5, report.Days[0].ActualCost)
	require.Equal(t, 2.5, report.Days[1].ActualCost)
	require.Equal(t, 5.0, report.Days[2].ActualCost)
	require.Equal(t, 7.5, report.Days[3].ActualCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSubAdminCommissionMonthlyReportEmptyAndInvalidRangesAvoidUsageQueries(t *testing.T) {
	for _, tc := range []struct {
		name, month string
		grants      bool
	}{
		{"future", "2100-01", false}, {"no grants", "2026-08", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock := monthlyReportRepo(t)
			if tc.grants {
				mock.ExpectQuery(monthlyGrantQuery).WithArgs("2026-08-24").WillReturnRows(sqlmock.NewRows([]string{"group_id", "group_name", "granted_date", "active"}))
			}
			report, err := repo.ListMonthlyReport(context.Background(), 17, tc.month, 0.2, time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC))
			require.NoError(t, err)
			require.NotNil(t, report.Days)
			require.NotNil(t, report.Groups)
			require.Empty(t, report.Days)
			require.Empty(t, report.Groups)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
	repo, mock := monthlyReportRepo(t)
	for _, month := range []string{"2026-13", "2026-8", "bad", "2026-08-01"} {
		_, err := repo.ListMonthlyReport(context.Background(), 17, month, 0.2, time.Now())
		require.Error(t, err, month)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSubAdminCommissionMonthlyReportHistoricalExclusionsAndZeroCostUsage(t *testing.T) {
	repo, mock := monthlyReportRepo(t)
	mock.ExpectQuery(monthlyGrantQuery).WithArgs("2000-02-01").WillReturnRows(sqlmock.NewRows([]string{"group_id", "group_name", "granted_date", "active"}).
		AddRow(3, "Inactive all excluded", "2000-01-31", false))
	mock.ExpectQuery(`(?s)SUM\(r\.actual_cost - r\.commission_excluded_cost\).*`+monthlyRollupQuery).WithArgs("2000-01-31", "2000-02-01").WillReturnRows(sqlmock.NewRows([]string{"group_id", "date", "actual_cost"}).AddRow(3, "2000-01-31", 0.0))
	report, err := repo.ListMonthlyReport(context.Background(), 17, "2000-01", 0.2, time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, report.Groups, 1)
	require.Len(t, report.Days, 1)
	require.Zero(t, report.Days[0].ActualCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSubAdminCommissionMonthlyReportPropagatesQueriesAndScanErrors(t *testing.T) {
	for _, phase := range []string{"grants", "grant scan", "rollup", "rollup scan", "live", "live scan"} {
		t.Run(phase, func(t *testing.T) {
			repo, mock := monthlyReportRepo(t)
			want := errors.New("database unavailable")
			grant := mock.ExpectQuery(monthlyGrantQuery)
			switch phase {
			case "grants":
				grant.WillReturnError(want)
			case "grant scan":
				grant.WillReturnRows(sqlmock.NewRows([]string{"group_id", "group_name", "granted_date", "active"}).AddRow("bad", "Group", "2026-08-20", true))
			default:
				grant.WillReturnRows(sqlmock.NewRows([]string{"group_id", "group_name", "granted_date", "active"}).AddRow(1, "Group", "2026-08-20", true))
				rollup := mock.ExpectQuery(monthlyRollupQuery)
				switch phase {
				case "rollup":
					rollup.WillReturnError(want)
				case "rollup scan":
					rollup.WillReturnRows(sqlmock.NewRows([]string{"group_id", "date", "actual_cost"}).AddRow(1, "2026-08-20", "bad"))
				default:
					rollup.WillReturnRows(sqlmock.NewRows([]string{"group_id", "date", "actual_cost"}))
					live := mock.ExpectQuery(monthlyLiveQuery)
					if phase == "live" {
						live.WillReturnError(want)
					} else {
						live.WillReturnRows(sqlmock.NewRows([]string{"group_id", "date", "actual_cost"}).AddRow(1, "2026-08-23", "bad"))
					}
				}
			}
			_, err := repo.ListMonthlyReport(context.Background(), 17, "2026-08", 0.2, time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC))
			require.Error(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
