package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

func (s *CheckinService) scheduleAccountRiskMaintenance(wheel *TimingWheelService) {
	wheel.ScheduleRecurring("account_risk_maintenance", time.Minute, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if err := s.maintainAccountRiskEvidence(ctx); err != nil {
			slog.Warn("account risk evidence maintenance failed", "error", err)
		}
	})
}

func (s *CheckinService) maintainAccountRiskEvidence(ctx context.Context) error {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	rows, err := client.QueryContext(ctx, "SELECT pg_try_advisory_xact_lock(731853002731)")
	if err != nil {
		return err
	}
	var acquired bool
	if err := scanRiskRow(rows, &acquired); err != nil || !acquired {
		return err
	}

	// Bound each delete so a backlog cannot monopolize production locks or WAL.
	for _, table := range []struct{ name, expired string }{
		{"account_risk_events", "created_at < NOW()-INTERVAL '90 days'"},
		{"checkin_browser_reviews", "created_at < NOW()-INTERVAL '90 days'"},
		{"checkin_browser_claims", "created_at < NOW()-INTERVAL '90 days'"},
		{"checkin_browser_audit", "created_at < NOW()-INTERVAL '90 days'"},
		{"account_registration_cooldowns", "expires_at < NOW()"},
		{"account_registration_captcha", "expires_at < NOW()"},
		{"account_registration_risk_counts", "day < CURRENT_DATE - 90"},
		{"account_risk_usage_ips", "bucket < date_trunc('hour', NOW()-INTERVAL '30 days')"},
		{"account_risk_usage_windows", "bucket < date_trunc('hour', NOW()-INTERVAL '30 days')"},
	} {
		stmt := "DELETE FROM " + table.name + " WHERE ctid IN (SELECT ctid FROM " + table.name + " WHERE " + table.expired + " LIMIT 2000)"
		if _, err := client.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("clean %s: %w", table.name, err)
		}
	}

	// Scan at most four indexed ledger hours per pass, newest missing buckets first.
	// Refresh open hours every five minutes and completed buckets weekly for late writes.
	rows, err = client.QueryContext(ctx, `SELECT hours.bucket FROM generate_series(date_trunc('hour',now()-interval '30 days'),date_trunc('hour',now()),interval '1 hour') hours(bucket)
        LEFT JOIN account_risk_usage_windows w USING(bucket)
        WHERE w.bucket IS NULL OR w.refreshed_at < LEAST(hours.bucket+interval '70 minutes',now()-interval '5 minutes') OR w.refreshed_at < now()-interval '7 days'
        ORDER BY (w.refreshed_at IS NOT NULL AND w.refreshed_at < now()-interval '7 days') ASC,hours.bucket DESC LIMIT 4`)
	if err != nil {
		return err
	}
	var buckets []time.Time
	for rows.Next() {
		var bucket time.Time
		if err = rows.Scan(&bucket); err != nil {
			rows.Close()
			return err
		}
		buckets = append(buckets, bucket)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, bucket := range buckets {
		// Rebuild atomically so removed ledger entries also disappear on refresh.
		if _, err := client.ExecContext(ctx, `DELETE FROM account_risk_usage_ips WHERE bucket=$1`, bucket); err != nil {
			return err
		}
		_, err = client.ExecContext(ctx, `INSERT INTO account_risk_usage_ips (bucket,user_id,ip_address,first_seen_at,last_seen_at)
            SELECT $1::timestamptz,l.user_id,l.ip_address,MIN(l.created_at),MAX(l.created_at) FROM usage_logs l
            JOIN users u ON u.id=l.user_id AND u.deleted_at IS NULL
            WHERE l.created_at >= $1 AND l.created_at < $1::timestamptz+interval '1 hour' AND l.ip_address IS NOT NULL AND l.ip_address<>''
            GROUP BY l.user_id,l.ip_address`, bucket)
		if err != nil {
			return err
		}
		_, err = client.ExecContext(ctx, `INSERT INTO account_risk_usage_windows (bucket) VALUES ($1) ON CONFLICT (bucket) DO UPDATE SET refreshed_at=NOW()`, bucket)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
