//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGeminiMeasuredUsagePolicyBillsExactCanceledUsageWithoutDelivery(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		events                []string
		input, cached, output int
	}{
		{name: "measured prompt and output", events: []string{`{"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":30}}`}, input: 100, output: 30},
		{name: "cached prompt and reasoning", events: []string{`{"usageMetadata":{"promptTokenCount":100,"cachedContentTokenCount":60,"candidatesTokenCount":30,"thoughtsTokenCount":20}}`}, input: 40, cached: 60, output: 50},
		{name: "fully cached input is measured zero", events: []string{`{"usageMetadata":{"promptTokenCount":100,"cachedContentTokenCount":100,"candidatesTokenCount":30}}`}, cached: 100, output: 30},
		{name: "explicit all zero", events: []string{`{"usageMetadata":{"promptTokenCount":0,"candidatesTokenCount":0}}`}},
		{name: "fields split across events", events: []string{`{"usageMetadata":{"promptTokenCount":100}}`, `{"usageMetadata":{"candidatesTokenCount":30,"thoughtsTokenCount":20}}`}, input: 100, output: 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := measuredPolicyResult(t, tc.events, 0)
			billing, logs, commands := newGeminiInputUsageRegressionBilling()
			billing.cfg.Default.RateMultiplier = 1
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := billing.RecordUsage(ctx, &RecordUsageInput{Result: result, APIKey: &APIKey{ID: 501, Quota: 100}, User: &User{ID: 601}, Account: &Account{ID: 701, Platform: PlatformGemini}, APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}})
			require.NoError(t, err)
			require.Equal(t, 1, commands.calls)
			require.NoError(t, commands.lastCtxErr)
			require.Equal(t, tc.input, commands.lastCmd.InputTokens)
			require.Equal(t, tc.output, commands.lastCmd.OutputTokens)
			require.NotNil(t, logs.lastLog)
			require.Equal(t, tc.cached, logs.lastLog.CacheReadTokens)
			require.Equal(t, tc.output, logs.lastLog.OutputTokens)
			require.Equal(t, UsageTokenSourceUpstream, logs.lastLog.OutputTokensSource)
			wantCost := float64(tc.input)*1e-6 + float64(tc.cached)*1e-7 + float64(tc.output)*5e-6
			require.InDelta(t, wantCost, commands.lastCmd.BalanceCost, 1e-10)
			require.InDelta(t, wantCost, commands.lastCmd.APIKeyQuotaCost, 1e-10)
		})
	}
}

func TestGeminiMeasuredUsagePolicyRejectsIncompleteEvidenceBeforeSettlement(t *testing.T) {
	for _, tc := range []struct {
		name   string
		events []string
	}{
		{name: "no usage"},
		{name: "prompt only", events: []string{`{"usageMetadata":{"promptTokenCount":100}}`}},
		{name: "candidates only", events: []string{`{"usageMetadata":{"candidatesTokenCount":30}}`}},
		{name: "thoughts only", events: []string{`{"usageMetadata":{"promptTokenCount":100,"thoughtsTokenCount":30}}`}},
		{name: "cache and total only", events: []string{`{"usageMetadata":{"cachedContentTokenCount":100,"totalTokenCount":130}}`}},
		{name: "null candidate is absent", events: []string{`{"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":null}}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := measuredPolicyResult(t, tc.events, 50)
			billing, logs, commands := newGeminiInputUsageRegressionBilling()
			billing.cfg.Default.RateMultiplier = 1
			err := billing.RecordUsage(context.Background(), &RecordUsageInput{Result: result, APIKey: &APIKey{ID: 501, Quota: 100}, User: &User{ID: 601}, Account: &Account{ID: 701, Platform: PlatformGemini}, APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}})
			require.Error(t, err, "missing measurements require review rather than an estimated charge")
			require.Zero(t, commands.calls, "no wallet or dedup command before exact evidence")
			require.Nil(t, logs.lastLog, "the billing path must not write a fake zero-cost settlement")
		})
	}
}

func measuredPolicyResult(t *testing.T, events []string, delivered int) *ForwardResult {
	t.Helper()
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, geminiInputUsageRegressionModel, true)
	result := &ForwardResult{RequestID: "measured-policy", Model: geminiInputUsageRegressionModel, Stream: true, ClientDisconnect: true, DeliveredOutputTokens: &delivered, Outcome: &ResponseOutcome{DisconnectSource: DisconnectSourceClient, CollectorVersion: 1}}
	var acc geminiUsageAccumulator
	for _, event := range events {
		attempt.ObserveUsage([]byte(event))
		if usage := acc.Observe([]byte(event)); usage != nil {
			result.Usage = *usage
		}
	}
	finishGeminiUpstreamAttempt(attempt, result, nil)
	return result
}

func TestGeminiMeasuredUsagePolicyRecoveryOnlyUsesCurrentActualAttempt(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	require.Nil(t, GeminiResultFromLastAttempt(c, 701, geminiInputUsageRegressionModel, true, 0))
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "mapped-model", true)
	attempt.ObserveUsage([]byte(`{"usageMetadata":{"promptTokenCount":100,"cachedContentTokenCount":40,"candidatesTokenCount":20,"thoughtsTokenCount":10}}`))
	attempt.event.UpstreamRequestID = "actual-provider-request"
	attempt.event.UpstreamStatusCode = 200
	attempt.event.DurationMs = 93
	c.Set(OpsTimeToFirstTokenMsKey, int64(12))
	result := GeminiResultFromLastAttempt(c, 701, geminiInputUsageRegressionModel, true, 0)
	require.NotNil(t, result)
	require.True(t, result.RequireMeasuredUsage)
	require.True(t, result.MeasuredUsageComplete)
	require.False(t, NeedsUsageMeasurementReview(result))
	require.Equal(t, 60, result.Usage.InputTokens)
	require.Equal(t, 40, result.Usage.CacheReadInputTokens)
	require.Equal(t, 30, result.Usage.OutputTokens)
	require.Equal(t, "actual-provider-request", result.RequestID)
	require.Equal(t, "mapped-model", result.UpstreamModel)
	require.Equal(t, 93*time.Millisecond, result.Duration)
	require.Equal(t, 12, *result.FirstTokenMs)
	require.True(t, result.ClientDisconnect)
	require.Equal(t, DisconnectSourceClient, result.Outcome.DisconnectSource)
	require.False(t, result.Outcome.HasText)
	require.Nil(t, result.DeliveredOutputTokens)
	require.Nil(t, GeminiResultFromLastAttempt(c, 701, geminiInputUsageRegressionModel, true, 1), "a retry that never started upstream cannot reuse old measurements")
	require.Nil(t, GeminiResultFromLastAttempt(c, 999, geminiInputUsageRegressionModel, true, 0))
	BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformOpenAI}, "gemini-model-alias", true)
	require.Nil(t, GeminiResultFromLastAttempt(c, 701, geminiInputUsageRegressionModel, true, 1))
}

func TestGeminiMeasuredUsagePolicyRecoveryDoesNotReuseEndedRetryBeforeNextSend(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "mapped-model", true)
	attempt.ObserveResponse(&http.Response{StatusCode: http.StatusServiceUnavailable, Header: make(http.Header)})
	attempt.End(errors.New("retryable upstream response"))
	require.Nil(t, GeminiResultFromLastAttempt(c, 701, geminiInputUsageRegressionModel, true, 0), "cancellation before the next retry starts cannot reclassify a completed upstream error as canceled inference")

	// A successful attempt can be ended before the downstream collector detects
	// a disconnect; keep its actual identity/measurements available.
	attempt = BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "mapped-model", true)
	attempt.ObserveResponse(&http.Response{StatusCode: http.StatusOK, Header: make(http.Header)})
	attempt.End(nil)
	require.NotNil(t, GeminiResultFromLastAttempt(c, 701, geminiInputUsageRegressionModel, true, 1))

	attempt = BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "mapped-model", true)
	attempt.ObserveResponse(&http.Response{StatusCode: http.StatusServiceUnavailable, Header: make(http.Header)})
	attempt.EndClientCanceled()
	require.NotNil(t, GeminiResultFromLastAttempt(c, 701, geminiInputUsageRegressionModel, true, 2), "confirmed cancellation retains its real final attempt")
}

func TestGeminiMeasuredUsagePolicyPreservesOtherPlatformsAndImages(t *testing.T) {
	delivered := 7
	other := &ForwardResult{Usage: ClaudeUsage{OutputTokens: 30}, ClientDisconnect: true, DeliveredOutputTokens: &delivered}
	require.Equal(t, 7, recordedOutputTokens(other), "other platforms keep their existing output rule")
	for _, model := range []string{"gemini-3-pro-image", "custom-image-alias"} {
		result := measuredPolicyResult(t, nil, delivered)
		result.Model = model
		result.ImageCount = 1
		applyGeminiMeasuredUsageEvidence(result, 0)
		require.False(t, result.RequireMeasuredUsage)
		require.False(t, NeedsUsageMeasurementReview(result))
	}
}

func TestGeminiMeasuredUsagePolicyRecoveryPreservesOnlyActualConfiguredRequestHeader(t *testing.T) {
	for _, configuredPresent := range []bool{false, true} {
		t.Run(map[bool]string{true: "configured present", false: "fallback cannot impersonate configured header"}[configuredPresent], func(t *testing.T) {
			c := newGeminiAttemptTestContext(t)
			account := &Account{ID: 701, Platform: PlatformGemini, Extra: map[string]any{AccountExtraUpstreamRequestIDHeader: "X-Provider-Request-Id"}}
			attempt := BeginGeminiUpstreamAttempt(c, account, geminiInputUsageRegressionModel, true)
			header := http.Header{"X-Request-Id": []string{"fallback-real-request"}}
			if configuredPresent {
				header.Set("X-Provider-Request-Id", "  configured-real-request  ")
			}
			attempt.ObserveResponse(&http.Response{StatusCode: 200, Header: header})
			result := GeminiResultFromLastAttempt(c, account.ID, geminiInputUsageRegressionModel, true, 0)
			require.NotNil(t, result)
			if configuredPresent {
				require.Equal(t, "configured-real-request", UpstreamRequestIDFromHeaders(account, result.UpstreamHeaders))
				require.Equal(t, "configured-real-request", result.RequestID)
			} else {
				require.Empty(t, UpstreamRequestIDFromHeaders(account, result.UpstreamHeaders))
				require.Equal(t, "fallback-real-request", result.RequestID)
			}
		})
	}
}

func TestGeminiMeasuredUsagePolicyRecoveryPreservesObservedResponseModelAndConflict(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		c := newGeminiAttemptTestContext(t)
		BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, geminiInputUsageRegressionModel, true)
		observer := beginUpstreamResponseModelObservation(c)
		observer.ObserveGemini([]byte(`{"modelVersion":"gemini-3.1-pro"}`))
		if conflict {
			observer.ObserveGemini([]byte(`{"modelVersion":"gemini-3.2-pro"}`))
		}
		result := GeminiResultFromLastAttempt(c, 701, geminiInputUsageRegressionModel, true, 0)
		require.NotNil(t, result)
		require.Equal(t, observedUpstreamResponseModel(c), result.UpstreamResponseModel)
		require.Equal(t, conflict, result.UpstreamResponseModelConflict)
	}
}

func TestGeminiMeasuredUsagePolicyThoughtsWithoutCandidatesLeaveOutputUnreported(t *testing.T) {
	result := measuredPolicyResult(t, []string{`{"usageMetadata":{"promptTokenCount":100,"thoughtsTokenCount":30}}`}, 0)
	require.True(t, NeedsUsageMeasurementReview(result))
	require.Equal(t, 30, result.Usage.OutputTokens, "retain the observed thought component for diagnostics")
	require.Equal(t, UsageTokenSourceUnreported, result.OutputTokensSource, "a known component is not a measured total output count")
	require.Equal(t, UsageTokenSourceUnreported, recordedOutputTokensSource(result))
	complete := measuredPolicyResult(t, []string{`{"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":0,"thoughtsTokenCount":30}}`}, 0)
	require.False(t, NeedsUsageMeasurementReview(complete))
	require.Equal(t, UsageTokenSourceUpstream, complete.OutputTokensSource)
	require.Equal(t, 30, recordedOutputTokens(complete))
}
