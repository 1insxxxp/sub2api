//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
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

type legacyBillingCacheStub struct {
	BillingCache
	updatedUserID      int64
	updatedGroupID     int64
	updatedCost        float64
	updateCalls        int
	invalidatedUserID  int64
	invalidatedGroupID int64
	invalidateCalls    int
}

func (c *legacyBillingCacheStub) UpdateSubscriptionUsage(_ context.Context, userID, groupID int64, cost float64) error {
	c.updatedUserID, c.updatedGroupID, c.updatedCost = userID, groupID, cost
	c.updateCalls++
	return nil
}

func (c *legacyBillingCacheStub) InvalidateSubscriptionCache(_ context.Context, userID, groupID int64) error {
	c.invalidatedUserID, c.invalidatedGroupID = userID, groupID
	c.invalidateCalls++
	return nil
}

type fallbackPlatformQuotaRepoStub struct{ UserPlatformQuotaRepository }

type failingFallbackAPIKeyUpdater struct {
	quotaErr  error
	rateErr   error
	quotaCall int
	rateCall  int
}

func (u *failingFallbackAPIKeyUpdater) UpdateQuotaUsed(context.Context, int64, float64) error {
	u.quotaCall++
	return u.quotaErr
}

func (u *failingFallbackAPIKeyUpdater) UpdateRateLimitUsage(context.Context, int64, float64) error {
	u.rateCall++
	return u.rateErr
}

type transactionalFallbackWalletRepo struct {
	UserRepository
	balance float64
	gift    float64
	calls   int
}

func (r *transactionalFallbackWalletRepo) DeductBalanceWithGiftAllocation(_ context.Context, _ int64, amount float64) (BalanceDeductionResult, error) {
	r.calls++
	giftUsed := math.Min(r.gift, amount)
	r.balance -= amount
	r.gift -= giftUsed
	return BalanceDeductionResult{NewBalance: r.balance, ThresholdExemptCost: giftUsed}, nil
}

type atomicFallbackUsageRepo struct {
	UsageLogRepository
	logs             map[string]UsageLog
	wallet           *transactionalFallbackWalletRepo
	failUpdate       error
	transactionCalls int
}

func newAtomicFallbackUsageRepo(wallet *transactionalFallbackWalletRepo) *atomicFallbackUsageRepo {
	return &atomicFallbackUsageRepo{logs: make(map[string]UsageLog), wallet: wallet}
}

func (r *atomicFallbackUsageRepo) RunUsageBillingTransaction(ctx context.Context, fn func(context.Context) error) error {
	r.transactionCalls++
	logsSnapshot := make(map[string]UsageLog, len(r.logs))
	for key, log := range r.logs {
		logsSnapshot[key] = log
	}
	var balanceSnapshot, giftSnapshot float64
	if r.wallet != nil {
		balanceSnapshot, giftSnapshot = r.wallet.balance, r.wallet.gift
	}
	if err := fn(ctx); err != nil {
		r.logs = logsSnapshot
		if r.wallet != nil {
			r.wallet.balance, r.wallet.gift = balanceSnapshot, giftSnapshot
		}
		return err
	}
	return nil
}

func (r *atomicFallbackUsageRepo) Create(_ context.Context, log *UsageLog) (bool, error) {
	key := fmt.Sprintf("%s:%d", log.RequestID, log.APIKeyID)
	if _, exists := r.logs[key]; exists {
		return false, nil
	}
	log.ID = int64(len(r.logs) + 1)
	r.logs[key] = *log
	return true, nil
}

func (r *atomicFallbackUsageRepo) UpdateThresholdExemptCost(_ context.Context, usageLogID int64, amount float64) error {
	if r.failUpdate != nil {
		return r.failUpdate
	}
	for key, log := range r.logs {
		if log.ID == usageLogID {
			log.ThresholdExemptCost = amount
			r.logs[key] = log
			return nil
		}
	}
	return ErrUsageLogNotFound
}

func newLegacyFallbackDeps(userRepo UserRepository, usageRepo *atomicFallbackUsageRepo) *billingDeps {
	return &billingDeps{
		usageLogRepo:        usageRepo,
		userRepo:            userRepo,
		userSubRepo:         &giftFallbackSubscriptionRepo{},
		billingCacheService: &BillingCacheService{},
		deferredService:     &DeferredService{},
	}
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
			}, newLegacyFallbackDeps(userRepo, newAtomicFallbackUsageRepo(nil)), nil)

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
	}, newLegacyFallbackDeps(userRepo, newAtomicFallbackUsageRepo(nil)), nil)

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
	}, newLegacyFallbackDeps(&giftAllocatingBlindUserRepo{}, newAtomicFallbackUsageRepo(nil)), nil)

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
	}, newLegacyFallbackDeps(userRepo, newAtomicFallbackUsageRepo(nil)), nil)

	require.ErrorIs(t, err, wantErr)
	require.False(t, applied)
}

func TestApplyUsageBilling_LegacyFallbackDuplicateRequestIsAtomicAndIdempotent(t *testing.T) {
	wallet := &transactionalFallbackWalletRepo{balance: 20, gift: 10}
	usageRepo := newAtomicFallbackUsageRepo(wallet)
	deps := newLegacyFallbackDeps(wallet, usageRepo)
	params := &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 12, TotalCost: 12},
		User: &User{ID: 1}, APIKey: &APIKey{ID: 2}, Account: &Account{ID: 3},
	}

	firstLog := &UsageLog{APIKeyID: 2, ActualCost: 12}
	first, err := applyUsageBilling(context.Background(), "legacy-duplicate", firstLog, params, deps, nil)
	require.NoError(t, err)
	require.True(t, first)
	secondLog := &UsageLog{APIKeyID: 2, ActualCost: 12}
	second, err := applyUsageBilling(context.Background(), "legacy-duplicate", secondLog, params, deps, nil)
	require.NoError(t, err)
	require.False(t, second)

	require.Equal(t, 1, wallet.calls)
	require.InDelta(t, 8, wallet.balance, 0.00000001)
	require.Zero(t, wallet.gift)
	require.Len(t, usageRepo.logs, 1)
	persisted := usageRepo.logs["legacy-duplicate:2"]
	require.InDelta(t, 10, persisted.ThresholdExemptCost, 0.00000001)
}

func TestApplyUsageBilling_LegacySubscriptionUpdatesCacheAndReplayInvalidates(t *testing.T) {
	wallet := &transactionalFallbackWalletRepo{balance: 20, gift: 10}
	usageRepo := newAtomicFallbackUsageRepo(wallet)
	cache := &legacyBillingCacheStub{}
	deps := newLegacyFallbackDeps(wallet, usageRepo)
	deps.billingCacheService = &BillingCacheService{cache: cache}
	groupID := int64(9)
	params := &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 3, TotalCost: 3},
		User: &User{ID: 1}, APIKey: &APIKey{ID: 2, GroupID: &groupID}, Account: &Account{ID: 3},
		Subscription: &UserSubscription{ID: 4, GroupID: groupID}, IsSubscriptionBill: true,
	}

	first, err := applyUsageBilling(context.Background(), "legacy-subscription-cache", &UsageLog{APIKeyID: 2, ActualCost: 3}, params, deps, nil)
	require.NoError(t, err)
	require.True(t, first)
	require.Equal(t, 1, cache.updateCalls)
	require.Equal(t, int64(1), cache.updatedUserID)
	require.Equal(t, groupID, cache.updatedGroupID)
	require.Equal(t, 3.0, cache.updatedCost)

	second, err := applyUsageBilling(context.Background(), "legacy-subscription-cache", &UsageLog{APIKeyID: 2, ActualCost: 3}, params, deps, nil)
	require.NoError(t, err)
	require.False(t, second)
	require.Equal(t, 1, cache.updateCalls, "replay must not double-increment cached usage")
	require.Equal(t, 1, cache.invalidateCalls)
	require.Equal(t, int64(1), cache.invalidatedUserID)
	require.Equal(t, groupID, cache.invalidatedGroupID)
}

func TestApplyUsageBilling_LegacyFallbackRollsBackWalletAndLogOnAttributionFailure(t *testing.T) {
	wallet := &transactionalFallbackWalletRepo{balance: 20, gift: 10}
	usageRepo := newAtomicFallbackUsageRepo(wallet)
	wantErr := errors.New("attribution update failed")
	usageRepo.failUpdate = wantErr

	applied, err := applyUsageBilling(context.Background(), "legacy-rollback", &UsageLog{APIKeyID: 2, ActualCost: 12}, &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 12, TotalCost: 12},
		User: &User{ID: 1}, APIKey: &APIKey{ID: 2}, Account: &Account{ID: 3},
	}, newLegacyFallbackDeps(wallet, usageRepo), nil)

	require.ErrorIs(t, err, wantErr)
	require.False(t, applied)
	require.Empty(t, usageRepo.logs)
	require.InDelta(t, 20, wallet.balance, 0.00000001)
	require.InDelta(t, 10, wallet.gift, 0.00000001)
}

func TestApplyUsageBilling_LegacyFallbackFailsClosedBeforeAPIKeyQuotaMutation(t *testing.T) {
	wallet := &transactionalFallbackWalletRepo{balance: 20, gift: 10}
	usageRepo := newAtomicFallbackUsageRepo(wallet)
	updater := &failingFallbackAPIKeyUpdater{}

	applied, err := applyUsageBilling(context.Background(), "legacy-quota-rollback", &UsageLog{APIKeyID: 2, ActualCost: 12}, &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 12, TotalCost: 12},
		User: &User{ID: 1}, APIKey: &APIKey{ID: 2, Quota: 100}, Account: &Account{ID: 3},
		APIKeyService: updater,
	}, newLegacyFallbackDeps(wallet, usageRepo), nil)

	require.ErrorIs(t, err, ErrUsageBillingSideEffectRepositoryRequired)
	require.False(t, applied)
	require.Zero(t, usageRepo.transactionCalls)
	require.Zero(t, updater.quotaCall)
	require.Empty(t, usageRepo.logs)
	require.InDelta(t, 20, wallet.balance, 0.00000001)
	require.InDelta(t, 10, wallet.gift, 0.00000001)
}

func TestApplyUsageBilling_LegacyFallbackFailsClosedBeforeRateLimitMutation(t *testing.T) {
	wallet := &transactionalFallbackWalletRepo{balance: 20, gift: 10}
	usageRepo := newAtomicFallbackUsageRepo(wallet)
	updater := &failingFallbackAPIKeyUpdater{}

	applied, err := applyUsageBilling(context.Background(), "legacy-rate-rollback", &UsageLog{APIKeyID: 2, ActualCost: 12}, &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 12, TotalCost: 12},
		User: &User{ID: 1}, APIKey: &APIKey{ID: 2, RateLimit5h: 100}, Account: &Account{ID: 3},
		APIKeyService: updater,
	}, newLegacyFallbackDeps(wallet, usageRepo), nil)

	require.ErrorIs(t, err, ErrUsageBillingSideEffectRepositoryRequired)
	require.False(t, applied)
	require.Zero(t, usageRepo.transactionCalls)
	require.Zero(t, updater.rateCall)
	require.Empty(t, usageRepo.logs)
	require.InDelta(t, 20, wallet.balance, 0.00000001)
	require.InDelta(t, 10, wallet.gift, 0.00000001)
}

func TestApplyUsageBilling_LegacyFallbackFailsClosedBeforeAccountQuotaMutation(t *testing.T) {
	wallet := &transactionalFallbackWalletRepo{balance: 20, gift: 10}
	usageRepo := newAtomicFallbackUsageRepo(wallet)
	deps := newLegacyFallbackDeps(wallet, usageRepo)

	applied, err := applyUsageBilling(context.Background(), "legacy-account-rollback", &UsageLog{APIKeyID: 2, ActualCost: 12}, &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 12, TotalCost: 12},
		User: &User{ID: 1}, APIKey: &APIKey{ID: 2},
		Account:               &Account{ID: 3, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_limit": 100.0}},
		AccountRateMultiplier: 1,
	}, deps, nil)

	require.ErrorIs(t, err, ErrUsageBillingSideEffectRepositoryRequired)
	require.False(t, applied)
	require.Zero(t, usageRepo.transactionCalls)
	require.Empty(t, usageRepo.logs)
	require.InDelta(t, 20, wallet.balance, 0.00000001)
	require.InDelta(t, 10, wallet.gift, 0.00000001)
}

func TestApplyUsageBilling_LegacyFallbackFailsClosedBeforePlatformQuotaMutation(t *testing.T) {
	wallet := &transactionalFallbackWalletRepo{balance: 20, gift: 10}
	usageRepo := newAtomicFallbackUsageRepo(wallet)
	deps := newLegacyFallbackDeps(wallet, usageRepo)
	deps.billingCacheService = &BillingCacheService{cfg: &config.Config{}}
	deps.userPlatformQuotaRepo = &fallbackPlatformQuotaRepoStub{}

	applied, err := applyUsageBilling(context.Background(), "legacy-platform-rollback", &UsageLog{APIKeyID: 2, ActualCost: 12}, &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 12, TotalCost: 12},
		User: &User{ID: 1}, APIKey: &APIKey{ID: 2}, Account: &Account{ID: 3}, Platform: PlatformOpenAI,
	}, deps, nil)

	require.ErrorIs(t, err, ErrUsageBillingSideEffectRepositoryRequired)
	require.False(t, applied)
	require.Zero(t, usageRepo.transactionCalls)
	require.Empty(t, usageRepo.logs)
	require.InDelta(t, 20, wallet.balance, 0.00000001)
	require.InDelta(t, 10, wallet.gift, 0.00000001)
}

func TestApplyUsageBilling_LegacyFallbackRejectsNonFiniteActualBeforeMutation(t *testing.T) {
	wallet := &transactionalFallbackWalletRepo{balance: 20, gift: 10}
	usageRepo := newAtomicFallbackUsageRepo(wallet)

	applied, err := applyUsageBilling(context.Background(), "legacy-nan", &UsageLog{APIKeyID: 2, ActualCost: math.NaN()}, &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: math.NaN(), TotalCost: 1},
		User: &User{ID: 1}, APIKey: &APIKey{ID: 2}, Account: &Account{ID: 3},
	}, newLegacyFallbackDeps(wallet, usageRepo), nil)

	require.ErrorIs(t, err, ErrUsageBillingNonFiniteAmount)
	require.False(t, applied)
	require.Zero(t, usageRepo.transactionCalls)
	require.Zero(t, wallet.calls)
	require.Empty(t, usageRepo.logs)
}

func TestApplyUsageBilling_LegacyFallbackRequiresExplicitRequestID(t *testing.T) {
	wallet := &transactionalFallbackWalletRepo{balance: 20, gift: 10}
	usageRepo := newAtomicFallbackUsageRepo(wallet)

	applied, err := applyUsageBilling(context.Background(), "", &UsageLog{APIKeyID: 2, ActualCost: 1}, &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 1, TotalCost: 1},
		User: &User{ID: 1}, APIKey: &APIKey{ID: 2}, Account: &Account{ID: 3},
	}, newLegacyFallbackDeps(wallet, usageRepo), nil)

	require.ErrorIs(t, err, ErrUsageBillingRequestIDRequired)
	require.False(t, applied)
	require.Zero(t, usageRepo.transactionCalls)
	require.Zero(t, wallet.calls)
	require.Empty(t, usageRepo.logs)
}

type giftAllocatingBlindUserRepo struct{ UserRepository }

func (r *giftAllocatingBlindUserRepo) DeductBalance(context.Context, int64, float64) error {
	return nil
}
