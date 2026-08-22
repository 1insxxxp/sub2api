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

func newSubAdminCommissionRepoMock(t *testing.T) (serviceRepo, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewSubAdminCommissionRepository(nil, db), mock
}

type serviceRepo interface {
	ListCalendar(ctx context.Context, subAdminID int64, month string, commissionRate float64, now time.Time) ([]service.SubAdminCommissionCalendarDay, error)
	ListDayGroups(ctx context.Context, subAdminID int64, date string, commissionRate float64) ([]service.SubAdminCommissionDayGroup, error)
	ListDayGroupLogs(ctx context.Context, subAdminID, groupID int64, date string, params pagination.PaginationParams) ([]service.SubAdminCommissionUsageLog, pagination.PaginationResult, error)
}

func TestSubAdminCommissionRepositoryListCalendarUsesEnabledCurrentSubAdminGrants(t *testing.T) {
	repo, mock := newSubAdminCommissionRepoMock(t)
	now := time.Date(2026, 8, 22, 9, 30, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)SELECT .*group_id.*granted_date.*FROM sub_admin_commission_grants.*sub_admin_user_id = \$1.*enabled = TRUE`).
		WithArgs(int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "granted_date"}).
			AddRow(int64(9), "2026-08-20"))
	mock.ExpectQuery(`(?s)FROM usage_group_daily_rollups r.*JOIN sub_admin_commission_grants cg.*cg.sub_admin_user_id = \$1.*cg.enabled = TRUE`).
		WillReturnRows(sqlmock.NewRows([]string{"date", "actual_cost"}).
			AddRow("2026-08-21", 12.5))
	mock.ExpectQuery(`(?s)FROM usage_logs ul.*JOIN sub_admin_commission_grants cg.*cg.sub_admin_user_id = \$1.*cg.enabled = TRUE`).
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

func TestSubAdminCommissionRepositoryListDayGroupsHidesDatesBeforeGrantedDate(t *testing.T) {
	repo, mock := newSubAdminCommissionRepoMock(t)

	mock.ExpectQuery(regexp.QuoteMeta("$3::date >= cg.granted_date")).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "group_name", "requests", "total_tokens", "actual_cost"}))

	groups, err := repo.ListDayGroups(context.Background(), 17, "2000-01-01", 0.2)

	require.NoError(t, err)
	require.Empty(t, groups)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSubAdminCommissionRepositoryListDayGroupsUsesRollupsForHistoricalDates(t *testing.T) {
	repo, mock := newSubAdminCommissionRepoMock(t)

	mock.ExpectQuery(`(?s)FROM sub_admin_commission_grants cg.*LEFT JOIN usage_group_daily_rollups r.*r\.bucket_date = \$2::date.*\$3::date >= cg\.granted_date`).
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

func TestSubAdminCommissionRepositoryListDayGroupLogsPaginatesAndJoinsAssociations(t *testing.T) {
	repo, mock := newSubAdminCommissionRepoMock(t)
	createdAt := time.Date(2026, 8, 22, 8, 45, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)SELECT EXISTS.*FROM sub_admin_commission_grants.*sub_admin_user_id = \$1.*group_id = \$2.*enabled = TRUE`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM usage_logs ul.*JOIN sub_admin_commission_grants cg.*JOIN users u.*JOIN api_keys ak.*JOIN groups g`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`(?s)SELECT .*ul.id.*u.email.*ak.name.*g.name.*FROM usage_logs ul.*JOIN sub_admin_commission_grants cg.*JOIN users u.*JOIN api_keys ak.*JOIN groups g.*ORDER BY ul.created_at DESC.*LIMIT \$5 OFFSET \$6`).
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
