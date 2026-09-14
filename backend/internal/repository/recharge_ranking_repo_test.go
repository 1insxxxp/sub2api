package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGetRechargeRankingAggregatesSourcesAndGuardsDoubleCounting(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)
	when := start.Add(2 * time.Hour)
	// Keep this assertion focused on the safety-critical SQL predicates; the
	// rest of the query is exercised by scanning a complete result row below.
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("LOWER(po.status) IN ('paid','completed','success','succeeded','finished')") + `.*` + regexp.QuoteMeta("LEFT JOIN affiliate_reward_claims arc ON arc.redeem_code_id = rc.id") + `.*` + regexp.QuoteMeta("rc.status = 'used'") + `.*` + regexp.QuoteMeta("rc.type IN ('balance', 'admin_balance', 'checkin_reward', 'empty_response', 'affiliate_balance', 'promo_reward')")).
		WillReturnRows(mockRowsForRechargeRanking(1, when))
	result, err := repo.GetRechargeRanking(context.Background(), start, end, 0, "", "total_amount", "desc", 0, 20)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	require.Equal(t, int64(7), item.RechargeCount)
	require.InDelta(t, item.TotalAmount, item.OnlineAmount+item.RedeemAmount+item.AffiliateAmount+item.AdminAmount+item.RewardAmount+item.RefundAmount+item.OtherAmount, 1e-9)
	require.Equal(t, int64(1), result.Total)
	require.Equal(t, int64(1), result.Summary.RechargeUsers)
	require.NoError(t, mock.ExpectationsWereMet())
}

func mockRowsForRechargeRanking(userID int64, when time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"user_id", "email", "username", "balance", "total_amount", "online_amount", "redeem_amount",
		"affiliate_amount", "admin_amount", "reward_amount", "refund_amount", "other_amount", "recharge_count",
		"last_recharged_at", "total_users", "summary_total_amount", "summary_online_amount", "summary_redeem_amount",
		"summary_affiliate_amount", "summary_admin_amount", "summary_reward_amount", "summary_refund_amount", "summary_other_amount",
		"summary_recharge_count", "summary_recharge_users",
	}).AddRow(userID, "u@example.com", "u", 100.0, 70.0, 10.0, 20.0, 5.0, 6.0, 7.0, 8.0, 14.0, int64(7), when,
		int64(1), 70.0, 10.0, 20.0, 5.0, 6.0, 7.0, 8.0, 14.0, int64(7), int64(1))
}

var _ service.RechargeRankingRepository = (*usageLogRepository)(nil)
