package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

var ErrOpenAIChatUsageOutOfRange = errors.New("chat usage token count is outside the supported range")

// FinalizeOpenAIChatUsage resolves usage for each raw Chat Completions round.
// Financial effects remain in RecordUsage and its existing request dedup.
func (s *OpenAIGatewayService) FinalizeOpenAIChatUsage(ctx context.Context, input *OpenAIRecordUsageInput) {
	if input == nil || input.Result == nil || input.Result.ChatUsage == nil {
		return
	}
	result, evidence := input.Result, input.Result.ChatUsage
	evidence.HadDeliveredOutput = evidence.HadDeliveredOutput || hasDeliveredUsageOutput(result.DeliveredOutputTokens)
	defer func() {
		for _, round := range evidence.Rounds {
			if round != nil && round.InputFallback != nil {
				round.InputFallback.CountRequest = nil
			}
		}
	}()
	if evidence.Finalized {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// All continuation rounds share one bounded lookup budget. The usage task's
	// deadline must retain time for the original wallet/dedup/persistence work.
	budget := usageFollowupBudget(ctx, geminiUsageRecoveryTimeout)
	if budget > 0 && ctx.Err() == nil {
		lookupCtx, cancel := context.WithTimeout(ctx, budget)
		if err := s.recoverOpenAIChatLedger(lookupCtx, input.Account, evidence.Rounds); err != nil {
			// Recovery errors are fixed strings, never upstream bodies or URLs.
			logger.L().Debug("billing.openai_chat_ledger_unavailable", zap.Error(err))
		}
		cancel()
	}
	total := evidence.ExtraUsage
	inputSource, outputSource := UsageTokenSourceUnknown, UsageTokenSourceUnknown
	if openAIChatUsageHasTokens(evidence.ExtraUsage) {
		inputSource, outputSource = UsageTokenSourceUpstream, UsageTokenSourceUpstream
	}
	for _, round := range evidence.Rounds {
		if round == nil {
			continue
		}
		finalizeOpenAIChatRound(round)
		addOpenAIChatRoundUsage(&total, round.Usage)
		// A rejected continuation without any consumption evidence contributes
		// no unknown usage to an earlier measured round. Its failure outcome is
		// retained separately; accepted or partially measured rounds still merge.
		if !openAIChatRoundHasUsageEvidence(round) {
			continue
		}
		inputSource = mergeOpenAIChatTokenSource(inputSource, round.InputTokensSource)
		outputSource = mergeOpenAIChatTokenSource(outputSource, round.OutputTokensSource)
	}
	if len(evidence.Rounds) > 0 {
		if inputSource == UsageTokenSourceUnknown {
			inputSource = UsageTokenSourceUnreported
		}
		if outputSource == UsageTokenSourceUnknown {
			outputSource = UsageTokenSourceUnreported
		}
	}
	result.Usage = total
	result.InputTokensSource, result.OutputTokensSource = inputSource, outputSource
	// These totals already select provider measurements or received semantics
	// per round. A downstream-write estimate may not replace them on cancellation.
	result.DeliveredOutputTokens = nil
	evidence.Finalized = true
	if inputSource != UsageTokenSourceUpstream || outputSource != UsageTokenSourceUpstream {
		logger.L().Info("billing.openai_chat_usage_fallback",
			zap.String("request_id", result.RequestID),
			zap.String("input_source", string(inputSource)), zap.String("output_source", string(outputSource)),
			zap.Int("input_tokens", total.InputTokens), zap.Int("output_tokens", total.OutputTokens),
			zap.Int("rounds", len(evidence.Rounds)))
	}
}

func openAIChatRoundHasUsageEvidence(round *OpenAIChatUsageRound) bool {
	return round.UpstreamAccepted || round.InputReported || round.OutputReported ||
		round.CacheReadReported || round.CacheWriteReported || round.ReasoningReported || round.TotalReported ||
		openAIChatUsageHasTokens(round.Usage) || round.ReceivedOutputTokens > 0 || round.ReceivedReasoningTokens > 0 ||
		round.OutputTruncated || round.Outcome.HasEffectiveOutput()
}

func finalizeOpenAIChatRound(round *OpenAIChatUsageRound) {
	fallback := round.InputFallback
	if round.InputReported {
		round.InputTokensSource = UsageTokenSourceUpstream
		if fallback != nil && fallback.Known && !fallback.LocalTokensExcludeCachedContent {
			geminiObserveInputCalibration(fallback.CalibrationKey, fallback.Tokens, round.Usage.InputTokens)
		}
	} else {
		round.Usage.InputTokens = 0
		round.InputTokensSource = UsageTokenSourceUnreported
		if round.UpstreamAccepted && fallback != nil && fallback.Known {
			gross := max(0, fallback.Tokens)
			if calibrated, ok := geminiCalibratedInputEstimate(fallback.CalibrationKey, gross); ok {
				gross = calibrated
			}
			if fallback.LocalTokensExcludeCachedContent {
				gross += max(0, round.Usage.CacheReadInputTokens) + max(0, round.Usage.CacheCreationInputTokens)
			}
			round.Usage.InputTokens = gross
			round.InputTokensSource = UsageTokenSourceEstimated
		}
	}
	if round.OutputReported {
		round.OutputTokensSource = UsageTokenSourceUpstream
	} else {
		round.Usage.OutputTokens = 0
		round.OutputTokensSource = UsageTokenSourceUnreported
		if round.UpstreamAccepted {
			output := max(0, round.ReceivedOutputTokens)
			if round.ReasoningReported {
				output = max(0, output-round.ReceivedReasoningTokens) + max(0, round.ReasoningTokens)
			}
			round.Usage.OutputTokens = output
			round.OutputTokensSource = UsageTokenSourceEstimated
		} else if round.ReasoningReported {
			// A reported reasoning count is real consumption even if the provider
			// subsequently failed before returning a complete output counter.
			round.Usage.OutputTokens = max(0, round.ReasoningTokens)
		}
	}
}

func (s *OpenAIGatewayService) recoverOpenAIChatLedger(ctx context.Context, account *Account, rounds []*OpenAIChatUsageRound) error {
	if s == nil || s.httpUpstream == nil || account == nil || account.Type != AccountTypeAPIKey {
		return nil
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	targetURL, err := s.rawChatCompletionsURL(account)
	if err != nil {
		return ErrGeminiUsageRecoveryUnavailable
	}
	identity := GeminiUsageLedgerIdentity(targetURL, apiKey)
	if identity == "" {
		return nil
	}
	pending := make([]*OpenAIChatUsageRound, 0, len(rounds))
	for _, round := range rounds {
		if round == nil || (round.InputReported && round.OutputReported) || round.AccountID != account.ID ||
			round.IdentityHash != identity || round.UpstreamRequestID == "" || len(round.UpstreamRequestID) > maxUsageUpstreamRequestIDLen ||
			strings.ContainsAny(round.UpstreamRequestID, "\r\n\t") || strings.TrimSpace(round.UpstreamModel) == "" {
			continue
		}
		pending = append(pending, round)
	}
	if len(pending) == 0 {
		return nil
	}
	origin, _ := geminiUsageLedgerOrigin(targetURL)
	normalized, err := s.validateUpstreamBaseURL(origin)
	if err != nil {
		return ErrGeminiUsageRecoveryUnavailable
	}
	proxyURL := ""
	if account.ProxyID != nil {
		if account.Proxy == nil || account.Proxy.ID != *account.ProxyID {
			return ErrGeminiUsageRecoveryIdentityChanged
		}
		proxyURL = account.Proxy.URL()
	}
	endpoint := strings.TrimRight(normalized, "/") + "/api/log/token"
	proxyHash := sha256.Sum256([]byte(proxyURL))
	fetchKey := fmt.Sprintf("openai-chat:%p:%s:%x", s, identity, proxyHash)
	transport := &GatewayService{httpUpstream: s.httpUpstream}
	for _, delay := range []time.Duration{0, 250 * time.Millisecond, 750 * time.Millisecond, 1500 * time.Millisecond} {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ErrGeminiUsageRecoveryUnavailable
			case <-timer.C:
			}
		}
		if ctx.Err() != nil {
			return ErrGeminiUsageRecoveryUnavailable
		}
		fetch := geminiUsageLedgerFetchGroup.DoChan(fetchKey, func() (any, error) {
			return transport.fetchGeminiUsageLedger(ctx, endpoint, apiKey, proxyURL, account)
		})
		var raw []byte
		select {
		case <-ctx.Done():
			return ErrGeminiUsageRecoveryUnavailable
		case fetched := <-fetch:
			if fetched.Err != nil {
				return ErrGeminiUsageRecoveryUnavailable
			}
			raw, _ = fetched.Val.([]byte)
		}
		remaining := pending[:0]
		for _, round := range pending {
			usage, parseErr := lookupGeminiLedgerUsage(raw, round.UpstreamRequestID, round.UpstreamModel)
			if parseErr != nil {
				// An ambiguous or malformed match is never retried or substituted
				// with another row. Other independently matched rounds remain valid.
				continue
			}
			if usage == nil {
				remaining = append(remaining, round)
				continue
			}
			applyOpenAIChatLedgerUsage(round, usage)
		}
		pending = remaining
		if len(pending) == 0 {
			return nil
		}
	}
	return nil
}

func applyOpenAIChatLedgerUsage(round *OpenAIChatUsageRound, usage *ClaudeUsage) {
	// The parser returns mutually exclusive input/cache buckets; OpenAIUsage
	// expects gross input and separates cache exactly once during RecordUsage.
	gross := usage.InputTokens + usage.CacheReadInputTokens
	if !round.InputReported && round.Usage.CacheCreationInputTokens == 0 {
		round.Usage.InputTokens, round.InputReported = gross, true
	}
	if !round.OutputReported {
		round.Usage.OutputTokens, round.OutputReported = usage.OutputTokens, true
	}
	if !round.CacheReadReported && usage.CacheReadInputTokens <= max(0, round.Usage.InputTokens-round.Usage.CacheCreationInputTokens) {
		round.Usage.CacheReadInputTokens, round.CacheReadReported = usage.CacheReadInputTokens, true
	}
	// A cache-write counter has no verified representation in this ledger
	// contract. Preserve the response measurement and never invent a zero for it.
}

func addOpenAIChatRoundUsage(total *OpenAIUsage, usage OpenAIUsage) {
	total.InputTokens += usage.InputTokens
	total.OutputTokens += usage.OutputTokens
	total.CacheReadInputTokens += usage.CacheReadInputTokens
	total.CacheCreationInputTokens += usage.CacheCreationInputTokens
	total.ImageInputTokens += usage.ImageInputTokens
	total.ImageCacheReadTokens += usage.ImageCacheReadTokens
	total.ImageOutputTokens += usage.ImageOutputTokens
}

func mergeOpenAIChatTokenSource(current, next UsageTokenSource) UsageTokenSource {
	if current == UsageTokenSourceUnknown {
		return next.Normalize()
	}
	if current == UsageTokenSourceEstimated || next == UsageTokenSourceEstimated {
		return UsageTokenSourceEstimated
	}
	if current == UsageTokenSourceUnreported || next == UsageTokenSourceUnreported {
		return UsageTokenSourceUnreported
	}
	return UsageTokenSourceUpstream
}

func openAIChatUsageNeverAccepted(result *OpenAIForwardResult) bool {
	if result == nil || result.ChatUsage == nil || !result.ChatUsage.Finalized {
		return false
	}
	if openAIChatUsageHasTokens(result.Usage) {
		return false
	}
	for _, round := range result.ChatUsage.Rounds {
		if round != nil && round.UpstreamAccepted {
			return false
		}
	}
	return true
}

func openAIChatUsageHasTokens(usage OpenAIUsage) bool {
	return usage.InputTokens > 0 || usage.OutputTokens > 0 || usage.CacheReadInputTokens > 0 ||
		usage.CacheCreationInputTokens > 0 || usage.ImageInputTokens > 0 || usage.ImageCacheReadTokens > 0 || usage.ImageOutputTokens > 0
}

// Validate before any wallet/dedup/outbox write: INTEGER columns cannot store an
// otherwise finite multi-round sum above MaxInt32. Also check the original
// rounds with bounded additions so a machine-int overflow cannot hide the error.
func validateOpenAIChatUsageAggregate(result *OpenAIForwardResult) error {
	if result == nil || result.ChatUsage == nil {
		return nil
	}
	values := func(usage OpenAIUsage) [7]int {
		return [7]int{usage.InputTokens, usage.OutputTokens, usage.CacheReadInputTokens,
			usage.CacheCreationInputTokens, usage.ImageInputTokens, usage.ImageCacheReadTokens, usage.ImageOutputTokens}
	}
	for _, value := range values(result.Usage) {
		if value < 0 || value > math.MaxInt32 {
			return ErrOpenAIChatUsageOutOfRange
		}
	}
	totals := [7]int{}
	check := func(usage OpenAIUsage) error {
		for i, value := range values(usage) {
			if value < 0 || value > math.MaxInt32-totals[i] {
				return ErrOpenAIChatUsageOutOfRange
			}
			totals[i] += value
		}
		return nil
	}
	if err := check(result.ChatUsage.ExtraUsage); err != nil {
		return err
	}
	for _, round := range result.ChatUsage.Rounds {
		if round != nil {
			if err := check(round.Usage); err != nil {
				return err
			}
		}
	}
	return nil
}

// Cache is exclusive within each upstream call. A missing prompt estimate that
// is smaller than its reported cache may not offset another round's input.
func openAIChatUncachedInputTokens(result *OpenAIForwardResult) (int, bool) {
	if result == nil || result.ChatUsage == nil || !result.ChatUsage.Finalized {
		return 0, false
	}
	uncached := func(usage OpenAIUsage) int {
		return max(0, usage.InputTokens-usage.CacheReadInputTokens-usage.CacheCreationInputTokens)
	}
	total := uncached(result.ChatUsage.ExtraUsage)
	for _, round := range result.ChatUsage.Rounds {
		if round != nil {
			total += uncached(round.Usage)
		}
	}
	return total, true
}

func openAIChatUsageUpstreamRequestID(account *Account, result *OpenAIForwardResult) *string {
	if result.ChatUsage != nil {
		for i := len(result.ChatUsage.Rounds) - 1; i >= 0; i-- {
			round := result.ChatUsage.Rounds[i]
			if round != nil && round.IdentityHash != "" && round.UpstreamRequestID != "" && len(round.UpstreamRequestID) <= maxUsageUpstreamRequestIDLen {
				return optionalTrimmedStringPtr(round.UpstreamRequestID)
			}
		}
	}
	return usageUpstreamRequestIDPtr(account, result.UpstreamHeaders, result.OpenAIWSMode)
}
