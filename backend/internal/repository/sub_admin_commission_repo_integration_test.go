//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

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
			actual_cost NUMERIC NOT NULL DEFAULT 0
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
