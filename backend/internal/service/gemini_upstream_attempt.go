package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	geminiUpstreamAttemptKey         = "gemini_upstream_attempt"
	geminiUpstreamAttemptSequenceKey = "gemini_upstream_attempt_sequence"
)

// GeminiUpstreamAttempt collects request-local evidence for a real inference
// call. Callers begin immediately before sending upstream. Generated text never
// leaves the attempt; async settlement receives only its scalar token snapshot.
type GeminiUpstreamAttempt struct {
	c               *gin.Context
	event           OpsUpstreamErrorEvent
	started         time.Time
	firstEvent      int
	requestIDHeader string
	// Only the actually observed configured identifier, separate from fallback IDs.
	requestIDHeaderValue    string
	ledgerRequestID         string
	ledgerIdentityHash      string
	candidates              int64
	ended                   bool
	model                   string
	inputFallback           *GeminiInputUsageFallback
	fallbackCandidates      map[int64]*geminiFallbackCandidate
	upstreamAccepted        bool
	fallbackOutputBytes     int
	fallbackStreamCount     int
	fallbackOutputTruncated bool
}

func (a *GeminiUpstreamAttempt) ObserveRequest(request *http.Request) {
	if a == nil || a.ended {
		return
	}
	a.ledgerIdentityHash = geminiUsageLedgerRequestIdentity(request)
	a.inputFallback = prepareGeminiInputUsageFallback(request, a.event.AccountID, a.model)
}

func geminiUsageLedgerRequestIdentity(request *http.Request) string {
	if request == nil || request.URL == nil {
		return ""
	}
	key := request.Header.Get("x-goog-api-key")
	if key == "" {
		parts := strings.Fields(request.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return ""
		}
		key = parts[1]
	}
	return GeminiUsageLedgerIdentity(request.URL.String(), key)
}

func BeginGeminiUpstreamAttempt(c *gin.Context, account *Account, model string, stream bool) *GeminiUpstreamAttempt {
	if c == nil {
		return nil
	}
	sequence := c.GetInt(geminiUpstreamAttemptSequenceKey) + 1
	c.Set(geminiUpstreamAttemptSequenceKey, sequence)
	proxyID, proxyName := opsUpstreamProxyAttribution(account)
	attempt := &GeminiUpstreamAttempt{
		c: c, started: time.Now(), firstEvent: len(geminiUpstreamAttemptEvents(c)),
		model:           model,
		requestIDHeader: UpstreamRequestIDHeaderName(account),
		event: OpsUpstreamErrorEvent{Attempt: sequence, Model: truncateString(strings.TrimSpace(model), 128),
			Stream: stream, Stage: "inference", UsageState: "unreported", ProxyID: proxyID, ProxyName: proxyName},
	}
	if account != nil {
		attempt.event.AccountID = account.ID
		attempt.event.AccountName = truncateString(strings.TrimSpace(account.Name), 128)
		attempt.event.Platform = truncateString(strings.TrimSpace(account.Platform), 32)
	}
	c.Set(geminiUpstreamAttemptKey, attempt)
	return attempt
}

func (a *GeminiUpstreamAttempt) ObserveResponse(response *http.Response) {
	if a == nil || a.ended || response == nil {
		return
	}
	a.event.UpstreamStatusCode = response.StatusCode
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		a.upstreamAccepted = true
	}
	a.ledgerRequestID = ""
	ledgerIdentityMatches := true
	// net/http returns the final request after redirects. Never attach a new
	// origin's ledger ID to the original origin/key authorization boundary.
	if response.Request != nil && geminiUsageLedgerRequestIdentity(response.Request) != a.ledgerIdentityHash {
		a.ledgerIdentityHash = ""
		ledgerIdentityMatches = false
		if a.inputFallback != nil {
			a.inputFallback.CountRequest = nil
			a.inputFallback.CalibrationKey = ""
		}
	}
	ledgerID := strings.TrimSpace(response.Header.Get("X-Oneapi-Request-Id"))
	if ledgerIdentityMatches && len(ledgerID) <= maxUsageUpstreamRequestIDLen && !strings.ContainsAny(ledgerID, "\r\n\t") {
		a.ledgerRequestID = ledgerID
	}
	if a.requestIDHeader != "" {
		a.requestIDHeaderValue = truncateString(strings.TrimSpace(response.Header.Get(a.requestIDHeader)), maxUsageUpstreamRequestIDLen)
		a.event.UpstreamRequestID = a.requestIDHeaderValue
	}
	if a.event.UpstreamRequestID == "" {
		a.event.UpstreamRequestID = a.ledgerRequestID
	}
	if a.event.UpstreamRequestID == "" {
		// These are actual response identifiers, never an invented local ID.
		a.event.UpstreamRequestID = response.Header.Get("x-request-id")
		if a.event.UpstreamRequestID == "" {
			a.event.UpstreamRequestID = response.Header.Get("x-goog-request-id")
		}
	}
	a.event.UpstreamRequestID = truncateString(strings.TrimSpace(a.event.UpstreamRequestID), 128)
}

func ObserveGeminiUpstreamAttemptUsage(c *gin.Context, raw []byte) {
	if c == nil {
		return
	}
	if value, ok := c.Get(geminiUpstreamAttemptKey); ok {
		if attempt, ok := value.(*GeminiUpstreamAttempt); ok {
			attempt.ObserveUsage(raw)
		}
	}
}

func (a *GeminiUpstreamAttempt) ObserveUsage(raw []byte) {
	if a == nil || a.ended || !gjson.ValidBytes(raw) {
		return
	}
	if a.event.UsageFields&2 == 0 {
		a.observeFallbackOutput(raw)
	}
	usage := gjson.GetBytes(raw, "usageMetadata")
	if !usage.IsObject() {
		usage = gjson.GetBytes(raw, "response.usageMetadata")
	}
	if !usage.IsObject() {
		return
	}
	for index, field := range []struct {
		name  string
		value *int64
	}{
		{"promptTokenCount", &a.event.InputTokens},
		{"candidatesTokenCount", &a.candidates},
		{"thoughtsTokenCount", &a.event.ThoughtTokens},
		{"cachedContentTokenCount", &a.event.CacheReadTokens},
		{"totalTokenCount", &a.event.TotalTokens},
	} {
		count := usage.Get(field.name)
		if count.Type != gjson.Number || count.Int() < 0 {
			continue
		}
		*field.value = count.Int()
		a.event.UsageFields |= 1 << index
		a.event.UsageState = "observed"
	}
	a.event.OutputTokens = a.candidates + a.event.ThoughtTokens
	if a.event.UsageFields&2 != 0 {
		// A reported candidate count (including zero) is authoritative. Release
		// generated content now and avoid unused tokenization on normal calls.
		a.fallbackCandidates = nil
		a.fallbackOutputBytes, a.fallbackStreamCount = 0, 0
		a.fallbackOutputTruncated = false
	}
	if a.candidates > 0 || a.event.ThoughtTokens > 0 {
		a.upstreamAccepted = true
	}
	if a.event.OutputTokens < 0 { // defensive overflow guard; never a billing estimate
		a.event.OutputTokens = 0
	}
}

// End is idempotent. A successful call creates no diagnostic event. Failures
// decorate an already captured error where possible to avoid duplicate attempts.
func (a *GeminiUpstreamAttempt) End(err error) {
	if a == nil || a.ended {
		return
	}
	a.ended = true
	if err == nil {
		return
	}
	if a.c.Request != nil && a.c.Request.Context().Err() != nil && errors.Is(err, a.c.Request.Context().Err()) {
		a.event.ClientCanceled = true
	}
	var failover *UpstreamFailoverError
	if errors.As(err, &failover) {
		a.event.Reason = truncateString(string(failover.Reason), 128)
	}
	a.event.DurationMs = time.Since(a.started).Milliseconds()
	events := geminiUpstreamAttemptEvents(a.c)
	for i := len(events) - 1; i >= a.firstEvent; i-- {
		if event := events[i]; event != nil && event.AccountID == a.event.AccountID && event.Attempt == 0 {
			a.copyMetadata(event)
			return
		}
	}
	if failover != nil {
		a.event.Kind = "failover"
		a.event.UpstreamStatusCode = failover.StatusCode
		a.event.Message = "Upstream inference attempt failed"
	} else if a.event.ClientCanceled {
		a.event.Kind, a.event.Reason = "client_cancel", "client_canceled"
		a.event.ClientCanceled = true
		a.event.Message = "Client canceled upstream inference; provider usage may be unreported"
	} else {
		a.event.Kind, a.event.Reason = "request_error", "inference_interrupted"
		a.event.Message = "Upstream inference interrupted"
		if a.event.UpstreamStatusCode < 400 {
			a.event.UpstreamStatusCode = http.StatusBadGateway
		}
	}
	appendOpsUpstreamError(a.c, a.event)
	// The shared Ops sanitizer bounds the final summary to 256 events/512KiB
	// and stamps DroppedEarlierAttempts. No per-attempt job is enqueued here.
}

// EndClientCanceled handles confirmed downstream disconnects that can precede
// net/http canceling the request context (for example a broken client write).
// It signals local cancellation only, never that the provider acknowledged stop.
func (a *GeminiUpstreamAttempt) EndClientCanceled() {
	if a == nil || a.ended {
		return
	}
	a.event.ClientCanceled = true
	a.End(context.Canceled)
}

func (a *GeminiUpstreamAttempt) copyMetadata(event *OpsUpstreamErrorEvent) {
	event.Attempt, event.Model, event.Stream, event.DurationMs = a.event.Attempt, a.event.Model, a.event.Stream, a.event.DurationMs
	if event.Stage == "" {
		event.Stage = a.event.Stage
	}
	event.UsageState, event.UsageFields = a.event.UsageState, a.event.UsageFields
	event.InputTokens, event.OutputTokens = a.event.InputTokens, a.event.OutputTokens
	event.CacheReadTokens, event.ThoughtTokens, event.TotalTokens = a.event.CacheReadTokens, a.event.ThoughtTokens, a.event.TotalTokens
	event.ClientCanceled = event.ClientCanceled || a.event.ClientCanceled
	if event.Reason == "" {
		event.Reason = a.event.Reason
	}
	if event.UpstreamRequestID == "" {
		event.UpstreamRequestID = a.event.UpstreamRequestID
	}
}

func geminiUpstreamAttemptEvents(c *gin.Context) []*OpsUpstreamErrorEvent {
	value, _ := c.Get(OpsUpstreamErrorsKey)
	events, _ := value.([]*OpsUpstreamErrorEvent)
	return events
}
