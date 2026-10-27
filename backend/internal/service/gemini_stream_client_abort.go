package service

import (
	"context"
	"errors"
	"io"

	"github.com/gin-gonic/gin"
)

// geminiStreamAbortedWithBillableOutput preserves the existing completed/image
// cancellation return shape. Collected usage is provider evidence, not proof of
// customer delivery; handlers must apply the measured-usage settlement policy.
func geminiStreamAbortedWithBillableOutput(c *gin.Context, err error, usage *ClaudeUsage) bool {
	return geminiStreamClientDisconnected(c, err) && (usage.hasObservedTokens() || observedGeminiImageOutputs(c) > 0)
}

func geminiStreamClientDisconnected(c *gin.Context, err error) bool {
	if err == nil || errors.Is(err, io.EOF) {
		return false
	}
	return errors.Is(err, context.Canceled) ||
		(c != nil && c.Request != nil && c.Request.Context().Err() != nil)
}

func markGeminiInterruptedStream(c *gin.Context, err error, clientDisconnected bool) {
	if err == nil || clientDisconnected || geminiStreamClientDisconnected(c, err) {
		return
	}
	var failover *UpstreamFailoverError
	if errors.As(err, &failover) {
		return
	}
	MarkOpsStreamFailure(c, "upstream_error", "incomplete_stream", "Gemini upstream stream ended before completion", 502)
}

func finishGeminiUpstreamAttempt(attempt *GeminiUpstreamAttempt, result *ForwardResult, err error) {
	if attempt != nil && result != nil {
		result.UsageLedgerRequestID = attempt.ledgerRequestID
		result.UsageLedgerIdentityHash = attempt.ledgerIdentityHash
		result.GeminiUsageFallback = attempt.snapshotUsageFallback()
		applyGeminiMeasuredUsageEvidence(result, attempt.event.UsageFields)
	}
	if err == nil && result != nil && result.ClientDisconnect {
		attempt.EndClientCanceled()
		return
	}
	attempt.End(err)
}
