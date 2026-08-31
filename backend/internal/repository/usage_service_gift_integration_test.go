//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestUsageServiceCreate_PersistsGiftAllocationAndDeduplicates(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	userRepo := newUserRepositoryWithSQL(client, integrationDB)
	usageRepo := NewUsageLogRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email: "usage-service-gift-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 20,
	})
	_, err := integrationDB.ExecContext(ctx, "UPDATE users SET gift_balance = 10 WHERE id = $1", user.ID)
	require.NoError(t, err)
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-usage-service-" + uuid.NewString(), Name: "usage-service"})
	account := mustCreateAccount(t, client, &service.Account{Name: "usage-service-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	svc := service.NewUsageService(usageRepo, userRepo, client, nil)
	req := service.CreateUsageLogRequest{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
		RequestID: "usage-service-gift-" + uuid.NewString(), Model: "test-model",
		ActualCost: 12, TotalCost: 12, RateMultiplier: 1,
	}

	first, err := svc.Create(ctx, req)
	require.NoError(t, err)
	require.InDelta(t, 10, first.ThresholdExemptCost, 0.00000001)
	duplicate, err := svc.Create(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, duplicate)
	require.InDelta(t, 10, duplicate.ThresholdExemptCost, 0.00000001)

	var balance, gift float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance, gift_balance FROM users WHERE id = $1", user.ID).Scan(&balance, &gift))
	require.InDelta(t, 8, balance, 0.00000001)
	require.Zero(t, gift)
	var count int
	var exempt float64
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*), COALESCE(MAX(threshold_exempt_cost), 0) FROM usage_logs WHERE request_id = $1 AND api_key_id = $2",
		req.RequestID, apiKey.ID,
	).Scan(&count, &exempt))
	require.Equal(t, 1, count)
	require.InDelta(t, 10, exempt, 0.00000001)
}

func TestUsageServiceCreate_RejectsBlankRequestIDBeforeLogOrWalletMutation(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	userRepo := newUserRepositoryWithSQL(client, integrationDB)
	usageRepo := NewUsageLogRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email: "usage-service-blank-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 20,
	})
	_, err := integrationDB.ExecContext(ctx, "UPDATE users SET gift_balance = 10 WHERE id = $1", user.ID)
	require.NoError(t, err)
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-usage-blank-" + uuid.NewString(), Name: "usage-blank"})
	account := mustCreateAccount(t, client, &service.Account{Name: "usage-blank-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	svc := service.NewUsageService(usageRepo, userRepo, client, nil)

	for i := 0; i < 2; i++ {
		_, err := svc.Create(ctx, service.CreateUsageLogRequest{
			UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
			RequestID: "   ", Model: "test-model", ActualCost: 12, TotalCost: 12, RateMultiplier: 1,
		})
		require.ErrorIs(t, err, service.ErrUsageBillingRequestIDRequired)
	}

	var blankRows int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM usage_logs WHERE api_key_id = $1 AND BTRIM(request_id) = ''", apiKey.ID,
	).Scan(&blankRows))
	require.Zero(t, blankRows)
	require.Equal(t, batchImageWalletState{Balance: 20, GiftBalance: 10}, readBatchImageWallet(t, user.ID))
}

func TestUsageServiceCreate_UsesWalletQuantizedActualForGiftAttribution(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	userRepo := newUserRepositoryWithSQL(client, integrationDB)
	usageRepo := NewUsageLogRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email: "usage-service-precision-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 1,
	})
	_, err := integrationDB.ExecContext(ctx, "UPDATE users SET gift_balance = 1 WHERE id = $1", user.ID)
	require.NoError(t, err)
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-usage-precision-" + uuid.NewString(), Name: "usage-precision"})
	account := mustCreateAccount(t, client, &service.Account{Name: "usage-precision-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	svc := service.NewUsageService(usageRepo, userRepo, client, nil)

	usageLog, err := svc.Create(ctx, service.CreateUsageLogRequest{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
		RequestID: "usage-service-precision-" + uuid.NewString(), Model: "test-model",
		ActualCost: 0.000078125, TotalCost: 0.000078125, RateMultiplier: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 0.00007813, usageLog.ActualCost)
	require.Equal(t, usageLog.ActualCost, usageLog.ThresholdExemptCost)

	var balance, gift float64
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT balance, gift_balance FROM users WHERE id = $1", user.ID,
	).Scan(&balance, &gift))
	require.InDelta(t, 0.00007813, 1-balance, 1e-12)
	require.InDelta(t, usageLog.ThresholdExemptCost, 1-gift, 1e-12)
}

type failingUsageServiceGiftRepo struct {
	service.UserRepository
	err error
}

type attributionBlindUsageServiceUserRepo struct{ service.UserRepository }

type failingUsageThresholdRepo struct {
	service.UsageLogRepository
	err error
}

func (r *failingUsageThresholdRepo) UpdateThresholdExemptCost(context.Context, int64, float64) error {
	return r.err
}

func (r *failingUsageServiceGiftRepo) DeductBalanceWithGiftAllocation(context.Context, int64, float64) (service.BalanceDeductionResult, error) {
	return service.BalanceDeductionResult{}, r.err
}

func (r *failingUsageServiceGiftRepo) UpdateBalance(context.Context, int64, float64) error {
	return r.err
}

func TestUsageServiceCreate_RollsBackUsageLogWhenGiftDeductionFails(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	realUserRepo := newUserRepositoryWithSQL(client, integrationDB)
	usageRepo := NewUsageLogRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email: "usage-service-rollback-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 20,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-usage-rollback-" + uuid.NewString(), Name: "usage-rollback"})
	account := mustCreateAccount(t, client, &service.Account{Name: "usage-rollback-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	wantErr := errors.New("forced gift deduction failure")
	svc := service.NewUsageService(usageRepo, &failingUsageServiceGiftRepo{UserRepository: realUserRepo, err: wantErr}, client, nil)
	requestID := "usage-service-rollback-" + uuid.NewString()

	_, err := svc.Create(ctx, service.CreateUsageLogRequest{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
		RequestID: requestID, Model: "test-model", ActualCost: 2, TotalCost: 2, RateMultiplier: 1,
	})
	require.ErrorIs(t, err, wantErr)

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID,
	).Scan(&count))
	require.Zero(t, count, fmt.Sprintf("usage row for %s must roll back", requestID))
	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
	require.InDelta(t, 20, balance, 0.00000001)
}

func TestUsageServiceCreate_FailsClosedWithoutGiftAllocationCapability(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	realUserRepo := newUserRepositoryWithSQL(client, integrationDB)
	usageRepo := NewUsageLogRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email: "usage-service-missing-capability-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 20,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-usage-missing-" + uuid.NewString(), Name: "usage-missing"})
	account := mustCreateAccount(t, client, &service.Account{Name: "usage-missing-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	requestID := "usage-service-missing-" + uuid.NewString()
	svc := service.NewUsageService(usageRepo, &attributionBlindUsageServiceUserRepo{UserRepository: realUserRepo}, client, nil)

	_, err := svc.Create(ctx, service.CreateUsageLogRequest{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
		RequestID: requestID, Model: "test-model", ActualCost: 2, TotalCost: 2, RateMultiplier: 1,
	})
	require.ErrorIs(t, err, service.ErrGiftAllocatingBalanceRepositoryRequired)

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID,
	).Scan(&count))
	require.Zero(t, count)
	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
	require.InDelta(t, 20, balance, 0.00000001)
}

func TestUsageServiceCreate_RollsBackWalletWhenAttributionUpdateFails(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	userRepo := newUserRepositoryWithSQL(client, integrationDB)
	realUsageRepo := NewUsageLogRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email: "usage-service-attribution-rollback-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 20,
	})
	_, err := integrationDB.ExecContext(ctx, "UPDATE users SET gift_balance = 10 WHERE id = $1", user.ID)
	require.NoError(t, err)
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-usage-attribution-" + uuid.NewString(), Name: "usage-attribution"})
	account := mustCreateAccount(t, client, &service.Account{Name: "usage-attribution-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	wantErr := errors.New("forced attribution update failure")
	usageRepo := &failingUsageThresholdRepo{UsageLogRepository: realUsageRepo, err: wantErr}
	svc := service.NewUsageService(usageRepo, userRepo, client, nil)
	requestID := "usage-service-attribution-" + uuid.NewString()

	_, err = svc.Create(ctx, service.CreateUsageLogRequest{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
		RequestID: requestID, Model: "test-model", ActualCost: 12, TotalCost: 12, RateMultiplier: 1,
	})
	require.ErrorIs(t, err, wantErr)

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID,
	).Scan(&count))
	require.Zero(t, count)
	require.Equal(t, batchImageWalletState{Balance: 20, GiftBalance: 10}, readBatchImageWallet(t, user.ID))
}

func TestUsageServiceCreate_RejectsNonFiniteActualBeforeLogOrWalletMutation(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	userRepo := newUserRepositoryWithSQL(client, integrationDB)
	usageRepo := NewUsageLogRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email: "usage-service-nan-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 20,
	})
	_, err := integrationDB.ExecContext(ctx, "UPDATE users SET gift_balance = 10 WHERE id = $1", user.ID)
	require.NoError(t, err)
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-usage-nan-" + uuid.NewString(), Name: "usage-nan"})
	account := mustCreateAccount(t, client, &service.Account{Name: "usage-nan-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	requestID := "usage-service-nan-" + uuid.NewString()
	svc := service.NewUsageService(usageRepo, userRepo, client, nil)

	_, err = svc.Create(ctx, service.CreateUsageLogRequest{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
		RequestID: requestID, Model: "test-model", ActualCost: math.NaN(), TotalCost: 1, RateMultiplier: 1,
	})
	require.ErrorIs(t, err, service.ErrUsageBillingNonFiniteAmount)

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID,
	).Scan(&count))
	require.Zero(t, count)
	require.Equal(t, batchImageWalletState{Balance: 20, GiftBalance: 10}, readBatchImageWallet(t, user.ID))
}

func TestUsageLogRepository_LegacyBillingTransactionRollsBackWalletAndLog(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	userRepo := newUserRepositoryWithSQL(client, integrationDB)
	usageRepo := NewUsageLogRepository(client, integrationDB)
	runner := usageRepo.(service.UsageBillingTransactionRunner)
	thresholdRepo := usageRepo.(service.UsageLogThresholdExemptRepository)
	user := mustCreateUser(t, client, &service.User{
		Email: "legacy-billing-runner-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 20,
	})
	_, err := integrationDB.ExecContext(ctx, "UPDATE users SET gift_balance = 10 WHERE id = $1", user.ID)
	require.NoError(t, err)
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-legacy-runner-" + uuid.NewString(), Name: "legacy-runner"})
	account := mustCreateAccount(t, client, &service.Account{Name: "legacy-runner-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	requestID := "legacy-runner-" + uuid.NewString()
	wantErr := errors.New("force transaction rollback")

	err = runner.RunUsageBillingTransaction(ctx, func(txCtx context.Context) error {
		usageLog := &service.UsageLog{
			UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
			RequestID: requestID, Model: "test-model", ActualCost: 12, TotalCost: 12,
		}
		inserted, err := usageRepo.Create(txCtx, usageLog)
		require.NoError(t, err)
		require.True(t, inserted)
		result, err := userRepo.DeductBalanceWithGiftAllocation(txCtx, user.ID, 12)
		require.NoError(t, err)
		usageLog.ThresholdExemptCost = result.ThresholdExemptCost
		require.NoError(t, thresholdRepo.UpdateThresholdExemptCost(txCtx, usageLog.ID, usageLog.ThresholdExemptCost))
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID,
	).Scan(&count))
	require.Zero(t, count)
	require.Equal(t, batchImageWalletState{Balance: 20, GiftBalance: 10}, readBatchImageWallet(t, user.ID))
}
