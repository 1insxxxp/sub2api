package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestResolve_GroupRequestedModelPerRequestIsExactAndCaseInsensitive(t *testing.T) {
	const (
		groupID = int64(1700)
		price   = 0.05
	)
	r := newOpenAITextChannelPricingResolverForTest(t, groupID, "tavern-roleplay-pro", price)

	for _, model := range []string{"tavern-roleplay-pro", "TAVERN-ROLEPLAY-PRO"} {
		resolved := r.Resolve(context.Background(), PricingInput{
			Model:   model,
			GroupID: i64p(groupID),
		})

		require.Equal(t, BillingModePerRequest, resolved.Mode)
		require.Equal(t, PricingSourceChannel, resolved.Source)
		require.InDelta(t, price, resolved.DefaultPerRequestPrice, 1e-12)
	}

	resolved := r.Resolve(context.Background(), PricingInput{
		Model:   "tavern-roleplay-basic",
		GroupID: i64p(groupID),
	})

	require.Equal(t, BillingModeToken, resolved.Mode)
	require.NotEqual(t, PricingSourceChannel, resolved.Source)
}

func TestGatewayCalculateRecordUsageCost_PerRequestReplacesTokenCost(t *testing.T) {
	const (
		groupID = int64(1701)
		model   = "tavern-roleplay-pro"
		price   = 0.08
	)
	svc := &GatewayService{
		billingService: NewBillingService(&config.Config{}, nil),
		resolver:       newOpenAITextChannelPricingResolverForTest(t, groupID, model, price),
	}
	result := &ForwardResult{
		Model: model,
		Usage: ClaudeUsage{
			InputTokens:              5000,
			OutputTokens:             1200,
			CacheCreationInputTokens: 300,
			CacheReadInputTokens:     700,
		},
	}

	cost := svc.calculateRecordUsageCost(
		context.Background(),
		result,
		&APIKey{GroupID: i64p(groupID), Group: &Group{ID: groupID}},
		model,
		1.25,
		1,
		nil,
	)

	require.Equal(t, string(BillingModePerRequest), cost.BillingMode)
	require.Zero(t, cost.InputCost)
	require.Zero(t, cost.OutputCost)
	require.Zero(t, cost.CacheCreationCost)
	require.Zero(t, cost.CacheReadCost)
	require.InDelta(t, price, cost.TotalCost, 1e-12)
	require.InDelta(t, price*1.25, cost.ActualCost, 1e-12)

	usageLog := svc.buildRecordUsageLog(
		context.Background(),
		&recordUsageCoreInput{},
		result,
		&APIKey{ID: 1, GroupID: i64p(groupID), Group: &Group{ID: groupID}},
		&User{ID: 2},
		&Account{ID: 3},
		nil,
		model,
		1.25,
		1,
		1,
		BillingTypeBalance,
		false,
		cost,
		&recordUsageOpts{},
	)

	require.Equal(t, 5000, usageLog.InputTokens)
	require.Equal(t, 1200, usageLog.OutputTokens)
	require.Equal(t, 300, usageLog.CacheCreationTokens)
	require.Equal(t, 700, usageLog.CacheReadTokens)
	require.NotNil(t, usageLog.BillingMode)
	require.Equal(t, string(BillingModePerRequest), *usageLog.BillingMode)
}
