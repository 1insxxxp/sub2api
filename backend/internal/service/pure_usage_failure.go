package service

// A missing usage counter is not enough to drop a request. These predicates run
// only after provider/ledger recovery and finalization, before any accounting
// write. Unknown legacy evidence and every sign of accepted work are retained.
func shouldSkipPureOpenAIChatUsageFailure(result *OpenAIForwardResult) bool {
	if result == nil || result.ChatUsage == nil || !result.ChatUsage.Finalized ||
		len(result.ChatUsage.Rounds) == 0 || result.ChatUsage.HadDeliveredOutput || !hasPureFailureOutcome(result.Outcome) {
		return false
	}
	if result.Usage != (OpenAIUsage{}) || result.ChatUsage.ExtraUsage != (OpenAIUsage{}) ||
		result.ImageCount != 0 || result.VideoCount != 0 || result.VideoDurationSeconds != 0 ||
		result.WebSearchCalls != 0 || result.SearchCount != 0 || result.AudioUsage != nil ||
		result.ImageOutputSize != "" || len(result.ImageOutputSizes) != 0 || len(result.ImageSizeBreakdown) != 0 {
		return false
	}
	for _, round := range result.ChatUsage.Rounds {
		if round == nil || round.UpstreamAccepted || !hasPureFailureOutcome(&round.Outcome) ||
			round.Usage != (OpenAIUsage{}) || round.ReceivedOutputTokens != 0 || round.ReceivedReasoningTokens != 0 ||
			round.CompletionTokens != 0 || round.ReasoningTokens != 0 || round.TotalTokens != 0 || round.OutputTruncated {
			return false
		}
	}
	return true
}

func shouldSkipPureGeminiUsageFailure(result *ForwardResult) bool {
	if result == nil || !result.RequireMeasuredUsage || !result.GeminiUsageFinalized ||
		result.GeminiUsageFallback == nil || !hasPureFailureOutcome(result.Outcome) ||
		result.AntiTruncationRounds != 0 || hasDeliveredUsageOutput(result.DeliveredOutputTokens) {
		return false
	}
	// Native Gemini retains the last attempt's capture. A continuation result
	// cannot prove that all earlier rounds were unaccepted, so it is retained.
	evidence := result.GeminiUsageFallback
	if evidence.UpstreamAccepted || evidence.CandidateTokens != 0 || evidence.ThoughtTokens != 0 ||
		evidence.MeasuredCandidates != 0 || evidence.MeasuredThoughts != 0 || evidence.OutputTruncated {
		return false
	}
	return result.Usage == (ClaudeUsage{}) && result.ImageCount == 0 && result.SearchCount == 0 &&
		result.AudioUsage == nil && result.CacheStorageTokenHours == 0 && result.ImageOutputSize == "" &&
		len(result.ImageOutputSizes) == 0 && len(result.ImageSizeBreakdown) == 0
}

func hasDeliveredUsageOutput(tokens *int) bool {
	return tokens != nil && *tokens != 0
}

func hasPureFailureOutcome(outcome *ResponseOutcome) bool {
	if outcome == nil || outcome.CollectorVersion <= 0 || outcome.StreamCompleted || outcome.HasEffectiveOutput() {
		return false
	}
	status := ClassifyUsageRequestStatus(outcome)
	return status == UsageRequestFailed || status == UsageRequestInterrupted
}
