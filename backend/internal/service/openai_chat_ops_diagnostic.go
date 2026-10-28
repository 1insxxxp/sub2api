package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

const openAIChatOpsAttemptSequenceKey = "openai_chat_ops_attempt_sequence"

// recordOpsDiagnostic retains failed inference evidence independently of usage
// settlement. It only attaches scalar diagnostics to the request-local Ops
// context; it never writes a response, schedules billing, or changes failover.
func (capture *openAIChatUsageCapture) recordOpsDiagnostic(account *Account, stream bool, result *OpenAIForwardResult, forwardErr error) {
	if capture == nil || capture.opsRecorded || capture.opsContext == nil || capture.current == nil || account == nil || capture.current.AccountID != account.ID {
		return
	}
	// Evidence may outlive Gin's request in the usage worker. Never retain a
	// pooled Gin context through the settlement snapshot after this callback.
	defer func() { capture.opsContext = nil }()
	// A protocol fallback can return a successful result collected by its own
	// adapter. Do not turn the abandoned raw attempt into a new visible failure.
	if result != nil && result.ChatUsage != &capture.evidence {
		return
	}
	capture.opsRecorded = true
	capture.finishRound()
	c, round := capture.opsContext, capture.current
	outcome := capture.outcome.Snapshot()
	state := ClassifyUsageRequestStatus(&outcome)
	if forwardErr == nil && state != UsageRequestInterrupted && state != UsageRequestFailed {
		return
	}
	clientCanceled := outcome.DisconnectSource == DisconnectSourceClient
	serverFailure := outcome.DisconnectSource == DisconnectSourceServer
	events := geminiUpstreamAttemptEvents(c)
	var event *OpsUpstreamErrorEvent
	// Only decorate this actual round. An earlier retry on the same account must
	// retain its own failure, even if the final round was canceled by the client.
	for i := len(events) - 1; i >= capture.opsFirstEvent; i-- {
		if events[i] != nil && events[i].AccountID == account.ID && events[i].Attempt == 0 {
			event = events[i]
			break
		}
	}
	if event == nil {
		proxyID, proxyName := opsUpstreamProxyAttribution(account)
		fresh := OpsUpstreamErrorEvent{AccountID: account.ID, AccountName: account.Name, Platform: account.Platform,
			ProxyID: proxyID, ProxyName: proxyName, Kind: "request_error", Message: "Upstream inference interrupted"}
		if clientCanceled {
			fresh.Kind, fresh.Reason, fresh.Message = "client_cancel", "client_canceled", "Client connection closed before the final response was delivered"
		} else if serverFailure {
			fresh.Message = "Gateway interrupted response delivery"
		}
		appendOpsUpstreamError(c, fresh)
		events = geminiUpstreamAttemptEvents(c)
		event = events[len(events)-1]
	} else if clientCanceled && event.UpstreamStatusCode == 0 && errors.Is(forwardErr, context.Canceled) {
		// The transport helper can record a detached-context cancellation as a
		// request error. Confirmed downstream cancellation is not a provider fault.
		event.Kind, event.Reason, event.Message = "client_cancel", "client_canceled", "Client connection closed before the final response was delivered"
	}
	event.Stage, event.Attempt = "inference", capture.opsAttempt
	event.Model, event.Stream = truncateString(strings.TrimSpace(round.UpstreamModel), 128), stream
	event.DurationMs = time.Since(capture.opsStarted).Milliseconds()
	event.ClientCanceled = clientCanceled
	event.DownstreamStatusCode = c.Writer.Status()
	event.UpstreamStatusCode = round.Outcome.UpstreamStatus
	if round.UpstreamRequestID != "" {
		event.UpstreamRequestID = round.UpstreamRequestID
	}
	event.UsageState = "unreported"
	for index, present := range []bool{round.InputReported, round.OutputReported, round.ReasoningReported, round.CacheReadReported, round.TotalReported} {
		if present {
			event.UsageFields |= 1 << index
			event.UsageState = "observed"
		}
	}
	event.InputTokens, event.OutputTokens = int64(round.Usage.InputTokens), int64(round.Usage.OutputTokens)
	event.CacheReadTokens, event.ThoughtTokens, event.TotalTokens = int64(round.Usage.CacheReadInputTokens), int64(round.ReasoningTokens), int64(round.TotalTokens)
	event.EstimatedReceivedOutputTokens = round.ReceivedOutputTokens
	event.DownstreamDelivery = &OpsDownstreamDelivery{
		AnswerTextDelivered: round.Outcome.HasText, ToolCallDelivered: round.Outcome.HasToolCall,
		ReasoningDelivered: round.Outcome.HasReasoning, MediaDelivered: round.Outcome.HasMedia,
		TerminalDelivered: round.Outcome.StreamCompleted,
	}
	if serverFailure {
		event.Scope, event.Message = "gateway", "Gateway interrupted response delivery"
	}
	if result == nil || clientCanceled || c.Writer.Status() >= http.StatusBadRequest {
		return
	}
	errType, message := "upstream_error", "Upstream inference interrupted"
	if serverFailure {
		errType, message = "api_error", "Gateway interrupted response delivery"
	}
	// Existing explicit markers win. A logical failure can have an HTTP 200
	// wire response; the observed upstream HTTP status remains unchanged above.
	MarkOpsStreamErrorValue(c, OpsStreamError{ErrType: errType, Message: message,
		IntendedStatus: http.StatusBadGateway, CountTowardsSLA: true, NonStream: !stream})
}
