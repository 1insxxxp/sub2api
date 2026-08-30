//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
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
