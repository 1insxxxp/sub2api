package service

import "context"

// PendingProviderUsage contains observations only. OpenAI prompt counts include
// cache, whereas Claude and normalized Gemini input counts exclude it. Missing
// totals stay nil, including when a legacy fallback supplied a numeric estimate.
type PendingProviderUsage struct {
	InputTokens            *int `json:"input_tokens,omitempty"`
	OutputTokens           *int `json:"output_tokens,omitempty"`
	CacheReadTokens        int  `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens    int  `json:"cache_creation_tokens,omitempty"`
	CacheCreation5mTokens  int  `json:"cache_creation_5m_tokens,omitempty"`
	CacheCreation1hTokens  int  `json:"cache_creation_1h_tokens,omitempty"`
	CacheCreation30mTokens int  `json:"cache_creation_30m_tokens,omitempty"`
	CacheCreation12hTokens int  `json:"cache_creation_12h_tokens,omitempty"`
	ImageInputTokens       int  `json:"image_input_tokens,omitempty"`
	ImageCacheReadTokens   int  `json:"image_cache_read_tokens,omitempty"`
	ImageOutputTokens      int  `json:"image_output_tokens,omitempty"`
	DeliveredOutputTokens  *int `json:"delivered_output_tokens,omitempty"`
}

// PendingUsageEvidence is kept exclusively in the pending store. The cost is a raw, pre-multiplier
// quote, never a ledger entry. A partial observation cannot establish a total.
type PendingUsageEvidence struct {
	PendingProviderUsage
	InputIncludesCache     bool                         `json:"input_includes_cache,omitempty"`
	InputLimit             *int                         `json:"input_limit,omitempty"`
	ProjectedOutputTokens  *int                         `json:"projected_output_tokens,omitempty"`
	ProviderTotalCost      *float64                     `json:"provider_total_cost,omitempty"`
	CacheStorageTokenHours float64                      `json:"cache_storage_token_hours,omitempty"`
	CacheStorageCost       float64                      `json:"cache_storage_cost,omitempty"`
	Rounds                 map[int]PendingProviderUsage `json:"rounds,omitempty"`
}

func observedPendingCount(count int, source UsageTokenSource) *int {
	if count >= 0 && (source == UsageTokenSourceUpstream || source.Normalize() == UsageTokenSourceUnknown && count > 0) {
		return &count
	}
	return nil
}

func pendingProviderUsage(input, output int, inputSource, outputSource UsageTokenSource, u UsageTokens, delivered *int) PendingProviderUsage {
	return PendingProviderUsage{
		InputTokens: observedPendingCount(input, inputSource), OutputTokens: observedPendingCount(output, outputSource),
		CacheReadTokens: u.CacheReadTokens, CacheCreationTokens: u.CacheCreationTokens,
		CacheCreation5mTokens: u.CacheCreation5mTokens, CacheCreation1hTokens: u.CacheCreation1hTokens,
		CacheCreation30mTokens: u.CacheCreation30mTokens, CacheCreation12hTokens: u.CacheCreation12hTokens,
		ImageOutputTokens: u.ImageOutputTokens, DeliveredOutputTokens: cloneContinuationPointer(delivered),
	}
}

func forwardPendingProviderUsage(r *ForwardResult) PendingProviderUsage {
	source := r.InputTokensSource
	if r.InputTokensEstimated {
		source = UsageTokenSourceEstimated
	}
	return pendingProviderUsage(r.Usage.InputTokens, r.Usage.OutputTokens, source, r.OutputTokensSource, r.Usage.billingUsageTokens(), r.DeliveredOutputTokens)
}

func (e *PendingUsageEvidence) measuredTotals() bool {
	if e.InputTokens == nil || e.OutputTokens == nil && e.CacheStorageTokenHours == 0 {
		return false
	}
	for _, round := range e.Rounds {
		if round.InputTokens == nil || round.OutputTokens == nil {
			return false
		}
	}
	return true
}

func (e *PendingUsageEvidence) setPrice(cost *CostBreakdown, err error) {
	if err == nil && cost != nil && ValidateUsageBillingActualCost(cost.TotalCost) == nil {
		e.ProviderTotalCost = &cost.TotalCost
	}
}

func (s *GatewayService) forwardPendingEvidence(ctx context.Context, input *RecordUsageInput, opts *recordUsageOpts) *PendingUsageEvidence {
	r := input.Result
	e := &PendingUsageEvidence{PendingProviderUsage: forwardPendingProviderUsage(r)}
	if ValidateUsageBillingActualCost(r.CacheStorageTokenHours) == nil {
		e.CacheStorageTokenHours = r.CacheStorageTokenHours
	}
	if !r.RequireMeasuredUsage && (r.DeliveredOutputTokens != nil || len(r.ContinuationResults) > 0) {
		projected := recordedOutputTokens(r)
		if projected != r.Usage.OutputTokens || recordedOutputTokensSource(r) == UsageTokenSourceEstimated {
			e.ProjectedOutputTokens = &projected
		}
	}
	if len(r.ContinuationResults) > 0 {
		e.Rounds = make(map[int]PendingProviderUsage, len(r.ContinuationResults))
		for i, round := range r.ContinuationResults {
			e.Rounds[i] = forwardPendingProviderUsage(round)
		}
	}
	if s.billingService != nil && (e.measuredTotals() || e.CacheStorageTokenHours > 0) {
		model := s.recordUsageBillingModel(ctx, input.APIKey, r, input.ChannelUsageFields)
		if storage := s.cacheStorageCost(r, model); ValidateUsageBillingActualCost(storage) == nil {
			e.CacheStorageCost = storage
		}
		if e.measuredTotals() {
			m, im, at := s.recordUsageMultipliers(ctx, input.APIKey, input.User, input.PricingAt)
			// Tier resolution mutates a result. Preserve the provider snapshot for retries.
			priced := cloneForwardBillingResult(r)
			ApplyForwardImageBillingResolution(priced)
			ApplyForwardServiceTierBillingResolution(priced)
			e.setPrice(s.calculateRecordUsageCostWithResponseModel(ctx, input, priced, model, m, im, at, opts, false))
		}
	}
	return e
}

func (log *UsageLog) protectPendingInput(p *InputTokenProtection, e *PendingUsageEvidence) {
	log.PendingEvidence = e
	log.BillingStatus = UsageBillingStatusPendingReview
	log.BillingPendingReason = UsagePendingReasonCustomerInputUnavailable
	if !p.Pending && p.Limit >= 0 {
		limit := p.Limit
		e.InputLimit = &limit
		log.BillingPendingReason = UsagePendingReasonUpstreamMissing
	}
	log.InputTokensSource = UsageTokenSourceUnreported
	log.InputTokens, log.ImageInputTokens, log.CacheReadTokens, log.CacheCreationTokens = 0, 0, 0, 0
	log.CacheCreation5mTokens, log.CacheCreation1hTokens, log.CacheCreation30mTokens, log.CacheCreation12hTokens = 0, 0, 0, 0
	// Keep provider output and the selected delivery count independent on retries.
	log.OutputTokensSource, log.OutputTokens, log.UpstreamOutputTokens = UsageTokenSourceUnreported, 0, 0
	if e.OutputTokens != nil {
		log.OutputTokensSource, log.OutputTokens = UsageTokenSourceUpstream, *e.OutputTokens
		log.UpstreamOutputTokens = *e.OutputTokens
	}
	if e.ProjectedOutputTokens != nil {
		log.OutputTokensSource, log.OutputTokens = UsageTokenSourceEstimated, *e.ProjectedOutputTokens
	}
	log.ImageOutputTokens = e.ImageOutputTokens
}
