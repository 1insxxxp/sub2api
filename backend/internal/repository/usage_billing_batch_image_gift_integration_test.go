//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type batchImageWalletState struct {
	Balance           float64
	GiftBalance       float64
	FrozenBalance     float64
	FrozenGiftBalance float64
}

func createBatchImageGiftWallet(t *testing.T, balance, gift float64) (*usageBillingRepository, *service.User, *service.APIKey) {
	t.Helper()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB).(*usageBillingRepository)
	user := mustCreateUser(t, client, &service.User{
		Email: "batch-image-gift-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: balance,
	})
	_, err := integrationDB.ExecContext(context.Background(),
		"UPDATE users SET gift_balance = $1, frozen_balance = 0, frozen_gift_balance = 0 WHERE id = $2", gift, user.ID)
	require.NoError(t, err)
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-batch-gift-" + uuid.NewString(), Name: "batch-gift"})
	return repo, user, apiKey
}

func readBatchImageWallet(t *testing.T, userID int64) batchImageWalletState {
	t.Helper()
	var state batchImageWalletState
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), `
		SELECT balance, gift_balance, frozen_balance, frozen_gift_balance
		FROM users WHERE id = $1
	`, userID).Scan(&state.Balance, &state.GiftBalance, &state.FrozenBalance, &state.FrozenGiftBalance))
	return state
}

func reserveBatchImageGift(t *testing.T, repo *usageBillingRepository, user *service.User, apiKey *service.APIKey, batchID string, hold float64) *service.BatchImageBalanceHoldResult {
	t.Helper()
	result, err := repo.ReserveBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
		RequestID: service.BatchImageHoldRequestID(batchID), APIKeyID: apiKey.ID,
		UserID: user.ID, BatchID: batchID, HoldAmount: hold,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	return result
}

func TestUsageBillingRepositoryBatchImage_ReserveAndCaptureGiftAllocation(t *testing.T) {
	tests := []struct {
		name         string
		balance      float64
		gift         float64
		hold         float64
		actual       float64
		wantReserved batchImageWalletState
		wantCaptured batchImageWalletState
		wantGiftUsed float64
	}{
		{
			name: "full gift partial actual", balance: 20, gift: 20, hold: 12, actual: 7,
			wantReserved: batchImageWalletState{Balance: 8, GiftBalance: 8, FrozenBalance: 12, FrozenGiftBalance: 12},
			wantCaptured: batchImageWalletState{Balance: 13, GiftBalance: 13}, wantGiftUsed: 7,
		},
		{
			name: "mixed gift partial actual", balance: 20, gift: 10, hold: 12, actual: 7,
			wantReserved: batchImageWalletState{Balance: 8, GiftBalance: 0, FrozenBalance: 12, FrozenGiftBalance: 10},
			wantCaptured: batchImageWalletState{Balance: 13, GiftBalance: 3}, wantGiftUsed: 7,
		},
		{
			name: "mixed gift exceeds gift", balance: 20, gift: 10, hold: 12, actual: 11,
			wantReserved: batchImageWalletState{Balance: 8, GiftBalance: 0, FrozenBalance: 12, FrozenGiftBalance: 10},
			wantCaptured: batchImageWalletState{Balance: 9, GiftBalance: 0}, wantGiftUsed: 10,
		},
		{
			name: "zero actual refunds all sources", balance: 20, gift: 10, hold: 12, actual: 0,
			wantReserved: batchImageWalletState{Balance: 8, GiftBalance: 0, FrozenBalance: 12, FrozenGiftBalance: 10},
			wantCaptured: batchImageWalletState{Balance: 20, GiftBalance: 10}, wantGiftUsed: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, user, apiKey := createBatchImageGiftWallet(t, tt.balance, tt.gift)
			batchID := "batch-gift-" + uuid.NewString()
			reserveBatchImageGift(t, repo, user, apiKey, batchID, tt.hold)
			require.Equal(t, tt.wantReserved, readBatchImageWallet(t, user.ID))

			result, err := repo.CaptureBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
				RequestID: service.BatchImageCaptureRequestID(batchID), APIKeyID: apiKey.ID,
				UserID: user.ID, BatchID: batchID, HoldAmount: tt.hold, ActualAmount: tt.actual,
			})
			require.NoError(t, err)
			require.True(t, result.Applied)
			require.InDelta(t, tt.wantGiftUsed, result.ThresholdExemptCost, 0.00000001)
			require.Equal(t, tt.wantCaptured, readBatchImageWallet(t, user.ID))
		})
	}
}

func TestUsageBillingRepositoryBatchImage_LegacyHoldDoesNotConsumeAvailableGift(t *testing.T) {
	repo, user, apiKey := createBatchImageGiftWallet(t, 8, 10)
	batchID := "batch-legacy-" + uuid.NewString()
	_, err := integrationDB.ExecContext(context.Background(), `
		UPDATE users SET frozen_balance = 12, frozen_gift_balance = 0 WHERE id = $1
	`, user.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(context.Background(), `
		INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint)
		VALUES ($1, $2, 'legacy-hold')
	`, service.BatchImageHoldRequestID(batchID), apiKey.ID)
	require.NoError(t, err)

	result, err := repo.CaptureBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
		RequestID: service.BatchImageCaptureRequestID(batchID), APIKeyID: apiKey.ID,
		UserID: user.ID, BatchID: batchID, HoldAmount: 12, ActualAmount: 7,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Zero(t, result.ThresholdExemptCost)
	require.Equal(t, batchImageWalletState{Balance: 13, GiftBalance: 10}, readBatchImageWallet(t, user.ID))
}

func TestUsageBillingRepositoryBatchImage_ReleaseReturnsFrozenGift(t *testing.T) {
	repo, user, apiKey := createBatchImageGiftWallet(t, 20, 10)
	batchID := "batch-release-gift-" + uuid.NewString()
	reserveBatchImageGift(t, repo, user, apiKey, batchID, 12)

	cmd := &service.BatchImageBalanceHoldCommand{
		RequestID: service.BatchImageReleaseRequestID(batchID), APIKeyID: apiKey.ID,
		UserID: user.ID, BatchID: batchID, HoldAmount: 12,
	}
	result, err := repo.ReleaseBatchImageBalance(context.Background(), cmd)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Zero(t, result.ThresholdExemptCost)
	want := batchImageWalletState{Balance: 20, GiftBalance: 10}
	require.Equal(t, want, readBatchImageWallet(t, user.ID))
	duplicate, err := repo.ReleaseBatchImageBalance(context.Background(), cmd)
	require.NoError(t, err)
	require.False(t, duplicate.Applied)
	require.Equal(t, want, readBatchImageWallet(t, user.ID))
}

func TestUsageBillingRepositoryBatchImage_CaptureRejectsOverHoldAndDeduplicates(t *testing.T) {
	repo, user, apiKey := createBatchImageGiftWallet(t, 20, 10)
	batchID := "batch-capture-dedup-" + uuid.NewString()
	reserveBatchImageGift(t, repo, user, apiKey, batchID, 12)
	before := readBatchImageWallet(t, user.ID)

	_, err := repo.CaptureBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
		RequestID: service.BatchImageCaptureRequestID(batchID), APIKeyID: apiKey.ID,
		UserID: user.ID, BatchID: batchID, HoldAmount: 12, ActualAmount: 13,
	})
	require.ErrorIs(t, err, service.ErrBatchImageSettlementCostExceedsHold)
	require.Equal(t, before, readBatchImageWallet(t, user.ID))

	cmd := &service.BatchImageBalanceHoldCommand{
		RequestID: service.BatchImageCaptureRequestID(batchID), APIKeyID: apiKey.ID,
		UserID: user.ID, BatchID: batchID, HoldAmount: 12, ActualAmount: 7,
	}
	first, err := repo.CaptureBatchImageBalance(context.Background(), cmd)
	require.NoError(t, err)
	require.True(t, first.Applied)
	after := readBatchImageWallet(t, user.ID)
	duplicate, err := repo.CaptureBatchImageBalance(context.Background(), cmd)
	require.NoError(t, err)
	require.False(t, duplicate.Applied)
	require.Zero(t, duplicate.ThresholdExemptCost)
	require.Equal(t, after, readBatchImageWallet(t, user.ID))
}

func TestUsageBillingRepositoryBatchImage_ConcurrentReservationsPreserveGiftPools(t *testing.T) {
	repo, user, apiKey := createBatchImageGiftWallet(t, 20, 10)
	start := make(chan struct{})
	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := repo.ReserveBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
				RequestID: fmt.Sprintf("batch-concurrent-hold-%s-%d", uuid.NewString(), i), APIKeyID: apiKey.ID,
				UserID: user.ID, BatchID: fmt.Sprintf("batch-concurrent-%d", i), HoldAmount: 8,
			})
			errCh <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
	require.Equal(t, batchImageWalletState{Balance: 4, GiftBalance: 0, FrozenBalance: 16, FrozenGiftBalance: 10}, readBatchImageWallet(t, user.ID))
}

func TestUsageBillingRepositoryBatchImage_SanitizesCorruptNaNPools(t *testing.T) {
	repo, user, apiKey := createBatchImageGiftWallet(t, 20, 0)
	_, err := integrationDB.ExecContext(context.Background(), `
		UPDATE users
		SET gift_balance = 'NaN'::numeric,
			frozen_balance = 'NaN'::numeric,
			frozen_gift_balance = 'NaN'::numeric
		WHERE id = $1
	`, user.ID)
	require.NoError(t, err)

	reserveBatchImageGift(t, repo, user, apiKey, "batch-nan-"+uuid.NewString(), 5)
	require.Equal(t, batchImageWalletState{Balance: 15, GiftBalance: 0, FrozenBalance: 5, FrozenGiftBalance: 0}, readBatchImageWallet(t, user.ID))
}
