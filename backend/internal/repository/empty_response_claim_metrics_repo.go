package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *emptyResponseClaimRepository) GetEmptyResponseClaimMetrics(ctx context.Context, start, end time.Time) (*service.EmptyResponseClaimMetrics, error) {
	metrics := &service.EmptyResponseClaimMetrics{}
	if err := scanSingleRow(ctx, r.sql, `
		SELECT
			(SELECT COUNT(*) FROM usage_logs WHERE actual_cost > 0 AND created_at >= $1 AND created_at < $2),
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'compensated'),
			COUNT(*) FILTER (WHERE status = 'manual_review'),
			COUNT(*) FILTER (WHERE status = 'rejected'),
			COALESCE(SUM(balance_refund + subscription_refund), 0)::float8
		FROM empty_response_claims
		WHERE created_at >= $1 AND created_at < $2
	`, []any{start, end},
		&metrics.TotalChargedRequests, &metrics.TotalClaims, &metrics.CompensatedClaims,
		&metrics.ManualReviewClaims, &metrics.RejectedClaims, &metrics.TotalRefundAmount,
	); err != nil {
		return nil, fmt.Errorf("query empty response claim metrics: %w", err)
	}

	var err error
	metrics.ByGroup, err = r.queryEmptyResponseClaimMetricDimensions(ctx, `
		WITH claim_metrics AS (
			SELECT COALESCE(erc.group_id, 0) AS id, COALESCE(g.name, '未分组') AS name,
				COUNT(*) AS claims, COALESCE(SUM(erc.balance_refund + erc.subscription_refund), 0)::float8 AS refund_amount
			FROM empty_response_claims erc
			LEFT JOIN groups g ON g.id = erc.group_id
			WHERE erc.created_at >= $1 AND erc.created_at < $2
			GROUP BY erc.group_id, g.name
		), charged AS (
			SELECT COALESCE(group_id, 0) AS id, COUNT(*) AS charged_requests
			FROM usage_logs WHERE actual_cost > 0 AND created_at >= $1 AND created_at < $2
			GROUP BY group_id
		)
		SELECT cm.id, cm.name, cm.claims, cm.refund_amount, COALESCE(ch.charged_requests, 0)
		FROM claim_metrics cm LEFT JOIN charged ch ON ch.id = cm.id
		ORDER BY cm.claims DESC, cm.refund_amount DESC LIMIT 12
	`, start, end)
	if err != nil {
		return nil, err
	}
	metrics.ByAccount, err = r.queryEmptyResponseClaimMetricDimensions(ctx, `
		WITH claim_metrics AS (
			SELECT erc.account_id AS id, COALESCE(a.name, '') AS name,
				COUNT(*) AS claims, COALESCE(SUM(erc.balance_refund + erc.subscription_refund), 0)::float8 AS refund_amount
			FROM empty_response_claims erc
			LEFT JOIN accounts a ON a.id = erc.account_id
			WHERE erc.created_at >= $1 AND erc.created_at < $2
			GROUP BY erc.account_id, a.name
		), charged AS (
			SELECT account_id AS id, COUNT(*) AS charged_requests
			FROM usage_logs WHERE actual_cost > 0 AND created_at >= $1 AND created_at < $2
			GROUP BY account_id
		)
		SELECT cm.id, cm.name, cm.claims, cm.refund_amount, COALESCE(ch.charged_requests, 0)
		FROM claim_metrics cm LEFT JOIN charged ch ON ch.id = cm.id
		ORDER BY cm.claims DESC, cm.refund_amount DESC LIMIT 12
	`, start, end)
	if err != nil {
		return nil, err
	}
	metrics.ByModel, err = r.queryEmptyResponseClaimMetricDimensions(ctx, `
		WITH claim_metrics AS (
			SELECT COALESCE(NULLIF(TRIM(ul.requested_model), ''), ul.model) AS name,
				COUNT(*) AS claims, COALESCE(SUM(erc.balance_refund + erc.subscription_refund), 0)::float8 AS refund_amount
			FROM empty_response_claims erc JOIN usage_logs ul ON ul.id = erc.usage_log_id
			WHERE erc.created_at >= $1 AND erc.created_at < $2
			GROUP BY COALESCE(NULLIF(TRIM(ul.requested_model), ''), ul.model)
		), charged AS (
			SELECT COALESCE(NULLIF(TRIM(requested_model), ''), model) AS name, COUNT(*) AS charged_requests
			FROM usage_logs WHERE actual_cost > 0 AND created_at >= $1 AND created_at < $2
			GROUP BY COALESCE(NULLIF(TRIM(requested_model), ''), model)
		)
		SELECT 0, cm.name, cm.claims, cm.refund_amount, COALESCE(ch.charged_requests, 0)
		FROM claim_metrics cm LEFT JOIN charged ch ON ch.name = cm.name
		ORDER BY cm.claims DESC, cm.refund_amount DESC LIMIT 12
	`, start, end)
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

func (r *emptyResponseClaimRepository) queryEmptyResponseClaimMetricDimensions(ctx context.Context, query string, start, end time.Time) (result []service.EmptyResponseClaimMetricDimension, err error) {
	rows, err := r.sql.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("query empty response claim metric dimensions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	result = make([]service.EmptyResponseClaimMetricDimension, 0)
	for rows.Next() {
		var item service.EmptyResponseClaimMetricDimension
		if err := rows.Scan(&item.ID, &item.Name, &item.Claims, &item.RefundAmount, &item.ChargedRequests); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
