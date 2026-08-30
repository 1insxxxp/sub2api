//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type giftAllocatingFallbackUserRepo struct {
	UserRepository
	result      BalanceDeductionResult
	err         error
	calls       int
	deductCalls int
}

func (r *giftAllocatingFallbackUserRepo) DeductBalance(context.Context, int64, float64) error {
	r.deductCalls++
	return nil
}

func (r *giftAllocatingFallbackUserRepo) DeductBalanceWithGiftAllocation(_ context.Context, _ int64, _ float64) (BalanceDeductionResult, error) {
	r.calls++
	return r.result, r.err
}

type giftFallbackSubscriptionRepo struct{ UserSubscriptionRepository }

func (r *giftFallbackSubscriptionRepo) IncrementUsage(context.Context, int64, float64) error {
	return nil
}

func TestApplyUsageBilling_LegacyFallbackPersistsGiftAllocation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		actual     float64
		newBalance float64
		exempt     float64
	}{
		{name: "full gift", actual: 12, newBalance: 8, exempt: 12},
		{name: "mixed gift", actual: 12, newBalance: 8, exempt: 10},
		{name: "no gift", actual: 12, newBalance: 8, exempt: 0},
		{name: "overdraft", actual: 12, newBalance: -7, exempt: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			userRepo := &giftAllocatingFallbackUserRepo{result: BalanceDeductionResult{
				NewBalance: tc.newBalance, ThresholdExemptCost: tc.exempt,
			}}
			usageLog := &UsageLog{ActualCost: tc.actual}

			applied, err := applyUsageBilling(context.Background(), "legacy-gift", usageLog, &postUsageBillingParams{
				Cost:    &CostBreakdown{ActualCost: tc.actual, TotalCost: tc.actual},
				User:    &User{ID: 1},
				APIKey:  &APIKey{ID: 2},
				Account: &Account{ID: 3},
			}, &billingDeps{
				userRepo:            userRepo,
				userSubRepo:         &giftFallbackSubscriptionRepo{},
				billingCacheService: &BillingCacheService{},
				deferredService:     &DeferredService{},
			}, nil)

			require.NoError(t, err)
			require.True(t, applied)
			require.Equal(t, 1, userRepo.calls)
			require.Zero(t, userRepo.deductCalls, "legacy fallback must not use attribution-blind DeductBalance")
			require.InDelta(t, tc.exempt, usageLog.ThresholdExemptCost, 0.00000001)
		})
	}
}

func TestApplyUsageBilling_LegacyFallbackClampsGiftAllocation(t *testing.T) {
	userRepo := &giftAllocatingFallbackUserRepo{result: BalanceDeductionResult{ThresholdExemptCost: 0.00007813}}
	usageLog := &UsageLog{ActualCost: 0.000078125}

	_, err := applyUsageBilling(context.Background(), "legacy-gift-clamp", usageLog, &postUsageBillingParams{
		Cost:    &CostBreakdown{ActualCost: usageLog.ActualCost, TotalCost: usageLog.ActualCost},
		User:    &User{ID: 1},
		APIKey:  &APIKey{ID: 2},
		Account: &Account{ID: 3},
	}, &billingDeps{
		userRepo:            userRepo,
		userSubRepo:         &giftFallbackSubscriptionRepo{},
		billingCacheService: &BillingCacheService{},
		deferredService:     &DeferredService{},
	}, nil)

	require.NoError(t, err)
	require.Equal(t, usageLog.ActualCost, usageLog.ThresholdExemptCost)
}

func TestApplyUsageBilling_LegacyFallbackFailsWithoutGiftCapability(t *testing.T) {
	usageLog := &UsageLog{ActualCost: 1}

	applied, err := applyUsageBilling(context.Background(), "legacy-no-gift-capability", usageLog, &postUsageBillingParams{
		Cost:    &CostBreakdown{ActualCost: 1, TotalCost: 1},
		User:    &User{ID: 1},
		APIKey:  &APIKey{ID: 2},
		Account: &Account{ID: 3},
	}, &billingDeps{
		userRepo:            &giftAllocatingBlindUserRepo{},
		userSubRepo:         &giftFallbackSubscriptionRepo{},
		billingCacheService: &BillingCacheService{},
		deferredService:     &DeferredService{},
	}, nil)

	require.Error(t, err)
	require.False(t, applied)
	require.Zero(t, usageLog.ThresholdExemptCost)
}

func TestApplyUsageBilling_LegacyFallbackPropagatesGiftDeductionError(t *testing.T) {
	wantErr := errors.New("gift allocation failed")
	userRepo := &giftAllocatingFallbackUserRepo{err: wantErr}
	usageLog := &UsageLog{ActualCost: 1}

	applied, err := applyUsageBilling(context.Background(), "legacy-gift-error", usageLog, &postUsageBillingParams{
		Cost:    &CostBreakdown{ActualCost: 1, TotalCost: 1},
		User:    &User{ID: 1},
		APIKey:  &APIKey{ID: 2},
		Account: &Account{ID: 3},
	}, &billingDeps{
		userRepo:            userRepo,
		userSubRepo:         &giftFallbackSubscriptionRepo{},
		billingCacheService: &BillingCacheService{},
		deferredService:     &DeferredService{},
	}, nil)

	require.ErrorIs(t, err, wantErr)
	require.False(t, applied)
}

type giftAllocatingBlindUserRepo struct{ UserRepository }

func (r *giftAllocatingBlindUserRepo) DeductBalance(context.Context, int64, float64) error {
	return nil
}
