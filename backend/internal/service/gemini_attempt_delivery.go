package service

import (
	"time"

	"github.com/gin-gonic/gin"
)

// OpsDownstreamDelivery contains evidence from successful downstream writes.
// It must never retain text, reasoning, tool arguments, credentials or prompts.
// SettlementDecision describes the handler's decision, not a confirmed debit.
type OpsDownstreamDelivery struct {
	AnswerTextDelivered   bool   `json:"answer_text_delivered"`
	TerminalDelivered     bool   `json:"terminal_delivered"`
	EstimatedOutputTokens int    `json:"estimated_output_tokens"`
	SettlementDecision    string `json:"settlement_decision"`
}

func (d *OpsDownstreamDelivery) sanitizedCopy() *OpsDownstreamDelivery {
	if d == nil {
		return nil
	}
	copy := *d
	copy.EstimatedOutputTokens = max(copy.EstimatedOutputTokens, 0)
	switch copy.SettlementDecision {
	case "skipped", "partial_output", "upstream_usage", "pending_review", "recover_usage", "estimate_usage":
	default:
		copy.SettlementDecision = ""
	}
	return &copy
}

// GeminiUpstreamAttemptCount is a request-local marker taken before forwarding.
// A later diagnostic must belong to a new inference call, not a prior retry.
func GeminiUpstreamAttemptCount(c *gin.Context) int {
	if c == nil {
		return 0
	}
	return c.GetInt(geminiUpstreamAttemptSequenceKey)
}

// RecordGeminiAttemptDelivery supplements the existing diagnostic event even
// when a handler skips settlement. It neither creates usage rows nor schedules
// billing, and cannot overwrite a different attempt's delivery evidence.
func RecordGeminiAttemptDelivery(c *gin.Context, account *Account, collector *DownstreamOutputTokenCollector, result *ForwardResult, billablePartial bool, afterAttempt int) {
	if c == nil || account == nil || account.Platform != PlatformGemini {
		return
	}
	value, ok := c.Get(geminiUpstreamAttemptKey)
	if !ok {
		return
	}
	attempt, ok := value.(*GeminiUpstreamAttempt)
	if !ok || attempt == nil || attempt.event.AccountID != account.ID || attempt.event.Attempt <= afterAttempt {
		return
	}
	target := attempt.deliveryEvent(collector)
	if target == nil {
		return
	}
	decision := "skipped"
	if NeedsUsageMeasurementReview(result) {
		decision = "estimate_usage"
		if result.UsageLedgerRequestID != "" && result.UsageLedgerIdentityHash != "" {
			decision = "recover_usage"
		}
	} else if result != nil && result.RequireMeasuredUsage {
		decision = "upstream_usage"
	} else if billablePartial {
		decision = "partial_output"
	}
	target.DownstreamDelivery = &OpsDownstreamDelivery{
		AnswerTextDelivered:   collector.HasDeliveredText(),
		TerminalDelivered:     collector.HasDeliveredTerminal(),
		EstimatedOutputTokens: collector.TokenCount(),
		SettlementDecision:    decision,
	}
}

// deliveryEvent resolves one real inference attempt and retains a late failed
// final write, which some adapters discover only after returning End(nil).
func (a *GeminiUpstreamAttempt) deliveryEvent(collector *DownstreamOutputTokenCollector) *OpsUpstreamErrorEvent {
	events := geminiUpstreamAttemptEvents(a.c)
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event != nil && event.AccountID == a.event.AccountID && event.Attempt == a.event.Attempt {
			return event
		}
	}
	if collector == nil || !collector.Frozen() || collector.HasDeliveredTerminal() {
		return nil
	}
	event := a.event
	event.ClientCanceled = true
	event.Kind, event.Reason = "client_cancel", "client_canceled"
	event.DurationMs = time.Since(a.started).Milliseconds()
	event.Message = "Client connection closed before the final response was delivered"
	appendOpsUpstreamError(a.c, event)
	events = geminiUpstreamAttemptEvents(a.c)
	return events[len(events)-1]
}
