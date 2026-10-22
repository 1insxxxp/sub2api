package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type groupAccountUserExclusionRepository struct{ db *sql.DB }

func NewGroupAccountUserExclusionRepository(db *sql.DB) service.GroupAccountUserExclusionRepository {
	return &groupAccountUserExclusionRepository{db: db}
}

func loadGroupAccountExclusions(ctx context.Context, exec sqlExecutor, userID int64) (service.GroupAccountExclusions, error) {
	if exec == nil {
		return nil, errors.New("group account exclusions database unavailable")
	}
	rows, err := exec.QueryContext(ctx, `SELECT group_id, account_id FROM group_account_user_exclusions WHERE user_id = $1 ORDER BY group_id, account_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("load group account user exclusions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make(service.GroupAccountExclusions)
	for rows.Next() {
		var groupID, accountID int64
		if err := rows.Scan(&groupID, &accountID); err != nil {
			return nil, err
		}
		result[groupID] = append(result[groupID], accountID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *apiKeyRepository) withGroupAccountExclusions(ctx context.Context, key *service.APIKey) (*service.APIKey, error) {
	if key == nil || key.User == nil {
		return nil, service.ErrUserNotFound
	}
	policy, err := loadGroupAccountExclusions(ctx, r.sql, key.UserID)
	if err != nil {
		return nil, err
	}
	key.User.GroupAccountExclusions = policy
	return key, nil
}

func (r *groupAccountUserExclusionRepository) List(ctx context.Context, groupID int64, params pagination.PaginationParams) ([]service.GroupAccountUserExclusion, pagination.PaginationResult, error) {
	params = normalizePagination(params)
	var id int64
	if err := scanSingleRow(ctx, r.db, `SELECT id FROM groups WHERE id = $1 AND deleted_at IS NULL`, []any{groupID}, &id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pagination.PaginationResult{}, service.ErrGroupNotFound
		}
		return nil, pagination.PaginationResult{}, err
	}
	const from = ` FROM group_account_user_exclusions e JOIN accounts a ON a.id = e.account_id AND a.deleted_at IS NULL JOIN users u ON u.id = e.user_id AND u.deleted_at IS NULL WHERE e.group_id = $1 `
	var total int64
	if err := scanSingleRow(ctx, r.db, `SELECT COUNT(*)`+from, []any{groupID}, &total); err != nil {
		return nil, pagination.PaginationResult{}, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT e.id,e.group_id,e.account_id,a.name,e.user_id,u.email,COALESCE(u.username,''),COALESCE(u.notes,''),e.created_at`+from+`ORDER BY e.created_at DESC,e.id DESC LIMIT $2 OFFSET $3`, groupID, params.Limit(), params.Offset())
	if err != nil {
		return nil, pagination.PaginationResult{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.GroupAccountUserExclusion, 0)
	for rows.Next() {
		var item service.GroupAccountUserExclusion
		if err := rows.Scan(&item.ID, &item.GroupID, &item.AccountID, &item.AccountName, &item.UserID, &item.UserEmail, &item.Username, &item.Notes, &item.CreatedAt); err != nil {
			return nil, pagination.PaginationResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, pagination.PaginationResult{}, err
	}
	return items, *paginationResultFromTotal(total, params), nil
}

func (r *groupAccountUserExclusionRepository) Add(ctx context.Context, groupID, accountID, userID int64) (*service.GroupAccountUserExclusion, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var id int64
	if err := scanSingleRow(ctx, tx, `SELECT id FROM groups WHERE id = $1 AND deleted_at IS NULL FOR SHARE`, []any{groupID}, &id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrGroupNotFound
		}
		return nil, err
	}
	item := &service.GroupAccountUserExclusion{GroupID: groupID, AccountID: accountID, UserID: userID}
	// Lock both live membership and account until commit, so a concurrent removal
	// cannot create an apparently valid rule for an account outside this group.
	if err := scanSingleRow(ctx, tx, `SELECT a.name FROM accounts a JOIN account_groups ag ON ag.account_id = a.id WHERE a.id = $1 AND ag.group_id = $2 AND a.deleted_at IS NULL FOR SHARE OF a,ag`, []any{accountID, groupID}, &item.AccountName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAccountNotFound
		}
		return nil, err
	}
	if err := scanSingleRow(ctx, tx, `SELECT email,COALESCE(username,''),COALESCE(notes,'') FROM users WHERE id = $1 AND deleted_at IS NULL AND status = 'active' FOR NO KEY UPDATE`, []any{userID}, &item.UserEmail, &item.Username, &item.Notes); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrUserNotFound
		}
		return nil, err
	}
	if err := scanSingleRow(ctx, tx, `INSERT INTO group_account_user_exclusions (group_id,account_id,user_id) VALUES ($1,$2,$3) ON CONFLICT (group_id,account_id,user_id) DO UPDATE SET account_id = EXCLUDED.account_id RETURNING id,created_at`, []any{groupID, accountID, userID}, &item.ID, &item.CreatedAt); err != nil {
		return nil, err
	}
	if err := enqueueGroupAccountUserAuthInvalidation(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

func (r *groupAccountUserExclusionRepository) Remove(ctx context.Context, groupID, exclusionID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var userID int64
	if err := scanSingleRow(ctx, tx, `DELETE FROM group_account_user_exclusions WHERE id = $1 AND group_id = $2 RETURNING user_id`, []any{exclusionID, groupID}, &userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, service.ErrGroupAccountUserExclusionNotFound
		}
		return 0, err
	}
	if err := enqueueGroupAccountUserAuthInvalidation(ctx, tx, userID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return userID, nil
}

func enqueueGroupAccountUserAuthInvalidation(ctx context.Context, tx *sql.Tx, userID int64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO auth_cache_invalidation_outbox (cache_key) SELECT encode(sha256(convert_to(k.key, 'UTF8')), 'hex') FROM api_keys k WHERE k.user_id = $1 AND k.deleted_at IS NULL AND k.key <> ''`, userID)
	return err
}
