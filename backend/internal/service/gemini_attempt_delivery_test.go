//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiAttemptDeliveryKeepsCanceledEvidenceSeparateFromPriorFailure(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	account := &Account{ID: 3, Platform: PlatformGemini}
	first := BeginGeminiUpstreamAttempt(c, account, "gemini-test", true)
	first.End(&UpstreamFailoverError{StatusCode: 503})
	last := BeginGeminiUpstreamAttempt(c, account, "gemini-test", true)
	last.EndClientCanceled()
	collector := NewDownstreamOutputTokenCollector("gemini-test")
	collector.ObserveWritten([]byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"private thought\"}}]}\n\n"))
	collector.Freeze()
	RecordGeminiAttemptDelivery(c, account, collector, nil, false, 0)
	events := geminiUpstreamAttemptEvents(c)
	require.Len(t, events, 2)
	require.Nil(t, events[0].DownstreamDelivery)
	require.NotNil(t, events[1].DownstreamDelivery)
	require.False(t, events[1].DownstreamDelivery.AnswerTextDelivered)
	require.Positive(t, events[1].DownstreamDelivery.EstimatedOutputTokens)
	require.Equal(t, "skipped", events[1].DownstreamDelivery.SettlementDecision)
	raw, err := json.Marshal(events)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "private thought")
	// Retrying the audit hook cannot create another attempt or settlement.
	RecordGeminiAttemptDelivery(c, account, collector, nil, false, 0)
	require.Len(t, geminiUpstreamAttemptEvents(c), 2)
}

func TestGeminiAttemptDeliveryDoesNotMarkCompletedResponseCanceled(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	account := &Account{ID: 3, Platform: PlatformGemini}
	attempt := BeginGeminiUpstreamAttempt(c, account, "gemini-test", true)
	attempt.End(nil)
	collector := NewDownstreamOutputTokenCollector("gemini-test")
	collector.ObserveWritten([]byte("data: {\"choices\":[{\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"))
	require.True(t, collector.HasDeliveredTerminal())
	collector.Freeze()
	RecordGeminiAttemptDelivery(c, account, collector, nil, false, 0)
	require.Empty(t, geminiUpstreamAttemptEvents(c))
	// Nor may a diagnostic be invented before a real matching upstream call.
	other := &Account{ID: 4, Platform: PlatformGemini}
	RecordGeminiAttemptDelivery(c, other, nil, nil, false, 0)
	require.Empty(t, geminiUpstreamAttemptEvents(c))
}

func TestOpsDeliverySanitizationPreservesBoundedImmutableEvidence(t *testing.T) {
	snapshot := &OpsDownstreamDelivery{EstimatedOutputTokens: -10, SettlementDecision: "private untrusted payload"}
	input := &OpsInsertErrorLogInput{UpstreamErrors: []*OpsUpstreamErrorEvent{{
		Kind: "client_cancel", Stage: "inference", Message: context.Canceled.Error(), DownstreamDelivery: snapshot,
	}}}
	require.NoError(t, sanitizeOpsUpstreamErrors(input))
	require.NotNil(t, input.UpstreamErrorsJSON)
	var events []*OpsUpstreamErrorEvent
	require.NoError(t, json.Unmarshal([]byte(*input.UpstreamErrorsJSON), &events))
	require.Len(t, events, 1)
	got := events[0].DownstreamDelivery
	require.NotNil(t, got)
	require.Zero(t, got.EstimatedOutputTokens)
	require.Empty(t, got.SettlementDecision)
	snapshot.EstimatedOutputTokens = 99
	require.Zero(t, got.EstimatedOutputTokens, "queue snapshot must not share mutable request evidence")
}

func TestOpsCancellationRetainsOnlyValidDownstreamStatus(t *testing.T) {
	for _, code := range []int{200, 499, -1, 999} {
		encoded, err := json.Marshal(map[string]any{
			"message": "canceled", "downstream_status_code": code,
		})
		require.NoError(t, err)
		var event OpsUpstreamErrorEvent
		require.NoError(t, json.Unmarshal(encoded, &event))
		input := &OpsInsertErrorLogInput{UpstreamErrors: []*OpsUpstreamErrorEvent{&event}}
		require.NoError(t, sanitizeOpsUpstreamErrors(input))
		var got []map[string]any
		require.NoError(t, json.Unmarshal([]byte(*input.UpstreamErrorsJSON), &got))
		if code >= 100 && code <= 599 {
			require.Equal(t, float64(code), got[0]["downstream_status_code"])
		} else {
			require.NotContains(t, got[0], "downstream_status_code")
		}
	}
}

func TestGeminiAttemptDeliveryDoesNotRewritePriorAttemptAfterPreflightFailure(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	account := &Account{ID: 3, Platform: PlatformGemini}
	first := BeginGeminiUpstreamAttempt(c, account, "gemini-test", true)
	first.End(&UpstreamFailoverError{StatusCode: 503})
	events := geminiUpstreamAttemptEvents(c)
	require.Len(t, events, 1)
	beforeForward := GeminiUpstreamAttemptCount(c)
	// A same-account retry fails while obtaining credentials, before inference.
	collector := NewDownstreamOutputTokenCollector("gemini-test")
	collector.Freeze()
	RecordGeminiAttemptDelivery(c, account, collector, nil, false, beforeForward)
	require.Nil(t, events[0].DownstreamDelivery, "preflight must not overwrite previous attempt evidence")
	require.Len(t, geminiUpstreamAttemptEvents(c), 1)
}
