//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

const continuationBillingModel = "claude-sonnet-4"

func continuationTestBilling() *BillingService {
	billing := NewBillingService(&config.Config{}, nil)
	billing.fallbackPrices[continuationBillingModel] = &ModelPricing{
		InputPricePerToken: 1e-6, OutputPricePerToken: 10e-6,
		CacheReadPricePerToken: 0.1e-6, SupportsCacheBreakdown: true,
		LongContextInputThreshold: 200000, LongContextInputMultiplier: 2, LongContextOutputMultiplier: 2,
	}
	return billing
}

func TestContinuationRoundGatewayPricesEachContextIndependently(t *testing.T) {
	svc := &GatewayService{billingService: continuationTestBilling()}
	first := &ForwardResult{RequestID: "first", Model: continuationBillingModel, Usage: ClaudeUsage{InputTokens: 120000, OutputTokens: 1000}}
	second := &ForwardResult{RequestID: "second", Model: continuationBillingModel, Usage: ClaudeUsage{InputTokens: 120000, OutputTokens: 1000}}
	result := &ForwardResult{Model: continuationBillingModel, Usage: ClaudeUsage{InputTokens: 240000, OutputTokens: 2000}, ContinuationResults: []*ForwardResult{first, second}}
	cost, err := svc.calculateRecordUsageCostChecked(context.Background(), result, &APIKey{}, continuationBillingModel, 1, 1, time.Now(), nil)
	require.NoError(t, err)
	require.InDelta(t, 0.26, cost.TotalCost, 1e-10, "two 120k contexts must not be priced as one long 240k context")
	require.False(t, cost.LongContextBillingApplied)
}

func TestContinuationRoundOpenAIPricesEachContextAndDeliveredOutput(t *testing.T) {
	svc := &OpenAIGatewayService{billingService: continuationTestBilling()}
	delivered := 100
	first := &OpenAIForwardResult{RequestID: "first", Model: continuationBillingModel, Usage: OpenAIUsage{InputTokens: 120000, OutputTokens: 1000}}
	second := &OpenAIForwardResult{RequestID: "second", Model: continuationBillingModel, Usage: OpenAIUsage{InputTokens: 120000, OutputTokens: 1000},
		ClientDisconnect: true, DeliveredOutputTokens: &delivered, Outcome: &ResponseOutcome{DisconnectSource: DisconnectSourceClient}}
	result := &OpenAIForwardResult{Model: continuationBillingModel, Usage: OpenAIUsage{InputTokens: 240000, OutputTokens: 2000}, ContinuationResults: []*OpenAIForwardResult{first, second}}
	cost, err := svc.calculateOpenAIRecordUsageCost(context.Background(), result, &APIKey{}, []string{continuationBillingModel}, 1, 1, 1, 1,
		UsageTokens{InputTokens: 240000, OutputTokens: 2000}, "", nil, false, time.Now())
	require.NoError(t, err)
	require.InDelta(t, 0.251, cost.TotalCost, 1e-10)
	require.False(t, cost.LongContextBillingApplied)
}

func TestContinuationRoundGeminiRecoveryUsesEveryObservedLedgerID(t *testing.T) {
	input, upstream := newGeminiUsageRecoveryFixture()
	first := *input.Result
	second := first
	second.UsageLedgerRequestID = "provider-request-2"
	ledger := `{"success":true,"data":[{"type":2,"request_id":"provider-request-1","model_name":"gemini-2.5-flash","prompt_tokens":120000,"completion_tokens":1000,"other":"{\"cache_tokens\":0,\"cache_input_includes_cache\":true}"},{"type":2,"request_id":"provider-request-2","model_name":"gemini-2.5-flash","prompt_tokens":130000,"completion_tokens":2000,"other":"{\"cache_tokens\":0,\"cache_input_includes_cache\":true}"}]}`
	upstream.responses = append(upstream.responses, geminiUsageRecoveryResponse(200, ledger), geminiUsageRecoveryResponse(200, ledger))
	input.Result = &ForwardResult{Model: first.Model, ContinuationResults: []*ForwardResult{&first, &second}}
	require.True(t, NeedsUsageMeasurementReview(input.Result))
	recovered, err := (&GatewayService{httpUpstream: upstream, cfg: &config.Config{}}).RecoverGeminiUsage(context.Background(), input)
	require.NoError(t, err)
	require.True(t, recovered)
	require.Equal(t, 120000, first.Usage.InputTokens)
	require.Equal(t, 130000, second.Usage.InputTokens)
	require.Equal(t, 250000, input.Result.Usage.InputTokens)
	require.Equal(t, 3000, input.Result.Usage.OutputTokens)
	require.False(t, NeedsUsageMeasurementReview(input.Result))
}

func TestContinuationRoundRejectsNestedOrRepeatedRoundPointers(t *testing.T) {
	leaf := &ForwardResult{Model: continuationBillingModel, Usage: ClaudeUsage{InputTokens: 120000, OutputTokens: 1}}
	svc := &GatewayService{billingService: continuationTestBilling()}
	for _, rounds := range [][]*ForwardResult{{leaf, leaf}, {{ContinuationResults: []*ForwardResult{leaf}}}, {nil, leaf}} {
		_, err := svc.calculateRecordUsageCostChecked(context.Background(), &ForwardResult{Model: continuationBillingModel, ContinuationResults: rounds}, &APIKey{}, continuationBillingModel, 1, 1, time.Now(), nil)
		require.Error(t, err, "malformed round evidence must not silently bill partial or repeated usage")
	}
}

func TestContinuationRoundSettlementUsesOneDurableSnapshot(t *testing.T) {
	for _, replay := range []bool{false, true} {
		logs := &openAIRecordUsageBestEffortLogRepoStub{}
		billing := &durableUsageBillingStub{}
		if replay {
			billing.result = &UsageBillingApplyResult{Applied: false}
		}
		svc := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, nil, nil)
		svc.billingService = continuationTestBilling()
		first := &ForwardResult{RequestID: "first", Model: continuationBillingModel, Usage: ClaudeUsage{InputTokens: 120000, OutputTokens: 1000}}
		second := &ForwardResult{RequestID: "second", Model: continuationBillingModel, Usage: ClaudeUsage{InputTokens: 120000, OutputTokens: 1000}}
		result, err := AggregateForwardContinuationResults([]*ForwardResult{first, second})
		require.NoError(t, err)
		err = svc.RecordUsage(context.Background(), &RecordUsageInput{Result: result, APIKey: &APIKey{ID: 1}, User: &User{ID: 2}, Account: &Account{ID: 3, Type: AccountTypeAPIKey}})
		require.NoError(t, err)
		require.Equal(t, 1, billing.calls)
		require.Zero(t, logs.bestEffortCalls)
		require.Equal(t, 240000, billing.lastCmd.UsageLog.InputTokens)
		require.InDelta(t, .26, billing.lastCmd.UsageLog.TotalCost, 1e-10)
		require.False(t, billing.lastCmd.UsageLog.LongContextBillingApplied)
	}
}

func TestContinuationRoundCustomerProjectionIsIndependentAndKeepsRaw(t *testing.T) {
	first, second := geminiInputCountingBillingResult(), geminiInputCountingBillingResult()
	second.RequestID = "second"
	second.GeminiInputCounting.CandidateInputTokens = 100
	second.Usage.InputTokens = 1000
	result, err := AggregateForwardContinuationResults([]*ForwardResult{first, second})
	require.NoError(t, err)
	projected := customerBillingForwardResultForAccount(result, &Account{ID: 103})
	require.Equal(t, 521, projected.Usage.InputTokens)
	require.Equal(t, 1673, result.Usage.InputTokens)
	require.Equal(t, 673, first.Usage.InputTokens)
	wrongAccount := customerBillingForwardResultForAccount(result, &Account{ID: 104})
	require.Equal(t, 1673, wrongAccount.Usage.InputTokens)
	first.Usage.InputTokens = 999
	first.GeminiInputCounting.CandidateInputTokens = 999
	require.Equal(t, 673, result.ContinuationResults[0].Usage.InputTokens)
	require.Equal(t, 421, result.ContinuationResults[0].GeminiInputCounting.CandidateInputTokens)
}

func TestContinuationRoundAccountCostUsesIndependentRawContexts(t *testing.T) {
	logs := &openAIRecordUsageBestEffortLogRepoStub{}
	billing := &durableUsageBillingStub{}
	svc := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, nil, nil)
	svc.billingService = continuationTestBilling()
	groupID := int64(10)
	price, outputPrice := .000001, .00001
	boundary, longPrice := 200000, .000002
	group := &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 1, LongContextPricingEnabled: true, ModelPricing: []ChannelModelPricing{{Models: []string{continuationBillingModel}, BillingMode: BillingModeToken, InputPrice: &price, OutputPrice: &outputPrice, Intervals: []PricingInterval{{MinTokens: 0, MaxTokens: &boundary, InputPrice: &price, OutputPrice: &outputPrice}, {MinTokens: boundary, InputPrice: &longPrice, OutputPrice: &outputPrice}}}}}
	svc.channelService = newTestChannelServiceForStats(t, &Channel{ID: 1, Status: StatusActive, ApplyPricingToAccountStats: true, ModelPricing: group.ModelPricing}, groupID, PlatformAnthropic)
	svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)
	first := &ForwardResult{RequestID: "first", Model: continuationBillingModel, Usage: ClaudeUsage{InputTokens: 120000, OutputTokens: 1000}}
	second := &ForwardResult{RequestID: "second", Model: continuationBillingModel, Usage: ClaudeUsage{InputTokens: 120000, OutputTokens: 1000}}
	result, err := AggregateForwardContinuationResults([]*ForwardResult{first, second})
	require.NoError(t, err)
	err = svc.RecordUsage(context.Background(), &RecordUsageInput{Result: result, APIKey: &APIKey{ID: 1, GroupID: &groupID, Group: group}, User: &User{ID: 2}, Account: &Account{ID: 3, Type: AccountTypeAPIKey}})
	require.NoError(t, err)
	require.NotNil(t, billing.lastCmd.UsageLog.AccountStatsCost)
	require.InDelta(t, .26, *billing.lastCmd.UsageLog.AccountStatsCost, 1e-10)
}

func TestContinuationRoundAccountStatsCustomTierIsPerRound(t *testing.T) {
	logs := &openAIRecordUsageBestEffortLogRepoStub{}
	billing := &durableUsageBillingStub{}
	svc := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, nil, nil)
	svc.billingService = continuationTestBilling()
	groupID := int64(10)
	price, outputPrice := .000001, .00001
	boundary, longPrice := 200000, .000002
	pricing := []ChannelModelPricing{{Models: []string{continuationBillingModel}, BillingMode: BillingModeToken, InputPrice: &price, OutputPrice: &outputPrice, Intervals: []PricingInterval{{MinTokens: 0, MaxTokens: &boundary, InputPrice: &price, OutputPrice: &outputPrice}, {MinTokens: boundary, InputPrice: &longPrice, OutputPrice: &outputPrice}}}}
	group := &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 1}
	svc.channelService = newTestChannelServiceForStats(t, &Channel{ID: 1, Status: StatusActive, AccountStatsPricingRules: []AccountStatsPricingRule{{AccountIDs: []int64{3}, Pricing: pricing}}}, groupID, PlatformAnthropic)
	first := &ForwardResult{RequestID: "first", Model: continuationBillingModel, Usage: ClaudeUsage{InputTokens: 120000, OutputTokens: 1000}}
	second := &ForwardResult{RequestID: "second", Model: continuationBillingModel, Usage: ClaudeUsage{InputTokens: 120000, OutputTokens: 1000}}
	result, err := AggregateForwardContinuationResults([]*ForwardResult{first, second})
	require.NoError(t, err)
	err = svc.RecordUsage(context.Background(), &RecordUsageInput{Result: result, APIKey: &APIKey{ID: 1, GroupID: &groupID, Group: group}, User: &User{ID: 2}, Account: &Account{ID: 3, Type: AccountTypeAPIKey}})
	require.NoError(t, err)
	require.NotNil(t, billing.lastCmd.UsageLog.AccountStatsCost)
	require.InDelta(t, .26, *billing.lastCmd.UsageLog.AccountStatsCost, 1e-10)
}

func TestContinuationRoundMissingInputFallbackAppliesIndependently(t *testing.T) {
	first := &ForwardResult{RequestID: "first", Usage: ClaudeUsage{InputTokens: 10, OutputTokens: 1}}
	second := &ForwardResult{RequestID: "second", Usage: ClaudeUsage{OutputTokens: 1}, EstimatedInputTokens: 21}
	result, err := AggregateForwardContinuationResults([]*ForwardResult{first, second})
	require.NoError(t, err)
	applyMissingInputTokenEstimate(context.Background(), result, &Account{Platform: PlatformAnthropic})
	require.Equal(t, 31, result.Usage.InputTokens)
	require.Equal(t, 21, result.ContinuationResults[1].Usage.InputTokens)
	require.True(t, result.ContinuationResults[1].InputTokensEstimated)
}

func TestContinuationRoundProjectionPersistsEachPolicy(t *testing.T) {
	logs := &openAIRecordUsageBestEffortLogRepoStub{}
	billing := &durableUsageBillingStub{}
	svc := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, nil, nil)
	svc.billingService = continuationTestBilling()
	first, second := geminiInputCountingBillingResult(), geminiInputCountingBillingResult()
	first.Model, second.Model = continuationBillingModel, continuationBillingModel
	first.UpstreamModel, second.UpstreamModel = "", ""
	second.RequestID = "second"
	result, err := AggregateForwardContinuationResults([]*ForwardResult{first, second})
	require.NoError(t, err)
	require.NotPanics(t, func() {
		err = svc.RecordUsage(context.Background(), &RecordUsageInput{Result: result, APIKey: &APIKey{ID: 1}, User: &User{ID: 2}, Account: &Account{ID: 103, Type: AccountTypeAPIKey}})
	})
	require.NoError(t, err)
	require.Equal(t, 842, billing.lastCmd.UsageLog.InputTokens)
}

func TestContinuationRoundFailedSecondCallDoesNotChargeCustomer(t *testing.T) {
	first := &ForwardResult{RequestID: "first", Model: continuationBillingModel, Usage: ClaudeUsage{InputTokens: 120000, OutputTokens: 1000}, Outcome: &ResponseOutcome{HasText: true, StreamCompleted: true, CollectorVersion: 1}}
	second := &ForwardResult{RequestID: "second", Model: continuationBillingModel, Usage: ClaudeUsage{InputTokens: 120000}, Outcome: &ResponseOutcome{HTTPStatus: 503, UpstreamStatus: 503, CollectorVersion: 1}}
	result, err := AggregateForwardContinuationResults([]*ForwardResult{first, second})
	require.NoError(t, err)
	customer := customerBillingForwardResult(result)
	svc := &GatewayService{billingService: continuationTestBilling()}
	cost, err := svc.calculateRecordUsageCostChecked(context.Background(), customer, &APIKey{}, continuationBillingModel, 1, 1, time.Now(), nil)
	require.NoError(t, err)
	require.InDelta(t, .13, cost.TotalCost, 1e-10)
	require.Equal(t, 120000, customer.Usage.InputTokens)
	require.Equal(t, 240000, result.Usage.InputTokens)
}

func TestContinuationRoundServiceTierDowngradesIndependently(t *testing.T) {
	fast := "priority"
	first := &OpenAIForwardResult{RequestID: "first", Model: "gpt-5", ServiceTier: &fast, UpstreamResponseServiceTier: "default"}
	second := &OpenAIForwardResult{RequestID: "second", Model: "gpt-5", ServiceTier: &fast, UpstreamResponseServiceTier: "priority"}
	result, err := AggregateOpenAIContinuationResults([]*OpenAIForwardResult{first, second})
	require.NoError(t, err)
	ApplyOpenAIServiceTierBillingResolution(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, result)
	require.Equal(t, "default", *result.ContinuationResults[0].ServiceTier)
	require.Equal(t, "priority", *result.ContinuationResults[1].ServiceTier)
	require.Equal(t, "priority", *first.ServiceTier, "caller-owned snapshots remain immutable")
	require.Equal(t, "second", result.RequestID)
	require.Equal(t, "first", result.ContinuationPublicResponseID)
}

func TestContinuationRoundBufferedFailureDoesNotChargeAnyUndeliveredOutput(t *testing.T) {
	first := &ForwardResult{RequestID: "first", Model: continuationBillingModel, Usage: ClaudeUsage{InputTokens: 120000, OutputTokens: 1000}, RequireMeasuredUsage: true, MeasuredUsageComplete: true, Outcome: &ResponseOutcome{HTTPStatus: 502, UpstreamStatus: 200, CollectorVersion: 1}}
	zero := 0
	first.DeliveredOutputTokens = &zero
	second := cloneForwardContinuationLeaf(first)
	second.RequestID = "second"
	result, err := AggregateForwardContinuationResults([]*ForwardResult{first, second})
	require.NoError(t, err)
	customer := customerBillingForwardResult(result)
	require.Zero(t, customer.Usage.InputTokens)
	require.Zero(t, recordedOutputTokens(result))
	cost, err := (&GatewayService{billingService: continuationTestBilling()}).calculateRecordUsageCostChecked(context.Background(), customer, &APIKey{}, continuationBillingModel, 1, 1, time.Now(), nil)
	require.NoError(t, err)
	require.Zero(t, cost.ActualCost)
	require.Equal(t, 240000, result.Usage.InputTokens)
	require.Equal(t, 2000, result.Usage.OutputTokens)
}

func TestContinuationRoundInterruptedGeminiBillsDeliveredTextOnly(t *testing.T) {
	delivered := 10
	first := &ForwardResult{RequestID: "first", Model: continuationBillingModel, Usage: ClaudeUsage{InputTokens: 120000, OutputTokens: 1000}, RequireMeasuredUsage: true, MeasuredUsageComplete: true, ClientDisconnect: true, DeliveredOutputTokens: &delivered, Outcome: &ResponseOutcome{HasText: true, DisconnectSource: DisconnectSourceClient, CollectorVersion: 1}}
	result, err := AggregateForwardContinuationResults([]*ForwardResult{first})
	require.NoError(t, err)
	customer := customerBillingForwardResult(result)
	require.Equal(t, 10, customer.Usage.OutputTokens)
	require.Equal(t, 10, recordedOutputTokens(result))
	require.Equal(t, 1000, result.Usage.OutputTokens)
}

func TestContinuationRoundAggregateDeliveryIncludesEarlierRounds(t *testing.T) {
	firstCount, secondCount := 100, 0
	first := &ForwardResult{Usage: ClaudeUsage{OutputTokens: 100}, DeliveredOutputTokens: &firstCount}
	second := &ForwardResult{DeliveredOutputTokens: &secondCount}
	result, err := AggregateForwardContinuationResults([]*ForwardResult{first, second})
	require.NoError(t, err)
	require.NotNil(t, result.DeliveredOutputTokens)
	require.Equal(t, 100, *result.DeliveredOutputTokens)
	openAIResult, err := AggregateOpenAIContinuationResults([]*OpenAIForwardResult{{DeliveredOutputTokens: &firstCount}, {DeliveredOutputTokens: &secondCount}})
	require.NoError(t, err)
	require.NotNil(t, openAIResult.DeliveredOutputTokens)
	require.Equal(t, 100, *openAIResult.DeliveredOutputTokens)
	firstCount = 999
	require.Equal(t, 100, *result.DeliveredOutputTokens)
}

func TestContinuationRoundOpenAIRecordUsageKeepsMixedTierPrices(t *testing.T) {
	for _, freeFast := range []bool{false, true} {
		logs := &openAIRecordUsageLogRepoStub{inserted: true}
		svc := newOpenAIRecordUsageServiceForTest(logs, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
		svc.billingService = continuationTestBilling()
		svc.resolver = NewModelPricingResolver(nil, svc.billingService)
		svc.cfg.Default.RateMultiplier = 1
		fast := "priority"
		first := &OpenAIForwardResult{RequestID: "first", Model: continuationBillingModel, Usage: OpenAIUsage{InputTokens: 120000, OutputTokens: 1000}, ServiceTier: &fast, UpstreamResponseServiceTier: "default"}
		second := &OpenAIForwardResult{RequestID: "second", Model: continuationBillingModel, Usage: OpenAIUsage{InputTokens: 120000, OutputTokens: 1000}, ServiceTier: &fast}
		result, err := AggregateOpenAIContinuationResults([]*OpenAIForwardResult{first, second})
		require.NoError(t, err)
		err = svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: result, APIKey: &APIKey{ID: 1, Group: &Group{Platform: PlatformOpenAI, FreeOpenAIFast: freeFast}}, User: &User{ID: 2}, Account: &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}})
		require.NoError(t, err)
		require.NotNil(t, logs.lastLog)
		require.InDelta(t, .39, logs.lastLog.TotalCost, 1e-10)
		actual := .39
		if freeFast {
			actual = .26
		}
		require.InDelta(t, actual, logs.lastLog.ActualCost, 1e-10)
		require.Len(t, logs.lastLog.Outcome.ContinuationUsage, 2)
		require.Equal(t, "default", logs.lastLog.Outcome.ContinuationUsage[0].ServiceTier)
		require.Equal(t, "priority", logs.lastLog.Outcome.ContinuationUsage[1].ServiceTier)
	}
}

func TestContinuationRoundOpenAIChatFinalizerPreservesDelivery(t *testing.T) {
	delivered := 10
	leaf := &OpenAIForwardResult{RequestID: "partial", Model: continuationBillingModel, Stream: true, ClientDisconnect: true, DeliveredOutputTokens: &delivered,
		Outcome:   &ResponseOutcome{HTTPStatus: 200, HasText: true, DisconnectSource: DisconnectSourceClient, CollectorVersion: 1},
		ChatUsage: &OpenAIChatUsageEvidence{Rounds: []*OpenAIChatUsageRound{{Usage: OpenAIUsage{InputTokens: 120000, OutputTokens: 1000}, InputReported: true, OutputReported: true, CacheReadReported: true, CacheWriteReported: true, CompletionTokens: 1000, UpstreamAccepted: true}}}}
	result, err := AggregateOpenAIContinuationResults([]*OpenAIForwardResult{leaf})
	require.NoError(t, err)
	(&OpenAIGatewayService{}).FinalizeOpenAIChatUsage(context.Background(), &OpenAIRecordUsageInput{Result: result})
	require.NotNil(t, result.ContinuationResults[0].DeliveredOutputTokens)
	require.Equal(t, 10, recordedOpenAIOutputTokens(result))
	require.Equal(t, 1000, result.Usage.OutputTokens)
}

func TestContinuationRoundOpenAIFailedAuditUsesCustomerZero(t *testing.T) {
	zero := 0
	leaf := &OpenAIForwardResult{RequestID: "buffered", Model: continuationBillingModel, Usage: OpenAIUsage{InputTokens: 120000, OutputTokens: 1000}, DeliveredOutputTokens: &zero,
		Outcome: &ResponseOutcome{HTTPStatus: 502, UpstreamStatus: 200, CollectorVersion: 1}}
	result, err := AggregateOpenAIContinuationResults([]*OpenAIForwardResult{leaf})
	require.NoError(t, err)
	audit := openAIContinuationAudit(result)
	require.Len(t, audit, 1)
	require.Zero(t, audit[0].CustomerInputTokens)
	require.Zero(t, audit[0].CustomerOutputTokens)
	require.Equal(t, 120000, audit[0].InputTokens)
	require.Equal(t, 1000, audit[0].OutputTokens)
}

func TestContinuationRoundGatewayQuotaKeepsRawOutputCost(t *testing.T) {
	logs := &openAIRecordUsageBestEffortLogRepoStub{}
	billing := &durableUsageBillingStub{}
	svc := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, nil, nil)
	svc.billingService = continuationTestBilling()
	svc.cfg.Default.RateMultiplier = 1
	delivered := 10
	result, err := AggregateForwardContinuationResults([]*ForwardResult{{RequestID: "partial", Model: continuationBillingModel, Usage: ClaudeUsage{InputTokens: 120000, OutputTokens: 1000}, RequireMeasuredUsage: true, MeasuredUsageComplete: true, ClientDisconnect: true, DeliveredOutputTokens: &delivered, Outcome: &ResponseOutcome{HTTPStatus: 200, HasText: true, DisconnectSource: DisconnectSourceClient, CollectorVersion: 1}}})
	require.NoError(t, err)
	err = svc.RecordUsage(context.Background(), &RecordUsageInput{Result: result, APIKey: &APIKey{ID: 1}, User: &User{ID: 2}, Account: &Account{ID: 3, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_daily_limit": 100.0}}})
	require.NoError(t, err)
	require.InDelta(t, .1201, billing.lastCmd.BalanceCost, 1e-10)
	require.InDelta(t, .13, billing.lastCmd.AccountQuotaCost, 1e-10)
}

func TestContinuationRoundOpenAIQuotaKeepsRawOutputCost(t *testing.T) {
	logs := &openAIRecordUsageBestEffortLogRepoStub{}
	billing := &durableUsageBillingStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.billingService = continuationTestBilling()
	svc.resolver = NewModelPricingResolver(nil, svc.billingService)
	svc.cfg.Default.RateMultiplier = 1
	delivered := 10
	result, err := AggregateOpenAIContinuationResults([]*OpenAIForwardResult{{RequestID: "partial", Model: continuationBillingModel, Usage: OpenAIUsage{InputTokens: 120000, OutputTokens: 1000}, ClientDisconnect: true, DeliveredOutputTokens: &delivered, Outcome: &ResponseOutcome{HTTPStatus: 200, HasText: true, DisconnectSource: DisconnectSourceClient, CollectorVersion: 1}}})
	require.NoError(t, err)
	err = svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: result, APIKey: &APIKey{ID: 1}, User: &User{ID: 2}, Account: &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_daily_limit": 100.0}}})
	require.NoError(t, err)
	require.InDelta(t, .1201, billing.lastCmd.BalanceCost, 1e-10)
	require.InDelta(t, .13, billing.lastCmd.AccountQuotaCost, 1e-10)
}
