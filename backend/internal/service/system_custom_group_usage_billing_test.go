//go:build unit

package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func systemCustomUsageContext(billingGroupID, sourceGroupID int64, publicModel, sourceModel, sourcePlatform string) context.Context {
	return WithSystemCustomGroupResolution(context.Background(), SystemCustomGroupResolution{
		BillingGroupID: billingGroupID,
		SourceGroupID:  sourceGroupID,
		PublicModel:    publicModel,
		SourceModel:    sourceModel,
		SourcePlatform: sourcePlatform,
	})
}

func TestGatewayServiceRecordUsage_SystemCustomRouteBillsSubscriptionAndPersistsBothGroups(t *testing.T) {
	const (
		billingGroupID = int64(101)
		sourceGroupID  = int64(202)
		subscriptionID = int64(303)
	)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	svc := newGatewayRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, subRepo)

	sourceGroup := &Group{ID: sourceGroupID, Platform: PlatformAnthropic, RateMultiplier: 1.75}
	billingGroup := &Group{ID: billingGroupID, Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription}
	subscription := &UserSubscription{ID: subscriptionID, UserID: 22, GroupID: billingGroupID, Group: billingGroup}
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, "tavern-sonnet", "claude-sonnet-4", PlatformAnthropic)

	err := svc.RecordUsage(ctx, &RecordUsageInput{
		Result: &ForwardResult{
			RequestID:     "system-custom-anthropic",
			Model:         "claude-sonnet-4",
			UpstreamModel: "claude-sonnet-4-20250514",
			Usage:         ClaudeUsage{InputTokens: 1200, OutputTokens: 300},
			Duration:      time.Second,
		},
		APIKey:       &APIKey{ID: 11, GroupID: i64p(sourceGroupID), Group: sourceGroup},
		User:         &User{ID: 22},
		Account:      &Account{ID: 33, Platform: PlatformAnthropic},
		Subscription: subscription,
	})

	require.NoError(t, err)
	require.NotNil(t, billingRepo.lastCmd)
	require.Greater(t, billingRepo.lastCmd.SubscriptionCost, 0.0)
	require.Zero(t, billingRepo.lastCmd.BalanceCost)
	require.NotNil(t, billingRepo.lastCmd.SubscriptionID)
	require.Equal(t, subscriptionID, *billingRepo.lastCmd.SubscriptionID)
	require.Zero(t, userRepo.deductCalls, "system custom monthly access must never fall back to wallet billing")

	require.NotNil(t, usageRepo.lastLog)
	require.Equal(t, BillingTypeSubscription, usageRepo.lastLog.BillingType)
	require.Equal(t, "tavern-sonnet", usageRepo.lastLog.RequestedModel)
	require.Equal(t, "claude-sonnet-4", usageRepo.lastLog.Model)
	require.NotNil(t, usageRepo.lastLog.UpstreamModel)
	require.Equal(t, "claude-sonnet-4-20250514", *usageRepo.lastLog.UpstreamModel)
	require.Equal(t, billingGroupID, *usageRepo.lastLog.GroupID)
	require.Equal(t, sourceGroupID, *usageRepo.lastLog.SourceGroupID)
	require.Equal(t, sourceGroup.RateMultiplier, usageRepo.lastLog.RateMultiplier)
}

func TestOpenAIGatewayServiceRecordUsage_SystemCustomRouteBillsSameSubscriptionFromAnotherSource(t *testing.T) {
	const (
		billingGroupID = int64(101)
		sourceGroupID  = int64(204)
		subscriptionID = int64(303)
	)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, subRepo, nil)

	sourceGroup := &Group{ID: sourceGroupID, Platform: PlatformOpenAI, RateMultiplier: 0.8}
	billingGroup := &Group{ID: billingGroupID, Platform: PlatformComposite, SubscriptionType: SubscriptionTypeSubscription}
	subscription := &UserSubscription{ID: subscriptionID, UserID: 22, GroupID: billingGroupID, Group: billingGroup}
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, "tavern-gpt", "gpt-5.1", PlatformOpenAI)

	err := svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:     "system-custom-openai",
			Model:         "gpt-5.1",
			UpstreamModel: "gpt-5.1-2026-01-01",
			Usage:         OpenAIUsage{InputTokens: 900, OutputTokens: 100},
			Duration:      time.Second,
		},
		APIKey:       &APIKey{ID: 12, GroupID: i64p(sourceGroupID), Group: sourceGroup},
		User:         &User{ID: 22},
		Account:      &Account{ID: 34, Platform: PlatformOpenAI},
		Subscription: subscription,
	})

	require.NoError(t, err)
	require.NotNil(t, billingRepo.lastCmd)
	require.Greater(t, billingRepo.lastCmd.SubscriptionCost, 0.0)
	require.Zero(t, billingRepo.lastCmd.BalanceCost)
	require.NotNil(t, billingRepo.lastCmd.SubscriptionID)
	require.Equal(t, subscriptionID, *billingRepo.lastCmd.SubscriptionID,
		"different source groups must accumulate into the same monthly subscription")
	require.Zero(t, userRepo.deductCalls)

	require.NotNil(t, usageRepo.lastLog)
	require.Equal(t, BillingTypeSubscription, usageRepo.lastLog.BillingType)
	require.Equal(t, "tavern-gpt", usageRepo.lastLog.RequestedModel)
	require.Equal(t, "gpt-5.1", usageRepo.lastLog.Model)
	require.NotNil(t, usageRepo.lastLog.UpstreamModel)
	require.Equal(t, "gpt-5.1-2026-01-01", *usageRepo.lastLog.UpstreamModel)
	require.Equal(t, billingGroupID, *usageRepo.lastLog.GroupID)
	require.Equal(t, sourceGroupID, *usageRepo.lastLog.SourceGroupID)
	require.Equal(t, sourceGroup.RateMultiplier, usageRepo.lastLog.RateMultiplier)
}

func TestOpenAIGatewayServiceRecordUsage_SystemCustomMissingPricingFailsClosed(t *testing.T) {
	const (
		billingGroupID = int64(101)
		sourceGroupID  = int64(204)
	)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, "tavern-unpriced", "unpriced-system-custom-model", PlatformOpenAI)

	err := svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "system-custom-unpriced",
			Model:     "unpriced-system-custom-model",
			Usage:     OpenAIUsage{InputTokens: 900, OutputTokens: 100},
		},
		APIKey:       &APIKey{ID: 12, GroupID: i64p(sourceGroupID), Group: &Group{ID: sourceGroupID, Platform: PlatformOpenAI, RateMultiplier: 1}},
		User:         &User{ID: 22},
		Account:      &Account{ID: 34, Platform: PlatformOpenAI},
		Subscription: &UserSubscription{ID: 303, GroupID: billingGroupID},
	})

	require.Error(t, err)
	require.Zero(t, billingRepo.calls)
	require.Zero(t, usageRepo.calls)
	require.Zero(t, userRepo.deductCalls)
}

func TestSystemCustomRecordUsage_FailsClosedOnMissingOrMismatchedSubscription(t *testing.T) {
	const (
		billingGroupID = int64(101)
		sourceGroupID  = int64(202)
	)
	tests := []struct {
		name         string
		subscription *UserSubscription
	}{
		{name: "missing subscription"},
		{name: "wrong subscription group id", subscription: &UserSubscription{ID: 1, GroupID: billingGroupID + 1}},
		{name: "wrong loaded subscription group", subscription: &UserSubscription{ID: 1, GroupID: billingGroupID, Group: &Group{ID: billingGroupID + 1, SubscriptionType: SubscriptionTypeSubscription}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
			userRepo := &openAIRecordUsageUserRepoStub{}
			svc := newGatewayRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, &openAIRecordUsageSubRepoStub{})
			ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, "tavern-sonnet", "claude-sonnet-4", PlatformAnthropic)

			err := svc.RecordUsage(ctx, &RecordUsageInput{
				Result:       &ForwardResult{RequestID: "system-custom-mismatch", Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 100}},
				APIKey:       &APIKey{ID: 11, GroupID: i64p(sourceGroupID), Group: &Group{ID: sourceGroupID, Platform: PlatformAnthropic, RateMultiplier: 1}},
				User:         &User{ID: 22},
				Account:      &Account{ID: 33, Platform: PlatformAnthropic},
				Subscription: tt.subscription,
			})

			require.ErrorIs(t, err, ErrSubscriptionInvalid)
			require.Zero(t, billingRepo.calls)
			require.Zero(t, userRepo.deductCalls)
			require.Zero(t, usageRepo.calls)
		})
	}
}

func TestOpenAISystemCustomRecordUsage_FailsClosedOnMismatchedSubscription(t *testing.T) {
	const (
		billingGroupID = int64(101)
		sourceGroupID  = int64(204)
	)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, "tavern-gpt", "gpt-5.1", PlatformOpenAI)

	err := svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result:       &OpenAIForwardResult{RequestID: "system-custom-openai-mismatch", Model: "gpt-5.1", Usage: OpenAIUsage{InputTokens: 100}},
		APIKey:       &APIKey{ID: 12, GroupID: i64p(sourceGroupID), Group: &Group{ID: sourceGroupID, Platform: PlatformOpenAI, RateMultiplier: 1}},
		User:         &User{ID: 22},
		Account:      &Account{ID: 34, Platform: PlatformOpenAI},
		Subscription: &UserSubscription{ID: 303, GroupID: billingGroupID + 1},
	})

	require.ErrorIs(t, err, ErrSubscriptionInvalid)
	require.Zero(t, billingRepo.calls)
	require.Zero(t, userRepo.deductCalls)
	require.Zero(t, usageRepo.calls)
}

func TestGatewayServiceRecordUsage_SystemCustomLegacyFallbackStillBillsSubscription(t *testing.T) {
	const (
		billingGroupID = int64(101)
		sourceGroupID  = int64(202)
		subscriptionID = int64(303)
	)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	svc := newGatewayRecordUsageServiceForTest(usageRepo, userRepo, subRepo)
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, "tavern-sonnet", "claude-sonnet-4", PlatformAnthropic)

	err := svc.RecordUsage(ctx, &RecordUsageInput{
		Result:       &ForwardResult{RequestID: "system-custom-legacy", Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 1000}},
		APIKey:       &APIKey{ID: 11, GroupID: i64p(sourceGroupID), Group: &Group{ID: sourceGroupID, Platform: PlatformAnthropic, RateMultiplier: 1}},
		User:         &User{ID: 22},
		Account:      &Account{ID: 33, Platform: PlatformAnthropic},
		Subscription: &UserSubscription{ID: subscriptionID, GroupID: billingGroupID},
	})

	require.NoError(t, err)
	require.Equal(t, 1, subRepo.incrementCalls)
	require.Zero(t, userRepo.deductCalls)
}

type subscriptionCacheGroupCapture struct {
	billingCacheWorkerStub
	lastGroupID atomic.Int64
}

func (b *subscriptionCacheGroupCapture) UpdateSubscriptionUsage(ctx context.Context, userID, groupID int64, cost float64) error {
	b.lastGroupID.Store(groupID)
	return b.billingCacheWorkerStub.UpdateSubscriptionUsage(ctx, userID, groupID, cost)
}

func TestFinalizePostUsageBilling_SystemCustomUsesAuthoritativeBillingGroupCacheKey(t *testing.T) {
	const (
		billingGroupID = int64(101)
		sourceGroupID  = int64(202)
	)
	cache := &subscriptionCacheGroupCapture{}
	cacheService := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(cacheService.Stop)
	ctx := systemCustomUsageContext(billingGroupID, sourceGroupID, "tavern-sonnet", "claude-sonnet-4", PlatformAnthropic)

	finalizePostUsageBilling(ctx, &postUsageBillingParams{
		Cost:               &CostBreakdown{TotalCost: 1, ActualCost: 1},
		User:               &User{ID: 22},
		APIKey:             &APIKey{ID: 11, GroupID: i64p(sourceGroupID)},
		Account:            &Account{ID: 33},
		Subscription:       &UserSubscription{ID: 303, GroupID: billingGroupID},
		IsSubscriptionBill: true,
	}, &billingDeps{
		billingCacheService: cacheService,
		deferredService:     &DeferredService{},
	}, &UsageBillingApplyResult{Applied: true})

	require.Eventually(t, func() bool {
		return cache.lastGroupID.Load() != 0
	}, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, billingGroupID, cache.lastGroupID.Load())
}
