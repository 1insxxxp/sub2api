package repository

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type usageBillingRepository struct {
	db *sql.DB
}

var errBatchImageCaptureHoldNotReserved = errors.New("batch image capture hold was not reserved")
var errBatchImageReleaseHoldNotReserved = errors.New("batch image release hold was not reserved")
var errBatchImageCaptureHoldReleased = errors.New("batch image hold was already released")
var errBatchImageReleaseHoldCaptured = errors.New("batch image hold was already captured")

type usageBillingClaimResult struct {
	Applied             bool
	ThresholdExemptCost sql.NullFloat64
}

func NewUsageBillingRepository(_ *dbent.Client, sqlDB *sql.DB) service.UsageBillingRepository {
	return &usageBillingRepository{db: sqlDB}
}

func (r *usageBillingRepository) Apply(ctx context.Context, cmd *service.UsageBillingCommand) (_ *service.UsageBillingApplyResult, err error) {
	if cmd == nil {
		return &service.UsageBillingApplyResult{}, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}
	if err := service.ValidateUsageBillingCommandAmounts(cmd); err != nil {
		return nil, err
	}

	cmd.Normalize()
	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	claim, err := r.claimUsageBillingKey(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if !claim.Applied {
		result := &service.UsageBillingApplyResult{Applied: false}
		if claim.ThresholdExemptCost.Valid {
			result.ThresholdExemptCost = clampUsageBillingDedupThresholdExemptCost(claim.ThresholdExemptCost.Float64, cmd.BalanceCost)
		}
		return result, nil
	}

	result := &service.UsageBillingApplyResult{Applied: true}
	if err := r.applyUsageBillingEffects(ctx, tx, cmd, result); err != nil {
		return nil, err
	}
	result.ThresholdExemptCost = clampUsageBillingDedupThresholdExemptCost(result.ThresholdExemptCost, cmd.BalanceCost)
	if err := persistUsageBillingThresholdExemptCost(ctx, tx, cmd.RequestID, cmd.APIKeyID, result.ThresholdExemptCost); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

func (r *usageBillingRepository) claimUsageBillingKey(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (usageBillingClaimResult, error) {
	return r.claimUsageBillingRequest(ctx, tx, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint)
}

func (r *usageBillingRepository) claimUsageBillingRequest(ctx context.Context, tx *sql.Tx, requestID string, apiKeyID int64, requestFingerprint string) (usageBillingClaimResult, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint)
		VALUES ($1, $2, $3)
		ON CONFLICT (request_id, api_key_id) DO NOTHING
		RETURNING id
	`, requestID, apiKeyID, requestFingerprint).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var existingFingerprint string
		var thresholdExemptCost sql.NullFloat64
		if err := tx.QueryRowContext(ctx, `
			SELECT request_fingerprint, threshold_exempt_cost
			FROM usage_billing_dedup
			WHERE request_id = $1 AND api_key_id = $2
		`, requestID, apiKeyID).Scan(&existingFingerprint, &thresholdExemptCost); err != nil {
			return usageBillingClaimResult{}, err
		}
		if strings.TrimSpace(existingFingerprint) != strings.TrimSpace(requestFingerprint) {
			return usageBillingClaimResult{}, service.ErrUsageBillingRequestConflict
		}
		return usageBillingClaimResult{ThresholdExemptCost: thresholdExemptCost}, nil
	}
	if err != nil {
		return usageBillingClaimResult{}, err
	}
	var archivedFingerprint string
	var archivedThresholdExemptCost sql.NullFloat64
	err = tx.QueryRowContext(ctx, `
		SELECT request_fingerprint, threshold_exempt_cost
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, requestID, apiKeyID).Scan(&archivedFingerprint, &archivedThresholdExemptCost)
	if err == nil {
		if strings.TrimSpace(archivedFingerprint) != strings.TrimSpace(requestFingerprint) {
			return usageBillingClaimResult{}, service.ErrUsageBillingRequestConflict
		}
		return usageBillingClaimResult{ThresholdExemptCost: archivedThresholdExemptCost}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return usageBillingClaimResult{}, err
	}
	return usageBillingClaimResult{Applied: true}, nil
}

func (r *usageBillingRepository) ReserveBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, false, reserveUsageBillingBatchImageBalance)
}

func (r *usageBillingRepository) CaptureBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, true, captureUsageBillingBatchImageBalance)
}

func (r *usageBillingRepository) ReleaseBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, false, releaseUsageBillingBatchImageBalance)
}

func (r *usageBillingRepository) applyBatchImageBalanceHold(
	ctx context.Context,
	cmd *service.BatchImageBalanceHoldCommand,
	replayThresholdExemptCost bool,
	apply func(context.Context, *sql.Tx, *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error),
) (_ *service.BatchImageBalanceHoldResult, err error) {
	if cmd == nil {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}
	cmd.Normalize()
	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}
	if math.IsNaN(cmd.HoldAmount) || math.IsInf(cmd.HoldAmount, 0) || cmd.HoldAmount < 0 ||
		math.IsNaN(cmd.ActualAmount) || math.IsInf(cmd.ActualAmount, 0) || cmd.ActualAmount < 0 {
		return nil, errors.New("batch image billing amounts must be finite and nonnegative")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	claim, err := r.claimUsageBillingRequest(ctx, tx, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint)
	if err != nil {
		return nil, err
	}
	if !claim.Applied {
		result := &service.BatchImageBalanceHoldResult{Applied: false}
		if replayThresholdExemptCost && claim.ThresholdExemptCost.Valid {
			result.ThresholdExemptCost = clampUsageBillingDedupThresholdExemptCost(claim.ThresholdExemptCost.Float64, cmd.ActualAmount)
		}
		return result, nil
	}

	result, err := apply(ctx, tx, cmd)
	if errors.Is(err, errBatchImageReleaseHoldNotReserved) || errors.Is(err, errBatchImageReleaseHoldCaptured) {
		return &service.BatchImageBalanceHoldResult{Applied: false}, nil
	}
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = &service.BatchImageBalanceHoldResult{}
	}
	result.Applied = true
	if replayThresholdExemptCost {
		result.ThresholdExemptCost = clampUsageBillingDedupThresholdExemptCost(result.ThresholdExemptCost, cmd.ActualAmount)
		if err := persistUsageBillingThresholdExemptCost(ctx, tx, cmd.RequestID, cmd.APIKeyID, result.ThresholdExemptCost); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

func clampUsageBillingDedupThresholdExemptCost(amount, actualAmount float64) float64 {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 ||
		math.IsNaN(actualAmount) || math.IsInf(actualAmount, 0) || actualAmount <= 0 {
		return 0
	}
	amount = service.QuantizeUsageBillingAmount(amount)
	if amount > actualAmount {
		return actualAmount
	}
	return amount
}

func persistUsageBillingThresholdExemptCost(ctx context.Context, tx *sql.Tx, requestID string, apiKeyID int64, amount float64) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE usage_billing_dedup
		SET threshold_exempt_cost = $1
		WHERE request_id = $2 AND api_key_id = $3
	`, amount, requestID, apiKeyID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("usage billing replay result row was not updated")
	}
	return nil
}

func (r *usageBillingRepository) applyUsageBillingEffects(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, result *service.UsageBillingApplyResult) error {
	if cmd.SubscriptionCost > 0 && cmd.SubscriptionID != nil {
		if err := incrementUsageBillingSubscription(ctx, tx, *cmd.SubscriptionID, cmd.SubscriptionCost); err != nil {
			return err
		}
	}

	if cmd.BalanceCost > 0 {
		newBalance, sufficient, giftUsed, err := deductUsageBillingBalance(ctx, tx, cmd.UserID, cmd.BalanceCost)
		if err != nil {
			return err
		}
		result.NewBalance = &newBalance
		result.BalanceOverdrafted = !sufficient
		result.ThresholdExemptCost = giftUsed
	}

	if cmd.APIKeyQuotaCost > 0 {
		exhausted, err := incrementUsageBillingAPIKeyQuota(ctx, tx, cmd.APIKeyID, cmd.APIKeyQuotaCost)
		if err != nil {
			return err
		}
		result.APIKeyQuotaExhausted = exhausted
	}

	if cmd.APIKeyRateLimitCost > 0 {
		if err := incrementUsageBillingAPIKeyRateLimit(ctx, tx, cmd.APIKeyID, cmd.APIKeyRateLimitCost); err != nil {
			return err
		}
	}

	if cmd.AccountQuotaCost > 0 && (strings.EqualFold(cmd.AccountType, service.AccountTypeAPIKey) || strings.EqualFold(cmd.AccountType, service.AccountTypeBedrock)) {
		quotaState, err := incrementUsageBillingAccountQuota(ctx, tx, cmd.AccountID, cmd.AccountQuotaCost)
		if err != nil {
			return err
		}
		result.QuotaState = quotaState
	}

	return nil
}

func incrementUsageBillingSubscription(ctx context.Context, tx *sql.Tx, subscriptionID int64, costUSD float64) error {
	const updateSQL = `
		UPDATE user_subscriptions us
		SET
			daily_usage_usd = us.daily_usage_usd + $1,
			weekly_usage_usd = us.weekly_usage_usd + $1,
			monthly_usage_usd = us.monthly_usage_usd + $1,
			updated_at = NOW()
		FROM groups g
		WHERE us.id = $2
			AND us.deleted_at IS NULL
			AND us.group_id = g.id
			AND g.deleted_at IS NULL
	`
	res, err := tx.ExecContext(ctx, updateSQL, costUSD, subscriptionID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	return service.ErrSubscriptionNotFound
}

// deductUsageBillingBalance locks the wallet row so total and gift balances are
// allocated from the same pre-deduction snapshot, including the overdraft path.
func deductUsageBillingBalance(ctx context.Context, tx *sql.Tx, userID int64, amount float64) (float64, bool, float64, error) {
	var newBalance, giftUsed float64
	var sufficient bool
	err := tx.QueryRowContext(ctx, `
		WITH wallet AS (
			SELECT
				id,
				balance,
				CASE
					WHEN gift_balance IS NULL OR gift_balance::text = 'NaN' OR gift_balance < 0 THEN 0
					ELSE gift_balance
				END AS old_gift_balance
			FROM users
			WHERE id = $2 AND deleted_at IS NULL
			FOR UPDATE
		), updated AS (
			UPDATE users AS u
			SET
				balance = wallet.balance - $1,
				gift_balance = GREATEST(wallet.old_gift_balance - $1, 0),
				updated_at = NOW()
			FROM wallet
			WHERE u.id = wallet.id
			RETURNING
				u.balance,
				wallet.balance >= $1 AS sufficient,
				LEAST(wallet.old_gift_balance, $1) AS gift_used
		)
		SELECT balance, sufficient, gift_used
		FROM updated
	`, amount, userID).Scan(&newBalance, &sufficient, &giftUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, 0, service.ErrUserNotFound
	}
	if err != nil {
		return 0, false, 0, err
	}

	giftUsed = service.QuantizeUsageBillingAmount(giftUsed)
	if giftUsed < 0 {
		giftUsed = 0
	}
	quantizedAmount := service.QuantizeUsageBillingAmount(amount)
	if giftUsed > quantizedAmount {
		giftUsed = quantizedAmount
	}
	return newBalance, sufficient, giftUsed, nil
}

func reserveUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 {
		if err := persistUsageBillingThresholdExemptCost(ctx, tx, cmd.RequestID, cmd.APIKeyID, 0); err != nil {
			return nil, err
		}
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	var balance, frozen, giftHeld float64
	err := tx.QueryRowContext(ctx, `
		WITH wallet_raw AS (
			SELECT
				id,
				balance,
				CASE WHEN gift_balance IS NULL OR gift_balance::text = 'NaN' OR gift_balance < 0 THEN 0 ELSE gift_balance END AS old_gift_balance,
				CASE WHEN frozen_balance IS NULL OR frozen_balance::text = 'NaN' OR frozen_balance < 0 THEN 0 ELSE frozen_balance END AS old_frozen_balance,
				CASE WHEN frozen_gift_balance IS NULL OR frozen_gift_balance::text = 'NaN' OR frozen_gift_balance < 0 THEN 0 ELSE frozen_gift_balance END AS raw_frozen_gift_balance
			FROM users
			WHERE id = $2 AND deleted_at IS NULL
			FOR UPDATE
		), wallet AS (
			SELECT *, LEAST(raw_frozen_gift_balance, old_frozen_balance) AS old_frozen_gift_balance
			FROM wallet_raw
		), updated AS (
			UPDATE users AS u
			SET
				balance = wallet.balance - $1,
				gift_balance = wallet.old_gift_balance - LEAST(wallet.old_gift_balance, $1),
				frozen_balance = wallet.old_frozen_balance + $1,
				frozen_gift_balance = wallet.old_frozen_gift_balance + LEAST(wallet.old_gift_balance, $1),
				updated_at = NOW()
			FROM wallet
			WHERE u.id = wallet.id AND wallet.balance >= $1
			RETURNING u.balance, u.frozen_balance, LEAST(wallet.old_gift_balance, $1) AS gift_held
		)
		SELECT balance, frozen_balance, gift_held FROM updated
	`, cmd.HoldAmount, cmd.UserID).Scan(&balance, &frozen, &giftHeld)
	if err == nil {
		giftHeld = clampUsageBillingDedupThresholdExemptCost(giftHeld, cmd.HoldAmount)
		if err := persistUsageBillingThresholdExemptCost(ctx, tx, cmd.RequestID, cmd.APIKeyID, giftHeld); err != nil {
			return nil, err
		}
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, cmd.UserID); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, service.ErrBatchImageInsufficientBalance
}

func captureUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.ActualAmount > cmd.HoldAmount {
		return nil, service.ErrBatchImageSettlementCostExceedsHold
	}
	giftHeld, held, err := lockBatchImageHoldGiftAllocation(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if !held {
		return nil, errBatchImageCaptureHoldNotReserved
	}
	released, err := batchImageBillingRequestExists(ctx, tx, service.BatchImageReleaseRequestID(cmd.BatchID), cmd.APIKeyID)
	if err != nil {
		return nil, err
	}
	if released {
		return nil, errBatchImageCaptureHoldReleased
	}
	if cmd.HoldAmount <= 0 && cmd.ActualAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	var balance, frozen, giftUsed float64
	err = tx.QueryRowContext(ctx, `
		WITH wallet_raw AS (
			SELECT
				id,
				balance,
				CASE WHEN gift_balance IS NULL OR gift_balance::text = 'NaN' OR gift_balance < 0 THEN 0 ELSE gift_balance END AS old_gift_balance,
				CASE WHEN frozen_balance IS NULL OR frozen_balance::text = 'NaN' OR frozen_balance < 0 THEN 0 ELSE frozen_balance END AS old_frozen_balance,
				CASE WHEN frozen_gift_balance IS NULL OR frozen_gift_balance::text = 'NaN' OR frozen_gift_balance < 0 THEN 0 ELSE frozen_gift_balance END AS raw_frozen_gift_balance
			FROM users
			WHERE id = $3 AND deleted_at IS NULL
			FOR UPDATE
		), wallet AS (
			SELECT *, LEAST(raw_frozen_gift_balance, old_frozen_balance) AS old_frozen_gift_balance
			FROM wallet_raw
		), allocation AS (
			SELECT
				*,
				LEAST(old_frozen_gift_balance, $1, $4) AS gift_in_hold,
				LEAST(LEAST(old_frozen_gift_balance, $1, $4), $2) AS gift_used
			FROM wallet
		), updated AS (
			UPDATE users AS u
			SET
				balance = allocation.balance + ($1 - $2),
				gift_balance = allocation.old_gift_balance + (allocation.gift_in_hold - allocation.gift_used),
				frozen_balance = GREATEST(allocation.old_frozen_balance - $1, 0),
				frozen_gift_balance = GREATEST(allocation.old_frozen_gift_balance - allocation.gift_in_hold, 0),
				updated_at = NOW()
			FROM allocation
			WHERE u.id = allocation.id AND allocation.old_frozen_balance >= $1
			RETURNING u.balance, u.frozen_balance, allocation.gift_used
		)
		SELECT balance, frozen_balance, gift_used FROM updated
	`, cmd.HoldAmount, cmd.ActualAmount, cmd.UserID, giftHeld).Scan(&balance, &frozen, &giftUsed)
	if err == nil {
		giftUsed = service.QuantizeUsageBillingAmount(giftUsed)
		if giftUsed < 0 {
			giftUsed = 0
		}
		if giftUsed > cmd.ActualAmount {
			giftUsed = cmd.ActualAmount
		}
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen, ThresholdExemptCost: giftUsed}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, cmd.UserID); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, errors.New("batch image frozen balance is insufficient")
}

func releaseUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	giftHeld, held, err := lockBatchImageHoldGiftAllocation(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if !held {
		logger.LegacyPrintf("repository.usage_billing", "[BatchImage] release skipped, hold was never reserved: batch=%s", cmd.BatchID)
		return nil, errBatchImageReleaseHoldNotReserved
	}
	captured, err := batchImageBillingRequestExists(ctx, tx, service.BatchImageCaptureRequestID(cmd.BatchID), cmd.APIKeyID)
	if err != nil {
		return nil, err
	}
	if captured {
		return nil, errBatchImageReleaseHoldCaptured
	}
	if cmd.HoldAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	var balance, frozen float64
	err = tx.QueryRowContext(ctx, `
		WITH wallet_raw AS (
			SELECT
				id,
				balance,
				CASE WHEN gift_balance IS NULL OR gift_balance::text = 'NaN' OR gift_balance < 0 THEN 0 ELSE gift_balance END AS old_gift_balance,
				CASE WHEN frozen_balance IS NULL OR frozen_balance::text = 'NaN' OR frozen_balance < 0 THEN 0 ELSE frozen_balance END AS old_frozen_balance,
				CASE WHEN frozen_gift_balance IS NULL OR frozen_gift_balance::text = 'NaN' OR frozen_gift_balance < 0 THEN 0 ELSE frozen_gift_balance END AS raw_frozen_gift_balance
			FROM users
			WHERE id = $2 AND deleted_at IS NULL
			FOR UPDATE
		), wallet AS (
			SELECT *, LEAST(raw_frozen_gift_balance, old_frozen_balance) AS old_frozen_gift_balance
			FROM wallet_raw
		), allocation AS (
			SELECT *, LEAST(old_frozen_gift_balance, $1, $3) AS gift_release
			FROM wallet
		), updated AS (
			UPDATE users AS u
			SET
				balance = allocation.balance + $1,
				gift_balance = allocation.old_gift_balance + allocation.gift_release,
				frozen_balance = GREATEST(allocation.old_frozen_balance - $1, 0),
				frozen_gift_balance = GREATEST(allocation.old_frozen_gift_balance - allocation.gift_release, 0),
				updated_at = NOW()
			FROM allocation
			WHERE u.id = allocation.id AND allocation.old_frozen_balance >= $1
			RETURNING u.balance, u.frozen_balance
		)
		SELECT balance, frozen_balance FROM updated
	`, cmd.HoldAmount, cmd.UserID, giftHeld).Scan(&balance, &frozen)
	if err == nil {
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, cmd.UserID); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, errors.New("batch image frozen balance is insufficient")
}

func lockBatchImageHoldGiftAllocation(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (float64, bool, error) {
	requestID := service.BatchImageHoldRequestID(cmd.BatchID)
	var giftHeld sql.NullFloat64
	err := tx.QueryRowContext(ctx, `
		SELECT threshold_exempt_cost
		FROM usage_billing_dedup
		WHERE request_id = $1 AND api_key_id = $2
		FOR UPDATE
	`, requestID, cmd.APIKeyID).Scan(&giftHeld)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
			SELECT threshold_exempt_cost
			FROM usage_billing_dedup_archive
			WHERE request_id = $1 AND api_key_id = $2
			FOR UPDATE
		`, requestID, cmd.APIKeyID).Scan(&giftHeld)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if !giftHeld.Valid {
		// Rolling-upgrade compatibility: predecessor reservations moved gift into
		// frozen_gift_balance before the per-hold allocation column existed. Passing
		// the hold amount lets the locked wallet SQL recover min(FG, H). A stored
		// numeric zero remains an explicit cash-only allocation.
		return cmd.HoldAmount, true, nil
	}
	return clampUsageBillingDedupThresholdExemptCost(giftHeld.Float64, cmd.HoldAmount), true, nil
}

func batchImageBillingRequestExists(ctx context.Context, tx *sql.Tx, requestID string, apiKeyID int64) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM usage_billing_dedup
		WHERE request_id = $1 AND api_key_id = $2
	`, requestID, apiKeyID).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	err = tx.QueryRowContext(ctx, `
		SELECT 1
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, requestID, apiKeyID).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func userExistsForBilling(ctx context.Context, tx *sql.Tx, userID int64) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func incrementUsageBillingAPIKeyQuota(ctx context.Context, tx *sql.Tx, apiKeyID int64, amount float64) (bool, error) {
	var exhausted bool
	err := tx.QueryRowContext(ctx, `
		UPDATE api_keys
		SET quota_used = quota_used + $1,
			status = CASE
				WHEN quota > 0
					AND status = $3
					AND quota_used < quota
					AND quota_used + $1 >= quota
				THEN $4
				ELSE status
			END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING quota > 0 AND quota_used >= quota AND quota_used - $1 < quota
	`, amount, apiKeyID, service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted).Scan(&exhausted)
	if errors.Is(err, sql.ErrNoRows) {
		return false, service.ErrAPIKeyNotFound
	}
	if err != nil {
		return false, err
	}
	return exhausted, nil
}

func incrementUsageBillingAPIKeyRateLimit(ctx context.Context, tx *sql.Tx, apiKeyID int64, cost float64) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE api_keys SET
			usage_5h = CASE WHEN window_5h_start IS NOT NULL AND window_5h_start + INTERVAL '5 hours' <= NOW() THEN $1 ELSE usage_5h + $1 END,
			usage_1d = CASE WHEN window_1d_start IS NOT NULL AND window_1d_start + INTERVAL '24 hours' <= NOW() THEN $1 ELSE usage_1d + $1 END,
			usage_7d = CASE WHEN window_7d_start IS NOT NULL AND window_7d_start + INTERVAL '7 days' <= NOW() THEN $1 ELSE usage_7d + $1 END,
			window_5h_start = CASE WHEN window_5h_start IS NULL OR window_5h_start + INTERVAL '5 hours' <= NOW() THEN NOW() ELSE window_5h_start END,
			window_1d_start = CASE WHEN window_1d_start IS NULL OR window_1d_start + INTERVAL '24 hours' <= NOW() THEN date_trunc('day', NOW()) ELSE window_1d_start END,
			window_7d_start = CASE WHEN window_7d_start IS NULL OR window_7d_start + INTERVAL '7 days' <= NOW() THEN date_trunc('day', NOW()) ELSE window_7d_start END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`, cost, apiKeyID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAPIKeyNotFound
	}
	return nil
}

func incrementUsageBillingAccountQuota(ctx context.Context, tx *sql.Tx, accountID int64, amount float64) (*service.AccountQuotaState, error) {
	rows, err := tx.QueryContext(ctx,
		`UPDATE accounts SET extra = (
			COALESCE(extra, '{}'::jsonb)
			|| jsonb_build_object('quota_used', COALESCE((extra->>'quota_used')::numeric, 0) + $1)
			|| CASE WHEN COALESCE((extra->>'quota_daily_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_daily_used',
					CASE WHEN `+dailyExpiredExpr+`
					THEN $1
					ELSE COALESCE((extra->>'quota_daily_used')::numeric, 0) + $1 END,
					'quota_daily_start',
					CASE WHEN `+dailyExpiredExpr+`
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_daily_start', `+nowUTC+`) END
				)
				|| CASE WHEN `+dailyExpiredExpr+` AND `+nextDailyResetAtExpr+` IS NOT NULL
				   THEN jsonb_build_object('quota_daily_reset_at', `+nextDailyResetAtExpr+`)
				   ELSE '{}'::jsonb END
			ELSE '{}'::jsonb END
			|| CASE WHEN COALESCE((extra->>'quota_weekly_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_weekly_used',
					CASE WHEN `+weeklyExpiredExpr+`
					THEN $1
					ELSE COALESCE((extra->>'quota_weekly_used')::numeric, 0) + $1 END,
					'quota_weekly_start',
					CASE WHEN `+weeklyExpiredExpr+`
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_weekly_start', `+nowUTC+`) END
				)
				|| CASE WHEN `+weeklyExpiredExpr+` AND `+nextWeeklyResetAtExpr+` IS NOT NULL
				   THEN jsonb_build_object('quota_weekly_reset_at', `+nextWeeklyResetAtExpr+`)
				   ELSE '{}'::jsonb END
			ELSE '{}'::jsonb END
		), updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING
			COALESCE((extra->>'quota_used')::numeric, 0),
			COALESCE((extra->>'quota_limit')::numeric, 0),
			COALESCE((extra->>'quota_daily_used')::numeric, 0),
			COALESCE((extra->>'quota_daily_limit')::numeric, 0),
			COALESCE((extra->>'quota_weekly_used')::numeric, 0),
			COALESCE((extra->>'quota_weekly_limit')::numeric, 0)`,
		amount, accountID)
	if err != nil {
		return nil, err
	}

	var state service.AccountQuotaState
	if rows.Next() {
		if err := rows.Scan(
			&state.TotalUsed, &state.TotalLimit,
			&state.DailyUsed, &state.DailyLimit,
			&state.WeeklyUsed, &state.WeeklyLimit,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
	} else {
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
		return nil, service.ErrAccountNotFound
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	// 必须在执行下一条 SQL 前显式关闭 rows：pq 驱动在同一连接上
	// 不允许前一条查询的结果集未耗尽时启动新查询，否则会返回
	// "unexpected Parse response" 错误。
	if err := rows.Close(); err != nil {
		return nil, err
	}
	// 任意维度额度在本次递增中从"未超"跨越到"已超"时，必须刷新调度快照，
	// 否则 Redis 中缓存的 Account 仍显示旧的 used 值，后续请求会继续选中本账号，
	// 最终观察到 daily_used / weekly_used 大幅超过配置的 limit。
	// 对于日/周额度，即使本次触发了周期重置（pre=0、post=amount），
	// 判定式 (post-amount) < limit 同样成立，逻辑与总额度保持一致。
	crossedTotal := state.TotalLimit > 0 && state.TotalUsed >= state.TotalLimit && (state.TotalUsed-amount) < state.TotalLimit
	crossedDaily := state.DailyLimit > 0 && state.DailyUsed >= state.DailyLimit && (state.DailyUsed-amount) < state.DailyLimit
	crossedWeekly := state.WeeklyLimit > 0 && state.WeeklyUsed >= state.WeeklyLimit && (state.WeeklyUsed-amount) < state.WeeklyLimit
	if crossedTotal || crossedDaily || crossedWeekly {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
			logger.LegacyPrintf("repository.usage_billing", "[SchedulerOutbox] enqueue quota exceeded failed: account=%d err=%v", accountID, err)
			return nil, err
		}
	}
	return &state, nil
}
