package service

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

const ResponseOutcomeCollectorVersion = 1

type DisconnectSource string

const (
	DisconnectSourceNone     DisconnectSource = "none"
	DisconnectSourceClient   DisconnectSource = "client"
	DisconnectSourceUpstream DisconnectSource = "upstream"
	DisconnectSourceServer   DisconnectSource = "server"
)

type UpstreamErrorKind string

const (
	UpstreamErrorNone     UpstreamErrorKind = "none"
	UpstreamErrorTimeout  UpstreamErrorKind = "timeout"
	UpstreamErrorHTTP5xx  UpstreamErrorKind = "http_5xx"
	UpstreamErrorProtocol UpstreamErrorKind = "protocol"
	UpstreamErrorOther    UpstreamErrorKind = "other"
)

// ResponseOutcome is a privacy-safe summary of the response delivered for one
// request. It deliberately contains no prompt, response, or tool argument data.
type ResponseOutcome struct {
	HTTPStatus        int               `json:"http_status"`
	UpstreamStatus    int               `json:"upstream_status"`
	HasText           bool              `json:"has_text"`
	HasToolCall       bool              `json:"has_tool_call"`
	HasReasoning      bool              `json:"has_reasoning"`
	HasMedia          bool              `json:"has_media"`
	OutputBytes       int64             `json:"output_bytes"`
	EventCount        int               `json:"event_count"`
	StreamCompleted   bool              `json:"stream_completed"`
	FinishReason      string            `json:"finish_reason"`
	DisconnectSource  DisconnectSource  `json:"disconnect_source"`
	UpstreamErrorKind UpstreamErrorKind `json:"upstream_error_kind"`
	CollectorVersion  int               `json:"collector_version"`
}

// UsageOutcomeStatus is the privacy-safe classification exposed to admin
// usage consumers. It intentionally contains no provider message or endpoint.
type UsageOutcomeStatus string

const (
	UsageOutcomeSuccess UsageOutcomeStatus = "success"
	UsageOutcomeFailure UsageOutcomeStatus = "failure"
	UsageOutcomeEmpty   UsageOutcomeStatus = "empty"
	UsageOutcomeUnknown UsageOutcomeStatus = "unknown"
)

// ClassifyUsageOutcome classifies persisted response evidence without making
// another upstream request. Missing or client-cancelled evidence is unknown so
// the monitor never guesses a model failure from incomplete information.
func ClassifyUsageOutcome(outcome *ResponseOutcome) UsageOutcomeStatus {
	if outcome == nil {
		return UsageOutcomeUnknown
	}
	if outcome.DisconnectSource == DisconnectSourceClient {
		return UsageOutcomeUnknown
	}
	if outcome.HTTPStatus >= 400 || outcome.UpstreamStatus >= 400 ||
		(outcome.UpstreamErrorKind != "" && outcome.UpstreamErrorKind != UpstreamErrorNone) ||
		outcome.DisconnectSource == DisconnectSourceUpstream ||
		outcome.DisconnectSource == DisconnectSourceServer {
		return UsageOutcomeFailure
	}
	if !outcome.HasEffectiveOutput() {
		return UsageOutcomeEmpty
	}
	return UsageOutcomeSuccess
}

func (o ResponseOutcome) HasEffectiveOutput() bool {
	return o.HasText || o.HasToolCall || o.HasReasoning || o.HasMedia
}

// ResponseOutcomeCollector records only structural response signals.
type ResponseOutcomeCollector struct {
	mu      sync.Mutex
	outcome ResponseOutcome
}

type responseOutcomeCollectorContextKey struct{}

func WithResponseOutcomeCollector(ctx context.Context, httpStatus, upstreamStatus int) (context.Context, *ResponseOutcomeCollector) {
	if ctx == nil {
		ctx = context.Background()
	}
	if existing, ok := ResponseOutcomeCollectorFromContext(ctx); ok {
		existing.SetStatuses(httpStatus, upstreamStatus)
		return ctx, existing
	}
	collector := NewResponseOutcomeCollector(httpStatus, upstreamStatus)
	return context.WithValue(ctx, responseOutcomeCollectorContextKey{}, collector), collector
}

func ResponseOutcomeCollectorFromContext(ctx context.Context) (*ResponseOutcomeCollector, bool) {
	if ctx == nil {
		return nil, false
	}
	collector, ok := ctx.Value(responseOutcomeCollectorContextKey{}).(*ResponseOutcomeCollector)
	return collector, ok && collector != nil
}

func ResponseOutcomeSnapshotFromContext(ctx context.Context) *ResponseOutcome {
	collector, ok := ResponseOutcomeCollectorFromContext(ctx)
	if !ok {
		return nil
	}
	snapshot := collector.Snapshot()
	return &snapshot
}

func EnsureResponseOutcomeCollector(ctx context.Context, c *gin.Context, httpStatus, upstreamStatus int) (context.Context, *ResponseOutcomeCollector) {
	if ctx == nil {
		ctx = context.Background()
	}
	if existing, ok := ResponseOutcomeCollectorFromContext(ctx); ok {
		existing.SetStatuses(httpStatus, upstreamStatus)
		if c != nil && c.Request != nil {
			c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), responseOutcomeCollectorContextKey{}, existing))
		}
		return ctx, existing
	}
	if c != nil && c.Request != nil {
		if existing, ok := ResponseOutcomeCollectorFromContext(c.Request.Context()); ok {
			existing.SetStatuses(httpStatus, upstreamStatus)
			return context.WithValue(ctx, responseOutcomeCollectorContextKey{}, existing), existing
		}
	}
	ctx, collector := WithResponseOutcomeCollector(ctx, httpStatus, upstreamStatus)
	if c != nil && c.Request != nil {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), responseOutcomeCollectorContextKey{}, collector))
	}
	return ctx, collector
}

func NewResponseOutcomeCollector(httpStatus, upstreamStatus int) *ResponseOutcomeCollector {
	collector := &ResponseOutcomeCollector{}
	collector.outcome.HTTPStatus = httpStatus
	collector.outcome.UpstreamStatus = upstreamStatus
	collector.outcome.DisconnectSource = DisconnectSourceNone
	collector.outcome.UpstreamErrorKind = UpstreamErrorNone
	collector.outcome.CollectorVersion = ResponseOutcomeCollectorVersion
	if upstreamStatus >= 500 {
		collector.outcome.UpstreamErrorKind = UpstreamErrorHTTP5xx
	}
	return collector
}

func (c *ResponseOutcomeCollector) SetStatuses(httpStatus, upstreamStatus int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	if httpStatus > 0 {
		c.outcome.HTTPStatus = httpStatus
	}
	if upstreamStatus > 0 {
		c.outcome.UpstreamStatus = upstreamStatus
		if upstreamStatus >= 500 {
			c.outcome.UpstreamErrorKind = UpstreamErrorHTTP5xx
		}
	}
	c.mu.Unlock()
}

func (c *ResponseOutcomeCollector) ObserveText(value string) {
	if c == nil || strings.TrimSpace(value) == "" {
		return
	}
	c.mu.Lock()
	c.outcome.HasText = true
	c.mu.Unlock()
}

func (c *ResponseOutcomeCollector) ObserveReasoning(value string) {
	if c == nil || strings.TrimSpace(value) == "" {
		return
	}
	c.mu.Lock()
	c.outcome.HasReasoning = true
	c.mu.Unlock()
}

func (c *ResponseOutcomeCollector) ObserveToolCall() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.outcome.HasToolCall = true
	c.mu.Unlock()
}

func (c *ResponseOutcomeCollector) ObserveMedia() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.outcome.HasMedia = true
	c.mu.Unlock()
}

func (c *ResponseOutcomeCollector) ObserveEvent(outputBytes int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.outcome.EventCount++
	if outputBytes > 0 {
		c.outcome.OutputBytes += int64(outputBytes)
	}
	c.mu.Unlock()
}

func (c *ResponseOutcomeCollector) MarkCompleted(finishReason string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.outcome.StreamCompleted = true
	c.outcome.FinishReason = sanitizeResponseOutcomeFinishReason(finishReason)
	c.outcome.DisconnectSource = DisconnectSourceNone
	c.mu.Unlock()
}

func (c *ResponseOutcomeCollector) ObserveFinishReason(finishReason string) {
	if c == nil || strings.TrimSpace(finishReason) == "" {
		return
	}
	c.mu.Lock()
	c.outcome.FinishReason = sanitizeResponseOutcomeFinishReason(finishReason)
	c.mu.Unlock()
}

// sanitizeResponseOutcomeFinishReason keeps only protocol-level lifecycle
// categories. Upstreams control this value, so persisting it verbatim could
// retain URLs, model output, or other untrusted content in otherwise
// privacy-safe response evidence.
func sanitizeResponseOutcomeFinishReason(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	switch value {
	case "stop", "length", "tool_calls", "function_call", "content_filter",
		"end_turn", "max_tokens", "stop_sequence", "tool_use", "pause_turn", "refusal",
		"done", "completed", "incomplete", "failed", "cancelled", "canceled",
		"message_stop", "http_response", "response.completed", "response.done",
		"response.failed", "response.incomplete", "response.cancelled", "response.canceled",
		"finish_reason_unspecified", "safety", "recitation", "language", "blocklist",
		"prohibited_content", "spii", "malformed_function_call", "image_safety",
		"unexpected_tool_call", "too_many_tool_calls", "no_image", "other":
		return value
	default:
		if strings.Contains(value, "image_generation") &&
			(strings.HasSuffix(value, ".completed") || strings.HasSuffix(value, ".done")) {
			return "image_generation_completed"
		}
		return "other"
	}
}

func (c *ResponseOutcomeCollector) MarkStreamError(err error, clientCanceled bool) {
	if c == nil || err == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if clientCanceled {
		c.outcome.DisconnectSource = DisconnectSourceClient
		c.outcome.UpstreamErrorKind = UpstreamErrorNone
		return
	}
	c.outcome.DisconnectSource = DisconnectSourceUpstream
	switch {
	case errors.Is(err, context.DeadlineExceeded), isTimeoutError(err):
		c.outcome.UpstreamErrorKind = UpstreamErrorTimeout
	case strings.Contains(strings.ToLower(err.Error()), "timeout"):
		c.outcome.UpstreamErrorKind = UpstreamErrorTimeout
	case strings.Contains(strings.ToLower(err.Error()), "eof"), strings.Contains(strings.ToLower(err.Error()), "protocol"), strings.Contains(strings.ToLower(err.Error()), "missing terminal"):
		c.outcome.UpstreamErrorKind = UpstreamErrorProtocol
	default:
		c.outcome.UpstreamErrorKind = UpstreamErrorOther
	}
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func (c *ResponseOutcomeCollector) Snapshot() ResponseOutcome {
	if c == nil {
		return ResponseOutcome{DisconnectSource: DisconnectSourceNone, UpstreamErrorKind: UpstreamErrorNone, CollectorVersion: ResponseOutcomeCollectorVersion}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.outcome
}
