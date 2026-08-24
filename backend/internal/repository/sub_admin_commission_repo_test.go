package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newSubAdminCommissionRepoMock(t *testing.T) (subAdminCommissionRepoTestInterface, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewSubAdminCommissionRepository(nil, db), mock
}

type subAdminCommissionRepoTestInterface interface {
	ListAllGrants(ctx context.Context) ([]service.SubAdminCommissionGrant, error)
	ReplaceGrants(ctx context.Context, input service.ReplaceSubAdminCommissionGrantsInput, grantedDate string) ([]service.SubAdminCommissionGrant, error)
	ListCalendar(ctx context.Context, subAdminID int64, month string, commissionRate float64, now time.Time) ([]service.SubAdminCommissionCalendarDay, error)
	ListDayGroups(ctx context.Context, subAdminID int64, date string, commissionRate float64) ([]service.SubAdminCommissionDayGroup, error)
	ListDayGroupLogs(ctx context.Context, subAdminID, groupID int64, date string, params pagination.PaginationParams) ([]service.SubAdminCommissionUsageLog, pagination.PaginationResult, error)
}

func TestSubAdminCommissionRepositoryListAllGrantsDedupesEnabledGroups(t *testing.T) {
	repo, mock := newSubAdminCommissionRepoMock(t)

	mock.ExpectQuery(`(?s)WITH global_grants AS .*FROM sub_admin_commission_grants.*GROUP BY cg\.group_id.*ORDER BY g\.name ASC, g\.id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "sub_admin_user_id", "sub_admin_email", "group_id", "group_name",
			"granted_date", "enabled", "created_at", "updated_at",
		}).AddRow(
			int64(0), int64(0), "", int64(3), "Claude 特价", "2026-08-01", true,
			time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
		).AddRow(
			int64(0), int64(0), "", int64(4), "Gemini 池", "2026-08-03", true,
			time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC),
		))

	grants, err := repo.ListAllGrants(context.Background())

	require.NoError(t, err)
	require.Len(t, grants, 2)
	require.Equal(t, int64(3), grants[0].GroupID)
	require.Equal(t, int64(4), grants[1].GroupID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSubAdminCommissionRepositoryReplaceGrantsSyncsAllSubAdmins(t *testing.T) {
	repo, mock := newSubAdminCommissionRepoMock(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT cg\.group_id, MIN\(cg\.granted_date\)::text AS granted_date.*FROM sub_admin_commission_grants cg.*GROUP BY cg\.group_id`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "granted_date"}).
			AddRow(int64(3), "2026-08-01"))
	mock.ExpectExec(`(?s)UPDATE sub_admin_commission_grants.*WHERE enabled = TRUE AND NOT \(group_id = ANY\(\$1\)\)`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`(?s)SELECT id\s+FROM users\s+WHERE role = 'sub_admin' AND deleted_at IS NULL\s+ORDER BY id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(11)).AddRow(int64(12)))
	for _, expectation := range []struct {
		subAdminID  int64
		groupID     int64
		grantedDate string
	}{
		{11, 3, "2026-08-01"},
		{12, 3, "2026-08-01"},
		{11, 4, "2026-08-22"},
		{12, 4, "2026-08-22"},
	} {
		mock.ExpectQuery(`(?s)UPDATE sub_admin_commission_grants.*WHERE sub_admin_user_id = \$1 AND group_id = \$2 AND enabled = TRUE.*RETURNING id`).
			WithArgs(expectation.subAdminID, expectation.groupID, expectation.grantedDate).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1000 + expectation.subAdminID + expectation.groupID)))
	}
	mock.ExpectCommit()
	mock.ExpectQuery(`(?s)WITH global_grants AS .*FROM sub_admin_commission_grants.*GROUP BY cg\.group_id.*ORDER BY g\.name ASC, g\.id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "sub_admin_user_id", "sub_admin_email", "group_id", "group_name",
			"granted_date", "enabled", "created_at", "updated_at",
		}).AddRow(
			int64(0), int64(0), "", int64(3), "Claude 特价", "2026-08-01", true,
			time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
		).AddRow(
			int64(0), int64(0), "", int64(4), "Gemini 池", "2026-08-22", true,
			time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC),
		))

	grants, err := repo.ReplaceGrants(context.Background(), service.ReplaceSubAdminCommissionGrantsInput{
		GroupIDs:   []int64{4, 3},
		OperatorID: 1,
	}, "2026-08-22")

	require.NoError(t, err)
	require.Len(t, grants, 2)
	require.Equal(t, int64(3), grants[0].GroupID)
	require.Equal(t, "2026-08-01", grants[0].GrantedDate)
	require.Equal(t, int64(4), grants[1].GroupID)
	require.Equal(t, "2026-08-22", grants[1].GrantedDate)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSubAdminCommissionRepositoryListCalendarUsesGlobalGrants(t *testing.T) {
	repo, mock := newSubAdminCommissionRepoMock(t)
	now := time.Date(2026, 8, 22, 9, 30, 0, 0, time.UTC)
	todayStart, err := service.ParseGroupUsageDate(service.GroupUsageDate(now))
	require.NoError(t, err)

	mock.ExpectQuery(`(?s)WITH global_grants AS .*FROM sub_admin_commission_grants.*GROUP BY cg\.group_id.*ORDER BY granted_date ASC, group_id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "granted_date"}).
			AddRow(int64(9), "2026-08-20"))
	mock.ExpectQuery(`(?s)FROM usage_group_daily_rollups r.*JOIN global_grants gg.*GROUP BY r\.bucket_date.*ORDER BY r\.bucket_date ASC`).
		WithArgs("2026-08-01", "2026-08-21").
		WillReturnRows(sqlmock.NewRows([]string{"date", "actual_cost"}).
			AddRow("2026-08-21", 12.5))
	mock.ExpectQuery(`(?s)FROM usage_logs ul.*JOIN global_grants gg.*GROUP BY 1`).
		WithArgs(todayStart.AddDate(0, 0, -1).UTC(), todayStart.AddDate(0, 0, 1).UTC(), service.GroupUsageTimezoneName()).
		WillReturnRows(sqlmock.NewRows([]string{"date", "actual_cost"}))

	days, err := repo.ListCalendar(context.Background(), 17, "2026-08", 0.1, now)

	require.NoError(t, err)
	require.Len(t, days, 3)
	require.Equal(t, "2026-08-20", days[0].Date)
	require.True(t, days[0].Enabled)
	require.Zero(t, days[0].ActualCost)
	require.Equal(t, "2026-08-21", days[1].Date)
	require.Equal(t, 12.5, days[1].ActualCost)
	require.Equal(t, 1.25, days[1].CommissionAmount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSubAdminCommissionRepositoryListCalendarUsesLiveLogsForYesterdayWithGlobalGrants(t *testing.T) {
	repo, mock := newSubAdminCommissionRepoMock(t)
	todayStart, err := service.ParseGroupUsageDate("2026-08-23")
	require.NoError(t, err)
	yesterdayStart, err := service.ParseGroupUsageDate("2026-08-22")
	require.NoError(t, err)
	now := todayStart.Add(10 * time.Hour)

	mock.ExpectQuery(`(?s)WITH global_grants AS .*FROM sub_admin_commission_grants.*GROUP BY cg\.group_id.*ORDER BY granted_date ASC, group_id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "granted_date"}).
			AddRow(int64(9), "2026-08-20"))
	mock.ExpectQuery(`(?s)FROM usage_group_daily_rollups r.*JOIN global_grants gg.*GROUP BY r\.bucket_date.*ORDER BY r\.bucket_date ASC`).
		WithArgs("2026-08-01", "2026-08-22").
		WillReturnRows(sqlmock.NewRows([]string{"date", "actual_cost"}))
	mock.ExpectQuery(`(?s)FROM usage_logs ul.*JOIN global_grants gg.*GROUP BY 1`).
		WithArgs(yesterdayStart.UTC(), todayStart.AddDate(0, 0, 1).UTC(), service.GroupUsageTimezoneName()).
		WillReturnRows(sqlmock.NewRows([]string{"date", "actual_cost"}).
			AddRow("2026-08-22", 9.5))

	days, err := repo.ListCalendar(context.Background(), 17, "2026-08", 0.1, now)

	require.NoError(t, err)
	require.Len(t, days, 4)
	require.Equal(t, "2026-08-22", days[2].Date)
	require.Equal(t, 9.5, days[2].ActualCost)
	require.InDelta(t, 0.95, days[2].CommissionAmount, 0.0000001)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSubAdminCommissionRepositoryListDayGroupsHidesDatesBeforeGrantedDateWithGlobalGrants(t *testing.T) {
	repo, mock := newSubAdminCommissionRepoMock(t)

	mock.ExpectQuery(regexp.QuoteMeta("$2::date >= gg.granted_date")).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "group_name", "requests", "total_tokens", "actual_cost"}))

	groups, err := repo.ListDayGroups(context.Background(), 17, "2000-01-01", 0.2)

	require.NoError(t, err)
	require.Empty(t, groups)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSubAdminCommissionRepositoryListDayGroupsUsesRollupsForHistoricalDatesWithGlobalGrants(t *testing.T) {
	repo, mock := newSubAdminCommissionRepoMock(t)

	mock.ExpectQuery(`(?s)WITH global_grants AS .*LEFT JOIN usage_group_daily_rollups r ON r\.group_id = gg\.group_id AND r\.bucket_date = \$1::date.*\$2::date >= gg\.granted_date`).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "group_name", "requests", "total_tokens", "actual_cost"}).
			AddRow(int64(9), "Claude 特价", int64(0), int64(0), 12.5))

	groups, err := repo.ListDayGroups(context.Background(), 17, "2000-01-02", 0.2)

	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, int64(9), groups[0].GroupID)
	require.Equal(t, "Claude 特价", groups[0].GroupName)
	require.Zero(t, groups[0].Requests)
	require.Zero(t, groups[0].TotalTokens)
	require.Equal(t, 12.5, groups[0].ActualCost)
	require.Equal(t, 2.5, groups[0].CommissionAmount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSubAdminCommissionRepositoryListDayGroupsUsesLiveLogsForYesterdayWithGlobalGrants(t *testing.T) {
	repo, mock := newSubAdminCommissionRepoMock(t)
	todayStart, err := service.ParseGroupUsageDate(service.GroupUsageDate(time.Now()))
	require.NoError(t, err)
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	yesterday := service.GroupUsageDate(yesterdayStart)

	mock.ExpectQuery(`(?s)WITH global_grants AS .*JOIN groups g ON g.id = gg\.group_id.*LEFT JOIN usage_logs ul ON ul\.group_id = gg\.group_id AND ul\.created_at >= \$1.*ul\.created_at < \$2.*\$3::date >= gg\.granted_date`).
		WithArgs(yesterdayStart.UTC(), todayStart.UTC(), yesterday).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "group_name", "requests", "total_tokens", "actual_cost"}).
			AddRow(int64(9), "Claude 特价", int64(3), int64(4200), 12.5))

	groups, err := repo.ListDayGroups(context.Background(), 17, yesterday, 0.2)

	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, int64(9), groups[0].GroupID)
	require.Equal(t, int64(3), groups[0].Requests)
	require.Equal(t, int64(4200), groups[0].TotalTokens)
	require.Equal(t, 12.5, groups[0].ActualCost)
	require.Equal(t, 2.5, groups[0].CommissionAmount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSubAdminCommissionRepositoryListDayGroupLogsPaginatesAndJoinsAssociationsWithGlobalGrants(t *testing.T) {
	repo, mock := newSubAdminCommissionRepoMock(t)
	createdAt := time.Date(2026, 8, 22, 8, 45, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)SELECT EXISTS.*WITH global_grants AS .*FROM sub_admin_commission_grants.*WHERE gg\.group_id = \$1.*\$2::date >= gg\.granted_date`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM usage_logs ul.*JOIN \(.*sub_admin_commission_grants.*\) gg.*JOIN users u.*JOIN api_keys ak.*JOIN groups g`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`(?s)SELECT .*ul.id.*u.email.*ak.name.*g.name.*FROM usage_logs ul.*JOIN \(.*sub_admin_commission_grants.*\) gg.*JOIN users u.*JOIN api_keys ak.*JOIN groups g.*ORDER BY ul.created_at DESC.*LIMIT \$4 OFFSET \$5`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "request_id", "created_at", "user_id", "user_email", "api_key_id", "api_key_name",
			"group_id", "group_name", "model", "requested_model", "input_tokens", "output_tokens",
			"cache_creation_tokens", "cache_read_tokens", "actual_cost",
		}).AddRow(
			int64(1001), "req_abc", createdAt, int64(501), "user@example.com", int64(601), "main key",
			int64(9), "Claude 特价", "claude-sonnet-4", "claude-sonnet-4", 1200, 300, 100, 50, 0.42,
		))

	logs, page, err := repo.ListDayGroupLogs(context.Background(), 17, 9, "2026-08-22", pagination.PaginationParams{Page: 1, PageSize: 20})

	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, int64(1001), logs[0].ID)
	require.Equal(t, "user@example.com", logs[0].UserEmail)
	require.Equal(t, "main key", logs[0].APIKeyName)
	require.Equal(t, "Claude 特价", logs[0].GroupName)
	require.NotNil(t, logs[0].RequestedModel)
	require.Equal(t, int64(1), page.Total)
	require.Equal(t, 20, page.PageSize)
	require.NoError(t, mock.ExpectationsWereMet())
}
