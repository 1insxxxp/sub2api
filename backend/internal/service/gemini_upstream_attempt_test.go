package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newGeminiAttemptTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c
}

func geminiAttemptTestEvents(t *testing.T, c *gin.Context) []*OpsUpstreamErrorEvent {
	t.Helper()
	v, ok := c.Get(OpsUpstreamErrorsKey)
	if !ok {
		return nil
	}
	events, ok := v.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	return events
}

func TestGeminiUpstreamAttempt_ObservedZeroAndUnreportedAreDistinct(t *testing.T) {
	for _, tc := range []struct{ name, raw, state string }{
		{"explicit zero", `{"usageMetadata":{"promptTokenCount":0,"candidatesTokenCount":0}}`, "observed"},
		{"missing", `{}`, "unreported"},
		{"null", `{"usageMetadata":null}`, "unreported"},
		{"empty", `{"usageMetadata":{}}`, "unreported"},
		{"invalid count", `{"usageMetadata":{"promptTokenCount":null}}`, "unreported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newGeminiAttemptTestContext(t)
			attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 7, Platform: "gemini"}, "gemini-test", true)
			ObserveGeminiUpstreamAttemptUsage(c, []byte(tc.raw))
			attempt.End(io.ErrUnexpectedEOF)
			events := geminiAttemptTestEvents(t, c)
			require.Len(t, events, 1)
			require.Equal(t, tc.state, events[0].UsageState)
			require.Zero(t, events[0].InputTokens)
			require.Zero(t, events[0].OutputTokens)
		})
	}
}

func TestGeminiUpstreamAttempt_PartialMetadataMergesWithoutErasingObservedCounts(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 7}, "gemini-test", true)
	attempt.ObserveUsage([]byte(`{"response":{"usageMetadata":{"promptTokenCount":500,"cachedContentTokenCount":450,"candidatesTokenCount":30,"thoughtsTokenCount":12}}}`))
	attempt.ObserveUsage([]byte(`{"usageMetadata":{}}`))
	attempt.ObserveUsage([]byte(`{"usageMetadata":null}`))
	attempt.ObserveUsage([]byte(`{"usageMetadata":{"candidatesTokenCount":0,"thoughtsTokenCount":2}}`))
	attempt.End(io.ErrUnexpectedEOF)
	event := geminiAttemptTestEvents(t, c)[0]
	require.EqualValues(t, 500, event.InputTokens)
	require.EqualValues(t, 450, event.CacheReadTokens)
	require.EqualValues(t, 2, event.OutputTokens)
	require.EqualValues(t, 2, event.ThoughtTokens)
	require.Equal(t, "observed", event.UsageState)
	require.NotZero(t, event.UsageFields)
}

func TestGeminiUpstreamAttempt_SuccessIsSilentAndFailureEndIsIdempotent(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	account := &Account{ID: 7, Platform: "gemini"}
	first := BeginGeminiUpstreamAttempt(c, account, "gemini-test", false)
	first.ObserveUsage([]byte(`{"usageMetadata":{"promptTokenCount":50}}`))
	first.End(nil)
	require.Empty(t, geminiAttemptTestEvents(t, c), "ordinary success must not add an Ops write")
	first.End(io.ErrUnexpectedEOF)
	require.Empty(t, geminiAttemptTestEvents(t, c))
	second := BeginGeminiUpstreamAttempt(c, account, "gemini-test", true)
	second.End(io.ErrUnexpectedEOF)
	second.End(io.ErrUnexpectedEOF)
	events := geminiAttemptTestEvents(t, c)
	require.Len(t, events, 1)
	require.Equal(t, 2, events[0].Attempt)
	require.Equal(t, "unreported", events[0].UsageState)
}

func TestGeminiUpstreamAttempt_DecoratesExistingFailureWithoutDuplicating(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	account := &Account{ID: 7, Platform: "gemini"}
	attempt := BeginGeminiUpstreamAttempt(c, account, "gemini-test", true)
	attempt.ObserveResponse(&http.Response{StatusCode: http.StatusOK, Header: http.Header{"X-Request-Id": {"actual-upstream-id"}}})
	attempt.ObserveUsage([]byte(`{"usageMetadata":{"promptTokenCount":500}}`))
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{AccountID: 7, Kind: "first_output_timeout", UpstreamStatusCode: 504, Message: "First output timed out"})
	attempt.End(&UpstreamFailoverError{StatusCode: 504, Reason: "first_output_timeout"})
	events := geminiAttemptTestEvents(t, c)
	require.Len(t, events, 1)
	require.Equal(t, "first_output_timeout", events[0].Kind)
	require.Equal(t, 504, events[0].UpstreamStatusCode)
	require.Equal(t, "actual-upstream-id", events[0].UpstreamRequestID)
	require.EqualValues(t, 500, events[0].InputTokens)
	require.Equal(t, 1, events[0].Attempt)
}

func TestGeminiUpstreamAttempt_CancelOnlyAfterRealInference(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	ctx, cancel := context.WithCancel(c.Request.Context())
	c.Request = c.Request.WithContext(ctx)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 7, Platform: "gemini"}, "gemini-test", true)
	cancel()
	attempt.End(context.Canceled)
	event := geminiAttemptTestEvents(t, c)[0]
	require.True(t, event.IsClientCancelDiagnostic())
	require.Equal(t, "unreported", event.UsageState)
	require.Zero(t, event.UpstreamStatusCode, "no response is not a synthetic upstream HTTP status")
	other := newGeminiAttemptTestContext(t)
	BeginGeminiUpstreamAttempt(other, &Account{ID: 7}, "gemini-test", true).End(context.DeadlineExceeded)
	require.False(t, geminiAttemptTestEvents(t, other)[0].IsClientCancelDiagnostic(), "upstream timeout is not user cancellation")
}

func TestGeminiUpstreamAttempt_ContainsOnlyBoundedMetadata(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 7, Name: strings.Repeat("a", 500), Extra: map[string]any{AccountExtraUpstreamRequestIDHeader: "x-custom-request-id"}}, strings.Repeat("m", 1000), true)
	attempt.ObserveResponse(&http.Response{StatusCode: 200, Header: http.Header{"X-Custom-Request-Id": {strings.Repeat("r", 500)}, "Authorization": {"secret-credential"}}})
	attempt.ObserveUsage([]byte(`{"candidates":[{"content":{"parts":[{"text":"PRIVATE RESPONSE"}]}}],"usageMetadata":{"promptTokenCount":42}}`))
	attempt.End(errors.New("PRIVATE ERROR sk-secret-credential"))
	raw, err := json.Marshal(geminiAttemptTestEvents(t, c))
	require.NoError(t, err)
	require.NotContains(t, string(raw), "PRIVATE")
	require.NotContains(t, string(raw), "secret-credential")
	event := geminiAttemptTestEvents(t, c)[0]
	require.LessOrEqual(t, len(event.Model), 128)
	require.LessOrEqual(t, len(event.AccountName), 128)
	require.LessOrEqual(t, len(event.UpstreamRequestID), 128)
}

func TestGeminiUpstreamAttempt_CancelDecoratesRealFailureWithoutLosingAttribution(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	ctx, cancel := context.WithCancel(c.Request.Context())
	c.Request = c.Request.WithContext(ctx)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 7}, "gemini-test", true)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{AccountID: 7, Kind: "request_error", UpstreamStatusCode: 504, Message: "upstream timeout"})
	cancel()
	attempt.End(context.Canceled)
	events := geminiAttemptTestEvents(t, c)
	require.Len(t, events, 1)
	require.True(t, events[0].ClientCanceled)
	require.False(t, events[0].IsClientCancelDiagnostic(), "a real error is still real provider evidence")
	require.Equal(t, "request_error", events[0].Kind)
	require.Equal(t, 504, events[0].UpstreamStatusCode)
}

func TestGeminiUpstreamAttempt_MissingConfiguredRequestIDFallsBackToRealGoogleID(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 7, Extra: map[string]any{AccountExtraUpstreamRequestIDHeader: "x-custom-id"}}, "gemini-test", true)
	attempt.ObserveResponse(&http.Response{StatusCode: 200, Header: http.Header{"X-Goog-Request-Id": {"real-google-request"}}})
	attempt.End(io.ErrUnexpectedEOF)
	require.Equal(t, "real-google-request", geminiAttemptTestEvents(t, c)[0].UpstreamRequestID)
}

func TestGeminiUpstreamAttempt_UpstreamDeadlineIsNotCanceledByUnrelatedClientContext(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	ctx, cancel := context.WithCancel(c.Request.Context())
	c.Request = c.Request.WithContext(ctx)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 7}, "gemini-test", true)
	cancel()
	attempt.End(context.DeadlineExceeded)
	require.False(t, geminiAttemptTestEvents(t, c)[0].ClientCanceled)
}

func TestGeminiUpstreamAttempt_DisconnectedResultWithoutCanceledContextIsNeutral(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 7}, "gemini-test", true)
	finishGeminiUpstreamAttempt(attempt, &ForwardResult{ClientDisconnect: true}, nil)
	require.NoError(t, c.Request.Context().Err(), "fixture reproduces a broken write before request context cancellation")
	require.True(t, geminiAttemptTestEvents(t, c)[0].IsClientCancelDiagnostic())
}
