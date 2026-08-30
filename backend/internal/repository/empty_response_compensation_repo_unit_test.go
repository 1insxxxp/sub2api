//go:build unit

package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestEmptyResponseCompensationBalanceTransactionRefundsExactlyOnce(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := newEmptyResponseCompensationRepository(db)
	createdAt := time.Now().Add(-time.Hour)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, usage_log_id.*FROM empty_response_claims.*FOR UPDATE").WithArgs(int64(50)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "usage_log_id", "user_id", "api_key_id", "group_id", "subscription_id", "original_actual_cost"}).
			AddRow(service.EmptyResponseClaimApproved, 60, 7, 8, 9, nil, 1.25))
	mock.ExpectQuery("SELECT actual_cost.*FROM usage_logs.*FOR UPDATE").WithArgs(int64(60)).
		WillReturnRows(sqlmock.NewRows([]string{"actual_cost", "compensated_cost", "billing_type", "created_at"}).AddRow(1.25, 0, service.BillingTypeBalance, createdAt))
	mock.ExpectQuery("SELECT balance.*FROM users.*FOR UPDATE").WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(3.0))
	mock.ExpectQuery("SELECT key, quota_used.*FROM api_keys.*FOR UPDATE").WithArgs(int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"key", "quota_used"}).AddRow("sk-test", 2.0))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET balance = balance + $1, updated_at = NOW() WHERE id = $2")).WithArgs(1.25, int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO redeem_codes").
		WithArgs("EMPTY-COMP-50", "empty_response", 1.25, service.StatusUsed, int64(7), "empty_response_compensation", "Empty response compensation for usage log #60").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE api_keys SET quota_used = GREATEST(quota_used - $1, 0), updated_at = NOW() WHERE id = $2")).WithArgs(1.25, int64(8)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE usage_logs SET compensated_cost = \\$1.*actual_cost = \\$2").WithArgs(1.25, 1.25, int64(60), float64(0)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE empty_response_claims.*status = \\$1.*balance_refund = \\$2.*subscription_refund = 0.*api_key_quota_refund = \\$2").
		WithArgs(service.EmptyResponseClaimCompensated, 1.25, int64(50), service.EmptyResponseClaimApproved).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := repo.Compensate(context.Background(), 50)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, 1.25, result.RefundAmount)
	require.Equal(t, "sk-test", result.APIKey)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEmptyResponseCompensationSubscriptionOnlyReversesContainingWindows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := newEmptyResponseCompensationRepository(db)
	createdAt := time.Now().Add(-time.Hour)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, usage_log_id.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"status", "usage_log_id", "user_id", "api_key_id", "group_id", "subscription_id", "original_actual_cost"}).AddRow(service.EmptyResponseClaimApproved, 60, 7, 8, 9, 10, 0.75))
	mock.ExpectQuery("SELECT actual_cost.*FROM usage_logs.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"actual_cost", "compensated_cost", "billing_type", "created_at"}).AddRow(0.75, 0, service.BillingTypeSubscription, createdAt))
	mock.ExpectQuery("SELECT balance.*FROM users.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(3.0))
	mock.ExpectQuery("SELECT key, quota_used.*FROM api_keys.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"key", "quota_used"}).AddRow("sk-test", 2.0))
	mock.ExpectQuery("SELECT starts_at, expires_at.*FROM user_subscriptions.*FOR UPDATE").WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"starts_at", "expires_at", "daily_window_start", "weekly_window_start", "monthly_window_start"}).AddRow(createdAt.Add(-time.Hour), createdAt.Add(time.Hour), createdAt.Add(-time.Hour), createdAt.Add(-8*24*time.Hour), createdAt.Add(-time.Hour)))
	mock.ExpectExec("UPDATE user_subscriptions SET.*daily_usage_usd = CASE.*weekly_usage_usd = CASE.*monthly_usage_usd = CASE").WithArgs(0.75, createdAt, int64(10)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE api_keys SET quota_used").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE usage_logs SET compensated_cost").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE empty_response_claims.*subscription_refund = \\$2").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := repo.Compensate(context.Background(), 50)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.SubscriptionID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEmptyResponseCompensationRollsBackOnMidTransactionFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := newEmptyResponseCompensationRepository(db)
	createdAt := time.Now().Add(-time.Hour)
	forcedErr := errors.New("forced update failure")

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, usage_log_id.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"status", "usage_log_id", "user_id", "api_key_id", "group_id", "subscription_id", "original_actual_cost"}).AddRow(service.EmptyResponseClaimApproved, 60, 7, 8, 9, nil, 1.25))
	mock.ExpectQuery("SELECT actual_cost.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"actual_cost", "compensated_cost", "billing_type", "created_at"}).AddRow(1.25, 0, service.BillingTypeBalance, createdAt))
	mock.ExpectQuery("SELECT balance.*FROM users.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(3.0))
	mock.ExpectQuery("SELECT key, quota_used.*FROM api_keys.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"key", "quota_used"}).AddRow("sk-test", 2.0))
	mock.ExpectExec("UPDATE users SET balance").WillReturnError(forcedErr)
	mock.ExpectRollback()

	result, err := repo.Compensate(context.Background(), 50)
	require.Nil(t, result)
	require.ErrorIs(t, err, forcedErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
