package service

import (
	"context"
	"math"
)

// InputTokenProtectionBillsForResult quotes one unprojected provider response. In-progress WS counters
// use the possible tariffs until the terminal frame supplies a final quote.
// Model adoption compares pre-multiplier totals, so user multipliers are immaterial.
func (s *OpenAIGatewayService) InputTokenProtectionBillsForResult(ctx context.Context, input *OpenAIRecordUsageInput) bool {
	if s == nil || input == nil || input.Result == nil || input.APIKey == nil || input.APIKey.Group == nil {
		return false
	}
	key := input.APIKey
	r := *input.Result
	possibleTariff := func() bool {
		return s.InputTokenProtectionProjectsUsage(ctx, key, r.Model, InputTokenProtectionAttempt{UpstreamModel: r.UpstreamModel, ResponseModel: r.UpstreamResponseModel, ResponseConflict: r.UpstreamResponseModelConflict}, r.ImageEndpoint)
	}
	if s.billingService == nil || (r.OpenAIWSMode && r.UpstreamTerminalEvent == "") {
		return possibleTariff()
	}
	billingAccount, err := resolveCredentialAccount(ctx, s.accountRepo, input.Account)
	if err != nil {
		return possibleTariff()
	}
	if !isGrokVideoUsageResult(&r, nil) {
		ApplyOpenAIImageBillingResolution(&r)
	}
	ApplyOpenAIServiceTierBillingResolution(billingAccount, &r)
	quoteInput := *input
	quoteInput.Result = &r
	models := s.openAIRecordUsageBillingModels(ctx, &quoteInput)
	counts := r.Usage.billingUsageTokens()
	if uncached, ok := openAIChatUncachedInputTokens(&r); ok {
		counts.InputTokens = uncached
	}
	counts.OutputTokens = recordedOpenAIOutputTokens(&r)
	cost, _, err := s.quoteOpenAIResponseModel(ctx, &quoteInput, &r, models, func(candidates []string) (*CostBreakdown, error) {
		return s.calculateOpenAIRecordUsageCost(ctx, &r, key, candidates, 1, 1, 1, 1, counts, optionalStringValue(r.ServiceTier), openAILongContextBillingGate(billingAccount), openAISubscriptionLongContextExempt(billingAccount, &r), openAIUsagePricingAt(input))
	}, false)
	if err != nil {
		return possibleTariff()
	}
	return openAITokenPricedForProtection(&r, cost)
}

// The four exclusive billing buckets use the upstream's native TTL pair when
// present. Untyped writes use the short-write bucket for allocating the cap.
type openAIProtectedInputBuckets struct{ read, input, short, long int }

func openAITokenPricedForProtection(result *OpenAIForwardResult, cost *CostBreakdown) bool {
	if cost.BillingMode != "" {
		return cost.BillingMode == string(BillingModeToken)
	}
	// Legacy token quotes may omit their mode. WebSearchCalls identifies the
	// standalone alpha/search operation. SearchCount is additive to generation.
	return result.AudioUsage == nil && result.WebSearchCalls == 0 && result.ImageCount == 0 && result.VideoCount == 0
}

// Explicit upstream zero is measured. Accepted raw chat rounds can otherwise
// carry a fallback derived from the rewritten provider request.
func openAIUnmeasuredBillableInput(result *OpenAIForwardResult) bool {
	if result.ChatUsage != nil {
		for _, round := range result.ChatUsage.Rounds {
			if round != nil && round.UpstreamAccepted && !round.InputReported {
				return true
			}
		}
	}
	if result.InputTokensSource == UsageTokenSourceEstimated {
		return true
	}
	if result.InputTokensSource != UsageTokenSourceUnreported {
		return false
	}
	return result.Usage.CacheReadInputTokens > 0 || result.Usage.CacheCreationInputTokens > 0 ||
		result.Usage.OutputTokens > 0 || result.Usage.ImageOutputTokens > 0 ||
		(result.OpenAIWSMode && (result.UpstreamTerminalEvent == "response.completed" || result.UpstreamTerminalEvent == "response.done"))
}

func openAIInputBuckets(result *OpenAIForwardResult) openAIProtectedInputBuckets {
	tokens := result.Usage.billingUsageTokens()
	if input, ok := openAIChatUncachedInputTokens(result); ok {
		tokens.InputTokens = input
	}
	short, long := allocateCacheCreationBillingTokens(tokens)
	if openAIHasWriteTTL(tokens) {
		short, long = tokens.CacheCreation30mTokens, tokens.CacheCreation12hTokens
	}
	// Malformed cache details can exceed the gross input. They must never
	// enlarge the customer's gross count, even when the local limit is higher.
	gross := max(result.Usage.InputTokens, 0)
	read := min(max(tokens.CacheReadTokens, 0), gross)
	input := min(max(tokens.InputTokens, 0), gross-read)
	short = min(max(short, 0), gross-read-input)
	long = min(max(long, 0), gross-read-input-short)
	return openAIProtectedInputBuckets{read, input, short, long}
}

func openAIHasWriteTTL(t UsageTokens) bool {
	return t.CacheCreationTokens > 0 && t.CacheCreation30mTokens >= 0 && t.CacheCreation12hTokens >= 0 &&
		t.CacheCreation30mTokens <= t.CacheCreationTokens && t.CacheCreation12hTokens == t.CacheCreationTokens-t.CacheCreation30mTokens
}

func (b *openAIProtectedInputBuckets) take(want openAIProtectedInputBuckets) openAIProtectedInputBuckets {
	allocated := openAIProtectedInputBuckets{min(b.read, want.read), min(b.input, want.input), min(b.short, want.short), min(b.long, want.long)}
	b.read -= allocated.read
	b.input -= allocated.input
	b.short -= allocated.short
	b.long -= allocated.long
	return allocated
}

func projectOpenAIInputLeaf(r *OpenAIForwardResult, b openAIProtectedInputBuckets) *OpenAIForwardResult {
	leaf := *r
	// Prompt evidence has already been finalized, including disjoint per-call
	// cache counts. Do not let the pricing path recover uncapped inputs again.
	leaf.ChatUsage = nil
	u := &leaf.Usage
	u.CacheReadInputTokens = b.read
	u.CacheCreationInputTokens = b.short + b.long
	u.InputTokens = b.read + b.input + u.CacheCreationInputTokens
	u.CacheCreation5mTokens = min(r.Usage.CacheCreation5mTokens, b.short)
	u.CacheCreation1hTokens = min(r.Usage.CacheCreation1hTokens, b.long)
	u.CacheCreation30mInputTokens, u.CacheCreation12hInputTokens = 0, 0
	if openAIHasWriteTTL(r.Usage.billingUsageTokens()) {
		u.CacheCreation30mInputTokens, u.CacheCreation12hInputTokens = b.short, b.long
	}
	u.ImageCacheReadTokens = min(r.Usage.ImageCacheReadTokens, b.read)
	u.ImageInputTokens = u.ImageCacheReadTokens + min(max(0, r.Usage.ImageInputTokens-r.Usage.ImageCacheReadTokens), b.input)
	if leaf.Usage != r.Usage {
		leaf.InputTokensSource = UsageTokenSourceEstimated
	}
	return &leaf
}

// Apply the cap once to the logical request, then allocate each retained bucket
// across billable rounds in order. Provider results remain available for audit.
func projectOpenAIInputForBilling(r *OpenAIForwardResult, p *InputTokenProtection, tokenPriced map[*OpenAIForwardResult]bool) *OpenAIForwardResult {
	if !p.hasEstimate() {
		return r
	}
	rounds := r.ContinuationResults
	if len(rounds) == 0 {
		b := openAIInputBuckets(r)
		if openAIUnmeasuredBillableInput(r) || !p.exceedsTolerance(b.read, b.input, b.short, b.long) {
			return r
		}
		b.read, b.input, b.short, b.long = p.Cap(b.read, b.input, b.short, b.long)
		return projectOpenAIInputLeaf(r, b)
	}
	buckets := make([]openAIProtectedInputBuckets, len(rounds))
	var budget openAIProtectedInputBuckets
	for i, round := range rounds {
		if !tokenPriced[round] {
			continue
		}
		if openAIUnmeasuredBillableInput(round) {
			return r
		}
		b := openAIInputBuckets(round)
		buckets[i] = b
		budget.read += min(b.read, math.MaxInt-budget.read)
		budget.input += min(b.input, math.MaxInt-budget.input)
		budget.short += min(b.short, math.MaxInt-budget.short)
		budget.long += min(b.long, math.MaxInt-budget.long)
	}
	if !p.exceedsTolerance(budget.read, budget.input, budget.short, budget.long) {
		return r
	}
	budget.read, budget.input, budget.short, budget.long = p.Cap(budget.read, budget.input, budget.short, budget.long)
	projected := *r
	projected.ContinuationResults = make([]*OpenAIForwardResult, len(rounds))
	for i, round := range rounds {
		if tokenPriced[round] || continuationRoundFailedWithoutOutput(round.Outcome, round.DeliveredOutputTokens) {
			projected.ContinuationResults[i] = projectOpenAIInputLeaf(round, budget.take(buckets[i]))
		} else {
			projected.ContinuationResults[i] = round
		}
	}
	refreshOpenAIContinuationUsage(&projected)
	return &projected
}
