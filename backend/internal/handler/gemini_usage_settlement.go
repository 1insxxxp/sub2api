package handler

import (
	"context"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// recoverCanceledGeminiUsage retains only the current real inference attempt.
// A retry/preflight failure must never reuse another account's measurements.
func recoverCanceledGeminiUsage(c *gin.Context, account *service.Account, result *service.ForwardResult, err error, collector *service.DownstreamOutputTokenCollector, model string, stream bool, afterAttempt int) *service.ForwardResult {
	if result != nil || account == nil || account.Platform != service.PlatformGemini {
		return result
	}
	var failover *service.UpstreamFailoverError
	if errors.As(err, &failover) {
		return nil
	}
	canceled := errors.Is(err, context.Canceled) || (c.Request != nil && c.Request.Context().Err() != nil)
	if !canceled && (collector == nil || !collector.Frozen()) {
		return nil
	}
	return service.GeminiResultFromLastAttempt(c, account.ID, model, stream, afterAttempt)
}

// Interrupted measured Gemini calls must be retained even without delivered
// answer text. The shared recording path resolves measured usage before fallback.
func geminiInterruptedUsageRecordable(result *service.ForwardResult, err error, billablePartial bool) bool {
	var failover *service.UpstreamFailoverError
	if errors.As(err, &failover) || result == nil {
		return false
	}
	return result.RequireMeasuredUsage || billablePartial
}

func recordGatewayUsage(ctx context.Context, gateway *service.GatewayService, input *service.RecordUsageInput) error {
	if service.NeedsUsageMeasurementReview(input.Result) {
		_, err := gateway.RecoverGeminiUsage(ctx, input)
		if err != nil {
			// Recovery errors contain fixed messages only; never log ledger bodies
			// or upstream credentials. A failed lookup still retains the request.
			logger.FromContext(ctx).Warn("Gemini usage ledger unavailable; resolving final fallback",
				zap.Int64("account_id", input.Account.ID), zap.Error(err))
		}
	}
	gateway.FinalizeGeminiUsageFallback(ctx, input)
	return gateway.RecordUsage(ctx, input)
}
