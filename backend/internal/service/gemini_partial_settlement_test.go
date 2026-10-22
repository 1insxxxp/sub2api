//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Exercise the real normal-mode pricing and settlement command, not simple-mode
// logging: retaining provider usage must never charge its undelivered tail.
func TestGeminiPartialSettlementBillsDeliveredOutputAndRetainsProviderUsage(t *testing.T) {
	for _, source := range []DisconnectSource{DisconnectSourceUpstream, DisconnectSourceClient} {
		t.Run(string(source), func(t *testing.T) {
			logs := &openAIRecordUsageLogRepoStub{}
			billing := &openAIRecordUsageBillingRepoStub{}
			svc := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, nil, nil)
			svc.billingService.pricingService = &PricingService{pricingData: map[string]*LiteLLMModelPricing{
				"gemini-2.5-flash": {InputCostPerToken: 1e-6, OutputCostPerToken: 5e-6},
			}}
			delivered := 5
			result := &ForwardResult{
				RequestID: "gemini-partial-" + string(source),
				Model:     "gemini-2.5-flash", UpstreamModel: "gemini-2.5-flash",
				Stream: true, ClientDisconnect: source == DisconnectSourceClient,
				DeliveredOutputTokens: &delivered,
				Usage:                 ClaudeUsage{InputTokens: 1200, OutputTokens: 12000},
				Duration:              time.Second,
				Outcome: &ResponseOutcome{
					HasText: true, OutputBytes: 18, StreamCompleted: false,
					DisconnectSource: source, UpstreamErrorKind: UpstreamErrorProtocol,
				},
			}
			input := &RecordUsageInput{
				Result:        result,
				APIKey:        &APIKey{ID: 551, Quota: 100},
				User:          &User{ID: 661},
				Account:       &Account{ID: 771, Platform: PlatformGemini, Type: AccountTypeAPIKey},
				APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{},
			}
			customerResult := *result
			customerResult.Usage.OutputTokens = delivered
			customerInput := *input
			customerInput.Result = &customerResult
			wantCost := svc.QuoteUsageCost(context.Background(), &customerInput)
			providerCost := svc.QuoteUsageCost(context.Background(), input)
			require.Positive(t, wantCost)
			require.Greater(t, providerCost, wantCost)

			ctx, cancel := context.WithCancel(context.Background())
			if source == DisconnectSourceClient {
				cancel()
			}
			defer cancel()
			require.NoError(t, svc.RecordUsage(ctx, input))

			require.Equal(t, 1, billing.calls)
			require.NoError(t, billing.lastCtxErr, "settlement survives client cancellation")
			require.Equal(t, delivered, billing.lastCmd.OutputTokens)
			require.Equal(t, 1200, billing.lastCmd.InputTokens)
			require.InDelta(t, wantCost, billing.lastCmd.BalanceCost, 1e-8)
			require.InDelta(t, wantCost, billing.lastCmd.APIKeyQuotaCost, 1e-8)
			require.Less(t, billing.lastCmd.BalanceCost, providerCost)
			require.NotNil(t, logs.lastLog)
			require.Equal(t, delivered, logs.lastLog.OutputTokens)
			require.Equal(t, 12000, logs.lastLog.UpstreamOutputTokens)
			require.False(t, logs.lastLog.Outcome.StreamCompleted)
			require.Equal(t, 12000, result.Usage.OutputTokens, "retain provider evidence unchanged")
		})
	}
}
