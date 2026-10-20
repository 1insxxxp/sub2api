//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type subAdminCommissionVisibilityFixture struct {
	repo        *subAdminCommissionRepository
	adminID     int64
	liveDate    string
	historyDate string
}

func newSubAdminCommissionVisibilityFixture(t *testing.T) subAdminCommissionVisibilityFixture {
	t.Helper()
	ctx := context.Background()
	tx := testTx(t)
	schema := fmt.Sprintf("commission_visibility_%d", time.Now().UnixNano())

	_, err := tx.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "SET LOCAL search_path TO "+schema)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE users (
			id BIGINT PRIMARY KEY,
			deleted_at TIMESTAMPTZ,
			role TEXT NOT NULL
		);
		CREATE TABLE groups (
			id BIGINT PRIMARY KEY,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			deleted_at TIMESTAMPTZ
		);
		CREATE TABLE sub_admin_commission_grants (
			group_id BIGINT NOT NULL,
			sub_admin_user_id BIGINT NOT NULL,
			granted_date DATE NOT NULL,
			enabled BOOLEAN NOT NULL
		);
		CREATE TABLE usage_logs (
			id BIGINT PRIMARY KEY,
			user_id BIGINT NOT NULL DEFAULT 0,
			group_id BIGINT,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			actual_cost NUMERIC NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL
		);
		CREATE TABLE usage_group_daily_rollups (
			group_id BIGINT NOT NULL,
			bucket_date DATE NOT NULL,
			actual_cost NUMERIC NOT NULL DEFAULT 0,
			commission_excluded_cost NUMERIC NOT NULL DEFAULT 0
		);
		CREATE TABLE sub_admin_commission_exclusions (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			started_at TIMESTAMPTZ NOT NULL,
			ended_at TIMESTAMPTZ
		);
	`)
	require.NoError(t, err)

	const liveDate = "2100-01-02"
	const historyDate = "2000-01-02"
	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, role) VALUES (1, 'sub_admin');
		INSERT INTO groups (id, name, status, deleted_at) VALUES
			(1, 'active-empty', 'active', NULL),
			(2, 'inactive-empty', 'inactive', NULL),
			(3, 'inactive-with-usage', 'inactive', NULL),
			(4, 'deleted-empty', 'active', TIMESTAMPTZ '2026-01-01 00:00:00+00'),
			(5, 'deleted-with-usage', 'active', TIMESTAMPTZ '2026-01-01 00:00:00+00');
		INSERT INTO sub_admin_commission_grants (group_id, sub_admin_user_id, granted_date, enabled)
		VALUES (1, 1, DATE '1999-01-01', TRUE), (2, 1, DATE '1999-01-01', TRUE),
			(3, 1, DATE '1999-01-01', TRUE), (4, 1, DATE '1999-01-01', TRUE),
			(5, 1, DATE '1999-01-01', TRUE);
		INSERT INTO usage_logs (id, group_id, input_tokens, actual_cost, created_at) VALUES
			(301, 3, 100, 1.25, TIMESTAMPTZ '2100-01-02 09:00:00+00'),
			(501, 5, 200, 2.50, TIMESTAMPTZ '2100-01-02 10:00:00+00');
		INSERT INTO usage_group_daily_rollups (group_id, bucket_date, actual_cost) VALUES
			(3, DATE '2000-01-02', 1.25), (5, DATE '2000-01-02', 2.50);
	`)
	require.NoError(t, err)

	return subAdminCommissionVisibilityFixture{
		repo:        &subAdminCommissionRepository{sql: tx},
		adminID:     1,
		liveDate:    liveDate,
		historyDate: historyDate,
	}
}

func commissionGroupsByID(groups []service.SubAdminCommissionDayGroup) map[int64]service.SubAdminCommissionDayGroup {
	result := make(map[int64]service.SubAdminCommissionDayGroup, len(groups))
	for _, group := range groups {
		result[group.GroupID] = group
	}
	return result
}

func TestSubAdminCommissionRepositoryListDayGroupsLiveOmitsEmptyInactiveAndDeletedGroups(t *testing.T) {
	fixture := newSubAdminCommissionVisibilityFixture(t)

	groups, err := fixture.repo.ListDayGroups(context.Background(), fixture.adminID, fixture.liveDate, 0.2)

	require.NoError(t, err)
	byID := commissionGroupsByID(groups)
	require.Contains(t, byID, int64(1))
	require.Contains(t, byID, int64(3))
	require.Contains(t, byID, int64(5))
	require.NotContains(t, byID, int64(2))
	require.NotContains(t, byID, int64(4))
	require.Equal(t, 0.25, byID[3].CommissionAmount)
	require.Equal(t, 0.5, byID[5].CommissionAmount)
}

func TestSubAdminCommissionRepositoryListDayGroupsHistoricalOmitsEmptyInactiveAndDeletedGroups(t *testing.T) {
	fixture := newSubAdminCommissionVisibilityFixture(t)

	groups, err := fixture.repo.ListDayGroups(context.Background(), fixture.adminID, fixture.historyDate, 0.2)

	require.NoError(t, err)
	byID := commissionGroupsByID(groups)
	require.Contains(t, byID, int64(1))
	require.Contains(t, byID, int64(3))
	require.Contains(t, byID, int64(5))
	require.NotContains(t, byID, int64(2))
	require.NotContains(t, byID, int64(4))
	require.Equal(t, 0.25, byID[3].CommissionAmount)
	require.Equal(t, 0.5, byID[5].CommissionAmount)
}

func TestSubAdminCommissionExclusionLiveHonorsIntervalsAndCalendarParity(t *testing.T) {
	fixture := newSubAdminCommissionVisibilityFixture(t)
	ctx := context.Background()
	_, err := fixture.repo.sql.ExecContext(ctx, `
		INSERT INTO sub_admin_commission_exclusions (user_id, started_at, ended_at) VALUES
			(42, TIMESTAMPTZ '2100-01-02 10:00:00+00', TIMESTAMPTZ '2100-01-02 11:00:00+00'),
			(42, TIMESTAMPTZ '2100-01-02 12:00:00+00', NULL);
		INSERT INTO usage_logs (id, user_id, group_id, input_tokens, actual_cost, created_at) VALUES
			(601, 42, 1, 10, 1, TIMESTAMPTZ '2100-01-02 09:59:59+00'),
			(602, 42, 1, 20, 2, TIMESTAMPTZ '2100-01-02 10:00:00+00'),
			(603, 42, 1, 30, 3, TIMESTAMPTZ '2100-01-02 11:00:00+00'),
			(604, 42, 1, 40, 4, TIMESTAMPTZ '2100-01-02 12:00:00+00'),
			(605, 43, 1, 50, 5, TIMESTAMPTZ '2100-01-02 12:00:00+00');
	`)
	require.NoError(t, err)
	groups, err := fixture.repo.ListDayGroups(ctx, 1, fixture.liveDate, 0.2)
	require.NoError(t, err)
	group := commissionGroupsByID(groups)[1]
	require.Equal(t, int64(3), group.Requests)
	require.Equal(t, int64(90), group.TotalTokens)
	require.Equal(t, float64(9), group.ActualCost)
	require.InDelta(t, 1.8, group.CommissionAmount, 1e-9)
	now := time.Date(2100, 1, 2, 13, 0, 0, 0, time.UTC)
	days, err := fixture.repo.ListCalendar(ctx, 1, "2100-01", 0.2, now)
	require.NoError(t, err)
	var sum float64
	for _, group := range groups {
		sum += group.ActualCost
	}
	for _, day := range days {
		if day.Date == fixture.liveDate {
			require.Equal(t, sum, day.ActualCost)
		}
	}
}

func TestSubAdminCommissionExclusionHistoricalDeductsOnlyCommissionCost(t *testing.T) {
	fixture := newSubAdminCommissionVisibilityFixture(t)
	ctx := context.Background()
	_, err := fixture.repo.sql.ExecContext(ctx, `UPDATE usage_group_daily_rollups SET commission_excluded_cost = 0.5 WHERE group_id = 3`)
	require.NoError(t, err)
	groups, err := fixture.repo.ListDayGroups(ctx, 1, fixture.historyDate, 0.2)
	require.NoError(t, err)
	require.Equal(t, 0.75, commissionGroupsByID(groups)[3].ActualCost)
	days, err := fixture.repo.ListCalendar(ctx, 1, "2000-01", 0.2, time.Date(2000, 1, 5, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	for _, day := range days {
		if day.Date == fixture.historyDate {
			require.Equal(t, 3.25, day.ActualCost)
		}
	}
	var actualCost float64
	require.NoError(t, scanSingleRow(ctx, fixture.repo.sql, `SELECT actual_cost FROM usage_group_daily_rollups WHERE group_id = 3`, nil, &actualCost))
	require.Equal(t, 1.25, actualCost)
}

func TestSubAdminCommissionMonthlyReportLiveDedupesGrantsAndHonorsExclusionIntervals(t *testing.T) {
	fixture := newSubAdminCommissionVisibilityFixture(t)
	ctx := context.Background()
	_, err := fixture.repo.sql.ExecContext(ctx, `
  INSERT INTO users (id,role,deleted_at) VALUES (2,'sub_admin',NULL),(3,'sub_admin',NOW()),(4,'user',NULL);
  UPDATE sub_admin_commission_grants SET granted_date=DATE '2100-01-02' WHERE group_id=1;
  INSERT INTO groups (id,name,status) VALUES (6,'late-grant','active'),(7,'future-grant','active'),(8,'revoked','active'),(9,'deleted-admin','active'),(10,'wrong-role','active');
  INSERT INTO sub_admin_commission_grants (group_id,sub_admin_user_id,granted_date,enabled) VALUES
   (1,2,DATE '2100-01-02',TRUE),
   (6,1,DATE '2100-01-02',TRUE),(7,1,DATE '2100-01-03',TRUE),
   (8,1,DATE '2100-01-01',FALSE),(9,3,DATE '2100-01-01',TRUE),(10,4,DATE '2100-01-01',TRUE);
  INSERT INTO sub_admin_commission_exclusions (user_id,started_at,ended_at) VALUES
   (42,TIMESTAMPTZ '2100-01-02 10:00:00+00',TIMESTAMPTZ '2100-01-02 11:00:00+00'),
   (42,TIMESTAMPTZ '2100-01-02 12:00:00+00',NULL);
  INSERT INTO usage_logs (id,user_id,group_id,actual_cost,created_at) VALUES
   (601,42,1,1,TIMESTAMPTZ '2100-01-02 09:59:59+00'),
   (602,42,1,2,TIMESTAMPTZ '2100-01-02 10:00:00+00'),
   (603,42,1,3,TIMESTAMPTZ '2100-01-02 11:00:00+00'),
   (604,42,1,4,TIMESTAMPTZ '2100-01-02 12:00:00+00'),
   (605,43,1,5,TIMESTAMPTZ '2100-01-02 12:00:00+00'),
   (606,43,1,900,TIMESTAMPTZ '2100-01-01 12:00:00+00'),
   (607,43,6,100,TIMESTAMPTZ '2100-01-01 12:00:00+00'),
   (608,43,6,7,TIMESTAMPTZ '2100-01-02 12:00:00+00'),
   (609,43,7,100,TIMESTAMPTZ '2100-01-02 12:00:00+00'),
   (610,43,8,100,TIMESTAMPTZ '2100-01-02 12:00:00+00'),
   (611,43,9,100,TIMESTAMPTZ '2100-01-02 12:00:00+00'),
   (612,43,10,100,TIMESTAMPTZ '2100-01-02 12:00:00+00'),
   (613,43,1,100,TIMESTAMPTZ '2100-01-03 00:00:00+00');
  INSERT INTO usage_group_daily_rollups(group_id,bucket_date,actual_cost) VALUES
   (1,DATE '2100-01-01',500),(1,DATE '2100-01-02',500);
 `)
	require.NoError(t, err)
	now := time.Date(2100, 1, 2, 13, 0, 0, 0, time.UTC)
	report, err := fixture.repo.ListMonthlyReport(ctx, 1, "2100-01", 0.2, now)
	require.NoError(t, err)
	byID := monthlyCommissionGroupsByID(report.Groups)
	require.Len(t, byID, 4)
	require.Contains(t, byID, int64(1))
	require.Contains(t, byID, int64(3))
	require.Contains(t, byID, int64(5))
	require.Contains(t, byID, int64(6))
	require.Len(t, byID[1].Days, 1)
	require.Equal(t, 9.0, byID[1].Days[0].ActualCost)
	require.InDelta(t, 1.8, byID[1].Days[0].CommissionAmount, 1e-9)
	require.Len(t, byID[6].Days, 1)
	require.Equal(t, 7.0, byID[6].Days[0].ActualCost)
	requireMonthlyReportParity(t, report)
	require.Equal(t, 19.75, report.Days[len(report.Days)-1].ActualCost)
	// The selected sub-admin ID does not change the existing shared global grants.
	other, err := fixture.repo.ListMonthlyReport(ctx, 2, "2100-01", 0.2, now)
	require.NoError(t, err)
	require.Equal(t, report, other)
}

func TestSubAdminCommissionMonthlyReportHistoricalUsesNetRollupsAndKeepsZeroCostUsage(t *testing.T) {
	fixture := newSubAdminCommissionVisibilityFixture(t)
	ctx := context.Background()
	_, err := fixture.repo.sql.ExecContext(ctx, `
  UPDATE usage_group_daily_rollups SET commission_excluded_cost=0.5 WHERE group_id=3;
  UPDATE usage_group_daily_rollups SET commission_excluded_cost=actual_cost WHERE group_id=5;
  INSERT INTO usage_logs(id,user_id,group_id,actual_cost,created_at) VALUES
   (601,42,3,900,TIMESTAMPTZ '2000-01-02 12:00:00+00');
 `)
	require.NoError(t, err)
	report, err := fixture.repo.ListMonthlyReport(ctx, 1, "2000-01", 0.2, time.Date(2000, 1, 5, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	byID := monthlyCommissionGroupsByID(report.Groups)
	require.Len(t, byID, 3)
	require.Equal(t, 0.75, byID[3].Days[1].ActualCost)
	require.Zero(t, byID[5].Days[1].ActualCost)
	requireMonthlyReportParity(t, report)
	require.Equal(t, 0.75, report.Days[1].ActualCost)
	var billedCost float64
	require.NoError(t, scanSingleRow(ctx, fixture.repo.sql, `SELECT actual_cost FROM usage_group_daily_rollups WHERE group_id=3`, nil, &billedCost))
	require.Equal(t, 1.25, billedCost)
}

func TestSubAdminCommissionMonthlyReportTimezoneMonthBoundary(t *testing.T) {
	fixture := newSubAdminCommissionVisibilityFixture(t)
	ctx := context.Background()
	_, err := fixture.repo.sql.ExecContext(ctx, `
  UPDATE sub_admin_commission_grants SET enabled=FALSE WHERE group_id<>1;
  UPDATE sub_admin_commission_grants SET granted_date=DATE '2100-01-01' WHERE group_id=1;
  INSERT INTO usage_logs(id,user_id,group_id,actual_cost,created_at) VALUES
   (601,42,1,1,TIMESTAMPTZ '2099-12-31 23:59:59+00'),
   (602,42,1,2,TIMESTAMPTZ '2100-01-01 00:00:00+00'),
   (603,42,1,3,TIMESTAMPTZ '2100-01-02 00:00:00+00');
 `)
	require.NoError(t, err)
	report, err := fixture.repo.ListMonthlyReport(ctx, 1, "", 0.2, time.Date(2100, 1, 1, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, report.Groups, 1)
	require.Len(t, report.Days, 1)
	require.Equal(t, "2100-01-01", report.Days[0].Date)
	require.Equal(t, 2.0, report.Days[0].ActualCost)
	requireMonthlyReportParity(t, report)
}

func monthlyCommissionGroupsByID(groups []service.SubAdminCommissionMonthlyGroup) map[int64]service.SubAdminCommissionMonthlyGroup {
	result := make(map[int64]service.SubAdminCommissionMonthlyGroup, len(groups))
	for _, group := range groups {
		result[group.GroupID] = group
	}
	return result
}
func requireMonthlyReportParity(t *testing.T, report service.SubAdminCommissionMonthlyReport) {
	t.Helper()
	for _, day := range report.Days {
		var cost, commission float64
		for _, group := range report.Groups {
			for _, entry := range group.Days {
				if entry.Date == day.Date {
					cost += entry.ActualCost
					commission += entry.CommissionAmount
				}
			}
		}
		require.Equal(t, cost, day.ActualCost)
		require.Equal(t, commission, day.CommissionAmount)
	}
}

func TestSubAdminCommissionMonthlyReportUsesConfiguredTimezoneAcrossUTCMonthBoundary(t *testing.T) {
	require.NoError(t, timezone.Init("Asia/Shanghai"))
	t.Cleanup(func() { require.NoError(t, timezone.Init("UTC")) })
	fixture := newSubAdminCommissionVisibilityFixture(t)
	ctx := context.Background()
	_, err := fixture.repo.sql.ExecContext(ctx, `
  UPDATE sub_admin_commission_grants SET enabled=FALSE WHERE group_id<>1;
  UPDATE sub_admin_commission_grants SET granted_date=DATE '2100-01-01' WHERE group_id=1;
  INSERT INTO usage_logs(id,user_id,group_id,actual_cost,created_at) VALUES
   (601,42,1,100,TIMESTAMPTZ '2099-12-31 15:59:59+00'),
   (602,42,1,2,TIMESTAMPTZ '2099-12-31 16:00:00+00'),
   (603,42,1,3,TIMESTAMPTZ '2100-01-01 15:59:59+00'),
   (604,42,1,100,TIMESTAMPTZ '2100-01-01 16:00:00+00');
 `)
	require.NoError(t, err)
	report, err := fixture.repo.ListMonthlyReport(ctx, 1, "", 0.2, time.Date(2099, 12, 31, 18, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, report.Groups, 1)
	require.Len(t, report.Days, 1)
	require.Equal(t, "2100-01-01", report.Days[0].Date)
	require.Equal(t, 5.0, report.Days[0].ActualCost)
	requireMonthlyReportParity(t, report)
}
