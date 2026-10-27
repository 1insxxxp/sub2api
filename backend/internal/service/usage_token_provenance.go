package service

// UsageTokenSource describes evidence for an individual stored token count.
// An absent historical source is unknown, never an observed zero.
type UsageTokenSource string

const (
	UsageTokenSourceUnknown    UsageTokenSource = "unknown"
	UsageTokenSourceUpstream   UsageTokenSource = "upstream"
	UsageTokenSourceUnreported UsageTokenSource = "unreported"
	UsageTokenSourceEstimated  UsageTokenSource = "estimated"
)

func (s UsageTokenSource) Normalize() UsageTokenSource {
	switch s {
	case UsageTokenSourceUpstream, UsageTokenSourceUnreported, UsageTokenSourceEstimated:
		return s
	default:
		return UsageTokenSourceUnknown
	}
}

// HasResponseMetadata includes source-only evidence without inventing a response
// outcome. Such rows use collector_version=0 and must not imply an empty answer.
func (u *UsageLog) HasResponseMetadata() bool {
	return u != nil && (u.Outcome != nil || u.InputTokensSource.Normalize() != UsageTokenSourceUnknown || u.OutputTokensSource.Normalize() != UsageTokenSourceUnknown)
}

// recordedOutputTokensSource mirrors recordedOutputTokens' selection, not a
// comparison of numeric values: an estimate may happen to equal a measurement.
func recordedOutputTokensSource(result *ForwardResult) UsageTokenSource {
	if result == nil {
		return UsageTokenSourceUnknown
	}
	providerSource := result.OutputTokensSource.Normalize()
	if result.RequireMeasuredUsage {
		return providerSource
	}
	if providerSource == UsageTokenSourceUnknown {
		return providerSource
	}
	if result.Usage.OutputTokens > 0 && (upstreamUsageComplete(result.Outcome) || (!result.Stream && !result.ClientDisconnect)) {
		return providerSource
	}
	if result.DeliveredOutputTokens != nil && (result.Outcome != nil || result.ClientDisconnect || result.Usage.OutputTokens <= 0) {
		return UsageTokenSourceEstimated
	}
	return providerSource
}
