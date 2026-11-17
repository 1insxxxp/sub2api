package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIUltrafastAliasBillingKeepsCustomBasePrice(t *testing.T) {
	for _, tc := range []struct {
		name       string
		upstream   string
		perRequest bool
		multiplier float64
	}{
		{"sol", "gpt-6.1-sol", false, 6},
		{"astra", "gpt-6-astra", false, 6},
		{"other_model", "gpt-5.4", false, 2},
		{"fixed_price", "gpt-6.1-sol", true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			userRepo := &openAIRecordUsageUserRepoStub{}
			svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
			svc.resolver = NewModelPricingResolver(nil, svc.billingService)
			groupID := int64(77)
			inputPrice, outputPrice, fixedPrice, fastMultiplier := 0.001, 0.002, 0.25, 3.0
			price := ChannelModelPricing{Models: []string{"sol-public"}, BillingMode: BillingModeToken,
				InputPrice: &inputPrice, OutputPrice: &outputPrice, FastMultiplier: &fastMultiplier}
			base := 100*inputPrice + 20*outputPrice
			if tc.perRequest {
				price.BillingMode, price.PerRequestPrice = BillingModePerRequest, &fixedPrice
				base = fixedPrice
			}
			group := &Group{ID: groupID, Platform: PlatformOpenAI, Hydrated: true, RateMultiplier: 0.5,
				ForceOpenAIUltrafast: true, FreeOpenAIFast: true, ModelPricing: []ChannelModelPricing{price}}
			tier := "ultrafast"
			err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
				Result: &OpenAIForwardResult{RequestID: "ultrafast-alias-" + tc.name, Model: "sol-public",
					BillingModel: "sol-public", UpstreamModel: tc.upstream, ServiceTier: &tier,
					Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 20}},
				APIKey: &APIKey{ID: 1008, GroupID: &groupID, Group: group}, User: &User{ID: 2008},
				Account:            &Account{ID: 3008, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
				ChannelUsageFields: ChannelUsageFields{OriginalModel: "sol-public", BillingModelSource: BillingModelSourceRequested},
			})
			require.NoError(t, err)
			require.NotNil(t, usageRepo.lastLog)
			require.Equal(t, "sol-public", usageRepo.lastLog.RequestedModel)
			require.InDelta(t, base*tc.multiplier, usageRepo.lastLog.TotalCost, 1e-12)
			require.InDelta(t, base*tc.multiplier*0.5, usageRepo.lastLog.ActualCost, 1e-12,
				"free Fast must not waive Ultrafast; fixed request prices must stay fixed")
			require.InDelta(t, base*tc.multiplier*0.5, userRepo.lastAmount, 1e-12)
		})
	}
}
