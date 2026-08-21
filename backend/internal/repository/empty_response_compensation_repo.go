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
	case service.BillingTypeBalance, service.BillingTypeSubscription:
	default:
		return nil, service.ErrEmptyResponseCompensationInvalidState
	}
	var update sql.Result
	if update, err = tx.ExecContext(ctx, "UPDATE users SET balance = balance + $1, updated_at = NOW() WHERE id = $2", refund, state.userID); err != nil {
		return nil, fmt.Errorf("refund user balance: %w", err)
	}
	if affected, rowsErr := update.RowsAffected(); rowsErr != nil || affected != 1 {
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
