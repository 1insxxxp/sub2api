package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type adminWorkbenchAccessRepository struct{ db *sql.DB }

func NewAdminWorkbenchAccessRepository(db *sql.DB) service.AdminWorkbenchAccessRepository {
	return &adminWorkbenchAccessRepository{db: db}
}

const workbenchMainAdminSQL = `SELECT EXISTS (SELECT 1 FROM users WHERE id=$1 AND deleted_at IS NULL AND status='active' AND role='admin')`

func (r *adminWorkbenchAccessRepository) GetAccess(ctx context.Context, actorID int64) (*service.AdminWorkbenchAccess, error) {
	if actorID <= 0 {
		return nil, service.ErrUserSupportForbidden
	}
	var role string
	var commission, leaderboard, support bool
	err := r.db.QueryRowContext(ctx, `SELECT u.role,COALESCE(w.commission_enabled,FALSE),COALESCE(w.affiliate_leaderboard_enabled,FALSE),COALESCE(s.enabled,FALSE)
	FROM users u LEFT JOIN sub_admin_workbench_access w ON w.user_id=u.id
	LEFT JOIN user_support_access s ON s.user_id=u.id
	WHERE u.id=$1 AND u.deleted_at IS NULL AND u.status='active' AND u.role IN ('admin','sub_admin')`, actorID).Scan(&role, &commission, &leaderboard, &support)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserSupportForbidden
	}
	if err != nil {
		return nil, err
	}
	return workbenchAccessForRole(role, commission, leaderboard, support), nil
}

func (r *adminWorkbenchAccessRepository) GetUserAccess(ctx context.Context, actorID, targetID int64) (*service.AdminWorkbenchAccess, error) {
	if actorID <= 0 || targetID <= 0 {
		return nil, service.ErrUserSupportForbidden
	}
	var main bool
	var role sql.NullString
	var commission, leaderboard, support bool
	// Check the actor and project the target in one statement and snapshot. The
	// target joins only when the current actor is an active main administrator.
	err := r.db.QueryRowContext(ctx, `SELECT actor.allowed,u.role,COALESCE(w.commission_enabled,FALSE),COALESCE(w.affiliate_leaderboard_enabled,FALSE),COALESCE(s.enabled,FALSE)
	FROM (SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND deleted_at IS NULL AND status='active' AND role='admin') AS allowed) actor
	LEFT JOIN users u ON actor.allowed AND u.id=$2 AND u.deleted_at IS NULL AND u.status='active' AND u.role IN ('admin','sub_admin')
	LEFT JOIN sub_admin_workbench_access w ON w.user_id=u.id
	LEFT JOIN user_support_access s ON s.user_id=u.id`, actorID, targetID).Scan(&main, &role, &commission, &leaderboard, &support)
	if err != nil {
		return nil, err
	}
	if !main {
		return nil, service.ErrUserSupportForbidden
	}
	if !role.Valid {
		return nil, service.ErrUserSupportUserNotFound
	}
	return workbenchAccessForRole(role.String, commission, leaderboard, support), nil
}

func workbenchAccessForRole(role string, commission, leaderboard, support bool) *service.AdminWorkbenchAccess {
	if role == service.RoleAdmin {
		return &service.AdminWorkbenchAccess{Commission: true, AffiliateLeaderboard: true, UserSupport: true, BalanceTransfer: true}
	}
	return &service.AdminWorkbenchAccess{Commission: commission, AffiliateLeaderboard: leaderboard, UserSupport: support, BalanceTransfer: true}
}

func (r *adminWorkbenchAccessRepository) SetUserAccess(ctx context.Context, actorID, targetID int64, commission, leaderboard, support bool) error {
	if actorID <= 0 || targetID <= 0 {
		return service.ErrUserSupportForbidden
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var main bool
	if err = tx.QueryRowContext(ctx, workbenchMainAdminSQL, actorID).Scan(&main); err != nil {
		return err
	}
	if !main {
		return service.ErrUserSupportForbidden
	}
	var role, status string
	err = tx.QueryRowContext(ctx, `SELECT role,status FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, targetID).Scan(&role, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrUserSupportUserNotFound
	}
	if err != nil {
		return err
	}
	if role == service.RoleAdmin {
		return service.ErrUserSupportForbidden
	}
	if role != service.RoleSubAdmin || status != service.StatusActive {
		return service.ErrUserSupportUserNotFound
	}
	// The target row serializes three-capability updates. The actor can lose
	// permission while this transaction waits, so check the database again.
	if err = tx.QueryRowContext(ctx, workbenchMainAdminSQL, actorID).Scan(&main); err != nil {
		return err
	}
	if !main {
		return service.ErrUserSupportForbidden
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sub_admin_workbench_access(user_id,commission_enabled,affiliate_leaderboard_enabled,updated_by)
	VALUES($1,$2,$3,$4) ON CONFLICT(user_id) DO UPDATE SET commission_enabled=EXCLUDED.commission_enabled,
	affiliate_leaderboard_enabled=EXCLUDED.affiliate_leaderboard_enabled,updated_by=EXCLUDED.updated_by,updated_at=NOW()`, targetID, commission, leaderboard, actorID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO user_support_access(user_id,enabled,updated_by) VALUES($1,$2,$3)
	ON CONFLICT(user_id) DO UPDATE SET enabled=EXCLUDED.enabled,updated_by=EXCLUDED.updated_by,updated_at=NOW()`, targetID, support, actorID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
