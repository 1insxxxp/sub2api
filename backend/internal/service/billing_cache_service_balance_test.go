//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type balanceEligibilityCacheStub struct {
	billingCacheWorkerStub

	balance                  float64
	cacheMissAfterInvalidate bool
	invalidated              atomic.Bool
	deductCalls              atomic.Int64
	invalidateCalls          atomic.Int64
}

func (s *balanceEligibilityCacheStub) GetUserBalance(context.Context, int64) (float64, error) {
	if s.cacheMissAfterInvalidate && s.invalidated.Load() {
		return 0, errors.New("cache miss")
	}
	return s.balance, nil
}

func (s *balanceEligibilityCacheStub) DeductUserBalance(context.Context, int64, float64) error {
	s.deductCalls.Add(1)
	return nil
}

func (s *balanceEligibilityCacheStub) InvalidateUserBalance(context.Context, int64) error {
	s.invalidateCalls.Add(1)
	s.invalidated.Store(true)
	return nil
}

func TestCheckBillingEligibility_RejectsBalanceBelowMinimumReserve(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 0.005}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, &balanceLoadUserRepoStub{balance: 0.005}, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, "")
	require.ErrorIs(t, err, ErrInsufficientBalance)
}

func TestCheckBillingEligibility_AllowsBalanceAtMinimumReserve(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 0.01}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, "")
	require.NoError(t, err)
}

type recentBalanceCostUserRepoStub struct {
	mockUserRepo
	samples      []float64
	historyCalls atomic.Int64
}

func (s *recentBalanceCostUserRepoStub) ListRecentBalanceCosts(context.Context, int64, string, int) ([]float64, error) {
	s.historyCalls.Add(1)
	return s.samples, nil
}

func TestCheckBillingEligibility_ExpensiveHistoryDoesNotBlockLowPositiveBalance(t *testing.T) {
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	cfg.Billing.BalancePreauthorizationSampleCount = 100
	cfg.Billing.BalancePreauthorizationMeanSafetyFactor = 2
	cfg.Billing.BalancePreauthorizationP95Quantile = 0.95
	cfg.Billing.BalancePreauthorizationMaxOverdraftFraction = 0.5
	userRepo := &recentBalanceCostUserRepoStub{samples: []float64{5, 5, 5}}

	cache := &balanceEligibilityCacheStub{balance: 0.02}
	svc := NewBillingCacheService(cache, userRepo, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)
	require.NoError(t, svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, ""))
	require.Zero(t, userRepo.historyCalls.Load(), "admission must not query past costs from other groups or models")
}

func TestCheckBillingEligibility_StaleZeroRechecksFundedWallet(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 0}
	repo := &balanceLoadUserRepoStub{balance: 0.02}
	svc := NewBillingCacheService(cache, repo, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)
	require.NoError(t, svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, ""))
	require.Equal(t, int64(1), repo.calls.Load())
	require.Equal(t, int64(1), cache.invalidateCalls.Load())
}

func TestCheckBillingEligibility_VerifiesTrueWalletRejection(t *testing.T) {
	for _, balance := range []float64{-1, 0, 0.005} {
		t.Run(fmt.Sprint(balance), func(t *testing.T) {
			cache := &balanceEligibilityCacheStub{balance: 0}
			repo := &balanceLoadUserRepoStub{balance: balance}
			cfg := &config.Config{}
			cfg.Billing.MinimumBalanceReserve = 0.01
			svc := NewBillingCacheService(cache, repo, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(svc.Stop)
			require.ErrorIs(t, svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, ""), ErrInsufficientBalance)
			require.Equal(t, int64(1), repo.calls.Load())
		})
	}
}

func TestCheckBillingEligibility_RecheckFailureIsServiceUnavailable(t *testing.T) {
	svc := NewBillingCacheService(&balanceEligibilityCacheStub{}, &mockUserRepo{getByIDErr: errors.New("db unavailable")}, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)
	err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, "")
	require.ErrorIs(t, err, ErrBillingServiceUnavailable)
	require.NotErrorIs(t, err, ErrInsufficientBalance)
}

func TestReserveFixedCharge_RechecksStaleBalanceWithoutDroppingKnownCost(t *testing.T) {
	for _, balance := range []float64{0.02, 0.5} {
		t.Run(fmt.Sprint(balance), func(t *testing.T) {
			svc := NewBillingCacheService(&balanceEligibilityCacheStub{}, &balanceLoadUserRepoStub{balance: balance}, nil, nil, nil, nil, &config.Config{}, nil)
			t.Cleanup(svc.Stop)
			_, err := svc.ReserveFixedCharge(context.Background(), &User{ID: 1}, nil, nil, nil, 0.5)
			if balance < 0.5 {
				require.ErrorIs(t, err, ErrInsufficientBalance)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSyncBalanceCacheAfterDeduction_InvalidatesExhaustedBalance(t *testing.T) {
	cache := &balanceEligibilityCacheStub{
		balance:                  0.50,
		cacheMissAfterInvalidate: true,
	}
	userRepo := &balanceLoadUserRepoStub{balance: -0.25}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, userRepo, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	newBalance := -0.25
	syncBalanceCacheAfterDeduction(context.Background(), &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 0.75},
		User: &User{ID: 1},
	}, &billingDeps{billingCacheService: svc}, &UsageBillingApplyResult{
		NewBalance:         &newBalance,
		BalanceOverdrafted: true,
	})

	require.Equal(t, int64(1), cache.invalidateCalls.Load())
	require.Equal(t, int64(0), cache.deductCalls.Load())

	err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, "")
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Equal(t, int64(1), userRepo.calls.Load())
}

func TestSyncBalanceCacheAfterDeduction_InvalidatesWhenBalanceFallsBelowReserve(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 0.50}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	newBalance := 0.005
	syncBalanceCacheAfterDeduction(context.Background(), &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 0.495},
		User: &User{ID: 1},
	}, &billingDeps{billingCacheService: svc}, &UsageBillingApplyResult{NewBalance: &newBalance})

	require.Equal(t, int64(1), cache.invalidateCalls.Load())
	require.Equal(t, int64(0), cache.deductCalls.Load())
}

func TestSyncBalanceCacheAfterDeduction_QueuesDeductWhenBalanceStillEligible(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 1}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	newBalance := 0.75
	syncBalanceCacheAfterDeduction(context.Background(), &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 0.25},
		User: &User{ID: 1},
	}, &billingDeps{billingCacheService: svc}, &UsageBillingApplyResult{NewBalance: &newBalance})

	require.Equal(t, int64(0), cache.invalidateCalls.Load())
	require.Eventually(t, func() bool {
		return cache.deductCalls.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)
}
