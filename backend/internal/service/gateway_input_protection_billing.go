package service

import "context"

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
// budget across the four buckets first so a later cached read takes precedence
// over an earlier ordinary input. Within a bucket, retain provider call order.
func capForwardBillingRounds(rounds []*ForwardResult, protection *InputTokenProtection) {
	type buckets struct{ read, input, short, long int }
	var total buckets
	counts := make([]buckets, len(rounds))
	for i, round := range rounds {
		u := round.Usage
		short, long := allocateCacheCreationBillingTokens(u.billingUsageTokens())
		counts[i] = buckets{max(0, u.CacheReadInputTokens), max(0, u.InputTokens), short, long}
		// Each bucket can consume at most Limit, including malformed oversized counts.
		total.read += min(counts[i].read, protection.Limit-total.read)
		total.input += min(counts[i].input, protection.Limit-total.input)
		total.short += min(short, protection.Limit-total.short)
		total.long += min(long, protection.Limit-total.long)
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
func protectForwardBilling(result, provider *ForwardResult, protection *InputTokenProtection, original *CostBreakdown, price forwardUsagePrice) (customer *ForwardResult, cost *CostBreakdown, pending bool, err error) {
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
				return nil, nil, false, err
			}
		}
		prices[i] = baseline
		if forwardTokenPriced(leaf, baseline) {
			if protection.Pending || protection.Limit < 0 || forwardBillableInputMissing(observations[i], leaf) {
				return result, original, true, nil
			}
			eligible = append(eligible, leaf)
		} else {
			leaf.InputTokenProtection = nil
		}
	}
	if len(eligible) == 0 {
		return result, original, false, nil
	}
	capForwardBillingRounds(eligible, protection)
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
				return nil, nil, false, err
			}
			if err := ValidateUsageBillingActualCost(capped.ActualCost); err != nil {
				return nil, nil, false, err
			}
			baseline = lessExpensiveForwardCost(baseline, capped)
		}
		addContinuationCost(total, baseline)
	}
	return projected, total, false, nil
}

// An explicit measured zero is valid. A generated answer without either
// upstream input usage stays in review even if a legacy estimate of the
// transformed provider request was available.
func forwardBillableInputMissing(provider, customer *ForwardResult) bool {
	return provider.InputTokensSource != UsageTokenSourceUpstream && anthropicInputUsageMissing(provider.Usage) &&
		(customer.Usage.OutputTokens > 0 || customer.Usage.ImageOutputTokens > 0 || customer.ImageCount > 0)
}

// A protected request may arrive directly with incomplete Gemini measurements.
// Keep the historical measurement error for an explicitly non-token tariff.
func (s *GatewayService) forwardConfiguredTokenTariff(ctx context.Context, result *ForwardResult, key *APIKey, model string) bool {
	if forwardNonTokenUsage(result) {
		return false
	}
	resolved := s.resolveChannelPricing(ctx, model, key)
	if result.ImageCount > 0 {
		return !imageCountBillingApplies(key, result.ImageEndpoint) && resolved != nil && resolved.Mode == BillingModeToken
	}
	return resolved == nil || resolved.Mode == "" || resolved.Mode == BillingModeToken
}
