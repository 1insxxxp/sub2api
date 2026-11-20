//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForwardInputProtectionSharesBudgetAcrossRounds(t *testing.T) {
	first := &ForwardResult{Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 100, CacheCreationInputTokens: 8, OutputTokens: 4}}
	second := &ForwardResult{Model: first.Model, Usage: ClaudeUsage{CacheReadInputTokens: 70, OutputTokens: 6}}
	raw, err := AggregateForwardContinuationResults([]*ForwardResult{first, second})
	require.NoError(t, err)
	raw.InputTokenProtection = &InputTokenProtection{Limit: 80}
	repo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newGatewayRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
	ledger := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	svc.usageBillingRepo = ledger
	group := &Group{ID: 4, RateMultiplier: 1, Platform: PlatformAnthropic}
	input := &RecordUsageInput{Result: raw, User: &User{ID: 1}, Account: &Account{ID: 3, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_limit": 100.0}},
		APIKey: &APIKey{ID: 2, UserID: 1, GroupID: &group.ID, Group: group, Quota: 100}, APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}}
	full := svc.calculateRecordUsageCost(context.Background(), raw, input.APIKey, first.Model, 1, 1, input.PricingAt)
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	log := repo.lastLog
	require.NotNil(t, log)
	require.Equal(t, 10, log.InputTokens)
	require.Equal(t, 70, log.CacheReadTokens)
	require.Zero(t, log.CacheCreationTokens)
	require.Equal(t, 10, log.OutputTokens)
	require.Equal(t, 10, log.Outcome.ContinuationUsage[0].CustomerInputTokens)
	require.Equal(t, 100, log.Outcome.ContinuationUsage[0].InputTokens)
	require.InDelta(t, full.TotalCost, *log.AccountStatsCost, 1e-12)
	require.InDelta(t, full.TotalCost, ledger.lastCmd.AccountQuotaCost, 1e-8)
	require.Less(t, log.TotalCost, full.TotalCost)
	require.InDelta(t, log.ActualCost, ledger.lastCmd.APIKeyQuotaCost, 1e-8)
	require.InDelta(t, log.ActualCost, ledger.lastCmd.BalanceCost, 1e-8)
	require.Equal(t, 80, raw.InputTokenProtection.Limit)
	require.Equal(t, 100, raw.ContinuationResults[0].Usage.InputTokens)
}

func TestForwardInputProtectionPendingAndReplaySnapshot(t *testing.T) {
	repo := &pendingUsageWriterStub{}
	svc := &GatewayService{usageLogRepo: repo}
	raw := &ForwardResult{Model: "claude-sonnet-4", RequestID: "held", Usage: ClaudeUsage{InputTokens: 999, CacheReadInputTokens: 17, OutputTokens: 5}, InputTokenProtection: &InputTokenProtection{Pending: true}}
	encoded, err := json.Marshal(raw)
	require.NoError(t, err)
	var replay ForwardResult
	require.NoError(t, json.Unmarshal(encoded, &replay))
	require.Equal(t, raw.InputTokenProtection, replay.InputTokenProtection)
	input := &RecordUsageInput{Result: &replay, APIKey: &APIKey{ID: 2, UserID: 1}, User: &User{ID: 1}, Account: &Account{ID: 3}}
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Equal(t, UsageBillingStatusPendingReview, repo.log.BillingStatus)
	require.Equal(t, UsagePendingReasonCustomerInputUnavailable, repo.log.BillingPendingReason)
	require.Zero(t, repo.log.InputTokens)
	require.Zero(t, repo.log.CacheReadTokens)
	require.Equal(t, 999, *repo.log.PendingEvidence.InputTokens)
	require.Equal(t, 17, repo.log.PendingEvidence.CacheReadTokens)
	require.Equal(t, 5, repo.log.OutputTokens)
	require.Zero(t, repo.log.ActualCost)
}

func TestForwardInputProtectionPerRequestIgnoresPending(t *testing.T) {
	const model = "flat-tariff"
	repo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newGatewayRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
	svc.usageBillingRepo = &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	group := &Group{ID: 77, RateMultiplier: 1}
	svc.resolver = newOpenAITextChannelPricingResolverForTest(t, group.ID, model, .07)
	input := &RecordUsageInput{
		Result: &ForwardResult{RequestID: "flat", Model: model, Usage: ClaudeUsage{InputTokens: 100}, InputTokenProtection: &InputTokenProtection{Pending: true}},
		APIKey: &APIKey{ID: 1, GroupID: &group.ID, Group: group}, User: &User{ID: 2}, Account: &Account{ID: 3},
	}
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Equal(t, 100, repo.lastLog.InputTokens)
	require.Equal(t, string(BillingModePerRequest), *repo.lastLog.BillingMode)
	require.InDelta(t, .07, repo.lastLog.ActualCost, 1e-8)
}

func TestForwardInputProtectionUsesClientLimitOnceAcrossGeminiRounds(t *testing.T) {
	first, second := geminiInputCountingBillingResult(), geminiInputCountingBillingResult()
	for _, round := range []*ForwardResult{first, second} {
		round.Model, round.UpstreamModel = "gemini-2.5-pro", ""
	}
	raw, err := AggregateForwardContinuationResults([]*ForwardResult{first, second})
	require.NoError(t, err)
	raw.InputTokenProtection = &InputTokenProtection{Limit: 900}
	svc, repo := newGeminiCacheBillingGatewayForTest(t)
	svc.usageBillingRepo = &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	group := &Group{ID: 104, Platform: PlatformGemini, RateMultiplier: 1}
	input := &RecordUsageInput{Result: raw, APIKey: &APIKey{ID: 1, Group: group, GroupID: &group.ID}, User: &User{ID: 2}, Account: &Account{ID: 103}}
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Equal(t, 900, repo.lastLog.InputTokens)
	require.Equal(t, 673, repo.lastLog.Outcome.ContinuationUsage[0].CustomerInputTokens)
	require.Equal(t, 227, repo.lastLog.Outcome.ContinuationUsage[1].CustomerInputTokens)
	require.Equal(t, "group_input_token_protection", repo.lastLog.Outcome.InputTokenPolicy)
}

func TestForwardInputProtectionVolumeDiscountAndCacheWriteTTL(t *testing.T) {
	result := &ForwardResult{Usage: ClaudeUsage{InputTokens: 20, CacheReadInputTokens: 10, CacheCreationInputTokens: 80, CacheCreation1hTokens: 60}}
	price := func(r *ForwardResult) (*CostBreakdown, error) {
		// Dropping below 100 tokens switches the whole request to the dearer tier.
		n := r.Usage.InputTokens + r.Usage.CacheReadInputTokens + r.Usage.CacheCreationInputTokens
		rate := 1.0
		if n < 100 {
			rate = 3
		}
		amount := float64(n) * rate
		return &CostBreakdown{ActualCost: amount, TotalCost: amount}, nil
	}
	baseline, err := price(result)
	require.NoError(t, err)
	customer, cost, pending, err := protectForwardBilling(result, result, &InputTokenProtection{Limit: 70}, baseline, price)
	require.NoError(t, err)
	require.False(t, pending)
	require.Equal(t, 10, customer.Usage.CacheReadInputTokens)
	require.Equal(t, 20, customer.Usage.InputTokens)
	require.Equal(t, 40, customer.Usage.CacheCreationInputTokens)
	require.Equal(t, 20, customer.Usage.CacheCreation5mTokens)
	require.Equal(t, 20, customer.Usage.CacheCreation1hTokens)
	require.Equal(t, baseline.ActualCost, cost.ActualCost)
	require.Equal(t, 80, result.Usage.CacheCreationInputTokens)
}

func TestForwardInputProtectionForcedReadsKeepProviderCost(t *testing.T) {
	repo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newGatewayRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
	ledger := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	svc.usageBillingRepo = ledger
	group := &Group{ID: 4, RateMultiplier: 1}
	input := &RecordUsageInput{
		Result:            &ForwardResult{RequestID: "forced", Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 500, OutputTokens: 20}, InputTokenProtection: &InputTokenProtection{Limit: 100}},
		ForceCacheBilling: true,
		APIKey:            &APIKey{ID: 2, GroupID: &group.ID, Group: group}, User: &User{ID: 1},
		Account: &Account{ID: 3, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_limit": 100.0}},
	}
	raw := svc.calculateRecordUsageCost(context.Background(), input.Result, input.APIKey, input.Result.Model, 1, 1, input.PricingAt)
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Zero(t, repo.lastLog.InputTokens)
	require.Equal(t, 100, repo.lastLog.CacheReadTokens)
	require.Equal(t, 500, input.Result.Usage.InputTokens)
	require.InDelta(t, raw.TotalCost, *repo.lastLog.AccountStatsCost, 1e-12)
	require.InDelta(t, raw.TotalCost, ledger.lastCmd.AccountQuotaCost, 1e-8)
}

func TestForwardInputProtectionCacheCreationQuoteAndStorageExtension(t *testing.T) {
	svc, logs := newGeminiCacheBillingGatewayForTest(t)
	group := &Group{ID: 33, RateMultiplier: 1, Platform: PlatformGemini}
	input := &RecordUsageInput{
		Result: &ForwardResult{Model: "gemini-2.5-pro", RequestID: "create", Usage: ClaudeUsage{InputTokens: 1000}, CacheStorageTokenHours: 5000},
		APIKey: &APIKey{ID: 2, GroupID: &group.ID, Group: group}, User: &User{ID: 1}, Account: &Account{ID: 3},
	}
	ctx := WithInputTokenProtection(context.Background(), &InputTokenProtection{Limit: 100})
	want := 100*0.00000125 + 5000*0.0000045
	require.InDelta(t, want, svc.QuoteUsageCost(ctx, input), 1e-12)
	require.NoError(t, svc.RecordUsage(ctx, input))
	require.InDelta(t, want, logs.lastLog.ActualCost, 1e-8)
	require.Equal(t, 100, logs.lastLog.InputTokens)
	input.Result = &ForwardResult{Model: "gemini-2.5-pro", RequestID: "extend", CacheStorageTokenHours: 5000, InputTokenProtection: &InputTokenProtection{Pending: true}}
	require.InDelta(t, 5000*0.0000045, svc.QuoteUsageCost(ctx, input), 1e-12)
	require.NoError(t, svc.RecordUsage(ctx, input))
	require.InDelta(t, 5000*0.0000045, logs.lastLog.ActualCost, 1e-8)
}

func TestForwardInputProtectionMissingProviderInputHoldsCharge(t *testing.T) {
	svc := newGatewayRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
	repo := &pendingUsageWriterStub{}
	svc.usageLogRepo = repo
	input := &RecordUsageInput{
		Result: &ForwardResult{Model: "claude-sonnet-4", EstimatedInputTokens: 500, Usage: ClaudeUsage{OutputTokens: 100}, OutputTokensSource: UsageTokenSourceUpstream,
			InputTokenProtection: &InputTokenProtection{Limit: 200}},
		APIKey: &APIKey{ID: 2, UserID: 1}, User: &User{ID: 1}, Account: &Account{ID: 3},
	}
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Equal(t, UsageBillingStatusPendingReview, repo.log.BillingStatus)
	require.Equal(t, 100, repo.log.OutputTokens)
	require.Zero(t, repo.log.ActualCost)
}

func TestProtectedOpenAIPendingKeepsRawPriceAndDeliveredOutput(t *testing.T) {
	ledger := &openAIRecordUsageBillingRepoStub{}
	repo := &pendingUsageWriterStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(repo, ledger, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.usageLogRepo = repo
	delivered := 3
	r := &OpenAIForwardResult{Model: "gpt-5.1", RequestID: "held-openai", InputTokenProtection: &InputTokenProtection{Pending: true},
		Usage:             OpenAIUsage{InputTokens: 1000, CacheReadInputTokens: 500, CacheCreationInputTokens: 50, OutputTokens: 10},
		InputTokensSource: UsageTokenSourceUpstream, OutputTokensSource: UsageTokenSourceUpstream,
		Stream: true, ClientDisconnect: true, DeliveredOutputTokens: &delivered}
	require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: r, User: &User{ID: 1}, APIKey: &APIKey{ID: 2, UserID: 1}, Account: &Account{ID: 3}}))
	log := repo.log
	require.Equal(t, UsagePendingReasonCustomerInputUnavailable, log.BillingPendingReason)
	require.Zero(t, log.InputTokens)
	require.Zero(t, log.CacheReadTokens)
	require.Zero(t, log.CacheCreationTokens)
	require.Equal(t, delivered, log.OutputTokens)
	require.Equal(t, UsageTokenSourceEstimated, log.OutputTokensSource)
	require.Equal(t, 1000, *log.PendingEvidence.InputTokens)
	require.True(t, log.PendingEvidence.InputIncludesCache)
	require.Equal(t, 500, log.PendingEvidence.CacheReadTokens)
	require.Equal(t, 50, log.PendingEvidence.CacheCreationTokens)
	require.Equal(t, 10, *log.PendingEvidence.OutputTokens)
	want := expectedOpenAICost(t, svc, r.Model, r.Usage, 1)
	require.InDelta(t, want.TotalCost, *log.PendingEvidence.ProviderTotalCost, 1e-12)
	require.Zero(t, log.ActualCost)
	require.Zero(t, ledger.calls)
}

func TestPendingProviderEvidenceDistinguishesMissingAndMeasuredZero(t *testing.T) {
	for _, tc := range []struct {
		source UsageTokenSource
		value  int
		known  bool
	}{
		{UsageTokenSourceUpstream, 0, true}, {UsageTokenSourceUnknown, 0, false},
		{UsageTokenSourceUnreported, 99, false}, {UsageTokenSourceEstimated, 101, false},
	} {
		raw := openAIPendingProviderUsage(&OpenAIForwardResult{InputTokensSource: tc.source, Usage: OpenAIUsage{InputTokens: tc.value, CacheReadInputTokens: 19}})
		require.Equal(t, tc.known, raw.InputTokens != nil)
		require.Equal(t, 19, raw.CacheReadTokens)
	}
}
