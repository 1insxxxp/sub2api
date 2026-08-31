package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func coveragePrice(value float64) *float64 { return &value }

func newGroupPricingCoverageTestService(litellm map[string]*LiteLLMModelPricing) *GroupPricingCoverageService {
	pricingService := &PricingService{pricingData: litellm}
	billingService := NewBillingService(&config.Config{}, pricingService)
	return NewGroupPricingCoverageService(NewModelPricingResolver(nil, billingService))
}

func TestGroupPricingCoverageUsesProspectiveUnsavedGroupPricing(t *testing.T) {
	svc := newGroupPricingCoverageTestService(nil)
	pricing := []ChannelModelPricing{{
		Platform:    PlatformGemini,
		Models:      []string{"new-unique-model"},
		BillingMode: BillingModeToken,
		InputPrice:  coveragePrice(1e-6),
	}}

	result := svc.Preview(context.Background(), GroupPricingCoverageInput{
		Platform:     PlatformGemini,
		Models:       []string{"new-unique-model"},
		ModelPricing: &pricing,
	})

	require.Len(t, result.Models, 1)
	require.Equal(t, PricingCoveragePriced, result.Models[0].Status)
	require.Equal(t, PricingSourceGroup, result.Models[0].Source)
	require.Equal(t, BillingModeToken, result.Models[0].BillingMode)
}

func TestGroupPricingCoverageUsesExistingChannelPricing(t *testing.T) {
	groupID := int64(77)
	pricing := ChannelModelPricing{
		Platform:        PlatformGemini,
		Models:          []string{"channel-only-model"},
		BillingMode:     BillingModePerRequest,
		PerRequestPrice: coveragePrice(0.04),
	}
	channel := Channel{
		ID:           1,
		Status:       StatusActive,
		ModelPricing: []ChannelModelPricing{pricing},
		GroupIDs:     []int64{groupID},
	}
	channelService := &ChannelService{}
	channelService.cache.Store(populateChannelCache([]Channel{channel}, map[int64]string{groupID: PlatformGemini}))
	billingService := NewBillingService(&config.Config{}, &PricingService{pricingData: map[string]*LiteLLMModelPricing{}})
	svc := NewGroupPricingCoverageService(NewModelPricingResolver(channelService, billingService))

	result := svc.Preview(context.Background(), GroupPricingCoverageInput{
		GroupID:  &groupID,
		Platform: PlatformGemini,
		Models:   []string{"channel-only-model"},
	})

	require.Len(t, result.Models, 1)
	require.Equal(t, PricingCoveragePriced, result.Models[0].Status)
	require.Equal(t, PricingSourceChannel, result.Models[0].Source)
	require.Equal(t, BillingModePerRequest, result.Models[0].BillingMode)
}

func TestGroupPricingCoverageDistinguishesLiteLLMAndFallbackSources(t *testing.T) {
	svc := newGroupPricingCoverageTestService(map[string]*LiteLLMModelPricing{
		"catalog-only-model": {
			InputCostPerToken:  2e-6,
			OutputCostPerToken: 8e-6,
		},
	})

	result := svc.Preview(context.Background(), GroupPricingCoverageInput{
		Platform: PlatformAnthropic,
		Models:   []string{"catalog-only-model", "claude-sonnet-4"},
	})

	require.Len(t, result.Models, 2)
	require.Equal(t, PricingCoveragePriced, result.Models[0].Status)
	require.Equal(t, PricingSourceLiteLLM, result.Models[0].Source)
	require.Equal(t, PricingCoveragePriced, result.Models[1].Status)
	require.Equal(t, PricingSourceFallback, result.Models[1].Source)
}

func TestGroupPricingCoverageNormalizesAndDeduplicatesModelNames(t *testing.T) {
	svc := newGroupPricingCoverageTestService(nil)
	pricing := []ChannelModelPricing{{
		Platform:    PlatformGemini,
		Models:      []string{"custom.model"},
		BillingMode: BillingModeToken,
		OutputPrice: coveragePrice(2e-6),
	}}

	result := svc.Preview(context.Background(), GroupPricingCoverageInput{
		Platform:     PlatformGemini,
		Models:       []string{"  Custom.Model  ", "custom.model", ""},
		ModelPricing: &pricing,
	})

	require.Len(t, result.Models, 1)
	require.Equal(t, "custom.model", result.Models[0].Model)
	require.Equal(t, PricingCoveragePriced, result.Models[0].Status)
}

func TestGroupPricingCoverageSupportsPerRequestAndImageBilling(t *testing.T) {
	svc := newGroupPricingCoverageTestService(nil)
	pricing := []ChannelModelPricing{
		{
			Platform:        PlatformGemini,
			Models:          []string{"request-model"},
			BillingMode:     BillingModePerRequest,
			PerRequestPrice: coveragePrice(0.03),
		},
		{
			Platform:         PlatformGemini,
			Models:           []string{"image-model"},
			BillingMode:      BillingModeImage,
			PerRequestPrice:  coveragePrice(0.05),
			ImageOutputPrice: coveragePrice(4e-6),
		},
	}

	result := svc.Preview(context.Background(), GroupPricingCoverageInput{
		Platform:     PlatformGemini,
		Models:       []string{"request-model", "image-model"},
		ModelPricing: &pricing,
	})

	require.Len(t, result.Models, 2)
	require.Equal(t, PricingCoveragePriced, result.Models[0].Status)
	require.Equal(t, BillingModePerRequest, result.Models[0].BillingMode)
	require.Equal(t, PricingCoveragePriced, result.Models[1].Status)
	require.Equal(t, BillingModeImage, result.Models[1].BillingMode)
}

func TestGroupPricingCoverageMarksUnknownModelMissing(t *testing.T) {
	svc := newGroupPricingCoverageTestService(nil)

	result := svc.Preview(context.Background(), GroupPricingCoverageInput{
		Platform: PlatformGemini,
		Models:   []string{"new-unique-model"},
	})

	require.Len(t, result.Models, 1)
	require.Equal(t, PricingCoverageMissing, result.Models[0].Status)
	require.Equal(t, "PRICING_NOT_FOUND", result.Models[0].Reason)
}

func TestGroupPricingCoverageMarksIncompleteProspectivePricingInvalid(t *testing.T) {
	svc := newGroupPricingCoverageTestService(nil)
	pricing := []ChannelModelPricing{{
		Platform:    PlatformGemini,
		Models:      []string{"incomplete-request-model"},
		BillingMode: BillingModePerRequest,
	}}

	result := svc.Preview(context.Background(), GroupPricingCoverageInput{
		Platform:     PlatformGemini,
		Models:       []string{"incomplete-request-model"},
		ModelPricing: &pricing,
	})

	require.Len(t, result.Models, 1)
	require.Equal(t, PricingCoverageInvalid, result.Models[0].Status)
	require.Equal(t, BillingModePerRequest, result.Models[0].BillingMode)
	require.Equal(t, "BILLING_MODE_MISSING_PRICE", result.Models[0].Reason)
}

func TestGroupPricingCoverageMarksNegativeProspectivePricingInvalid(t *testing.T) {
	svc := newGroupPricingCoverageTestService(nil)
	pricing := []ChannelModelPricing{{
		Platform:    PlatformGemini,
		Models:      []string{"negative-price-model"},
		BillingMode: BillingModeToken,
		InputPrice:  coveragePrice(-1e-6),
	}}

	result := svc.Preview(context.Background(), GroupPricingCoverageInput{
		Platform:     PlatformGemini,
		Models:       []string{"negative-price-model"},
		ModelPricing: &pricing,
	})

	require.Len(t, result.Models, 1)
	require.Equal(t, PricingCoverageInvalid, result.Models[0].Status)
	require.Equal(t, BillingModeToken, result.Models[0].BillingMode)
	require.Equal(t, "NEGATIVE_PRICE", result.Models[0].Reason)
}
