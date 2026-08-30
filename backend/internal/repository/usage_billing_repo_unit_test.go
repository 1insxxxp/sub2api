//go:build unit

package repository

import (
	"context"
	"database/sql"
	"math"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	lockedGiftBalanceDeductSQL = `(?s)WITH wallet AS \(.*SELECT.*balance.*gift_balance.*FROM users.*FOR UPDATE.*\).*UPDATE users.*SET balance =.*gift_balance =.*RETURNING.*balance.*sufficient.*gift_used`
	exactGiftBalanceUpdateSQL  = `(?s)gift_balance = GREATEST\(wallet\.old_gift_balance - \$1, 0\),`
	reserveBatchImageHoldSQL   = `(?s)WITH wallet_raw AS \(.*gift_balance.*frozen_balance.*frozen_gift_balance.*FOR UPDATE.*UPDATE users AS u.*gift_balance = wallet\.old_gift_balance - LEAST.*frozen_gift_balance = wallet\.old_frozen_gift_balance \+ LEAST.*RETURNING u\.balance, u\.frozen_balance`
	captureBatchImageHoldSQL   = `(?s)WITH wallet_raw AS \(.*gift_balance.*frozen_balance.*frozen_gift_balance.*FOR UPDATE.*gift_in_hold.*gift_used.*UPDATE users AS u.*gift_balance = allocation\.old_gift_balance \+.*frozen_gift_balance = GREATEST.*RETURNING u\.balance, u\.frozen_balance, allocation\.gift_used`
	releaseBatchImageHoldSQL   = `(?s)WITH wallet_raw AS \(.*gift_balance.*frozen_balance.*frozen_gift_balance.*FOR UPDATE.*gift_release.*UPDATE users AS u.*gift_balance = allocation\.old_gift_balance \+ allocation\.gift_release.*frozen_gift_balance = GREATEST.*RETURNING u\.balance, u\.frozen_balance`
	userExistsForBillingSQL    = `(?s)SELECT 1\s+FROM users\s+WHERE id = \$1 AND deleted_at IS NULL`
	holdGiftAllocationSQL      = `(?s)SELECT threshold_exempt_cost\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2\s+FOR UPDATE`
	archivedHoldGiftSQL        = `(?s)SELECT threshold_exempt_cost\s+FROM usage_billing_dedup_archive\s+WHERE request_id = \$1 AND api_key_id = \$2\s+FOR UPDATE`
	billingRequestExistsSQL    = `(?s)SELECT 1\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2`
	archivedRequestExistsSQL   = `(?s)SELECT 1\s+FROM usage_billing_dedup_archive\s+WHERE request_id = \$1 AND api_key_id = \$2`
)

func TestUsageBillingRepositoryApply_RejectsEveryNonFiniteMonetaryFieldBeforeTransaction(t *testing.T) {
	fields := []struct {
		name string
		set  func(*service.UsageBillingCommand, float64)
	}{
		{name: "balance", set: func(cmd *service.UsageBillingCommand, value float64) { cmd.BalanceCost = value }},
		{name: "subscription", set: func(cmd *service.UsageBillingCommand, value float64) { cmd.SubscriptionCost = value }},
		{name: "api key quota", set: func(cmd *service.UsageBillingCommand, value float64) { cmd.APIKeyQuotaCost = value }},
		{name: "api key rate limit", set: func(cmd *service.UsageBillingCommand, value float64) { cmd.APIKeyRateLimitCost = value }},
		{name: "account quota", set: func(cmd *service.UsageBillingCommand, value float64) { cmd.AccountQuotaCost = value }},
	}
	values := []struct {
		name  string
		value float64
		want  error
	}{
		{name: "nan", value: math.NaN(), want: service.ErrUsageBillingNonFiniteAmount},
		{name: "positive infinity", value: math.Inf(1), want: service.ErrUsageBillingNonFiniteAmount},
		{name: "negative infinity", value: math.Inf(-1), want: service.ErrUsageBillingNonFiniteAmount},
		{name: "negative", value: -0.01, want: service.ErrUsageBillingNegativeAmount},
	}

	for _, field := range fields {
		for _, value := range values {
			t.Run(field.name+"/"+value.name, func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				defer func() { _ = db.Close() }()
				cmd := &service.UsageBillingCommand{RequestID: "non-finite", APIKeyID: 2, UserID: 1}
				field.set(cmd, value.value)

				_, err = (&usageBillingRepository{db: db}).Apply(context.Background(), cmd)
				require.ErrorIs(t, err, value.want)
				require.NoError(t, mock.ExpectationsWereMet(), "the repository must reject before BEGIN")
			})
		}
	}
}

func TestDeductUsageBillingBalance_PreservesRemainingGiftAcrossOverdraft(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(exactGiftBalanceUpdateSQL).
		WithArgs(7.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "sufficient", "gift_used"}).AddRow(-2.0, false, 7.0))
	mock.ExpectCommit()

	newBalance, sufficient, giftUsed, err := deductUsageBillingBalance(ctx, tx, 42, 7)
	require.NoError(t, err)
	require.False(t, sufficient)
	require.InDelta(t, -2.0, newBalance, 0.00000001)
	require.InDelta(t, 7.0, giftUsed, 0.00000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_AllocatesGiftFirst(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(lockedGiftBalanceDeductSQL).
		WithArgs(12.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "sufficient", "gift_used"}).AddRow(8.0, true, 10.0))
	mock.ExpectCommit()

	newBalance, sufficient, giftUsed, err := deductUsageBillingBalance(ctx, tx, 42, 12)
	require.NoError(t, err)
	require.True(t, sufficient)
	require.InDelta(t, 8.0, newBalance, 0.00000001)
	require.InDelta(t, 10.0, giftUsed, 0.00000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_QuantizesGiftAllocation(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(lockedGiftBalanceDeductSQL).
		WithArgs(1.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "sufficient", "gift_used"}).AddRow(9.0, true, 0.123456789))
	mock.ExpectCommit()

	_, _, giftUsed, err := deductUsageBillingBalance(ctx, tx, 42, 1)
	require.NoError(t, err)
	require.Equal(t, 0.12345679, giftUsed)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_UsesSufficientBalanceGuard(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(lockedGiftBalanceDeductSQL).
		WithArgs(2.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "sufficient", "gift_used"}).AddRow(7.5, true, 0.0))
	mock.ExpectCommit()

	newBalance, sufficient, giftUsed, err := deductUsageBillingBalance(ctx, tx, 42, 2.5)
	require.NoError(t, err)
	require.True(t, sufficient)
	require.InDelta(t, 7.5, newBalance, 0.000001)
	require.Zero(t, giftUsed)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_RecordsOverdraftWhenGuardMisses(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(lockedGiftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "sufficient", "gift_used"}).AddRow(-5.0, false, 3.0))
	mock.ExpectCommit()

	newBalance, sufficient, giftUsed, err := deductUsageBillingBalance(ctx, tx, 42, 10)
	require.NoError(t, err)
	require.False(t, sufficient)
	require.InDelta(t, -5.0, newBalance, 0.000001)
	require.InDelta(t, 3.0, giftUsed, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffects_FlagsBalanceOverdraft(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(lockedGiftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "sufficient", "gift_used"}).AddRow(-5.0, false, 3.0))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:      42,
		BalanceCost: 10,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, -5.0, *result.NewBalance, 0.000001)
	require.True(t, result.BalanceOverdrafted)
	require.InDelta(t, 3.0, result.ThresholdExemptCost, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_ReturnsUserNotFoundWhenNoUserUpdated(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(lockedGiftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, _, _, err = deductUsageBillingBalance(ctx, tx, 42, 10)
	require.ErrorIs(t, err, service.ErrUserNotFound)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveUsageBillingBatchImageBalance_MovesAvailableToFrozen(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(reserveBatchImageHoldSQL).
		WithArgs(2.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance", "gift_held"}).AddRow(7.5, 2.5, 1.5))
	mock.ExpectExec(`UPDATE usage_billing_dedup\s+SET threshold_exempt_cost = \$1`).
		WithArgs(1.5, "batch_image_hold:test", int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := reserveUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{
		RequestID: "batch_image_hold:test", APIKeyID: 7, UserID: 42, HoldAmount: 2.5,
	})
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.NotNil(t, result.FrozenBalance)
	require.InDelta(t, 7.5, *result.NewBalance, 0.000001)
	require.InDelta(t, 2.5, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveUsageBillingBatchImageBalance_InsufficientBalance(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(reserveBatchImageHoldSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(userExistsForBillingSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))
	mock.ExpectRollback()

	_, err = reserveUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 10})
	require.ErrorIs(t, err, service.ErrBatchImageInsufficientBalance)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_ReleasesRemainder(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(holdGiftAllocationSQL).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_capture"), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"threshold_exempt_cost"}).AddRow(0.25))
	mock.ExpectQuery(billingRequestExistsSQL).
		WithArgs(service.BatchImageReleaseRequestID("imgbatch_capture"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(archivedRequestExistsSQL).
		WithArgs(service.BatchImageReleaseRequestID("imgbatch_capture"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(captureBatchImageHoldSQL).
		WithArgs(1.0, 0.25, int64(42), 0.25).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance", "gift_used"}).AddRow(9.75, 0.0, 0.25))
	mock.ExpectCommit()

	result, err := captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{
		UserID: 42, APIKeyID: 7, BatchID: "imgbatch_capture", HoldAmount: 1, ActualAmount: 0.25,
	})
	require.NoError(t, err)
	require.InDelta(t, 9.75, *result.NewBalance, 0.000001)
	require.InDelta(t, 0.0, *result.FrozenBalance, 0.000001)
	require.InDelta(t, 0.25, result.ThresholdExemptCost, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_RejectsActualCostOverHold(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectRollback()

	_, err = captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 0.5, ActualAmount: 1})
	require.ErrorIs(t, err, service.ErrBatchImageSettlementCostExceedsHold)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUsageBillingBatchImageBalance_ReturnsFrozenToAvailable(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(holdGiftAllocationSQL).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_release"), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"threshold_exempt_cost"}).AddRow(1.0))
	mock.ExpectQuery(billingRequestExistsSQL).
		WithArgs(service.BatchImageCaptureRequestID("imgbatch_release"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(archivedRequestExistsSQL).
		WithArgs(service.BatchImageCaptureRequestID("imgbatch_release"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(releaseBatchImageHoldSQL).
		WithArgs(1.0, int64(42), 1.0).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(10.0, 0.0))
	mock.ExpectCommit()

	result, err := releaseUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, APIKeyID: 7, BatchID: "imgbatch_release", HoldAmount: 1})
	require.NoError(t, err)
	require.InDelta(t, 10.0, *result.NewBalance, 0.000001)
	require.InDelta(t, 0.0, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUsageBillingBatchImageBalance_SkipsWhenHoldNeverReserved(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	// dedup 与归档表均无 hold claim：说明该 job 从未成功冻结，
	// 释放必须跳过，不得从他人冻结资金池中凭空生成余额。
	mock.ExpectQuery(holdGiftAllocationSQL).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_phantom"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(archivedHoldGiftSQL).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_phantom"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	result, err := releaseUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, APIKeyID: 7, BatchID: "imgbatch_phantom", HoldAmount: 1})
	require.ErrorIs(t, err, errBatchImageReleaseHoldNotReserved)
	require.Nil(t, result)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageBillingRepositoryBatchImage_RejectsNonFiniteAmountsBeforeClaim(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &usageBillingRepository{db: db}

	for _, amount := range []float64{math.NaN(), math.Inf(1), -1} {
		_, err := repo.ReserveBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
			RequestID: "invalid-batch-amount", APIKeyID: 1, UserID: 2, HoldAmount: amount,
		})
		require.Error(t, err)
	}
	require.NoError(t, mock.ExpectationsWereMet(), "invalid amounts must not claim an idempotency key")
}
