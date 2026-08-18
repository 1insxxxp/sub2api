//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
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

func TestResolvedChannelPricingUsableAcceptsExplicitImageAndPerRequestRows(t *testing.T) {
	imagePrice := 0.12
	requestPrice := 0.03

	require.True(t, resolvedChannelPricingUsable(&ResolvedPricing{
		Mode:   BillingModeImage,
		Source: PricingSourceChannel,
		channelPricing: &ChannelModelPricing{
			BillingMode:      BillingModeImage,
			ImageOutputPrice: &imagePrice,
		},
	}))
	require.True(t, resolvedChannelPricingUsable(&ResolvedPricing{
		Mode:   BillingModePerRequest,
		Source: PricingSourceChannel,
		channelPricing: &ChannelModelPricing{
			BillingMode:     BillingModePerRequest,
			PerRequestPrice: &requestPrice,
		},
	}))
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
