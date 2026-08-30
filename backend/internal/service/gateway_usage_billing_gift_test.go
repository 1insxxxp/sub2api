//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGatewayServiceRecordUsage_PersistsThresholdExemptAllocation(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{
		Applied: true, ThresholdExemptCost: 0.00000001,
	}}
	svc := newGatewayRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})

	err := svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{RequestID: "gateway-gift-allocation", Usage: ClaudeUsage{InputTokens: 100, OutputTokens: 50}, Model: "claude-sonnet-4", Duration: time.Second},
		APIKey: &APIKey{ID: 701}, User: &User{ID: 702}, Account: &Account{ID: 703},
	})
	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.Greater(t, usageRepo.lastLog.ActualCost, 0.0)
	require.InDelta(t, 0.00000001, usageRepo.lastLog.ThresholdExemptCost, 0.000000001)
}

func TestOpenAIGatewayServiceRecordUsage_PersistsThresholdExemptAllocation(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{
		Applied: true, ThresholdExemptCost: 0.00000001,
	}}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)

	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{RequestID: "openai-gift-allocation", Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 50}, Model: "gpt-5.1", Duration: time.Second},
		APIKey: &APIKey{ID: 711}, User: &User{ID: 712}, Account: &Account{ID: 713},
	})
	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.Greater(t, usageRepo.lastLog.ActualCost, 0.0)
	require.InDelta(t, 0.00000001, usageRepo.lastLog.ThresholdExemptCost, 0.000000001)
}

func TestApplyUsageBilling_AttachesMixedGiftAllocation(t *testing.T) {
	usageLog := &UsageLog{ActualCost: 12}
	repo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true, ThresholdExemptCost: 10}}
	applied, err := applyUsageBilling(context.Background(), "mixed-gift", usageLog, &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 12, TotalCost: 12}, User: &User{ID: 1}, APIKey: &APIKey{ID: 2}, Account: &Account{ID: 3},
	}, &billingDeps{billingCacheService: &BillingCacheService{}, deferredService: &DeferredService{}}, repo)
	require.NoError(t, err)
	require.True(t, applied)
	require.InDelta(t, 12, usageLog.ActualCost, 0.00000001)
	require.InDelta(t, 10, usageLog.ThresholdExemptCost, 0.00000001)
}

func TestApplyUsageBilling_ClampsThresholdExemptAllocationToActualCost(t *testing.T) {
	usageLog := &UsageLog{ActualCost: 12}
	repo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true, ThresholdExemptCost: 99}}
	applied, err := applyUsageBilling(context.Background(), "clamped-gift", usageLog, &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 12, TotalCost: 12}, User: &User{ID: 1}, APIKey: &APIKey{ID: 2}, Account: &Account{ID: 3},
	}, &billingDeps{billingCacheService: &BillingCacheService{}, deferredService: &DeferredService{}}, repo)
	require.NoError(t, err)
	require.True(t, applied)
	require.InDelta(t, 12, usageLog.ActualCost, 0.00000001)
	require.InDelta(t, 12, usageLog.ThresholdExemptCost, 0.00000001)
}

func TestApplyUsageBilling_DoesNotAllocateOnFailureOrDedup(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result *UsageBillingApplyResult
		err    error
	}{
		{name: "billing failure", err: errors.New("billing failed")},
		{name: "deduplicated", result: &UsageBillingApplyResult{Applied: false, ThresholdExemptCost: 10}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usageLog := &UsageLog{ActualCost: 12}
			repo := &openAIRecordUsageBillingRepoStub{result: tc.result, err: tc.err}
			applied, err := applyUsageBilling(context.Background(), "no-gift-allocation", usageLog, &postUsageBillingParams{
				Cost: &CostBreakdown{ActualCost: 12, TotalCost: 12}, User: &User{ID: 1}, APIKey: &APIKey{ID: 2}, Account: &Account{ID: 3},
			}, &billingDeps{billingCacheService: &BillingCacheService{}, deferredService: &DeferredService{}}, repo)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}
			require.False(t, applied)
			require.Zero(t, usageLog.ThresholdExemptCost)
		})
	}
}
