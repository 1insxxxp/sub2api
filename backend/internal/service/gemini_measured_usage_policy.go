package service

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ErrUsageMeasurementIncomplete prevents unresolved Gemini text measurements
// from entering wallet/dedup settlement before recovery and final fallback.
var ErrUsageMeasurementIncomplete = errors.New("upstream usage measurement incomplete")

// NeedsUsageMeasurementReview distinguishes missing evidence from an explicit
// measured zero. A finalized fallback is resolved without claiming measured
// completeness. It applies to normal and interrupted Gemini text responses.
func NeedsUsageMeasurementReview(result *ForwardResult) bool {
	return result != nil && result.RequireMeasuredUsage && !result.MeasuredUsageComplete && !result.GeminiUsageFinalized
}

func applyGeminiMeasuredUsageEvidence(result *ForwardResult, fields uint8) {
	if result == nil {
		return
	}
	result.InputTokensSource = UsageTokenSourceUnreported
	result.OutputTokensSource = UsageTokenSourceUnreported
	if fields&1 != 0 {
		result.InputTokensSource = UsageTokenSourceUpstream
	}
	// A thought-only observation is a partial component, not an exact total.
	if fields&2 != 0 {
		result.OutputTokensSource = UsageTokenSourceUpstream
	}
	// Keep the existing image/audio billing policy outside this text-only change,
	// including an image model canceled before its first generated image.
	result.RequireMeasuredUsage = result.ImageCount == 0 && result.Usage.ImageOutputTokens == 0 &&
		result.AudioUsage == nil && (result.Outcome == nil || !result.Outcome.HasMedia) &&
		!isImageGenerationModel(result.Model) && !isImageGenerationModel(result.UpstreamModel) &&
		!isGeminiImageOutputModel(result.Model) && !isGeminiImageOutputModel(result.UpstreamModel)
	// Candidates must be explicitly present, even for thought-only generation.
	// Optional thoughts are already added by the provider usage accumulator.
	result.MeasuredUsageComplete = fields&(1|2) == 1|2
}

// GeminiResultFromLastAttempt recovers only current in-memory measurements when
// a terminal client cancellation left no ForwardResult. The handler must verify
// cancellation first. Historical Ops records are never replayed into billing.
func GeminiResultFromLastAttempt(c *gin.Context, accountID int64, model string, stream bool, afterAttempt int) *ForwardResult {
	if c == nil || accountID <= 0 || afterAttempt < 0 {
		return nil
	}
	value, ok := c.Get(geminiUpstreamAttemptKey)
	if !ok {
		return nil
	}
	attempt, ok := value.(*GeminiUpstreamAttempt)
	if !ok || attempt == nil || attempt.event.Attempt <= afterAttempt ||
		attempt.event.AccountID != accountID || attempt.event.Platform != PlatformGemini {
		return nil
	}
	// An internal HTTP retry may end before the next request is built. If that
	// build is canceled, do not relabel the previous rejected request as a new
	// canceled inference. Successful End(nil) and confirmed cancellation remain
	// eligible for the downstream collector's later disconnect observation.
	if attempt.ended && !attempt.event.ClientCanceled && !attempt.upstreamAccepted && attempt.event.UpstreamStatusCode >= http.StatusBadRequest {
		return nil
	}
	event := attempt.event
	duration := time.Duration(event.DurationMs) * time.Millisecond
	if duration <= 0 {
		duration = time.Since(attempt.started)
	}
	inputTokens := event.InputTokens - event.CacheReadTokens
	if inputTokens < 0 {
		inputTokens = 0
	}
	result := &ForwardResult{
		RequestID: event.UpstreamRequestID, Model: model, UpstreamModel: event.Model,
		UsageLedgerRequestID:          attempt.ledgerRequestID,
		UsageLedgerIdentityHash:       attempt.ledgerIdentityHash,
		GeminiUsageFallback:           attempt.snapshotUsageFallback(),
		UpstreamResponseModel:         observedUpstreamResponseModel(c),
		UpstreamResponseModelConflict: observedUpstreamResponseModelConflict(c),
		Stream:                        stream, ClientDisconnect: true, Duration: duration,
		Usage: ClaudeUsage{
			InputTokens:          int(inputTokens),
			CacheReadInputTokens: int(event.CacheReadTokens),
			OutputTokens:         int(event.OutputTokens),
		},
		Outcome: &ResponseOutcome{
			HTTPStatus: c.Writer.Status(), UpstreamStatus: event.UpstreamStatusCode,
			DisconnectSource: DisconnectSourceClient, CollectorVersion: 1,
		},
	}
	if attempt.requestIDHeader != "" && attempt.requestIDHeaderValue != "" {
		result.UpstreamHeaders = make(http.Header)
		result.UpstreamHeaders.Set(attempt.requestIDHeader, attempt.requestIDHeaderValue)
	}
	if value, ok := c.Get(OpsTimeToFirstTokenMsKey); ok {
		if ms, ok := value.(int64); ok && ms >= 0 {
			firstToken := int(ms)
			result.FirstTokenMs = &firstToken
		}
	}
	applyGeminiMeasuredUsageEvidence(result, event.UsageFields)
	return result
}
