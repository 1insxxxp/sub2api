package service

import (
	"context"
	"errors"
	"math"
	"time"
)

var ErrInvalidContinuationUsage = errors.New("invalid continuation usage evidence")

const maxContinuationUsageResults = 4

func validateForwardContinuationResults(result *ForwardResult) error {
	if result == nil || len(result.ContinuationResults) == 0 {
		return nil
	}
	if len(result.ContinuationResults) > maxContinuationUsageResults {
		return ErrInvalidContinuationUsage
	}
	seen := make(map[*ForwardResult]bool, len(result.ContinuationResults))
	var totals ClaudeUsage
	deliveredTotal := 0
	for _, round := range result.ContinuationResults {
		if round == nil || round == result || seen[round] || len(round.ContinuationResults) != 0 {
			return ErrInvalidContinuationUsage
		}
		seen[round] = true
		if round.DeliveredOutputTokens != nil {
			if *round.DeliveredOutputTokens < 0 || *round.DeliveredOutputTokens > math.MaxInt32-deliveredTotal {
				return ErrInvalidContinuationUsage
			}
			deliveredTotal += *round.DeliveredOutputTokens
		}
		if !validContinuationTokens(claudeContinuationCounts(round.Usage), claudeContinuationCounts(totals)) {
			return ErrInvalidContinuationUsage
		}
		addClaudeUsage(&totals, round.Usage)
	}
	return nil
}

func validateOpenAIContinuationResults(result *OpenAIForwardResult) error {
	if result == nil || len(result.ContinuationResults) == 0 {
		return nil
	}
	if len(result.ContinuationResults) > maxContinuationUsageResults {
		return ErrInvalidContinuationUsage
	}
	seen := make(map[*OpenAIForwardResult]bool, len(result.ContinuationResults))
	var totals OpenAIUsage
	deliveredTotal := 0
	for _, round := range result.ContinuationResults {
		if round == nil || round == result || seen[round] || len(round.ContinuationResults) != 0 {
			return ErrInvalidContinuationUsage
		}
		seen[round] = true
		if round.DeliveredOutputTokens != nil {
			if *round.DeliveredOutputTokens < 0 || *round.DeliveredOutputTokens > math.MaxInt32-deliveredTotal {
				return ErrInvalidContinuationUsage
			}
			deliveredTotal += *round.DeliveredOutputTokens
		}
		if !validContinuationTokens(openAIContinuationCounts(round.Usage), openAIContinuationCounts(totals)) {
			return ErrInvalidContinuationUsage
		}
		addOpenAIUsage(&totals, round.Usage)
	}
	return nil
}

func validContinuationTokens(values, totals []int) bool {
	for i, n := range values {
		if n < 0 || n > math.MaxInt32-totals[i] {
			return false
		}
	}
	return true
}

func claudeContinuationCounts(usage ClaudeUsage) []int {
	return []int{usage.InputTokens, usage.OutputTokens, usage.CacheReadInputTokens,
		usage.CacheCreationInputTokens, usage.CacheCreation5mTokens, usage.CacheCreation1hTokens, usage.ImageOutputTokens}
}

func openAIContinuationCounts(usage OpenAIUsage) []int {
	return []int{usage.InputTokens, usage.OutputTokens, usage.CacheReadInputTokens,
		usage.CacheCreationInputTokens, usage.ImageInputTokens, usage.ImageCacheReadTokens, usage.ImageOutputTokens,
		usage.CacheCreation30mInputTokens, usage.CacheCreation12hInputTokens}
}

// AggregateForwardContinuationResults snapshots leaf results so a subsequent
// protocol round cannot replace an earlier request's ledger or usage evidence.
func AggregateForwardContinuationResults(rounds []*ForwardResult) (*ForwardResult, error) {
	result := &ForwardResult{ContinuationResults: rounds}
	if len(rounds) == 0 || validateForwardContinuationResults(result) != nil {
		return nil, ErrInvalidContinuationUsage
	}
	result = cloneForwardContinuationLeaf(rounds[len(rounds)-1])
	result.ContinuationResults = make([]*ForwardResult, 0, len(rounds))
	for _, round := range rounds {
		result.ContinuationResults = append(result.ContinuationResults, cloneForwardContinuationLeaf(round))
	}
	refreshForwardContinuationUsage(result)
	result.FirstTokenMs = cloneContinuationPointer(rounds[0].FirstTokenMs)
	result.Outcome = aggregateContinuationOutcome(result.Outcome, func(visit func(*ResponseOutcome)) {
		for _, round := range rounds {
			visit(round.Outcome)
		}
	})
	result.AntiTruncationRounds = len(rounds) - 1
	return result, nil
}

func AggregateOpenAIContinuationResults(rounds []*OpenAIForwardResult) (*OpenAIForwardResult, error) {
	result := &OpenAIForwardResult{ContinuationResults: rounds}
	if len(rounds) == 0 || validateOpenAIContinuationResults(result) != nil {
		return nil, ErrInvalidContinuationUsage
	}
	result = cloneOpenAIContinuationLeaf(rounds[len(rounds)-1])
	result.ContinuationResults = make([]*OpenAIForwardResult, 0, len(rounds))
	for _, round := range rounds {
		result.ContinuationResults = append(result.ContinuationResults, cloneOpenAIContinuationLeaf(round))
	}
	refreshOpenAIContinuationUsage(result)
	result.FirstTokenMs = cloneContinuationPointer(rounds[0].FirstTokenMs)
	result.Outcome = aggregateContinuationOutcome(result.Outcome, func(visit func(*ResponseOutcome)) {
		for _, round := range rounds {
			visit(round.Outcome)
		}
	})
	result.ContinuationPublicResponseID = rounds[0].ResponseID
	if result.ContinuationPublicResponseID == "" {
		result.ContinuationPublicResponseID = rounds[0].RequestID
	}
	result.AntiTruncationRounds = len(rounds) - 1
	return result, nil
}

// Account costs use provider measurements and each call's own context tier,
// independently of both the customer token projection and delivery fallback.
func (s *GatewayService) applyForwardContinuationAccountStatsCost(ctx context.Context, log *UsageLog, input *recordUsageCoreInput, result *ForwardResult, model string, multiplier, imageMultiplier float64, at time.Time, opts *recordUsageOpts) error {
	total := 0.0
	for _, round := range result.ContinuationResults {
		cost, err := s.calculateRecordUsageCostWithResponseModel(ctx, input, round, model, multiplier, imageMultiplier, at, opts, false)
		if err != nil {
			return err
		}
		leaf := &UsageLog{ServiceTier: round.ServiceTier, ReasoningEffort: round.ReasoningEffort}
		applyAccountStatsCost(ctx, leaf, s.channelService, s.billingService, input.Account.ID, *input.APIKey.GroupID, round.UpstreamModel, round.Model,
			UsageTokens{InputTokens: round.Usage.InputTokens, OutputTokens: round.Usage.OutputTokens, CacheReadTokens: round.Usage.CacheReadInputTokens, CacheCreationTokens: round.Usage.CacheCreationInputTokens}, cost.TotalCost, at, accountStatsLongContextPricingEnabled(nil))
		if leaf.AccountStatsCost != nil {
			total += *leaf.AccountStatsCost
		} else {
			total += cost.TotalCost
		}
	}
	log.AccountStatsCost = &total
	return nil
}

func cloneForwardContinuationLeaf(round *ForwardResult) *ForwardResult {
	clone := *round
	clone.ContinuationResults = nil
	clone.ServiceTier = cloneContinuationPointer(round.ServiceTier)
	clone.ReasoningEffort = cloneContinuationPointer(round.ReasoningEffort)
	clone.RequestedReasoningEffort = cloneContinuationPointer(round.RequestedReasoningEffort)
	clone.FirstTokenMs = cloneContinuationPointer(round.FirstTokenMs)
	clone.UpstreamHeaders = round.UpstreamHeaders.Clone()
	if round.Outcome != nil {
		outcome := *round.Outcome
		clone.Outcome = &outcome
	}
	if round.DeliveredOutputTokens != nil {
		delivered := *round.DeliveredOutputTokens
		clone.DeliveredOutputTokens = &delivered
	}
	if round.GeminiUsageFallback != nil {
		fallback := *round.GeminiUsageFallback
		clone.GeminiUsageFallback = &fallback
		if fallback.Input != nil {
			input := *fallback.Input
			input.CountRequest = cloneContinuationCountRequest(input.CountRequest)
			clone.GeminiUsageFallback.Input = &input
		}
	}
	if round.GeminiInputCounting != nil {
		counting := *round.GeminiInputCounting
		clone.GeminiInputCounting = &counting
	}
	return &clone
}

func cloneOpenAIContinuationLeaf(round *OpenAIForwardResult) *OpenAIForwardResult {
	clone := *round
	clone.ContinuationResults = nil
	clone.ServiceTier = cloneContinuationPointer(round.ServiceTier)
	clone.ReasoningEffort = cloneContinuationPointer(round.ReasoningEffort)
	clone.RequestedReasoningEffort = cloneContinuationPointer(round.RequestedReasoningEffort)
	clone.FirstTokenMs = cloneContinuationPointer(round.FirstTokenMs)
	clone.UpstreamHeaders = round.UpstreamHeaders.Clone()
	clone.ResponseHeaders = round.ResponseHeaders.Clone()
	if round.Outcome != nil {
		outcome := *round.Outcome
		clone.Outcome = &outcome
	}
	if round.DeliveredOutputTokens != nil {
		delivered := *round.DeliveredOutputTokens
		clone.DeliveredOutputTokens = &delivered
	}
	if round.ChatUsage != nil {
		evidence := *round.ChatUsage
		evidence.Rounds = make([]*OpenAIChatUsageRound, len(round.ChatUsage.Rounds))
		for i, usageRound := range round.ChatUsage.Rounds {
			if usageRound == nil {
				continue
			}
			leaf := *usageRound
			if leaf.InputFallback != nil {
				fallback := *leaf.InputFallback
				fallback.CountRequest = cloneContinuationCountRequest(fallback.CountRequest)
				leaf.InputFallback = &fallback
			}
			evidence.Rounds[i] = &leaf
		}
		clone.ChatUsage = &evidence
	}
	return &clone
}

func refreshForwardContinuationUsage(result *ForwardResult) {
	result.Usage = ClaudeUsage{}
	result.Duration = 0
	result.ClientDisconnect = false
	result.EstimatedInputTokens = 0
	result.InputTokensEstimated = false
	result.RequireMeasuredUsage = false
	result.MeasuredUsageComplete = true
	result.GeminiUsageFinalized = true
	result.GeminiUsageFallback = nil
	result.GeminiInputCounting = nil
	result.UsageLedgerRequestID, result.UsageLedgerIdentityHash = "", ""
	result.InputTokensSource, result.OutputTokensSource = UsageTokenSourceUnknown, UsageTokenSourceUnknown
	result.DeliveredOutputTokens = nil
	deliveredTotal, allDelivered := 0, true
	for _, round := range result.ContinuationResults {
		if round.DeliveredOutputTokens == nil {
			allDelivered = false
		} else {
			deliveredTotal += *round.DeliveredOutputTokens
		}
		addClaudeUsage(&result.Usage, round.Usage)
		result.Duration += round.Duration
		result.EstimatedInputTokens += round.EstimatedInputTokens
		result.InputTokensEstimated = result.InputTokensEstimated || round.InputTokensEstimated
		result.ClientDisconnect = result.ClientDisconnect || round.ClientDisconnect
		result.RequireMeasuredUsage = result.RequireMeasuredUsage || round.RequireMeasuredUsage
		if round.RequireMeasuredUsage {
			result.MeasuredUsageComplete = result.MeasuredUsageComplete && round.MeasuredUsageComplete
			result.GeminiUsageFinalized = result.GeminiUsageFinalized && (round.GeminiUsageFinalized || round.MeasuredUsageComplete)
		}
		result.InputTokensSource = mergeOpenAIChatTokenSource(result.InputTokensSource, round.InputTokensSource)
		result.OutputTokensSource = mergeOpenAIChatTokenSource(result.OutputTokensSource, round.OutputTokensSource)
	}
	if allDelivered {
		result.DeliveredOutputTokens = &deliveredTotal
	}
	if len(result.ContinuationResults) > 0 {
		result.ServiceTier = cloneContinuationPointer(result.ContinuationResults[len(result.ContinuationResults)-1].ServiceTier)
	}
}

func refreshOpenAIContinuationUsage(result *OpenAIForwardResult) {
	result.Usage = OpenAIUsage{}
	result.Duration = 0
	result.ClientDisconnect = false
	result.ChatUsage = nil
	result.InputTokensSource, result.OutputTokensSource = UsageTokenSourceUnknown, UsageTokenSourceUnknown
	result.DeliveredOutputTokens = nil
	deliveredTotal, allDelivered := 0, true
	for _, round := range result.ContinuationResults {
		if round.DeliveredOutputTokens == nil {
			allDelivered = false
		} else {
			deliveredTotal += *round.DeliveredOutputTokens
		}
		addOpenAIUsage(&result.Usage, round.Usage)
		result.Duration += round.Duration
		result.ClientDisconnect = result.ClientDisconnect || round.ClientDisconnect
		result.InputTokensSource = mergeOpenAIChatTokenSource(result.InputTokensSource, round.InputTokensSource)
		result.OutputTokensSource = mergeOpenAIChatTokenSource(result.OutputTokensSource, round.OutputTokensSource)
	}
	if allDelivered {
		result.DeliveredOutputTokens = &deliveredTotal
	}
	if len(result.ContinuationResults) > 0 {
		result.ServiceTier = cloneContinuationPointer(result.ContinuationResults[len(result.ContinuationResults)-1].ServiceTier)
	}
}

func addContinuationCost(total, round *CostBreakdown) {
	if round == nil {
		return
	}
	total.InputCost += round.InputCost
	total.ImageInputCost += round.ImageInputCost
	total.OutputCost += round.OutputCost
	total.ImageOutputCost += round.ImageOutputCost
	total.CacheCreationCost += round.CacheCreationCost
	total.CacheReadCost += round.CacheReadCost
	total.TotalCost += round.TotalCost
	total.ActualCost += round.ActualCost
	total.LongContextBillingApplied = total.LongContextBillingApplied || round.LongContextBillingApplied
	if total.BillingMode == "" {
		total.BillingMode = round.BillingMode
	}
}

func openAIContinuationUsageTokens(result *OpenAIForwardResult, delivered bool) UsageTokens {
	if delivered && continuationRoundFailedWithoutOutput(result.Outcome, result.DeliveredOutputTokens) {
		return UsageTokens{}
	}
	uncached := max(0, result.Usage.InputTokens-result.Usage.CacheReadInputTokens-result.Usage.CacheCreationInputTokens)
	if roundInput, ok := openAIChatUncachedInputTokens(result); ok {
		uncached = roundInput
	}
	output := result.Usage.OutputTokens
	if delivered {
		output = recordedOpenAIOutputTokens(result)
	}
	return UsageTokens{InputTokens: uncached, OutputTokens: output,
		CacheReadTokens: result.Usage.CacheReadInputTokens, CacheCreationTokens: result.Usage.CacheCreationInputTokens,
		CacheCreation30mTokens: result.Usage.CacheCreation30mInputTokens, CacheCreation12hTokens: result.Usage.CacheCreation12hInputTokens,
		ImageInputTokens:     max(0, result.Usage.ImageInputTokens-result.Usage.ImageCacheReadTokens),
		ImageCacheReadTokens: result.Usage.ImageCacheReadTokens, ImageOutputTokens: result.Usage.ImageOutputTokens}
}

func continuationRoundFailedWithoutOutput(outcome *ResponseOutcome, delivered *int) bool {
	if hasDeliveredUsageOutput(delivered) {
		return false
	}
	if delivered != nil && outcome != nil && outcome.CollectorVersion > 0 {
		status := ClassifyUsageRequestStatus(outcome)
		if status == UsageRequestFailed || status == UsageRequestInterrupted {
			return true
		}
	}
	return hasPureFailureOutcome(outcome)
}

func continuationCustomerForwardLeaf(round *ForwardResult, account *Account, verifyAccount bool) *ForwardResult {
	projected := customerBillingForwardResult(round)
	if verifyAccount {
		projected = customerBillingForwardResultForAccount(round, account)
	}
	if continuationRoundFailedWithoutOutput(round.Outcome, round.DeliveredOutputTokens) {
		clone := *projected
		clone.Usage = ClaudeUsage{}
		return &clone
	}
	if output := continuationRecordedOutputTokens(round); output != projected.Usage.OutputTokens {
		clone := *projected
		clone.Usage.OutputTokens = output
		return &clone
	}
	return projected
}

func aggregateContinuationOutcome(last *ResponseOutcome, visit func(func(*ResponseOutcome))) *ResponseOutcome {
	var summary ResponseOutcome
	if last != nil {
		summary = *last
	}
	found := last != nil
	summary.OutputBytes, summary.EventCount = 0, 0
	visit(func(outcome *ResponseOutcome) {
		if outcome == nil {
			return
		}
		found = true
		summary.HasText = summary.HasText || outcome.HasText
		summary.HasReasoning = summary.HasReasoning || outcome.HasReasoning
		summary.HasToolCall = summary.HasToolCall || outcome.HasToolCall
		summary.HasMedia = summary.HasMedia || outcome.HasMedia
		summary.OutputBytes += outcome.OutputBytes
		summary.EventCount += outcome.EventCount
	})
	if !found {
		return nil
	}
	return &summary
}

func (s *OpenAIGatewayService) calculateOpenAIContinuationRecordUsageCost(ctx context.Context, input *OpenAIRecordUsageInput, account *Account, models []string, multiplier, imageMultiplier, videoMultiplier, baseMultiplier float64, gate *bool, exempt bool, at time.Time) (*CostBreakdown, error) {
	total := &CostBreakdown{}
	for _, round := range input.Result.ContinuationResults {
		if continuationRoundFailedWithoutOutput(round.Outcome, round.DeliveredOutputTokens) {
			continue
		}
		cost, err := s.openAIContinuationLeafCost(ctx, input, round, models, multiplier, imageMultiplier, videoMultiplier, baseMultiplier, gate, exempt, at, true)
		if err != nil {
			return nil, err
		}
		if groupBillsOpenAIFastAtStandard(input.APIKey, account, optionalStringValue(round.ServiceTier)) {
			standard := *round
			standard.ServiceTier = nil
			base, err := s.openAIContinuationLeafCost(ctx, input, &standard, models, multiplier, imageMultiplier, videoMultiplier, baseMultiplier, gate, exempt, at, true)
			if err != nil {
				return nil, err
			}
			cost.ActualCost = base.ActualCost
		}
		addContinuationCost(total, cost)
	}
	return total, nil
}

func (s *OpenAIGatewayService) openAIContinuationLeafCost(ctx context.Context, input *OpenAIRecordUsageInput, round *OpenAIForwardResult, models []string, multiplier, imageMultiplier, videoMultiplier, baseMultiplier float64, gate *bool, exempt bool, at time.Time, delivered bool) (*CostBreakdown, error) {
	tokens := openAIContinuationUsageTokens(round, delivered)
	tier := optionalStringValue(round.ServiceTier)
	cost, err := s.calculateOpenAIRecordUsageCost(ctx, round, input.APIKey, models, multiplier, imageMultiplier, videoMultiplier, baseMultiplier, tokens, tier, gate, exempt, at)
	if err != nil {
		return nil, err
	}
	baselineModel := firstUsageBillingModel(models)
	if model := responseModelBillingDeclaration(input.BillingModelSource, round.UpstreamResponseModel, round.UpstreamResponseModelConflict, false); model != "" && model != baselineModel {
		if identified, responsePriced := s.hasIdentifiedOpenAIResponsePricing(ctx, model, input.APIKey); identified {
			candidates := s.filterCNProviderBillingModelCandidates(ctx, input.Account, input.APIKey, usageBillingModelCandidates(model))
			alternative, err := s.calculateOpenAIRecordUsageCost(ctx, round, input.APIKey, candidates, multiplier, imageMultiplier, videoMultiplier, baseMultiplier, tokens, tier, gate, exempt, at)
			baselinePriced := s.resolveOpenAIChannelPricing(ctx, baselineModel, input.APIKey) != nil
			if err == nil && responseModelBillingAdoptable(cost, alternative, baselinePriced, responsePriced) {
				cost = alternative
			}
		}
	}
	return cost, nil
}

func (s *OpenAIGatewayService) applyOpenAIContinuationAccountStatsCost(ctx context.Context, log *UsageLog, input *OpenAIRecordUsageInput, models []string, multiplier, imageMultiplier, videoMultiplier, baseMultiplier float64, gate *bool, exempt bool, at time.Time) error {
	total := 0.0
	for _, round := range input.Result.ContinuationResults {
		cost, err := s.openAIContinuationLeafCost(ctx, input, round, models, multiplier, imageMultiplier, videoMultiplier, baseMultiplier, gate, exempt, at, false)
		if err != nil {
			return err
		}
		leaf := &UsageLog{ServiceTier: round.ServiceTier, ReasoningEffort: round.ReasoningEffort}
		applyAccountStatsCost(ctx, leaf, s.channelService, s.billingService, input.Account.ID, *input.APIKey.GroupID, round.UpstreamModel, round.Model, openAIContinuationUsageTokens(round, false), cost.TotalCost, at, accountStatsLongContextPricingEnabled(gate) && !exempt)
		if leaf.AccountStatsCost != nil {
			total += *leaf.AccountStatsCost
		} else {
			total += cost.TotalCost
		}
	}
	log.AccountStatsCost = &total
	return nil
}

func (s *OpenAIGatewayService) calculateOpenAIContinuationRawCost(ctx context.Context, input *OpenAIRecordUsageInput, models []string, multiplier, imageMultiplier, videoMultiplier, baseMultiplier float64, gate *bool, exempt bool, at time.Time) (*CostBreakdown, error) {
	total := &CostBreakdown{}
	for _, round := range input.Result.ContinuationResults {
		cost, err := s.openAIContinuationLeafCost(ctx, input, round, models, multiplier, imageMultiplier, videoMultiplier, baseMultiplier, gate, exempt, at, false)
		if err != nil {
			return nil, err
		}
		addContinuationCost(total, cost)
	}
	return total, nil
}

// This path only handles safe-text account continuations. Retain provider usage
// for account costs, while an interrupted round bills its actual delivered text.
func continuationRecordedOutputTokens(round *ForwardResult) int {
	if continuationRoundFailedWithoutOutput(round.Outcome, round.DeliveredOutputTokens) {
		return 0
	}
	if round.DeliveredOutputTokens != nil && (round.ClientDisconnect || round.Outcome != nil && !round.Outcome.StreamCompleted) {
		return min(max(0, round.Usage.OutputTokens), max(0, *round.DeliveredOutputTokens))
	}
	return recordedOutputTokens(round)
}

func cloneContinuationPointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
func cloneContinuationCountRequest(request *geminiInputCountRequest) *geminiInputCountRequest {
	if request == nil {
		return nil
	}
	copy := *request
	copy.Body = append([]byte(nil), request.Body...)
	return &copy
}
