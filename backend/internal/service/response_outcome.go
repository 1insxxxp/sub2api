package service

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
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

func (o ResponseOutcome) HasEffectiveOutput() bool {
	return o.HasText || o.HasToolCall || o.HasReasoning || o.HasMedia
}

// ResponseOutcomeCollector records only structural response signals.
type ResponseOutcomeCollector struct {
	mu      sync.Mutex
	outcome ResponseOutcome
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
	c.outcome.FinishReason = strings.TrimSpace(finishReason)
	c.outcome.DisconnectSource = DisconnectSourceNone
	c.mu.Unlock()
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
	case strings.Contains(strings.ToLower(err.Error()), "eof"), strings.Contains(strings.ToLower(err.Error()), "protocol"):
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
