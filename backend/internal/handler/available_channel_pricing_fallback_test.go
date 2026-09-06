package handler

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type pricingFallbackAccountRepoStub struct {
	service.AccountRepository
	accounts []service.Account
}

func (s *pricingFallbackAccountRepoStub) ListSchedulableByGroupID(context.Context, int64) ([]service.Account, error) {
	return s.accounts, nil
}

func TestAttachGroupSupportedModels_FillsGlobalPricingForGroupOnlyModel(t *testing.T) {
	groupID := int64(23)
	pricingService := newAvailableChannelsPricingService(t)

	h := &AvailableChannelHandler{
		channelService: service.NewChannelService(nil, nil, nil, pricingService),
		gatewayService: service.NewGatewayService(
			&pricingFallbackAccountRepoStub{accounts: []service.Account{
				{
					ID:       1,
					Platform: service.PlatformAnthropic,
					Credentials: map[string]any{
						"model_mapping": map[string]any{
							"claude-opus-5": "claude-opus-5",
						},
					},
				},
			}},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		),
	}
	groups := []userAvailableGroup{
		{ID: groupID, Name: "anthropic-live", Platform: service.PlatformAnthropic},
	}

	out := h.attachGroupSupportedModels(context.Background(), service.AvailableChannel{}, groups)

	require.Len(t, out, 1)
	require.Len(t, out[0].SupportedModels, 1)
	pricing := out[0].SupportedModels[0].Pricing
	require.NotNil(t, pricing)
	require.NotNil(t, pricing.InputPrice)
	require.NotNil(t, pricing.OutputPrice)
	require.InDelta(t, 5e-6, *pricing.InputPrice, 1e-12)
	require.InDelta(t, 2.5e-5, *pricing.OutputPrice, 1e-12)
}

func TestToUserSupportedModelsByIDs_KeepsChannelPricingAheadOfGlobalFallback(t *testing.T) {
	pricingService := newAvailableChannelsPricingService(t)
	h := &AvailableChannelHandler{
		channelService: service.NewChannelService(nil, nil, nil, pricingService),
	}
	customInputPrice := 9e-6

	out := h.toUserSupportedModelsByIDs([]service.SupportedModel{
		{
			Name:     "claude-opus-5",
			Platform: service.PlatformAnthropic,
			Pricing: &service.ChannelModelPricing{
				BillingMode: service.BillingModeToken,
				InputPrice:  &customInputPrice,
			},
		},
	}, service.PlatformAnthropic, []string{"claude-opus-5"})

	require.Len(t, out, 1)
	require.NotNil(t, out[0].Pricing)
	require.Same(t, &customInputPrice, out[0].Pricing.InputPrice)
	require.Nil(t, out[0].Pricing.OutputPrice)
}

func TestToUserSupportedModelsByIDs_LeavesUnknownModelUnpriced(t *testing.T) {
	pricingService := newAvailableChannelsPricingService(t)
	h := &AvailableChannelHandler{
		channelService: service.NewChannelService(nil, nil, nil, pricingService),
	}

	out := h.toUserSupportedModelsByIDs(nil, service.PlatformAnthropic, []string{"unknown-model-without-price"})

	require.Len(t, out, 1)
	require.Nil(t, out[0].Pricing)
}

func newAvailableChannelsPricingService(t *testing.T) *service.PricingService {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	pricingService := service.NewPricingService(&config.Config{
		Pricing: config.PricingConfig{
			DataDir:      t.TempDir(),
			FallbackFile: filepath.Join(filepath.Dir(sourceFile), "../../data/model_pricing.json"),
		},
	}, nil)
	require.NoError(t, pricingService.Initialize())
	t.Cleanup(pricingService.Stop)
	return pricingService
}
