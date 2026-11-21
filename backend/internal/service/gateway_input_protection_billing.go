package service

import "math"

func forwardBillingRounds(result *ForwardResult) []*ForwardResult {
	if len(result.ContinuationResults) != 0 {
		return result.ContinuationResults
	}
	return []*ForwardResult{result}
}

// Equal aggregate counts can hide different per-round pricing tiers.
func sameForwardBillingUsage(a, b *ForwardResult) bool {
	if a == b {
		return true
	}
	if a.Usage != b.Usage {
		return false
	}
	for i, leaf := range a.ContinuationResults {
		if leaf.Usage != b.ContinuationResults[i].Usage {
			return false
		}
	}
	return true
}

// Only scalar billing fields change. Request/response evidence stays read-only.
func cloneForwardBillingResult(result *ForwardResult) *ForwardResult {
	clone := *result
	if len(result.ContinuationResults) > 0 {
		clone.ContinuationResults = make([]*ForwardResult, len(result.ContinuationResults))
		for i, round := range result.ContinuationResults {
			leaf := *round
			leaf.InputTokenProtection = result.InputTokenProtection
			clone.ContinuationResults[i] = &leaf
		}
	}
	return &clone
}

// Standalone search, audio and storage extensions have no input-token charge.
func forwardNonTokenUsage(result *ForwardResult) bool {
	return result.AudioUsage != nil || isCacheStorageOnlyResult(result) ||
		result.SearchCount > 0 && result.Usage == (ClaudeUsage{}) &&
			(result.Model == "grok-web-search" || result.Model == "grok-x-search")
}

func forwardTokenPriced(result *ForwardResult, cost *CostBreakdown) bool {
	return !forwardNonTokenUsage(result) &&
		(cost.BillingMode == string(BillingModeToken) || cost.BillingMode == "" && result.ImageCount == 0)
}

// Preserve per-call tiers and delivery projections. Allocate a single request
// budget across the four buckets so later cache writes take precedence
// over earlier ordinary input or cache reads. Within a bucket, retain provider call order.
func capForwardBillingRounds(rounds []*ForwardResult, protection *InputTokenProtection) bool {
	type buckets struct{ read, input, short, long int }
	var total buckets
	counts := make([]buckets, len(rounds))
	for i, round := range rounds {
		u := round.Usage
		short, long := allocateCacheCreationBillingTokens(u.billingUsageTokens())
		counts[i] = buckets{max(0, u.CacheReadInputTokens), max(0, u.InputTokens), short, long}
		// Saturate each aggregate before applying the request-wide threshold.
		total.read += min(counts[i].read, math.MaxInt-total.read)
		total.input += min(counts[i].input, math.MaxInt-total.input)
		total.short += min(short, math.MaxInt-total.short)
		total.long += min(long, math.MaxInt-total.long)
	}
	if !protection.exceedsTolerance(total.read, total.input, total.short, total.long) {
		return false
	}
	read, input, short, long := protection.Cap(total.read, total.input, total.short, total.long)
	take := func(remaining *int, n int) int {
		used := min(*remaining, n)
		*remaining -= used
		return used
	}
	for i, round := range rounds {
		u := &round.Usage
		before := *u
		u.CacheReadInputTokens = take(&read, counts[i].read)
		u.InputTokens = take(&input, counts[i].input)
		write5m, write1h := take(&short, counts[i].short), take(&long, counts[i].long)
		u.CacheCreation5mTokens, u.CacheCreation1hTokens = write5m, write1h
		u.CacheCreationInputTokens = write5m + write1h
		if u.CacheReadInputTokens != before.CacheReadInputTokens || u.InputTokens != before.InputTokens || write5m != counts[i].short || write1h != counts[i].long {
			round.InputTokensSource = UsageTokenSourceEstimated
			round.InputTokensEstimated = true
		}
	}
	return true
}

func lessExpensiveForwardCost(baseline, candidate *CostBreakdown) *CostBreakdown {
	if candidate.TotalCost <= baseline.TotalCost && candidate.ActualCost <= baseline.ActualCost {
		return candidate
	}
	return baseline
}

type forwardUsagePrice func(*ForwardResult) (*CostBreakdown, error)

// Price each original leaf before allocation: an adopted response model can
// have a per-request tariff even when the requested model is token-priced.
func protectForwardBilling(result, provider *ForwardResult, protection *InputTokenProtection, original *CostBreakdown, price forwardUsagePrice) (customer *ForwardResult, cost *CostBreakdown, err error) {
	if !protection.hasEstimate() {
		return result, original, nil
	}
	projected := cloneForwardBillingResult(result)
	leaves := forwardBillingRounds(projected)
	eligible := make([]*ForwardResult, 0, len(leaves))
	prices := make([]*CostBreakdown, len(leaves))
	observations := forwardBillingRounds(provider)
	for i, leaf := range leaves {
		baseline := original
		if len(leaves) > 1 {
			baseline, err = price(leaf)
			if err != nil {
				return nil, nil, err
			}
		}
		prices[i] = baseline
		if forwardTokenPriced(leaf, baseline) {
			if forwardBillableInputMissing(observations[i], leaf) {
				return result, original, nil
			}
			eligible = append(eligible, leaf)
		} else {
			leaf.InputTokenProtection = nil
		}
	}
	if len(eligible) == 0 || !capForwardBillingRounds(eligible, protection) {
		return result, original, nil
	}
	if len(projected.ContinuationResults) > 0 {
		refreshForwardContinuationUsage(projected)
	}
	total := &CostBreakdown{}
	raw := forwardBillingRounds(result)
	for i, leaf := range leaves {
		baseline := prices[i]
		if leaf.Usage != raw[i].Usage {
			capped, err := price(leaf)
			if err != nil {
				return nil, nil, err
			}
			if err := ValidateUsageBillingActualCost(capped.ActualCost); err != nil {
				return nil, nil, err
			}
			baseline = lessExpensiveForwardCost(baseline, capped)
		}
		addContinuationCost(total, baseline)
	}
	return projected, total, nil
}

// An explicit measured zero is valid. An estimate of the transformed
// provider request cannot establish a discrepancy with the original input.
func forwardBillableInputMissing(provider, customer *ForwardResult) bool {
	if provider.InputTokensEstimated || provider.InputTokensSource == UsageTokenSourceEstimated || provider.InputTokensSource == UsageTokenSourceUnreported {
		return true
	}
	return provider.InputTokensSource != UsageTokenSourceUpstream && anthropicInputUsageMissing(provider.Usage) &&
		(customer.Usage.OutputTokens > 0 || customer.Usage.ImageOutputTokens > 0 || customer.ImageCount > 0)
}
