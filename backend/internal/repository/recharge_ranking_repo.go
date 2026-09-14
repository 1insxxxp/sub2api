package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// GetRechargeRanking aggregates positive wallet credits. Payment orders are
// joined through their fulfillment redeem code, so an online payment is never
// counted again as a normal redeem code.
func (r *usageLogRepository) GetRechargeRanking(ctx context.Context, startTime, endTime time.Time, userID int64, source, sortBy, sortOrder string, offset, limit int) (*service.RechargeRankingResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	source = strings.ToLower(strings.TrimSpace(source))
	sortOrder = strings.ToLower(strings.TrimSpace(sortOrder))
	if sortOrder != "asc" {
		sortOrder = "desc"
	}
	orderColumns := map[string]string{
		"total_amount":      "total_amount",
		"online_amount":     "online_amount",
		"redeem_amount":     "redeem_amount",
		"affiliate_amount":  "affiliate_amount",
		"admin_amount":      "admin_amount",
		"reward_amount":     "reward_amount",
		"refund_amount":     "refund_amount",
		"other_amount":      "other_amount",
		"recharge_count":    "recharge_count",
		"last_recharged_at": "last_recharged_at",
		"balance":           "balance",
	}
	orderBy := orderColumns[sortBy]
	if orderBy == "" {
		orderBy = "total_amount"
	}

	query := fmt.Sprintf(`
WITH events AS (
    SELECT rc.used_by AS user_id, rc.value::double precision AS amount,
           COALESCE(rc.used_at, rc.created_at) AS occurred_at,
           CASE
             WHEN EXISTS (SELECT 1 FROM payment_orders po
                          WHERE po.recharge_code = rc.code
                            AND po.order_type = 'balance'
                            AND LOWER(po.status) IN ('paid','completed','success','succeeded','finished')) THEN 'online'
             WHEN rc.type = 'affiliate_balance' OR arc.redeem_code_id IS NOT NULL THEN 'affiliate'
             WHEN rc.type = 'admin_balance' THEN 'admin'
             WHEN rc.type IN ('checkin_reward', 'promo_reward') THEN 'reward'
             WHEN rc.type = 'empty_response' THEN 'refund'
             WHEN rc.type = 'balance' AND COALESCE(rc.source, '') <> 'user_balance_transfer' THEN 'redeem'
             ELSE 'other'
           END AS source
    FROM redeem_codes rc
    LEFT JOIN affiliate_reward_claims arc ON arc.redeem_code_id = rc.id
    WHERE rc.used_by IS NOT NULL AND rc.value > 0
      AND rc.status = 'used'
      AND rc.type IN ('balance', 'admin_balance', 'checkin_reward', 'empty_response', 'affiliate_balance', 'promo_reward')
      AND COALESCE(rc.used_at, rc.created_at) >= $1
      AND COALESCE(rc.used_at, rc.created_at) < $2
    UNION ALL
    SELECT ual.user_id, ual.amount::double precision, ual.created_at, 'affiliate'
    FROM user_affiliate_ledger ual
    WHERE ual.action = 'transfer' AND ual.amount > 0
      AND ual.created_at >= $1 AND ual.created_at < $2
    UNION ALL
    SELECT pcu.user_id, pcu.bonus_amount::double precision, pcu.used_at, 'reward'
    FROM promo_code_usages pcu
    WHERE pcu.bonus_amount > 0 AND pcu.used_at >= $1 AND pcu.used_at < $2
    UNION ALL
    SELECT ld.user_id, ld.balance_amount::double precision, ld.created_at, 'reward'
    FROM lottery_draws ld
    WHERE ld.prize_type = 'balance' AND ld.balance_amount > 0
      AND ld.created_at >= $1 AND ld.created_at < $2
), filtered AS (
    SELECT * FROM events
    WHERE ($3 = '' OR source = $3)
      AND ($4 = 0 OR user_id = $4)
), grouped AS (
    SELECT user_id,
       SUM(amount) AS total_amount,
       SUM(amount) FILTER (WHERE source = 'online') AS online_amount,
       SUM(amount) FILTER (WHERE source = 'redeem') AS redeem_amount,
       SUM(amount) FILTER (WHERE source = 'affiliate') AS affiliate_amount,
       SUM(amount) FILTER (WHERE source = 'admin') AS admin_amount,
       SUM(amount) FILTER (WHERE source = 'reward') AS reward_amount,
       SUM(amount) FILTER (WHERE source = 'refund') AS refund_amount,
       SUM(amount) FILTER (WHERE source = 'other') AS other_amount,
       COUNT(*) AS recharge_count, MAX(occurred_at) AS last_recharged_at
    FROM filtered GROUP BY user_id
), ranked AS (
    SELECT g.*, COUNT(*) OVER () AS total_users,
       COALESCE(SUM(g.total_amount) OVER (), 0) AS summary_total_amount,
       COALESCE(SUM(g.online_amount) OVER (), 0) AS summary_online_amount,
       COALESCE(SUM(g.redeem_amount) OVER (), 0) AS summary_redeem_amount,
       COALESCE(SUM(g.affiliate_amount) OVER (), 0) AS summary_affiliate_amount,
       COALESCE(SUM(g.admin_amount) OVER (), 0) AS summary_admin_amount,
       COALESCE(SUM(g.reward_amount) OVER (), 0) AS summary_reward_amount,
       COALESCE(SUM(g.refund_amount) OVER (), 0) AS summary_refund_amount,
       COALESCE(SUM(g.other_amount) OVER (), 0) AS summary_other_amount,
       COALESCE(SUM(g.recharge_count) OVER (), 0) AS summary_recharge_count,
       COUNT(*) OVER () AS summary_recharge_users
    FROM grouped g
)
SELECT r.user_id, COALESCE(u.email,''), COALESCE(u.username,''), COALESCE(u.balance,0),
       r.total_amount, COALESCE(r.online_amount,0), COALESCE(r.redeem_amount,0),
       COALESCE(r.affiliate_amount,0), COALESCE(r.admin_amount,0), COALESCE(r.reward_amount,0),
       COALESCE(r.refund_amount,0), COALESCE(r.other_amount,0), r.recharge_count,
       r.last_recharged_at, r.total_users, r.summary_total_amount, r.summary_online_amount,
       r.summary_redeem_amount, r.summary_affiliate_amount, r.summary_admin_amount,
       r.summary_reward_amount, r.summary_refund_amount, r.summary_other_amount,
       r.summary_recharge_count, r.summary_recharge_users
FROM ranked r JOIN users u ON u.id = r.user_id
ORDER BY %s %s, r.user_id ASC
LIMIT $5 OFFSET $6`, orderBy, sortOrder)

	rows, err := r.sql.QueryContext(ctx, query, startTime, endTime, source, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := &service.RechargeRankingResponse{Items: make([]service.RechargeRankingItem, 0)}
	for rows.Next() {
		var item service.RechargeRankingItem
		var last time.Time
		var totalUsers int64
		var summary service.RechargeRankingSummary
		var summaryUsers int64
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &item.Balance,
			&item.TotalAmount, &item.OnlineAmount, &item.RedeemAmount, &item.AffiliateAmount,
			&item.AdminAmount, &item.RewardAmount, &item.RefundAmount, &item.OtherAmount,
			&item.RechargeCount, &last, &totalUsers, &summary.TotalAmount, &summary.OnlineAmount,
			&summary.RedeemAmount, &summary.AffiliateAmount, &summary.AdminAmount, &summary.RewardAmount,
			&summary.RefundAmount, &summary.OtherAmount, &summary.RechargeCount, &summaryUsers); err != nil {
			return nil, err
		}
		item.LastRechargedAt = &last
		result.Items = append(result.Items, item)
		result.Total = totalUsers
		result.Summary = summary
		result.Summary.RechargeUsers = summaryUsers
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

var _ service.RechargeRankingRepository = (*usageLogRepository)(nil)
