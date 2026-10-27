package service

import (
	"context"
	"log/slog"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// prepareAnthropicInputTokenEstimate snapshots only a scalar estimate of the
// final upstream request, so asynchronous settlement never retains the prompt.
// Provider-reported input/cache usage always takes precedence.
func prepareAnthropicInputTokenEstimate(result *ForwardResult, body []byte) {
	if result == nil {
		return
	}
	result.EstimatedInputTokens = 0
	if !anthropicInputUsageMissing(result.Usage) || len(body) == 0 {
		return
	}
	if estimated, err := estimateAnthropicCountTokensLocally(body); err == nil && estimated > 0 {
		result.EstimatedInputTokens = estimated
	}
}

func anthropicInputUsageMissing(usage ClaudeUsage) bool {
	return usage.InputTokens == 0 && usage.CacheReadInputTokens == 0 &&
		usage.CacheCreationInputTokens == 0 && usage.CacheCreation5mTokens == 0 &&
		usage.CacheCreation1hTokens == 0
}

// applyMissingInputTokenEstimate repairs missing input accounting only after
// generation has been observed. Gemini uses its separate ledger/count/semantic
// fallback path; this legacy character-based fallback must not overwrite it.
func applyMissingInputTokenEstimate(ctx context.Context, result *ForwardResult, account *Account) {
	if result == nil || !anthropicInputUsageMissing(result.Usage) {
		return
	}
	hasGeneration := result.Usage.OutputTokens > 0 ||
		(result.DeliveredOutputTokens != nil && *result.DeliveredOutputTokens > 0) ||
		(result.Outcome != nil && result.Outcome.HasEffectiveOutput())
	if !hasGeneration || skipGeminiInputTokenEstimate(ctx, result, account) || result.EstimatedInputTokens <= 0 {
		return
	}
	result.Usage.InputTokens = result.EstimatedInputTokens
	result.InputTokensEstimated = true
	accountID := int64(0)
	if account != nil {
		accountID = account.ID
	}
	if ctx == nil {
		ctx = context.Background()
	}
	slog.InfoContext(ctx, "billing.input_tokens_estimated",
		"request_id", result.RequestID,
		"account_id", accountID,
		"model", result.Model,
		"input_tokens", result.EstimatedInputTokens,
		"reason", "missing_upstream_input_usage")
}

func skipGeminiInputTokenEstimate(ctx context.Context, result *ForwardResult, account *Account) bool {
	if account == nil || account.Platform != PlatformGemini {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	requestID, _ := ctx.Value(ctxkey.RequestID).(string)
	clientRequestID, _ := ctx.Value(ctxkey.ClientRequestID).(string)
	slog.DebugContext(ctx, "billing.gemini_input_estimate_skipped",
		"request_id", requestID,
		"client_request_id", clientRequestID,
		"upstream_request_id", result.RequestID,
		"account_id", account.ID,
		"model", result.Model,
		"reason", "gemini_fallback_resolved_separately")
	return true
}
