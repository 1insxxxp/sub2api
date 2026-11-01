package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userSupportRepository struct{ db *sql.DB }

func NewUserSupportRepository(sqlDB *sql.DB) service.UserSupportRepository {
	return &userSupportRepository{db: sqlDB}
}

const supportAccessSQL = `SELECT EXISTS (
	SELECT 1 FROM users u LEFT JOIN user_support_access a ON a.user_id=u.id
	WHERE u.id=$1 AND u.deleted_at IS NULL AND u.status='active'
	AND (u.role='admin' OR (u.role='sub_admin' AND COALESCE(a.enabled,FALSE)))
)`
const supportMainAdminSQL = `SELECT EXISTS (SELECT 1 FROM users WHERE id=$1 AND deleted_at IS NULL AND status='active' AND role='admin')`

func (r *userSupportRepository) CanAccess(ctx context.Context, actorID int64) (bool, error) {
	var allowed bool
	err := r.db.QueryRowContext(ctx, supportAccessSQL, actorID).Scan(&allowed)
	return allowed, err
}

func (r *userSupportRepository) GetAccess(ctx context.Context, actorID, targetID int64) (bool, error) {
	var main bool
	if err := r.db.QueryRowContext(ctx, supportMainAdminSQL, actorID).Scan(&main); err != nil {
		return false, err
	}
	if !main {
		return false, service.ErrUserSupportForbidden
	}
	var enabled bool
	err := r.db.QueryRowContext(ctx, `SELECT CASE WHEN u.role='admin' AND u.status='active' THEN TRUE ELSE COALESCE(a.enabled,FALSE) END FROM users u
	LEFT JOIN user_support_access a ON a.user_id=u.id WHERE u.id=$1 AND u.deleted_at IS NULL AND u.role IN ('sub_admin','admin')`, targetID).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, service.ErrUserSupportUserNotFound
	}
	return enabled, err
}

func (r *userSupportRepository) SetAccess(ctx context.Context, actorID, targetID int64, enabled bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var main bool
	if err = tx.QueryRowContext(ctx, supportMainAdminSQL, actorID).Scan(&main); err != nil {
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
	if role != service.RoleSubAdmin || (enabled && status != service.StatusActive) {
		return service.ErrUserSupportUserNotFound
	}
	if err = tx.QueryRowContext(ctx, supportMainAdminSQL, actorID).Scan(&main); err != nil {
		return err
	}
	if !main {
		return service.ErrUserSupportForbidden
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO user_support_access(user_id,enabled,updated_by) VALUES($1,$2,$3)
	ON CONFLICT(user_id) DO UPDATE SET enabled=EXCLUDED.enabled,updated_by=EXCLUDED.updated_by,updated_at=NOW()`, targetID, enabled, actorID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *userSupportRepository) LookupAndGrant(ctx context.Context, actorID int64, account string) (*service.UserSupportGrantResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var allowed bool
	if err = tx.QueryRowContext(ctx, supportAccessSQL, actorID).Scan(&allowed); err != nil {
		return nil, err
	}
	if !allowed {
		return nil, service.ErrUserSupportForbidden
	}
	account = strings.TrimSpace(account)
	result := &service.UserSupportGrantResult{Trial: service.UserSupportTrial{Status: "already_received", Amount: service.UserSupportTrialAmount}}
	// The wallet row serializes every support claim for this target, including
	// calls by different operators, and composes with ordinary balance updates.
	if id, parseErr := strconv.ParseInt(account, 10, 64); parseErr == nil && id > 0 {
		err = tx.QueryRowContext(ctx, `SELECT id,email,balance FROM users WHERE id=$1 AND deleted_at IS NULL AND status='active' FOR UPDATE`, id).Scan(&result.User.ID, &result.User.Email, &result.User.Balance)
	} else {
		// Share the registration/profile-update normalized-email lock so no
		// matching identity can be inserted or renamed between disambiguation
		// and the wallet lock. Legacy spacing/case collisions must never pick
		// whichever row QueryRow happens to return and credit the wrong user.
		if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, advisoryLockHash(normalizedEmailUniquenessLockKey(account))); err != nil {
			return nil, err
		}
		var id int64
		id, err = supportUniqueEmailID(ctx, tx, account)
		if err != nil {
			return nil, err
		}
		err = tx.QueryRowContext(ctx, `SELECT id,email,balance FROM users WHERE id=$1 AND LOWER(TRIM(email))=$2 AND deleted_at IS NULL AND status='active' FOR UPDATE`, id, normalizeEmailLookupValue(account)).Scan(&result.User.ID, &result.User.Email, &result.User.Balance)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserSupportUserNotFound
	}
	if err != nil {
		return nil, err
	}
	// Check again after waiting for a wallet lock. A revoked caller must not be
	// allowed to credit based on a stale service-layer or pre-lock decision.
	if err = tx.QueryRowContext(ctx, supportAccessSQL, actorID).Scan(&allowed); err != nil {
		return nil, err
	}
	if !allowed {
		return nil, service.ErrUserSupportForbidden
	}
	err = tx.QueryRowContext(ctx, `SELECT granted_at,legacy FROM support_trial_grants WHERE user_id=$1`, result.User.ID).Scan(&result.Trial.GrantedAt, &result.Trial.Legacy)
	if err == nil {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var ledgerID int64
	err = tx.QueryRowContext(ctx, `SELECT id,COALESCE(used_at,created_at) FROM redeem_codes
	WHERE used_by=$1 AND type='admin_balance' AND status='used' AND value=15 ORDER BY id ASC LIMIT 1`, result.User.ID).Scan(&ledgerID, &result.Trial.GrantedAt)
	if err == nil {
		result.Trial.Legacy = true
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	} else {
		code, codeErr := service.GenerateRedeemCode()
		if codeErr != nil {
			return nil, codeErr
		}
		// Same gift-wallet invariant as CreditGiftBalance: repay any overdraft
		// before allocating the remainder as non-transferable gifted balance.
		err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance+15,
		gift_balance=LEAST(gift_balance+15,GREATEST(balance+15,0)),updated_at=NOW()
		WHERE id=$1 AND gift_balance<=GREATEST(balance,0) RETURNING balance`, result.User.ID).Scan(&result.User.Balance)
		if err != nil {
			return nil, err
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO redeem_codes(code,type,value,status,used_by,used_at,created_by,source,threshold_exempt,notes)
		VALUES($1,'balance',15,'used',$2,NOW(),$3,'support_trial',TRUE,'Support trial credit') RETURNING id,used_at`, code, result.User.ID, actorID).Scan(&ledgerID, &result.Trial.GrantedAt)
		if err != nil {
			return nil, err
		}
		result.Trial.Status = "granted"
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO support_trial_grants(user_id,amount,operator_id,redeem_code_id,legacy,granted_at)
	VALUES($1,15,$2,$3,$4,$5)`, result.User.ID, actorID, ledgerID, result.Trial.Legacy, result.Trial.GrantedAt)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func supportUniqueEmailID(ctx context.Context, tx *sql.Tx, account string) (int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM users WHERE LOWER(TRIM(email))=$1 AND deleted_at IS NULL ORDER BY id LIMIT 2`, normalizeEmailLookupValue(account))
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	var id int64
	count := 0
	for rows.Next() {
		if err = rows.Scan(&id); err != nil {
			return 0, err
		}
		count++
	}
	if err = rows.Err(); err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, service.ErrUserSupportUserNotFound
	}
	if count > 1 {
		return 0, service.ErrUserSupportInvalidAccount
	}
	return id, nil
}

func (r *userSupportRepository) ListUsage(ctx context.Context, userID int64) ([]service.UserSupportUsageRecord, error) {
	if userID <= 0 {
		return nil, service.ErrUserSupportUserNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `SELECT ul.id,ul.created_at,COALESCE(ul.requested_model,''),COALESCE(g.name,''),
	CASE WHEN o.input_tokens_source='unreported' THEN NULL ELSE ul.input_tokens END,
	CASE WHEN o.output_tokens_source='unreported' THEN NULL ELSE ul.output_tokens END,
	ul.cache_read_tokens,GREATEST(ul.actual_cost-ul.compensated_cost,0),ul.duration_ms,ul.first_token_ms,ul.stream,
	COALESCE(o.collector_version,0),COALESCE(o.http_status,0),COALESCE(o.upstream_status,0),
	COALESCE(o.has_text,FALSE),COALESCE(o.has_tool_call,FALSE),COALESCE(o.has_reasoning,FALSE),COALESCE(o.has_media,FALSE),
	COALESCE(o.stream_completed,FALSE),COALESCE(o.disconnect_source,''),COALESCE(o.upstream_error_kind,'')
	FROM (SELECT id,created_at,requested_model,group_id,input_tokens,output_tokens,cache_read_tokens,
	actual_cost,compensated_cost,duration_ms,first_token_ms,stream FROM usage_logs WHERE user_id=$1 ORDER BY created_at DESC,id DESC LIMIT 10) ul
	LEFT JOIN groups g ON g.id=ul.group_id
	LEFT JOIN LATERAL (SELECT collector_version,http_status,upstream_status,has_text,has_tool_call,has_reasoning,has_media,
	stream_completed,disconnect_source,upstream_error_kind,input_tokens_source,output_tokens_source FROM usage_response_outcomes
	WHERE usage_log_id=ul.id AND user_id=$1 ORDER BY id DESC LIMIT 1) o ON TRUE ORDER BY ul.created_at DESC,ul.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]service.UserSupportUsageRecord, 0, 10)
	for rows.Next() {
		var item service.UserSupportUsageRecord
		var input, output, cache, duration, first sql.NullInt64
		var outcome service.ResponseOutcome
		if err = rows.Scan(&item.ID, &item.CreatedAt, &item.Model, &item.GroupName, &input, &output, &cache, &item.ChargedAmount, &duration, &first, &item.Stream,
			&outcome.CollectorVersion, &outcome.HTTPStatus, &outcome.UpstreamStatus, &outcome.HasText, &outcome.HasToolCall, &outcome.HasReasoning, &outcome.HasMedia,
			&outcome.StreamCompleted, &outcome.DisconnectSource, &outcome.UpstreamErrorKind); err != nil {
			return nil, err
		}
		item.InputTokens = supportInt64Pointer(input)
		item.OutputTokens = supportInt64Pointer(output)
		item.CacheReadTokens = supportInt64Pointer(cache)
		item.DurationMs = supportInt64Pointer(duration)
		item.FirstTokenMs = supportInt64Pointer(first)
		item.Model = service.PublicUserSupportModel(item.Model)
		item.Status = string(service.ClassifyUsageRequestStatus(&outcome))
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *userSupportRepository) ListErrors(ctx context.Context, userID int64) ([]service.UserSupportErrorRecord, error) {
	if userID <= 0 {
		return nil, service.ErrUserSupportUserNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// Reuse only the user-visible request filter, never the broad Ops DTO or
	// its provider-preferred status/message/model projection.
	where, args := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{UserID: &userID, View: "all", ExcludeCountTokens: true, ExcludeCanceledRequests: true})
	rows, err := r.db.QueryContext(ctx, `SELECT e.id,e.created_at,COALESCE(e.requested_model,''),COALESCE(g.name,''),
	CASE WHEN e.status_code BETWEEN 100 AND 599 THEN e.status_code ELSE NULL END,e.error_phase,e.error_type
	FROM ops_error_logs e LEFT JOIN groups g ON g.id=e.group_id `+where+` ORDER BY e.created_at DESC,e.id DESC LIMIT 10`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]service.UserSupportErrorRecord, 0, 10)
	for rows.Next() {
		var item service.UserSupportErrorRecord
		var status sql.NullInt64
		var phase, kind string
		if err = rows.Scan(&item.ID, &item.CreatedAt, &item.Model, &item.GroupName, &status, &phase, &kind); err != nil {
			return nil, err
		}
		item.Model = service.PublicUserSupportModel(item.Model)
		item.StatusCode = supportInt64Pointer(status)
		item.ErrorKind = service.PublicUserSupportErrorKind(phase)
		item.Category = service.UserSupportErrorCategory(phase, kind)
		result = append(result, item)
	}
	return result, rows.Err()
}

func supportInt64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	x := value.Int64
	return &x
}
