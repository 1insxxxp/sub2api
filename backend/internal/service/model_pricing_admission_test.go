//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func newPricingAdmissionTestBillingService() *BillingService {
	pricingService := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"gemini-3.5-flash": {
			InputCostPerToken:  1.5e-6,
			OutputCostPerToken: 9e-6,
		},
	}}
	return NewBillingService(&config.Config{}, pricingService)
}

func TestModelPricingAdmissionRejectsUnknownRequestedAndConcreteModels(t *testing.T) {
	svc := &GatewayService{billingService: newTestBillingService()}

	err := svc.ensureModelPricingAvailable(
		context.Background(),
		nil,
		"vendor-unknown-model",
		"vendor-unknown-model",
	)

	require.ErrorIs(t, err, ErrModelPricingUnavailable)
}

func TestModelPricingAdmissionRejectsEmptyModel(t *testing.T) {
	svc := &GatewayService{billingService: newTestBillingService()}

	err := svc.ensureModelPricingAvailable(context.Background(), nil, "")

	require.ErrorIs(t, err, ErrModelPricingUnavailable)
}

func TestModelPricingAdmissionAcceptsPricedConcreteFallback(t *testing.T) {
	svc := &GatewayService{billingService: newTestBillingService()}

	err := svc.ensureModelPricingAvailable(
		context.Background(),
		nil,
		"public-unpriced-alias",
		"gemini-3.1-pro",
	)

	require.NoError(t, err)
}

func TestModelPricingAdmissionAcceptsKnownGeminiAlias(t *testing.T) {
	svc := &GatewayService{billingService: newPricingAdmissionTestBillingService()}

	err := svc.ensureModelPricingAvailable(
		context.Background(),
		nil,
		"gemini-3.5-flash-low",
		"gemini-3.5-flash-low",
	)

	require.NoError(t, err)
}

func TestResolvedChannelPricingUsableRejectsEmptyTokenRow(t *testing.T) {
	require.False(t, resolvedChannelPricingUsable(&ResolvedPricing{
		Mode:           BillingModeToken,
		Source:         PricingSourceChannel,
		BasePricing:    &ModelPricing{},
		channelPricing: &ChannelModelPricing{BillingMode: BillingModeToken},
	}))
}

func TestResolvedChannelPricingUsableMatchesRequestSettlementModes(t *testing.T) {
	requestPrice := 0.03
	imageOutputPrice := 0.12
	tier := PricingInterval{TierLabel: "standard", PerRequestPrice: &requestPrice}

	tests := []struct {
		name     string
		resolved *ResolvedPricing
		want     bool
	}{
		{
			name: "image flat per request",
			resolved: &ResolvedPricing{Mode: BillingModeImage, Source: PricingSourceChannel,
				channelPricing: &ChannelModelPricing{BillingMode: BillingModeImage, PerRequestPrice: &requestPrice}},
			want: true,
		},
		{
			name: "image request tier",
			resolved: &ResolvedPricing{Mode: BillingModeImage, Source: PricingSourceChannel,
				channelPricing: &ChannelModelPricing{BillingMode: BillingModeImage, Intervals: []PricingInterval{tier}}, RequestTiers: []PricingInterval{tier}},
			want: true,
		},
		{
			name: "per request tier",
			resolved: &ResolvedPricing{Mode: BillingModePerRequest, Source: PricingSourceChannel,
				channelPricing: &ChannelModelPricing{BillingMode: BillingModePerRequest, Intervals: []PricingInterval{tier}}, RequestTiers: []PricingInterval{tier}},
			want: true,
		},
		{
			name: "video flat per request",
			resolved: &ResolvedPricing{Mode: BillingModeVideo, Source: PricingSourceChannel,
				channelPricing: &ChannelModelPricing{BillingMode: BillingModeVideo, PerRequestPrice: &requestPrice}},
			want: true,
		},
		{
			name: "video request tier",
			resolved: &ResolvedPricing{Mode: BillingModeVideo, Source: PricingSourceChannel,
				channelPricing: &ChannelModelPricing{BillingMode: BillingModeVideo, Intervals: []PricingInterval{tier}}, RequestTiers: []PricingInterval{tier}},
			want: true,
		},
		{
			name: "image output token price is not request settlement",
			resolved: &ResolvedPricing{Mode: BillingModeImage, Source: PricingSourceChannel,
				channelPricing: &ChannelModelPricing{BillingMode: BillingModeImage, ImageOutputPrice: &imageOutputPrice}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, resolvedChannelPricingUsable(tt.resolved))
		})
	}
}

func TestModelPricingAdmissionUsesLoadedGroupRequestSettlementPricing(t *testing.T) {
	tierPrice := 0.06
	tests := []struct {
		name    string
		mode    BillingMode
		flat    *float64
		tiers   []PricingInterval
		modelID string
	}{
		{name: "image flat", mode: BillingModeImage, flat: coveragePrice(0.04), modelID: "image-flat-only"},
		{name: "image tiers", mode: BillingModeImage, tiers: []PricingInterval{{TierLabel: "1k", PerRequestPrice: &tierPrice}}, modelID: "image-tier-only"},
		{name: "per request tiers", mode: BillingModePerRequest, tiers: []PricingInterval{{TierLabel: "standard", PerRequestPrice: &tierPrice}}, modelID: "request-tier-only"},
		{name: "video flat", mode: BillingModeVideo, flat: coveragePrice(0.08), modelID: "video-flat-only"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := &Group{
				ID: 71, Platform: PlatformGemini, Status: StatusActive, Hydrated: true,
				ModelPricing: []ChannelModelPricing{{
					Platform: PlatformGemini, Models: []string{tt.modelID}, BillingMode: tt.mode,
					PerRequestPrice: tt.flat, Intervals: tt.tiers,
				}},
			}
			billing := newTestBillingService()
			svc := &GatewayService{billingService: billing, resolver: NewModelPricingResolver(nil, billing)}
			ctx := context.WithValue(context.Background(), ctxkey.Group, group)

			err := svc.ensureModelPricingAvailable(ctx, &group.ID, tt.modelID)

			require.NoError(t, err)
		})
	}
}

func TestGatewaySelectedAccountPricingAdmissionUsesConcreteAccountMapping(t *testing.T) {
	svc := &GatewayService{billingService: newTestBillingService()}
	account := &Account{Credentials: map[string]any{
		"model_mapping": map[string]any{"public-alias": "gemini-3.1-pro"},
	}}

	err := svc.validateSelectedAccountPricing(context.Background(), nil, "public-alias", account)

	require.NoError(t, err)
}

func TestGatewaySelectedAccountPricingAdmissionRejectsUnknownConcreteModel(t *testing.T) {
	svc := &GatewayService{billingService: newTestBillingService()}
	account := &Account{Credentials: map[string]any{
		"model_mapping": map[string]any{"public-alias": "vendor-unknown-model"},
	}}

	err := svc.validateSelectedAccountPricing(context.Background(), nil, "public-alias", account)

	require.ErrorIs(t, err, ErrModelPricingUnavailable)
}

func TestOpenAISelectedAccountPricingAdmissionUsesConcreteAccountMapping(t *testing.T) {
	svc := &OpenAIGatewayService{billingService: newTestBillingService()}
	account := &Account{Credentials: map[string]any{
		"model_mapping": map[string]any{"public-alias": "gpt-5.4"},
	}}

	err := svc.validateSelectedAccountPricing(context.Background(), nil, "public-alias", account)

	require.NoError(t, err)
}

func TestOpenAISelectedAccountPricingAdmissionRejectsUnknownConcreteModel(t *testing.T) {
	svc := &OpenAIGatewayService{billingService: newTestBillingService()}
	account := &Account{Credentials: map[string]any{
		"model_mapping": map[string]any{"public-alias": "gpt-unknown-model"},
	}}

	err := svc.validateSelectedAccountPricing(context.Background(), nil, "public-alias", account)

	require.ErrorIs(t, err, ErrModelPricingUnavailable)
}

func TestGatewayRecordUsageRejectsUnexpectedPricingMiss(t *testing.T) {
	svc := &GatewayService{billingService: newTestBillingService()}
	result := &ForwardResult{Model: "vendor-unknown-model", UpstreamModel: "vendor-unknown-model"}

	cost, err := svc.calculateRecordUsageCostChecked(
		context.Background(),
		result,
		&APIKey{},
		"vendor-unknown-model",
		1,
		1,
		time.Now(),
		&recordUsageOpts{},
	)

	require.ErrorIs(t, err, ErrModelPricingUnavailable)
	require.Nil(t, cost)
}
