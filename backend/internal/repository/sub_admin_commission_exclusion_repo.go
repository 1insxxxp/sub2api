package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *subAdminCommissionRepository) ListExclusions(ctx context.Context, params pagination.PaginationParams) ([]service.SubAdminCommissionExclusion, pagination.PaginationResult, error) {
	params = normalizePagination(params)
	const from = `
		FROM sub_admin_commission_exclusions ce
		JOIN users u ON u.id = ce.user_id AND u.deleted_at IS NULL
		WHERE ce.ended_at IS NULL
	`
	var total int64
	if err := scanSingleRow(ctx, r.sql, "SELECT COUNT(*) "+from, nil, &total); err != nil {
		return nil, pagination.PaginationResult{}, err
	}
	rows, err := r.sql.QueryContext(ctx, `
		SELECT ce.id, ce.user_id, u.email, COALESCE(u.username, ''), COALESCE(u.notes, ''), ce.started_at
		`+from+`
		ORDER BY ce.started_at DESC, ce.id DESC
		LIMIT $1 OFFSET $2
	`, params.Limit(), params.Offset())
	if err != nil {
		return nil, pagination.PaginationResult{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.SubAdminCommissionExclusion, 0)
	for rows.Next() {
		var item service.SubAdminCommissionExclusion
		if err := rows.Scan(&item.ID, &item.UserID, &item.UserEmail, &item.Username, &item.Notes, &item.StartedAt); err != nil {
			return nil, pagination.PaginationResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, pagination.PaginationResult{}, err
	}
	return items, *paginationResultFromTotal(total, params), nil
}

func (r *subAdminCommissionRepository) AddExclusion(ctx context.Context, userID, operatorID int64) (*service.SubAdminCommissionExclusion, error) {
	if userID <= 0 {
		return nil, service.ErrUserNotFound
	}
	if r == nil || r.db == nil {
		return nil, service.ErrSubAdminCommissionRepoMissing
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	item := &service.SubAdminCommissionExclusion{UserID: userID}
	// A no-key row lock serializes add/remove, including the first exclusion,
	// while remaining compatible with created_by's foreign-key KEY SHARE lock
	// when two administrators exclude one another concurrently.
	// Taking a timestamp only after this lock avoids transaction-start NOW()
	// timestamps overlapping an interval closed while this transaction waited.
	if err := scanSingleRow(ctx, tx, `
		SELECT email, COALESCE(username, ''), COALESCE(notes, '') FROM users
		WHERE id = $1 AND deleted_at IS NULL FOR NO KEY UPDATE
	`, []any{userID}, &item.UserEmail, &item.Username, &item.Notes); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrUserNotFound
		}
		return nil, err
	}
	if err := lockCommissionExclusionPublisher(ctx, tx); err != nil {
		return nil, err
	}
	err = scanSingleRow(ctx, tx, `
		SELECT id, started_at FROM sub_admin_commission_exclusions
		WHERE user_id = $1 AND ended_at IS NULL
	`, []any{userID}, &item.ID, &item.StartedAt)
	if errors.Is(err, sql.ErrNoRows) {
		createdBy := sql.NullInt64{Int64: operatorID, Valid: operatorID > 0}
		err = scanSingleRow(ctx, tx, `
			INSERT INTO sub_admin_commission_exclusions (user_id, created_by, started_at)
			VALUES ($1, $2, GREATEST(clock_timestamp(), COALESCE(
				(SELECT MAX(ended_at) FROM sub_admin_commission_exclusions WHERE user_id = $1),
				'-infinity'::timestamptz)))
			RETURNING id, started_at
		`, []any{userID, createdBy}, &item.ID, &item.StartedAt)
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

func (r *subAdminCommissionRepository) RemoveExclusion(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return service.ErrUserNotFound
	}
	if r == nil || r.db == nil {
		return service.ErrSubAdminCommissionRepoMissing
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var id int64
	if err := scanSingleRow(ctx, tx, `SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL FOR NO KEY UPDATE`, []any{userID}, &id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrUserNotFound
		}
		return err
	}
	if err := lockCommissionExclusionPublisher(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sub_admin_commission_exclusions
		SET ended_at = GREATEST(clock_timestamp(), started_at)
		WHERE user_id = $1 AND ended_at IS NULL
	`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func lockCommissionExclusionPublisher(ctx context.Context, tx *sql.Tx) error {
	// Serialize interval boundaries with publishing closed daily buckets. Without
	// this lock a transaction crossing midnight could commit an exclusion after
	// a publisher had already read and closed that day without seeing it.
	var id int
	return scanSingleRow(ctx, tx, `SELECT id FROM usage_group_rollup_state WHERE id = 1 FOR KEY SHARE`, nil, &id)
}
