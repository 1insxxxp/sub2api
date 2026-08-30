//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

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

func readBatchImageHoldGift(t *testing.T, batchID string, apiKeyID int64) sql.NullFloat64 {
	t.Helper()
	var heldGift sql.NullFloat64
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), `
		SELECT threshold_exempt_cost
		FROM usage_billing_dedup
		WHERE request_id = $1 AND api_key_id = $2
	`, service.BatchImageHoldRequestID(batchID), apiKeyID).Scan(&heldGift))
	return heldGift
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
		{
			name: "zero hold and actual", balance: 20, gift: 10, hold: 0, actual: 0,
			wantReserved: batchImageWalletState{Balance: 20, GiftBalance: 10},
			wantCaptured: batchImageWalletState{Balance: 20, GiftBalance: 10}, wantGiftUsed: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, user, apiKey := createBatchImageGiftWallet(t, tt.balance, tt.gift)
			batchID := "batch-gift-" + uuid.NewString()
			reserveBatchImageGift(t, repo, user, apiKey, batchID, tt.hold)
			heldGift := readBatchImageHoldGift(t, batchID, apiKey.ID)
			require.True(t, heldGift.Valid)
			require.InDelta(t, min(tt.gift, tt.hold), heldGift.Float64, 0.00000001)
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
	repo, user, apiKey := createBatchImageGiftWallet(t, 10, 10)
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
	require.Equal(t, batchImageWalletState{Balance: 15, GiftBalance: 10}, readBatchImageWallet(t, user.ID))
}

func TestUsageBillingRepositoryBatchImage_NullLegacyHoldRecoversFrozenGiftAllocation(t *testing.T) {
	for _, action := range []string{"capture", "release"} {
		t.Run(action, func(t *testing.T) {
			repo, user, apiKey := createBatchImageGiftWallet(t, 8, 0)
			batchID := "batch-null-legacy-gift-" + uuid.NewString()
			_, err := integrationDB.ExecContext(context.Background(), `
				UPDATE users SET frozen_balance = 12, frozen_gift_balance = 10 WHERE id = $1
			`, user.ID)
			require.NoError(t, err)
			_, err = integrationDB.ExecContext(context.Background(), `
				INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint, threshold_exempt_cost)
				VALUES ($1, $2, 'legacy-null-hold', NULL)
			`, service.BatchImageHoldRequestID(batchID), apiKey.ID)
			require.NoError(t, err)

			switch action {
			case "capture":
				result, err := repo.CaptureBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
					RequestID: service.BatchImageCaptureRequestID(batchID), APIKeyID: apiKey.ID,
					UserID: user.ID, BatchID: batchID, HoldAmount: 12, ActualAmount: 7,
				})
				require.NoError(t, err)
				require.InDelta(t, 7, result.ThresholdExemptCost, 0.00000001)
				require.Equal(t, batchImageWalletState{Balance: 13, GiftBalance: 3}, readBatchImageWallet(t, user.ID))
			case "release":
				_, err := repo.ReleaseBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
					RequestID: service.BatchImageReleaseRequestID(batchID), APIKeyID: apiKey.ID,
					UserID: user.ID, BatchID: batchID, HoldAmount: 12,
				})
				require.NoError(t, err)
				require.Equal(t, batchImageWalletState{Balance: 20, GiftBalance: 10}, readBatchImageWallet(t, user.ID))
			}
		})
	}
}

func TestUsageBillingRepositoryBatchImage_ExplicitZeroHoldDoesNotConsumeAnotherHoldsGift(t *testing.T) {
	repo, user, apiKey := createBatchImageGiftWallet(t, 30, 8)
	giftBatch := "batch-explicit-gift-" + uuid.NewString()
	cashBatch := "batch-explicit-zero-" + uuid.NewString()
	reserveBatchImageGift(t, repo, user, apiKey, giftBatch, 8)
	reserveBatchImageGift(t, repo, user, apiKey, cashBatch, 8)
	require.Equal(t, sql.NullFloat64{Float64: 8, Valid: true}, readBatchImageHoldGift(t, giftBatch, apiKey.ID))
	require.Equal(t, sql.NullFloat64{Float64: 0, Valid: true}, readBatchImageHoldGift(t, cashBatch, apiKey.ID))

	captured, err := repo.CaptureBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
		RequestID: service.BatchImageCaptureRequestID(cashBatch), APIKeyID: apiKey.ID,
		UserID: user.ID, BatchID: cashBatch, HoldAmount: 8, ActualAmount: 8,
	})
	require.NoError(t, err)
	require.Zero(t, captured.ThresholdExemptCost)
	_, err = repo.ReleaseBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
		RequestID: service.BatchImageReleaseRequestID(giftBatch), APIKeyID: apiKey.ID,
		UserID: user.ID, BatchID: giftBatch, HoldAmount: 8,
	})
	require.NoError(t, err)
	require.Equal(t, batchImageWalletState{Balance: 22, GiftBalance: 8}, readBatchImageWallet(t, user.ID))
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
	require.InDelta(t, 7, duplicate.ThresholdExemptCost, 0.00000001)
	repeated, err := repo.CaptureBatchImageBalance(context.Background(), cmd)
	require.NoError(t, err)
	require.False(t, repeated.Applied)
	require.InDelta(t, 7, repeated.ThresholdExemptCost, 0.00000001)
	require.Equal(t, after, readBatchImageWallet(t, user.ID))
}

func TestUsageBillingRepositoryBatchImage_CaptureRequiresOwnReservedHold(t *testing.T) {
	for _, tc := range []struct {
		hold   float64
		actual float64
	}{{hold: 0, actual: 0}, {hold: 12, actual: 0}, {hold: 12, actual: 7}} {
		t.Run(fmt.Sprintf("hold_%g_actual_%g", tc.hold, tc.actual), func(t *testing.T) {
			repo, user, apiKey := createBatchImageGiftWallet(t, 20, 10)
			reservedBatchID := "batch-owned-hold-" + uuid.NewString()
			reserveBatchImageGift(t, repo, user, apiKey, reservedBatchID, 12)
			before := readBatchImageWallet(t, user.ID)

			phantomBatchID := "batch-phantom-capture-" + uuid.NewString()
			_, err := repo.CaptureBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
				RequestID: service.BatchImageCaptureRequestID(phantomBatchID), APIKeyID: apiKey.ID,
				UserID: user.ID, BatchID: phantomBatchID, HoldAmount: tc.hold, ActualAmount: tc.actual,
			})
			require.Error(t, err)
			require.Equal(t, before, readBatchImageWallet(t, user.ID), "another batch's live hold must remain untouched")

			var captureClaims int
			require.NoError(t, integrationDB.QueryRowContext(context.Background(), `
				SELECT COUNT(*) FROM usage_billing_dedup
				WHERE request_id = $1 AND api_key_id = $2
			`, service.BatchImageCaptureRequestID(phantomBatchID), apiKey.ID).Scan(&captureClaims))
			require.Zero(t, captureClaims, "a phantom capture must not poison its retry key")
		})
	}
}

func TestUsageBillingRepositoryBatchImage_PhantomReleaseDoesNotPoisonLaterHold(t *testing.T) {
	for _, hold := range []float64{0, 12} {
		t.Run(fmt.Sprintf("hold_%g", hold), func(t *testing.T) {
			repo, user, apiKey := createBatchImageGiftWallet(t, 20, 10)
			batchID := "batch-release-before-hold-" + uuid.NewString()
			cmd := &service.BatchImageBalanceHoldCommand{
				RequestID: service.BatchImageReleaseRequestID(batchID), APIKeyID: apiKey.ID,
				UserID: user.ID, BatchID: batchID, HoldAmount: hold,
			}

			phantom, err := repo.ReleaseBatchImageBalance(context.Background(), cmd)
			require.NoError(t, err)
			require.False(t, phantom.Applied)
			require.Equal(t, batchImageWalletState{Balance: 20, GiftBalance: 10}, readBatchImageWallet(t, user.ID))

			var releaseClaims int
			require.NoError(t, integrationDB.QueryRowContext(context.Background(), `
				SELECT COUNT(*) FROM usage_billing_dedup
				WHERE request_id = $1 AND api_key_id = $2
			`, cmd.RequestID, apiKey.ID).Scan(&releaseClaims))
			require.Zero(t, releaseClaims)

			reserveBatchImageGift(t, repo, user, apiKey, batchID, hold)
			released, err := repo.ReleaseBatchImageBalance(context.Background(), cmd)
			require.NoError(t, err)
			require.True(t, released.Applied)
			require.Equal(t, batchImageWalletState{Balance: 20, GiftBalance: 10}, readBatchImageWallet(t, user.ID))
		})
	}
}

func TestUsageBillingRepositoryBatchImage_CaptureReplayRestoresArchivedGiftAllocation(t *testing.T) {
	ctx := context.Background()
	repo, user, apiKey := createBatchImageGiftWallet(t, 20, 10)
	batchID := "batch-archived-capture-" + uuid.NewString()
	reserveBatchImageGift(t, repo, user, apiKey, batchID, 12)
	cmd := &service.BatchImageBalanceHoldCommand{
		RequestID: service.BatchImageCaptureRequestID(batchID), APIKeyID: apiKey.ID,
		UserID: user.ID, BatchID: batchID, HoldAmount: 12, ActualAmount: 7,
	}
	first, err := repo.CaptureBatchImageBalance(ctx, cmd)
	require.NoError(t, err)
	require.True(t, first.Applied)
	require.InDelta(t, 7, first.ThresholdExemptCost, 0.00000001)
	after := readBatchImageWallet(t, user.ID)

	_, err = integrationDB.ExecContext(ctx, `
		UPDATE usage_billing_dedup SET created_at = $1
		WHERE request_id = $2 AND api_key_id = $3
	`, time.Now().UTC().AddDate(0, 0, -400), cmd.RequestID, apiKey.ID)
	require.NoError(t, err)
	aggRepo := newDashboardAggregationRepositoryWithSQL(integrationDB)
	require.NoError(t, aggRepo.CleanupUsageBillingDedup(ctx, time.Now().UTC().AddDate(0, 0, -365)))

	replay, err := repo.CaptureBatchImageBalance(ctx, cmd)
	require.NoError(t, err)
	require.False(t, replay.Applied)
	require.InDelta(t, 7, replay.ThresholdExemptCost, 0.00000001)
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

func TestUsageBillingRepositoryBatchImage_OutOfOrderSettlementUsesEachHoldGiftSource(t *testing.T) {
	for _, tc := range []struct {
		name            string
		wantGiftBalance float64
		settle          func(t *testing.T, repo *usageBillingRepository, user *service.User, apiKey *service.APIKey, firstBatch, secondBatch string)
	}{
		{
			name:            "capture second then release first",
			wantGiftBalance: 8,
			settle: func(t *testing.T, repo *usageBillingRepository, user *service.User, apiKey *service.APIKey, firstBatch, secondBatch string) {
				captured, err := repo.CaptureBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
					RequestID: service.BatchImageCaptureRequestID(secondBatch), APIKeyID: apiKey.ID,
					UserID: user.ID, BatchID: secondBatch, HoldAmount: 8, ActualAmount: 8,
				})
				require.NoError(t, err)
				require.InDelta(t, 2, captured.ThresholdExemptCost, 0.00000001)
				_, err = repo.ReleaseBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
					RequestID: service.BatchImageReleaseRequestID(firstBatch), APIKeyID: apiKey.ID,
					UserID: user.ID, BatchID: firstBatch, HoldAmount: 8,
				})
				require.NoError(t, err)
			},
		},
		{
			name:            "release second then capture first",
			wantGiftBalance: 2,
			settle: func(t *testing.T, repo *usageBillingRepository, user *service.User, apiKey *service.APIKey, firstBatch, secondBatch string) {
				_, err := repo.ReleaseBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
					RequestID: service.BatchImageReleaseRequestID(secondBatch), APIKeyID: apiKey.ID,
					UserID: user.ID, BatchID: secondBatch, HoldAmount: 8,
				})
				require.NoError(t, err)
				captured, err := repo.CaptureBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
					RequestID: service.BatchImageCaptureRequestID(firstBatch), APIKeyID: apiKey.ID,
					UserID: user.ID, BatchID: firstBatch, HoldAmount: 8, ActualAmount: 8,
				})
				require.NoError(t, err)
				require.InDelta(t, 8, captured.ThresholdExemptCost, 0.00000001)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, user, apiKey := createBatchImageGiftWallet(t, 30, 10)
			firstBatch := "batch-first-" + uuid.NewString()
			secondBatch := "batch-second-" + uuid.NewString()
			reserveBatchImageGift(t, repo, user, apiKey, firstBatch, 8)
			reserveBatchImageGift(t, repo, user, apiKey, secondBatch, 8)
			require.Equal(t, sql.NullFloat64{Float64: 8, Valid: true}, readBatchImageHoldGift(t, firstBatch, apiKey.ID))
			require.Equal(t, sql.NullFloat64{Float64: 2, Valid: true}, readBatchImageHoldGift(t, secondBatch, apiKey.ID))

			tc.settle(t, repo, user, apiKey, firstBatch, secondBatch)
			require.Equal(t, batchImageWalletState{Balance: 22, GiftBalance: tc.wantGiftBalance}, readBatchImageWallet(t, user.ID))
		})
	}
}

func TestUsageBillingRepositoryBatchImage_CaptureThenReleaseCannotSettleHoldTwice(t *testing.T) {
	repo, user, apiKey := createBatchImageGiftWallet(t, 20, 10)
	batchID := "batch-single-terminal-" + uuid.NewString()
	reserveBatchImageGift(t, repo, user, apiKey, batchID, 12)

	_, err := repo.CaptureBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
		RequestID: service.BatchImageCaptureRequestID(batchID), APIKeyID: apiKey.ID,
		UserID: user.ID, BatchID: batchID, HoldAmount: 12, ActualAmount: 7,
	})
	require.NoError(t, err)
	afterCapture := readBatchImageWallet(t, user.ID)
	released, err := repo.ReleaseBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
		RequestID: service.BatchImageReleaseRequestID(batchID), APIKeyID: apiKey.ID,
		UserID: user.ID, BatchID: batchID, HoldAmount: 12,
	})
	require.NoError(t, err)
	require.False(t, released.Applied)
	require.Equal(t, afterCapture, readBatchImageWallet(t, user.ID))

	var releaseClaims int
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM usage_billing_dedup
		WHERE request_id = $1 AND api_key_id = $2
	`, service.BatchImageReleaseRequestID(batchID), apiKey.ID).Scan(&releaseClaims))
	require.Zero(t, releaseClaims)
}

func TestUsageBillingRepositoryBatchImage_ConcurrentCaptureAndReleaseCommitsOneTerminalAction(t *testing.T) {
	repo, user, apiKey := createBatchImageGiftWallet(t, 20, 10)
	batchID := "batch-terminal-race-" + uuid.NewString()
	reserveBatchImageGift(t, repo, user, apiKey, batchID, 12)

	start := make(chan struct{})
	type terminalResult struct {
		kind    string
		applied bool
		err     error
	}
	results := make(chan terminalResult, 2)
	go func() {
		<-start
		result, err := repo.CaptureBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
			RequestID: service.BatchImageCaptureRequestID(batchID), APIKeyID: apiKey.ID,
			UserID: user.ID, BatchID: batchID, HoldAmount: 12, ActualAmount: 7,
		})
		results <- terminalResult{kind: "capture", applied: result != nil && result.Applied, err: err}
	}()
	go func() {
		<-start
		result, err := repo.ReleaseBatchImageBalance(context.Background(), &service.BatchImageBalanceHoldCommand{
			RequestID: service.BatchImageReleaseRequestID(batchID), APIKeyID: apiKey.ID,
			UserID: user.ID, BatchID: batchID, HoldAmount: 12,
		})
		results <- terminalResult{kind: "release", applied: result != nil && result.Applied, err: err}
	}()
	close(start)

	applied := 0
	for i := 0; i < 2; i++ {
		result := <-results
		if result.kind == "release" {
			require.NoError(t, result.err)
		} else if result.err != nil {
			require.ErrorIs(t, result.err, errBatchImageCaptureHoldReleased)
		}
		if result.applied {
			applied++
		}
	}
	require.Equal(t, 1, applied)
	state := readBatchImageWallet(t, user.ID)
	require.Contains(t, []batchImageWalletState{
		{Balance: 20, GiftBalance: 10},
		{Balance: 13, GiftBalance: 3},
	}, state)
	var terminalClaims int
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM usage_billing_dedup
		WHERE api_key_id = $1 AND request_id IN ($2, $3)
	`, apiKey.ID, service.BatchImageCaptureRequestID(batchID), service.BatchImageReleaseRequestID(batchID)).Scan(&terminalClaims))
	require.Equal(t, 1, terminalClaims)
}

func TestUsageBillingRepositoryBatchImage_EarlyReleaseRacingReserveDoesNotPoisonRetry(t *testing.T) {
	repo, user, apiKey := createBatchImageGiftWallet(t, 20, 10)
	batchID := "batch-reserve-release-race-" + uuid.NewString()
	reserveCmd := &service.BatchImageBalanceHoldCommand{
		RequestID: service.BatchImageHoldRequestID(batchID), APIKeyID: apiKey.ID,
		UserID: user.ID, BatchID: batchID, HoldAmount: 12,
	}
	releaseCmd := &service.BatchImageBalanceHoldCommand{
		RequestID: service.BatchImageReleaseRequestID(batchID), APIKeyID: apiKey.ID,
		UserID: user.ID, BatchID: batchID, HoldAmount: 12,
	}

	start := make(chan struct{})
	errCh := make(chan error, 2)
	go func() {
		<-start
		_, err := repo.ReserveBatchImageBalance(context.Background(), reserveCmd)
		errCh <- err
	}()
	go func() {
		<-start
		_, err := repo.ReleaseBatchImageBalance(context.Background(), releaseCmd)
		errCh <- err
	}()
	close(start)
	require.NoError(t, <-errCh)
	require.NoError(t, <-errCh)

	_, err := repo.ReleaseBatchImageBalance(context.Background(), releaseCmd)
	require.NoError(t, err)
	require.Equal(t, batchImageWalletState{Balance: 20, GiftBalance: 10}, readBatchImageWallet(t, user.ID))
	var releaseClaims int
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM usage_billing_dedup
		WHERE request_id = $1 AND api_key_id = $2
	`, releaseCmd.RequestID, apiKey.ID).Scan(&releaseClaims))
	require.Equal(t, 1, releaseClaims)
}

func TestUsageBillingRepositoryBatchImage_DatabaseRejectsCorruptNaNPools(t *testing.T) {
	_, user, _ := createBatchImageGiftWallet(t, 20, 0)
	_, err := integrationDB.ExecContext(context.Background(), `
		UPDATE users
		SET gift_balance = 'NaN'::numeric,
			frozen_balance = 'NaN'::numeric,
			frozen_gift_balance = 'NaN'::numeric
		WHERE id = $1
	`, user.ID)
	require.Error(t, err)
	require.Equal(t, batchImageWalletState{Balance: 20}, readBatchImageWallet(t, user.ID))
}
