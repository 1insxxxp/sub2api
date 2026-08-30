package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type emptyResponseCompensationRepository struct {
	db *sql.DB
}

func NewEmptyResponseCompensationRepository(db *sql.DB) service.EmptyResponseCompensationRepository {
	return newEmptyResponseCompensationRepository(db)
}

func newEmptyResponseCompensationRepository(db *sql.DB) *emptyResponseCompensationRepository {
	return &emptyResponseCompensationRepository{db: db}
}

type emptyResponseCompensationState struct {
	status             string
	usageLogID         int64
	userID             int64
	apiKeyID           int64
	groupID            sql.NullInt64
	subscriptionID     sql.NullInt64
	originalActualCost float64
	actualCost         float64
	compensatedCost    float64
	billingType        int8
	usageCreatedAt     time.Time
	apiKey             string
}

func (r *emptyResponseCompensationRepository) Compensate(ctx context.Context, claimID int64) (result *service.EmptyResponseCompensationResult, err error) {
	if r == nil || r.db == nil || claimID <= 0 {
		return nil, service.ErrEmptyResponseCompensationInvalidInput
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("begin empty response compensation: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	state := &emptyResponseCompensationState{}
	if err = scanSingleRow(ctx, tx, `
		SELECT status, usage_log_id, user_id, api_key_id, group_id, subscription_id,
			original_actual_cost::float8
		FROM empty_response_claims
		WHERE id = $1
		FOR UPDATE
	`, []any{claimID}, &state.status, &state.usageLogID, &state.userID, &state.apiKeyID,
		&state.groupID, &state.subscriptionID, &state.originalActualCost); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrEmptyResponseClaimNotFound
		}
		return nil, fmt.Errorf("lock empty response claim: %w", err)
	}
	if state.status != service.EmptyResponseClaimApproved && state.status != service.EmptyResponseClaimCompensated {
		return nil, service.ErrEmptyResponseCompensationInvalidState
	}
	if err = scanSingleRow(ctx, tx, `
		SELECT actual_cost::float8, compensated_cost::float8, billing_type, created_at
		FROM usage_logs
		WHERE id = $1
		FOR UPDATE
	`, []any{state.usageLogID}, &state.actualCost, &state.compensatedCost, &state.billingType, &state.usageCreatedAt); err != nil {
		return nil, fmt.Errorf("lock compensated usage log: %w", err)
	}

	var balance float64
	if err = scanSingleRow(ctx, tx, "SELECT balance::float8 FROM users WHERE id = $1 FOR UPDATE", []any{state.userID}, &balance); err != nil {
		return nil, fmt.Errorf("lock compensated user: %w", err)
	}
	var quotaUsed float64
	if err = scanSingleRow(ctx, tx, "SELECT key, quota_used::float8 FROM api_keys WHERE id = $1 FOR UPDATE", []any{state.apiKeyID}, &state.apiKey, &quotaUsed); err != nil {
		return nil, fmt.Errorf("lock compensated api key: %w", err)
	}
	if state.subscriptionID.Valid {
		var startsAt, expiresAt time.Time
		var dailyWindowStart, weeklyWindowStart, monthlyWindowStart sql.NullTime
		if err = scanSingleRow(ctx, tx, `
			SELECT starts_at, expires_at, daily_window_start, weekly_window_start, monthly_window_start
			FROM user_subscriptions
			WHERE id = $1
			FOR UPDATE
		`, []any{state.subscriptionID.Int64}, &startsAt, &expiresAt, &dailyWindowStart, &weeklyWindowStart, &monthlyWindowStart); err != nil {
			return nil, fmt.Errorf("lock compensated subscription: %w", err)
		}
	}

	result = compensationResultFromState(claimID, state)
	if state.status == service.EmptyResponseClaimCompensated {
		if err = tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit existing empty response compensation: %w", err)
		}
		return result, nil
	}
	if state.originalActualCost <= 0 || state.originalActualCost > state.actualCost || state.compensatedCost != 0 {
		return nil, service.ErrEmptyResponseCompensationInvalidState
	}

	refund := state.originalActualCost
	switch state.billingType {
	case service.BillingTypeBalance:
		var update sql.Result
		if update, err = tx.ExecContext(ctx, "UPDATE users SET balance = balance + $1, updated_at = NOW() WHERE id = $2", refund, state.userID); err != nil {
			return nil, fmt.Errorf("refund user balance: %w", err)
		}
		if affected, rowsErr := update.RowsAffected(); rowsErr != nil || affected != 1 {
			return nil, service.ErrEmptyResponseCompensationInvalidState
		}
		if err = insertEmptyResponseBalanceHistory(ctx, tx, claimID, state, refund); err != nil {
			return nil, err
		}
	case service.BillingTypeSubscription:
		if !state.subscriptionID.Valid {
			return nil, service.ErrEmptyResponseCompensationInvalidState
		}
		update, updateErr := tx.ExecContext(ctx, `
			UPDATE user_subscriptions SET
				daily_usage_usd = CASE
					WHEN daily_window_start IS NOT NULL AND $2 >= daily_window_start AND $2 < daily_window_start + INTERVAL '24 hours'
					THEN GREATEST(daily_usage_usd - $1, 0) ELSE daily_usage_usd END,
				weekly_usage_usd = CASE
					WHEN weekly_window_start IS NOT NULL AND $2 >= weekly_window_start AND $2 < weekly_window_start + INTERVAL '7 days'
					THEN GREATEST(weekly_usage_usd - $1, 0) ELSE weekly_usage_usd END,
				monthly_usage_usd = CASE
					WHEN monthly_window_start IS NOT NULL AND $2 >= monthly_window_start AND $2 < monthly_window_start + INTERVAL '30 days'
					THEN GREATEST(monthly_usage_usd - $1, 0) ELSE monthly_usage_usd END,
				updated_at = NOW()
			WHERE id = $3 AND starts_at <= $2 AND expires_at > $2
		`, refund, state.usageCreatedAt, state.subscriptionID.Int64)
		if updateErr != nil {
			err = updateErr
			return nil, fmt.Errorf("refund subscription usage: %w", err)
		}
		if affected, rowsErr := update.RowsAffected(); rowsErr != nil || affected != 1 {
			return nil, service.ErrEmptyResponseCompensationInvalidState
		}
	default:
		return nil, service.ErrEmptyResponseCompensationInvalidState
	}

	apiKeyUpdate, err := tx.ExecContext(ctx, "UPDATE api_keys SET quota_used = GREATEST(quota_used - $1, 0), updated_at = NOW() WHERE id = $2", refund, state.apiKeyID)
	if err != nil {
		return nil, fmt.Errorf("refund api key quota: %w", err)
	}
	if affected, rowsErr := apiKeyUpdate.RowsAffected(); rowsErr != nil || affected != 1 {
		return nil, service.ErrEmptyResponseCompensationInvalidState
	}
	usageUpdate, err := tx.ExecContext(ctx, `
		UPDATE usage_logs SET compensated_cost = $1
		WHERE id = $3 AND actual_cost = $2 AND compensated_cost = $4
	`, refund, state.actualCost, state.usageLogID, state.compensatedCost)
	if err != nil {
		return nil, fmt.Errorf("mark usage compensated: %w", err)
	}
	if affected, rowsErr := usageUpdate.RowsAffected(); rowsErr != nil || affected != 1 {
		return nil, service.ErrEmptyResponseCompensationInvalidState
	}

	claimQuery := `
		UPDATE empty_response_claims SET
			status = $1,
			balance_refund = $2,
			subscription_refund = 0,
			api_key_quota_refund = $2,
			compensated_at = NOW(),
			updated_at = NOW()
		WHERE id = $3 AND status = $4
	`
	if state.billingType == service.BillingTypeSubscription {
		claimQuery = `
			UPDATE empty_response_claims SET
				status = $1,
				balance_refund = 0,
				subscription_refund = $2,
				api_key_quota_refund = $2,
				compensated_at = NOW(),
				updated_at = NOW()
			WHERE id = $3 AND status = $4
		`
	}
	claimUpdate, err := tx.ExecContext(ctx, claimQuery, service.EmptyResponseClaimCompensated, refund, claimID, service.EmptyResponseClaimApproved)
	if err != nil {
		return nil, fmt.Errorf("mark empty response claim compensated: %w", err)
	}
	if affected, rowsErr := claimUpdate.RowsAffected(); rowsErr != nil || affected != 1 {
		return nil, service.ErrEmptyResponseCompensationInvalidState
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit empty response compensation: %w", err)
	}
	result.Applied = true
	return result, nil
}

func compensationResultFromState(claimID int64, state *emptyResponseCompensationState) *service.EmptyResponseCompensationResult {
	result := &service.EmptyResponseCompensationResult{
		ClaimID:      claimID,
		UsageLogID:   state.usageLogID,
		UserID:       state.userID,
		APIKeyID:     state.apiKeyID,
		APIKey:       state.apiKey,
		RefundAmount: state.originalActualCost,
	}
	if state.groupID.Valid {
		result.GroupID = &state.groupID.Int64
	}
	if state.subscriptionID.Valid {
		result.SubscriptionID = &state.subscriptionID.Int64
	}
	return result
}

func insertEmptyResponseBalanceHistory(ctx context.Context, tx *sql.Tx, claimID int64, state *emptyResponseCompensationState, refund float64) error {
	notes := fmt.Sprintf("Empty response compensation for usage log #%d", state.usageLogID)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO redeem_codes (code, type, value, status, used_by, used_at, created_at, source, notes)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW(), $6, $7)
	`, fmt.Sprintf("EMPTY-COMP-%d", claimID), service.RedeemTypeEmptyResponse, refund, service.StatusUsed, state.userID, service.RedeemCodeSourceEmptyResponseCompensation, notes)
	if err != nil {
		return fmt.Errorf("record empty response compensation history: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		return service.ErrEmptyResponseCompensationInvalidState
	}
	return nil
}
