//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func geminiInputCountingBillingResult() *ForwardResult {
	return &ForwardResult{
		RequestID: "ent-policy-billing", Model: "gemini-4-argon", UpstreamModel: "[量]gemini-4-argon",
		Usage:             ClaudeUsage{InputTokens: 673, OutputTokens: 10},
		InputTokensSource: UsageTokenSourceUpstream, OutputTokensSource: UsageTokenSourceUpstream,
		RequireMeasuredUsage: true, MeasuredUsageComplete: true, GeminiUsageFinalized: true,
		GeminiUsageFallback: &GeminiUsageFallback{UsageFields: 15},
		GeminiInputCounting: &GeminiInputCountingEvidence{AccountID: 103, Policy: GeminiInputCountingVertexLikeDiscountV1, CandidateInputTokens: 421},
		Outcome:             &ResponseOutcome{CollectorVersion: ResponseOutcomeCollectorVersion, HasText: true, StreamCompleted: true},
		Duration:            time.Second,
	}
}

func TestGeminiInputCountingCustomerBillingClone(t *testing.T) {
	raw := geminiInputCountingBillingResult()
	customer := customerBillingForwardResult(raw)
	require.NotSame(t, raw, customer)
	require.Equal(t, 421, customer.Usage.InputTokens)
	require.Equal(t, UsageTokenSourceEstimated, customer.InputTokensSource)
	require.Equal(t, raw.Usage.OutputTokens, customer.Usage.OutputTokens)
	require.Equal(t, 673, raw.Usage.InputTokens)
	require.Equal(t, UsageTokenSourceUpstream, raw.InputTokensSource)
	for _, candidate := range []int{673, 800} {
		raw.GeminiInputCounting.CandidateInputTokens = candidate
		require.Same(t, raw, customerBillingForwardResult(raw), "policy must never increase measured input")
	}
}

func TestGeminiInputCountingRecordUsagePreservesRawCosts(t *testing.T) {
	for _, accountMatches := range []bool{true, false} {
		for _, channelPricing := range []bool{true, false} {
			for _, tiered := range []bool{false, true} {
				if tiered && !channelPricing {
					continue // Context intervals belong to channel pricing, not group cards.
				}
				t.Run(map[bool]string{true: "matching", false: "wrong-account"}[accountMatches]+"/"+map[bool]string{true: "channel-stats", false: "default-stats"}[channelPricing]+"/"+map[bool]string{true: "tiered", false: "flat"}[tiered], func(t *testing.T) {
					repo := &openAIRecordUsageLogRepoStub{inserted: true}
					userRepo := &openAIRecordUsageUserRepoStub{}
					svc := newGatewayRecordUsageServiceForTest(repo, userRepo, &openAIRecordUsageSubRepoStub{})
					billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
					svc.usageBillingRepo = billingRepo
					groupID := int64(104)
					if channelPricing {
						svc.channelService = newTestChannelServiceForStats(t, &Channel{ID: 1, Status: StatusActive, ApplyPricingToAccountStats: true}, groupID, PlatformGemini)
					}
					svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)
					inputPrice, outputPrice := 0.001, 0.002
					group := &Group{ID: groupID, Platform: PlatformGemini, RateMultiplier: 0.8,
						ModelPricing: []ChannelModelPricing{{Models: []string{"gemini-4-argon"}, BillingMode: BillingModeToken, InputPrice: &inputPrice, OutputPrice: &outputPrice}},
					}
					if tiered {
						group.LongContextPricingEnabled = true
						boundary := 600
						highInputPrice := inputPrice * 3
						group.ModelPricing[0].Intervals = []PricingInterval{
							{MinTokens: 0, MaxTokens: &boundary, InputPrice: &inputPrice, OutputPrice: &outputPrice},
							{MinTokens: boundary, InputPrice: &highInputPrice, OutputPrice: &outputPrice},
						}
						svc.channelService = newTestChannelServiceForStats(t, &Channel{
							ID: 1, Status: StatusActive, ApplyPricingToAccountStats: true, ModelPricing: group.ModelPricing,
						}, groupID, PlatformGemini)
						svc.channelService.cache.Load().(*channelCache).pricingByGroupModel[channelModelKey{groupID: groupID, platform: PlatformGemini, model: "gemini-4-argon"}] = &group.ModelPricing[0]
						group.ModelPricing = nil
						svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)
					}
					raw := geminiInputCountingBillingResult()
					if !accountMatches {
						raw.GeminiInputCounting.AccountID++
					}
					rate := 0.5
					err := svc.RecordUsage(context.Background(), &RecordUsageInput{
						Result: raw, APIKey: &APIKey{ID: 101, Quota: 100, GroupID: &groupID, Group: group}, User: &User{ID: 102},
						Account:            &Account{ID: 103, Platform: PlatformGemini, Type: AccountTypeAPIKey, RateMultiplier: &rate, Extra: map[string]any{"quota_limit": 100.0}},
						APIKeyService:      &openAIRecordUsageAPIKeyQuotaStub{},
						ChannelUsageFields: ChannelUsageFields{BillingModelSource: BillingModelSourceRequested, OriginalModel: "gemini-4-argon"},
					})
					require.NoError(t, err)
					require.NotNil(t, repo.lastLog)
					log := repo.lastLog
					wantInput := 673
					if accountMatches {
						wantInput = 421
					}
					require.Equal(t, wantInput, log.InputTokens)
					require.Equal(t, 10, log.OutputTokens)
					wantInputPrice, rawInputPrice := inputPrice, inputPrice
					if tiered {
						rawInputPrice *= 3
						if wantInput >= 600 {
							wantInputPrice *= 3
						}
					}
					wantCost := float64(wantInput)*wantInputPrice + 10*outputPrice
					rawCost := 673*rawInputPrice + 10*outputPrice
					require.InDelta(t, wantCost, log.TotalCost, 1e-12)
					require.InDelta(t, wantCost*0.8, log.ActualCost, 1e-8)
					require.NotNil(t, billingRepo.lastCmd)
					require.InDelta(t, wantCost*0.8, billingRepo.lastCmd.BalanceCost, 1e-8)
					require.InDelta(t, wantCost*0.8, billingRepo.lastCmd.APIKeyQuotaCost, 1e-8)
					require.Equal(t, 1, billingRepo.calls)
					require.InDelta(t, rawCost*rate, billingRepo.lastCmd.AccountQuotaCost, 1e-8)
					if accountMatches {
						require.Equal(t, UsageTokenSourceEstimated, log.InputTokensSource)
						require.NotNil(t, log.AccountStatsCost)
						require.InDelta(t, rawCost, *log.AccountStatsCost, 1e-12)
						require.NotNil(t, log.Outcome)
						require.Equal(t, 673, *log.Outcome.UpstreamInputTokens)
						require.Equal(t, 421, *log.Outcome.InputTokenPolicyCount)
						require.Equal(t, GeminiInputCountingVertexLikeDiscountV1, log.Outcome.InputTokenPolicy)
					} else {
						require.Equal(t, UsageTokenSourceUpstream, log.InputTokensSource)
						require.Nil(t, log.Outcome.UpstreamInputTokens)
					}
					require.Equal(t, 673, raw.Usage.InputTokens)
					require.Equal(t, UsageTokenSourceUpstream, raw.InputTokensSource)
					require.Nil(t, raw.Outcome.UpstreamInputTokens, "audit must not mutate captured upstream evidence")
				})
			}
		}
	}
}

func TestGeminiInputCountingAccountQuotaUsesRawOverride(t *testing.T) {
	rawCost := 7.0
	p := &postUsageBillingParams{
		Cost: &CostBreakdown{TotalCost: 4, ActualCost: 3}, AccountQuotaTotalCost: &rawCost,
		User: &User{ID: 1}, APIKey: &APIKey{ID: 2}, AccountRateMultiplier: 0.5,
		Account: &Account{ID: 3, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_limit": 100.0}},
	}
	cmd := buildUsageBillingCommand("ent-quota", nil, p)
	require.Equal(t, 3.0, cmd.BalanceCost)
	require.Equal(t, 3.5, cmd.AccountQuotaCost)
}

func TestGeminiInputCountingAuditWithoutCollectorStaysUnknown(t *testing.T) {
	raw := geminiInputCountingBillingResult()
	raw.Outcome = nil
	svc := &GatewayService{}
	log := svc.buildRecordUsageLog(context.Background(), &recordUsageCoreInput{}, raw,
		&APIKey{ID: 101}, &User{ID: 102}, &Account{ID: 103}, nil,
		"gemini-4-argon", 1, 1, 1, BillingTypeBalance, false, &CostBreakdown{}, nil)
	require.Equal(t, 421, log.InputTokens)
	require.NotNil(t, log.Outcome)
	require.Equal(t, 673, *log.Outcome.UpstreamInputTokens)
	require.Zero(t, log.Outcome.CollectorVersion)
	require.Equal(t, UsageRequestUnknown, ClassifyUsageRequestStatus(log.Outcome))
	require.Nil(t, raw.Outcome)
}

func TestGeminiInputCountingVolumeDiscountNeverIncreasesCharge(t *testing.T) {
	repo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newGatewayRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	svc.usageBillingRepo = billingRepo
	groupID := int64(104)
	lowInputPrice, highInputPrice, outputPrice := 0.003, 0.001, 0.002
	boundary := 600
	pricing := ChannelModelPricing{Models: []string{"gemini-4-argon"}, BillingMode: BillingModeToken, InputPrice: &lowInputPrice, OutputPrice: &outputPrice, Intervals: []PricingInterval{
		{MinTokens: 0, MaxTokens: &boundary, InputPrice: &lowInputPrice, OutputPrice: &outputPrice},
		{MinTokens: boundary, InputPrice: &highInputPrice, OutputPrice: &outputPrice},
	}}
	group := &Group{ID: groupID, Platform: PlatformGemini, RateMultiplier: 0.8, LongContextPricingEnabled: true}
	svc.channelService = newTestChannelServiceForStats(t, &Channel{ID: 1, Status: StatusActive, ApplyPricingToAccountStats: true, ModelPricing: []ChannelModelPricing{pricing}}, groupID, PlatformGemini)
	svc.channelService.cache.Load().(*channelCache).pricingByGroupModel[channelModelKey{groupID: groupID, platform: PlatformGemini, model: "gemini-4-argon"}] = &pricing
	svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)
	rate := 0.5
	err := svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: geminiInputCountingBillingResult(), APIKey: &APIKey{ID: 101, Quota: 100, GroupID: &groupID, Group: group}, User: &User{ID: 102},
		Account:       &Account{ID: 103, Platform: PlatformGemini, Type: AccountTypeAPIKey, RateMultiplier: &rate, Extra: map[string]any{"quota_limit": 100.0}},
		APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}, ChannelUsageFields: ChannelUsageFields{BillingModelSource: BillingModelSourceRequested, OriginalModel: "gemini-4-argon"},
	})
	require.NoError(t, err)
	require.NotNil(t, repo.lastLog)
	require.Equal(t, 421, repo.lastLog.InputTokens)
	require.Equal(t, 673, *repo.lastLog.Outcome.UpstreamInputTokens)
	require.InDelta(t, 673*highInputPrice+10*outputPrice, *repo.lastLog.AccountStatsCost, 1e-12)
	require.LessOrEqual(t, repo.lastLog.TotalCost, 673*highInputPrice+10*outputPrice, "input policy must never increase customer charges under the approved design")
}
