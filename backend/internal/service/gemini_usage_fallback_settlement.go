package service

import (
	"context"
	"log/slog"
)

// FinalizeGeminiUsageFallback runs after the exact-ledger recovery attempt. It
// never changes a reported zero or labels an estimate as a provider measurement.
// Financial effects remain exclusively in RecordUsage and its original dedup.
func (s *GatewayService) FinalizeGeminiUsageFallback(ctx context.Context, input *RecordUsageInput) {
	if input == nil || input.Result == nil || !input.Result.RequireMeasuredUsage {
		return
	}
	result := input.Result
	evidence := result.GeminiUsageFallback
	if evidence != nil && evidence.Input != nil {
		defer func() { evidence.Input.CountRequest = nil }()
	}
	if result.GeminiUsageFinalized {
		return
	}
	if result.MeasuredUsageComplete {
		if evidence != nil && evidence.Input != nil && result.InputTokensSource == UsageTokenSourceUpstream {
			geminiObserveInputCalibration(evidence.Input.CalibrationKey, evidence.Input.Tokens,
				result.Usage.InputTokens+result.Usage.CacheReadInputTokens)
		}
		result.GeminiUsageFinalized = true
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	inputMethod, outputMethod := "provider_usage", "provider_usage"
	accepted := evidence != nil && evidence.UpstreamAccepted
	if result.InputTokensSource != UsageTokenSourceUpstream {
		gross, inputMethodValue := 0, "no_upstream_acceptance"
		includesCache := true
		if accepted && evidence.Input != nil {
			count, ok, err := s.CountGeminiInputFallback(ctx, input)
			if err != nil {
				// Count helper errors are fixed messages; raw provider responses and
				// credentials must never enter settlement diagnostics.
				slog.DebugContext(ctx, "billing.gemini_count_fallback_unavailable", "error", err)
			}
			if ok {
				gross, inputMethodValue = count, "provider_count"
			} else if evidence.Input.Known {
				gross, inputMethodValue = evidence.Input.Tokens, "semantic_tokenizer"
				includesCache = !evidence.Input.LocalTokensExcludeCachedContent
				if calibrated, ok := geminiCalibratedInputEstimate(evidence.Input.CalibrationKey, gross); ok {
					gross, inputMethodValue = calibrated, "validated_calibration"
				}
			} else {
				inputMethodValue = "unavailable_evidence"
			}
		}
		// Cache measurements are already separate in ClaudeUsage. Never invent
		// a discount or subtract the same cached input twice.
		result.Usage.InputTokens = max(0, gross)
		if includesCache {
			result.Usage.InputTokens = max(0, gross-result.Usage.CacheReadInputTokens)
		}
		result.InputTokensSource = UsageTokenSourceEstimated
		result.InputTokensEstimated = true
		result.EstimatedInputTokens = result.Usage.InputTokens
		inputMethod = inputMethodValue
	}
	if result.OutputTokensSource != UsageTokenSourceUpstream {
		output := 0
		outputMethod = "no_upstream_acceptance"
		if accepted {
			output = max(0, evidence.CandidateTokens)
			if evidence.UsageFields&4 != 0 {
				output += max(0, evidence.MeasuredThoughts)
			} else {
				output += max(0, evidence.ThoughtTokens)
			}
			outputMethod = "received_semantic_output"
			if evidence.OutputTruncated {
				outputMethod = "received_semantic_prefix_limit"
			}
		}
		result.Usage.OutputTokens = output
		result.OutputTokensSource = UsageTokenSourceEstimated
	}
	result.GeminiUsageFinalized = true
	accountID := int64(0)
	if input.Account != nil {
		accountID = input.Account.ID
	}
	slog.InfoContext(ctx, "billing.gemini_usage_fallback",
		"account_id", accountID, "request_id", result.RequestID,
		"input_method", inputMethod, "output_method", outputMethod,
		"input_tokens", result.Usage.InputTokens, "output_tokens", result.Usage.OutputTokens)
}
