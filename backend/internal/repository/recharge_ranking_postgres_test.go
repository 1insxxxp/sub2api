package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRechargeRankingPostgresCountsOnlyRechargesAndReturnsInviter(t *testing.T) {
	dsn := os.Getenv("RECHARGE_RANKING_TEST_DSN")
	if dsn == "" {
		t.Skip("RECHARGE_RANKING_TEST_DSN is required for PostgreSQL integration")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
CREATE TEMP TABLE users (id bigint PRIMARY KEY, email text, username text, balance numeric);
CREATE TEMP TABLE user_affiliates (user_id bigint PRIMARY KEY, inviter_id bigint);
CREATE TEMP TABLE redeem_codes (
 id bigint PRIMARY KEY, used_by bigint, value numeric, used_at timestamptz, created_at timestamptz,
 code text, type text, source text, status text);
CREATE TEMP TABLE payment_orders (recharge_code text, order_type text, status text);
CREATE TEMP TABLE affiliate_reward_claims (redeem_code_id bigint);
CREATE TEMP TABLE user_affiliate_ledger (user_id bigint, amount numeric, created_at timestamptz, action text);
CREATE TEMP TABLE promo_code_usages (user_id bigint, bonus_amount numeric, used_at timestamptz);
CREATE TEMP TABLE lottery_draws (user_id bigint, balance_amount numeric, created_at timestamptz, prize_type text);
INSERT INTO users VALUES (1, 'invitee@example.com', 'invitee', 100), (2, 'reward@example.com', 'reward', 999), (3, 'unbound@example.com', 'unbound', 15), (9, 'inviter@example.com', 'inviter', 200);
INSERT INTO user_affiliates VALUES (1,9), (3,NULL);
INSERT INTO redeem_codes (id, used_by, value, used_at, created_at, code, type, source, status) VALUES
 (1,1,10,'2026-09-15 00:00+08','2026-09-14 00:00+08','PAY-1','balance',NULL,'used'),
 (2,1,20,'2026-09-15 01:00+08','2026-09-14 00:00+08','CODE-2','balance',NULL,'used'),
 (3,1,30,'2026-09-15 02:00+08','2026-09-14 00:00+08','ADMIN-3','admin_balance',NULL,'used'),
 (4,1,4,'2026-09-15 03:00+08','2026-09-14 00:00+08','CHECKIN-4','checkin_reward',NULL,'used'),
 (5,1,5,'2026-09-15 04:00+08','2026-09-14 00:00+08','REFUND-5','empty_response',NULL,'used'),
 (6,1,6,'2026-09-15 05:00+08','2026-09-14 00:00+08','TRANSFER-6','balance','user_balance_transfer','used'),
 (7,1,7,'2026-09-15 06:00+08','2026-09-14 00:00+08','AFFILIATE-7','balance',NULL,'used'),
 (8,1,8,'2026-09-15 07:00+08','2026-09-14 00:00+08','AFFILIATE-8','affiliate_balance',NULL,'used'),
 (9,2,999,'2026-09-15 08:00+08','2026-09-14 00:00+08','CHECKIN-9','checkin_reward',NULL,'used'),
 (10,1,100,'2026-09-16 00:00+08','2026-09-14 00:00+08','OUTSIDE-10','balance',NULL,'used'),
 (11,1,-10,'2026-09-15 10:00+08','2026-09-14 00:00+08','DEBIT-11','admin_balance',NULL,'used'),
 (12,1,10,'2026-09-15 11:00+08','2026-09-14 00:00+08','UNUSED-12','balance',NULL,'unused'),
 (13,3,15,'2026-09-15 12:00+08','2026-09-14 00:00+08','UNBOUND-13','balance',NULL,'used');
INSERT INTO payment_orders VALUES ('PAY-1','balance','completed');
INSERT INTO affiliate_reward_claims VALUES (7);
INSERT INTO user_affiliate_ledger VALUES (1,9,'2026-09-15 09:00+08','transfer');
INSERT INTO promo_code_usages VALUES (1,10,'2026-09-15 10:00+08');
INSERT INTO lottery_draws VALUES (1,11,'2026-09-15 11:00+08','balance');
`)
	require.NoError(t, err)
	start, err := time.Parse(time.RFC3339, "2026-09-15T00:00:00+08:00")
	require.NoError(t, err)
	repo := &usageLogRepository{sql: db}
	query := func(userID int64, source, sortOrder string) *service.RechargeRankingResponse {
		t.Helper()
		result, err := repo.GetRechargeRanking(context.Background(), start, start.AddDate(0, 0, 1), userID, source, "recharge_count", sortOrder, 0, 20)
		require.NoError(t, err)
		return result
	}
	identity := func(item service.RechargeRankingItem) map[string]any {
		t.Helper()
		b, err := json.Marshal(item)
		require.NoError(t, err)
		var fields map[string]any
		require.NoError(t, json.Unmarshal(b, &fields))
		return fields
	}

	t.Run("counts and summary use only the three recharge sources", func(t *testing.T) {
		result := query(0, "", "desc")
		require.Len(t, result.Items, 3)
		require.Equal(t, int64(1), result.Items[0].UserID)
		require.Equal(t, int64(3), result.Items[0].RechargeCount)
		require.Equal(t, int64(4), result.Summary.RechargeCount)
		require.InDelta(t, 120.0, result.Items[0].TotalAmount, 1e-9)
		require.InDelta(t, 1134.0, result.Summary.TotalAmount, 1e-9)
		require.Equal(t, int64(0), result.Items[2].RechargeCount)
	})
	t.Run("current inviter and users with no relationship are returned", func(t *testing.T) {
		bound := identity(query(1, "", "desc").Items[0])
		require.Equal(t, float64(9), bound["inviter_id"])
		require.Equal(t, "inviter@example.com", bound["inviter_email"])
		require.Equal(t, "inviter", bound["inviter_username"])
		for _, id := range []int64{2, 3} {
			unbound := identity(query(id, "", "desc").Items[0])
			require.Contains(t, unbound, "inviter_id")
			require.Nil(t, unbound["inviter_id"])
		}
	})
	t.Run("source filters preserve amounts without counting rewards or rebates", func(t *testing.T) {
		for _, source := range []string{"online", "redeem", "admin"} {
			result := query(1, source, "desc")
			require.Equal(t, int64(1), result.Items[0].RechargeCount, source)
			require.Equal(t, int64(1), result.Summary.RechargeCount, source)
		}
		for _, source := range []string{"affiliate", "reward", "refund", "other"} {
			result := query(1, source, "desc")
			require.Positive(t, result.Items[0].TotalAmount, source)
			require.Zero(t, result.Items[0].RechargeCount, source)
			require.Zero(t, result.Summary.RechargeCount, source)
		}
	})
	t.Run("count sorting and pagination use the filtered recharge count", func(t *testing.T) {
		result := query(0, "", "asc")
		require.Equal(t, int64(2), result.Items[0].UserID)
		page, err := repo.GetRechargeRanking(context.Background(), start, start.AddDate(0, 0, 1), 0, "", "recharge_count", "desc", 1, 1)
		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		require.Equal(t, int64(3), page.Items[0].UserID)
		require.Equal(t, int64(4), page.Summary.RechargeCount)
	})
	t.Run("balance sorting uses the ranked user rather than their inviter", func(t *testing.T) {
		result, err := repo.GetRechargeRanking(context.Background(), start, start.AddDate(0, 0, 1), 0, "", "balance", "desc", 0, 20)
		require.NoError(t, err)
		require.Equal(t, []int64{2, 1, 3}, []int64{result.Items[0].UserID, result.Items[1].UserID, result.Items[2].UserID})
	})
}
