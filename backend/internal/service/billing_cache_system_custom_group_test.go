//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type systemCustomBillingCacheStub struct {
	BillingCache
	subscription       *SubscriptionCacheData
	subscriptionUserID int64
	subscriptionGroup  int64
	balanceCalls       int
	platformQuotaCalls int
}

func (s *systemCustomBillingCacheStub) GetSubscriptionCache(_ context.Context, userID, groupID int64) (*SubscriptionCacheData, error) {
	s.subscriptionUserID = userID
	s.subscriptionGroup = groupID
	return s.subscription, nil
}

func (s *systemCustomBillingCacheStub) GetUserBalance(_ context.Context, _ int64) (float64, error) {
	s.balanceCalls++
	return 0, nil
}

func (s *systemCustomBillingCacheStub) GetUserPlatformQuotaCache(_ context.Context, _ int64, _ string) (*UserPlatformQuotaCacheEntry, bool, error) {
	s.platformQuotaCalls++
	return nil, false, nil
}

type systemCustomRPMCacheStub struct {
	UserRPMCache
	groupIDs []int64
	count    int
}

func (s *systemCustomRPMCacheStub) IncrementUserGroupRPM(_ context.Context, _, groupID int64) (int, error) {
	s.groupIDs = append(s.groupIDs, groupID)
	s.count++
	return s.count, nil
}

type systemCustomRPMOverrideRepoStub struct {
	UserGroupRateRepository
	groupIDs []int64
	override *int
}

func (s *systemCustomRPMOverrideRepoStub) GetRPMOverrideByUserAndGroup(_ context.Context, _, groupID int64) (*int, error) {
	s.groupIDs = append(s.groupIDs, groupID)
	return s.override, nil
}

func newSystemCustomBillingEligibilityService(t *testing.T, cache BillingCache, rpm UserRPMCache, rates UserGroupRateRepository) *BillingCacheService {
	t.Helper()
	svc := NewBillingCacheService(cache, nil, nil, nil, rpm, rates, &config.Config{}, nil)
	t.Cleanup(svc.Stop)
	return svc
}

func activeSystemCustomSubscriptionCache() *SubscriptionCacheData {
	return &SubscriptionCacheData{Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour)}
}

func systemCustomBillingEligibilityContext() context.Context {
	return WithSystemCustomGroupResolution(context.Background(), SystemCustomGroupResolution{
		BillingGroupID: 25, SourceGroupID: 42, PublicModel: "monthly-gpt",
		SourceModel: "gpt-5.4", SourcePlatform: PlatformOpenAI,
	})
}

func systemCustomEligibilityGroups() (*Group, *Group, *UserSubscription) {
	billing := &Group{
		ID: 25, Platform: PlatformComposite, Status: StatusActive, Hydrated: true,
		SubscriptionType: SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true,
	}
	source := &Group{ID: 42, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, SubscriptionType: SubscriptionTypeStandard, RPMLimit: 1}
	subscription := &UserSubscription{ID: 7, UserID: 9, GroupID: billing.ID, Group: billing, Status: SubscriptionStatusActive}
	return billing, source, subscription
}

func TestCheckBillingEligibilitySystemCustomUsesMonthlySubscriptionWithZeroBalance(t *testing.T) {
	_, source, subscription := systemCustomEligibilityGroups()
	cache := &systemCustomBillingCacheStub{subscription: activeSystemCustomSubscriptionCache()}
	svc := newSystemCustomBillingEligibilityService(t, cache, nil, nil)
	user := &User{ID: 9, Balance: 0}
	key := &APIKey{ID: 3, UserID: user.ID, User: user, GroupID: &source.ID, Group: source}

	err := svc.CheckBillingEligibility(systemCustomBillingEligibilityContext(), user, key, source, subscription, PlatformOpenAI)

	require.NoError(t, err)
	require.Zero(t, cache.balanceCalls, "a resolved monthly request must never fall back to wallet eligibility")
	require.Zero(t, cache.platformQuotaCalls, "subscription traffic bypasses user-platform quota")
	require.Equal(t, int64(25), cache.subscriptionGroup)
}

func TestCheckBillingEligibilitySystemCustomAppliesMonthlyGroupLimits(t *testing.T) {
	billing, source, subscription := systemCustomEligibilityGroups()
	daily := 1.0
	billing.DailyLimitUSD = &daily
	cacheData := activeSystemCustomSubscriptionCache()
	cacheData.DailyUsage = daily
	cache := &systemCustomBillingCacheStub{subscription: cacheData}
	svc := newSystemCustomBillingEligibilityService(t, cache, nil, nil)
	user := &User{ID: 9}

	err := svc.CheckBillingEligibility(systemCustomBillingEligibilityContext(), user, &APIKey{Group: source}, source, subscription, PlatformOpenAI)

	require.ErrorIs(t, err, ErrDailyLimitExceeded)
	require.Equal(t, billing.ID, cache.subscriptionGroup)
}

func TestCheckBillingEligibilitySystemCustomUsesMonthlyGroupRPMAndOverride(t *testing.T) {
	billing, source, subscription := systemCustomEligibilityGroups()
	billing.RPMLimit = 100
	cache := &systemCustomBillingCacheStub{subscription: activeSystemCustomSubscriptionCache()}
	rpm := &systemCustomRPMCacheStub{}
	override := 1
	rates := &systemCustomRPMOverrideRepoStub{override: &override}
	svc := newSystemCustomBillingEligibilityService(t, cache, rpm, rates)
	user := &User{ID: 9}
	key := &APIKey{Group: source}

	require.NoError(t, svc.CheckBillingEligibility(systemCustomBillingEligibilityContext(), user, key, source, subscription, PlatformOpenAI))
	require.ErrorIs(t, svc.CheckBillingEligibility(systemCustomBillingEligibilityContext(), user, key, source, subscription, PlatformOpenAI), ErrGroupRPMExceeded)
	require.Equal(t, []int64{billing.ID, billing.ID}, rates.groupIDs)
	require.Equal(t, []int64{billing.ID, billing.ID}, rpm.groupIDs)
}

func TestCheckBillingEligibilitySystemCustomRejectsMissingOrMismatchedSubscription(t *testing.T) {
	_, source, validSubscription := systemCustomEligibilityGroups()
	tests := []struct {
		name         string
		subscription *UserSubscription
	}{
		{name: "missing", subscription: nil},
		{name: "missing group", subscription: &UserSubscription{GroupID: 25}},
		{name: "mismatched group", subscription: &UserSubscription{GroupID: 99, Group: &Group{ID: 99, SubscriptionType: SubscriptionTypeSubscription}}},
		{name: "mismatched object", subscription: &UserSubscription{GroupID: 25, Group: &Group{ID: 99, SubscriptionType: SubscriptionTypeSubscription}}},
		{name: "valid control", subscription: validSubscription},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := &systemCustomBillingCacheStub{subscription: activeSystemCustomSubscriptionCache()}
			svc := newSystemCustomBillingEligibilityService(t, cache, nil, nil)
			err := svc.CheckBillingEligibility(systemCustomBillingEligibilityContext(), &User{ID: 9}, &APIKey{Group: source}, source, tt.subscription, PlatformOpenAI)
			if tt.name == "valid control" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrSubscriptionInvalid)
			require.Zero(t, cache.balanceCalls)
			require.Zero(t, cache.subscriptionGroup)
		})
	}
}
