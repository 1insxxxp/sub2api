//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func pureUsageFailureOutcome(canceled bool) ResponseOutcome {
	outcome := ResponseOutcome{HTTPStatus: http.StatusBadGateway, UpstreamStatus: http.StatusServiceUnavailable,
		DisconnectSource: DisconnectSourceUpstream, UpstreamErrorKind: UpstreamErrorHTTP5xx,
		CollectorVersion: ResponseOutcomeCollectorVersion}
	if canceled {
		outcome.HTTPStatus, outcome.UpstreamStatus = 499, 0
		outcome.DisconnectSource, outcome.UpstreamErrorKind = DisconnectSourceClient, UpstreamErrorNone
	}
	return outcome
}

func pureRawUsageFailureInput(canceled bool) *OpenAIRecordUsageInput {
	outcome := pureUsageFailureOutcome(canceled)
	input := openAIChatSettlementFixture(&OpenAIChatUsageRound{
		AccountID: 301, UpstreamModel: "gpt-5.1", Outcome: outcome,
		InputFallback: &GeminiInputUsageFallback{Tokens: 1000, Known: true},
	})
	input.Result.Outcome = &outcome
	input.Result.ClientDisconnect = canceled
	input.APIKey, input.User = &APIKey{ID: 1000, Quota: 100}, &User{ID: 2000}
	return input
}

func pureGeminiUsageFailureInput(canceled bool) *RecordUsageInput {
	input, _ := newGeminiUsageRecoveryFixture()
	input.Result.RequestID = "client-pure-gemini-failure"
	input.Result.UsageLedgerRequestID, input.Result.UsageLedgerIdentityHash = "", ""
	outcome := pureUsageFailureOutcome(canceled)
	input.Result.Outcome = &outcome
	input.Result.ClientDisconnect = canceled
	input.Result.GeminiUsageFallback = &GeminiUsageFallback{Input: &GeminiInputUsageFallback{Tokens: 1000, Known: true}}
	input.APIKey, input.User = &APIKey{ID: 1000, Quota: 100}, &User{ID: 2000}
	return input
}

func TestPureUsageFailureRawSkipsAllAccountingThenRecoveredUsageCanSettle(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "rejected"
		if canceled {
			name = "canceled_before_acceptance"
		}
		t.Run(name, func(t *testing.T) {
			logs := &openAIRecordUsageLogRepoStub{inserted: true}
			bills := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
			wallet, subscriptions, quota := &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, &openAIRecordUsageAPIKeyQuotaStub{}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, bills, wallet, subscriptions, nil)
			input := pureRawUsageFailureInput(canceled)
			input.APIKeyService = quota
			require.NoError(t, svc.RecordUsage(context.Background(), input))
			require.True(t, input.Result.ChatUsage.Finalized)
			require.Zero(t, bills.calls, "skip the atomic wallet/dedup/outbox transaction entirely")
			require.Zero(t, logs.calls, "pure failures belong only to request-error history")
			require.Nil(t, logs.lastLog)
			require.Zero(t, wallet.deductCalls)
			require.Zero(t, subscriptions.incrementCalls)
			require.Zero(t, quota.quotaCalls)
			require.Zero(t, quota.rateLimitCalls)

			// A later exact ledger result for the same client request must still be
			// able to settle; the skipped attempt cannot reserve the dedup key.
			recovered := pureRawUsageFailureInput(canceled)
			recovered.APIKeyService = quota
			round := recovered.Result.ChatUsage.Rounds[0]
			round.UpstreamRequestID = "exact-failed-round"
			round.IdentityHash = GeminiUsageLedgerIdentity(recovered.Account.GetCredential("base_url"), recovered.Account.GetCredential("api_key"))
			upstream := &httpUpstreamRecorder{resp: geminiUsageRecoveryResponse(200, `{"success":true,"data":[{"type":2,"request_id":"exact-failed-round","model_name":"gpt-5.1","prompt_tokens":100,"completion_tokens":10}]}`)}
			svc.httpUpstream = upstream
			require.NoError(t, svc.RecordUsage(context.Background(), recovered))
			require.Len(t, upstream.requests, 1)
			require.Equal(t, 1, bills.calls)
			require.Equal(t, 1, logs.calls)
			require.Equal(t, input.Result.RequestID, logs.lastLog.RequestID)
			require.Equal(t, 100, logs.lastLog.InputTokens)
			require.Equal(t, 10, logs.lastLog.OutputTokens)
			require.Positive(t, logs.lastLog.ActualCost)
		})
	}
}

func TestPureUsageFailureRawRetainsDeliveredEvidenceAcrossBillingRetry(t *testing.T) {
	logs := &openAIRecordUsageLogRepoStub{inserted: true}
	billingFailure := errors.New("temporary billing failure")
	bills := &openAIRecordUsageBillingRepoStub{err: billingFailure, result: &UsageBillingApplyResult{Applied: true}}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, bills, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	input := pureRawUsageFailureInput(false)
	delivered := 10
	input.Result.DeliveredOutputTokens = &delivered
	require.ErrorIs(t, svc.RecordUsage(context.Background(), input), billingFailure)
	require.Equal(t, 1, bills.calls)
	require.True(t, input.Result.ChatUsage.Finalized)
	require.Nil(t, input.Result.DeliveredOutputTokens, "finalized provider usage still takes precedence over the downstream billing fallback")

	bills.err = nil
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Equal(t, 2, bills.calls, "a failed persistence retry must not lose evidence and turn into a silent skip")
}

func TestPureUsageFailureGeminiSkipsAllAccountingThenRecoveredUsageCanSettle(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "rejected"
		if canceled {
			name = "canceled_before_acceptance"
		}
		t.Run(name, func(t *testing.T) {
			svc, logs, bills := newGeminiInputUsageRegressionBilling()
			quota := &openAIRecordUsageAPIKeyQuotaStub{}
			input := pureGeminiUsageFailureInput(canceled)
			input.APIKeyService = quota
			svc.FinalizeGeminiUsageFallback(context.Background(), input)
			require.NoError(t, svc.RecordUsage(context.Background(), input))
			require.True(t, input.Result.GeminiUsageFinalized)
			require.Zero(t, bills.calls, "skip the atomic wallet/dedup/outbox transaction entirely")
			require.Zero(t, logs.calls)
			require.Nil(t, logs.lastLog)
			require.Zero(t, quota.quotaCalls)
			require.Zero(t, quota.rateLimitCalls)

			recovered, upstream := newGeminiUsageRecoveryFixture()
			recovered.APIKey, recovered.User, recovered.APIKeyService = input.APIKey, input.User, quota
			recovered.Result.RequestID, recovered.Result.Outcome = input.Result.RequestID, input.Result.Outcome
			recovered.Result.GeminiUsageFallback = &GeminiUsageFallback{}
			svc.httpUpstream = upstream
			ok, err := svc.RecoverGeminiUsage(context.Background(), recovered)
			require.NoError(t, err)
			require.True(t, ok)
			svc.FinalizeGeminiUsageFallback(context.Background(), recovered)
			require.NoError(t, svc.RecordUsage(context.Background(), recovered))
			require.Len(t, upstream.requests, 1)
			require.Equal(t, 1, bills.calls)
			require.Equal(t, 1, logs.calls)
			require.Equal(t, input.Result.RequestID, logs.lastLog.RequestID)
			require.Equal(t, 21916, logs.lastLog.InputTokens)
			require.Equal(t, 4055, logs.lastLog.OutputTokens)
			require.Positive(t, logs.lastLog.ActualCost)
		})
	}
}

func TestPureUsageFailureRawPreservesConsumptionAndIncompleteEvidence(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*OpenAIForwardResult, *OpenAIChatUsageRound)
	}{
		{"accepted without usage", func(_ *OpenAIForwardResult, r *OpenAIChatUsageRound) { r.UpstreamAccepted = true }},
		{"measured prompt", func(_ *OpenAIForwardResult, r *OpenAIChatUsageRound) {
			r.InputReported = true
			r.Usage.InputTokens = 100
		}},
		{"cache read only", func(_ *OpenAIForwardResult, r *OpenAIChatUsageRound) {
			r.CacheReadReported = true
			r.Usage.CacheReadInputTokens = 10
		}},
		{"cache write only", func(_ *OpenAIForwardResult, r *OpenAIChatUsageRound) {
			r.CacheWriteReported = true
			r.Usage.CacheCreationInputTokens = 10
		}},
		{"reported reasoning", func(_ *OpenAIForwardResult, r *OpenAIChatUsageRound) {
			r.ReasoningReported = true
			r.ReasoningTokens = 10
		}},
		{"reported total only", func(_ *OpenAIForwardResult, r *OpenAIChatUsageRound) { r.TotalReported = true; r.TotalTokens = 10 }},
		{"received text", func(_ *OpenAIForwardResult, r *OpenAIChatUsageRound) { r.ReceivedOutputTokens = 10 }},
		{"received reasoning", func(_ *OpenAIForwardResult, r *OpenAIChatUsageRound) { r.ReceivedReasoningTokens = 10 }},
		{"received tool", func(_ *OpenAIForwardResult, r *OpenAIChatUsageRound) { r.Outcome.HasToolCall = true }},
		{"delivered reasoning", func(r *OpenAIForwardResult, _ *OpenAIChatUsageRound) { r.Outcome.HasReasoning = true }},
		{"delivered output counter", func(r *OpenAIForwardResult, _ *OpenAIChatUsageRound) { tokens := 10; r.DeliveredOutputTokens = &tokens }},
		{"truncated semantic capture", func(_ *OpenAIForwardResult, r *OpenAIChatUsageRound) { r.OutputTruncated = true }},
		{"bridge consumption", func(r *OpenAIForwardResult, _ *OpenAIChatUsageRound) {
			r.ChatUsage.ExtraUsage = OpenAIUsage{InputTokens: 50, ImageInputTokens: 50}
		}},
		{"media", func(r *OpenAIForwardResult, _ *OpenAIChatUsageRound) { r.ImageCount = 1 }},
		{"audio", func(r *OpenAIForwardResult, _ *OpenAIChatUsageRound) {
			r.AudioUsage = &AudioUsage{Mode: "tts", DurationOrUnits: 1}
		}},
		{"search", func(r *OpenAIForwardResult, _ *OpenAIChatUsageRound) { r.SearchCount = 1 }},
		{"web search", func(r *OpenAIForwardResult, _ *OpenAIChatUsageRound) { r.WebSearchCalls = 1 }},
		{"video", func(r *OpenAIForwardResult, _ *OpenAIChatUsageRound) { r.VideoCount = 1 }},
		{"missing final outcome", func(r *OpenAIForwardResult, _ *OpenAIChatUsageRound) { r.Outcome = nil }},
		{"legacy final outcome", func(r *OpenAIForwardResult, _ *OpenAIChatUsageRound) { r.Outcome.CollectorVersion = 0 }},
		{"missing round outcome", func(_ *OpenAIForwardResult, r *OpenAIChatUsageRound) { r.Outcome = ResponseOutcome{} }},
		{"empty rounds", func(r *OpenAIForwardResult, _ *OpenAIChatUsageRound) { r.ChatUsage.Rounds = nil }},
		{"missing round", func(r *OpenAIForwardResult, _ *OpenAIChatUsageRound) {
			r.ChatUsage.Rounds = []*OpenAIChatUsageRound{nil}
		}},
		{"legacy protocol", func(r *OpenAIForwardResult, _ *OpenAIChatUsageRound) { r.ChatUsage = nil }},
		{"successful explicit zero", func(r *OpenAIForwardResult, round *OpenAIChatUsageRound) {
			round.InputReported, round.OutputReported, round.UpstreamAccepted = true, true, true
			outcome := ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true, CollectorVersion: ResponseOutcomeCollectorVersion}
			r.Outcome, round.Outcome = &outcome, outcome
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs := &openAIRecordUsageLogRepoStub{inserted: true}
			bills := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, bills, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			svc.resolver = newOpenAITextChannelPricingResolverForTest(t, 123, "gpt-5.1", 0.05)
			input := pureRawUsageFailureInput(false)
			input.APIKey.GroupID, input.APIKey.Group = i64p(123), &Group{ID: 123, RateMultiplier: 1}
			tc.edit(input.Result, input.Result.ChatUsage.Rounds[0])
			require.NoError(t, svc.RecordUsage(context.Background(), input))
			require.Equal(t, 1, bills.calls)
			require.Equal(t, 1, logs.calls)
			if tc.name == "successful explicit zero" {
				require.InDelta(t, 0.05, logs.lastLog.ActualCost, 1e-12, "successful per-request billing remains independent of token count")
			}
		})
	}
}

func TestPureUsageFailureGeminiPreservesConsumptionAndIncompleteEvidence(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*ForwardResult)
	}{
		{"accepted without usage", func(r *ForwardResult) { r.GeminiUsageFallback.UpstreamAccepted = true }},
		{"measured prompt", func(r *ForwardResult) { r.InputTokensSource = UsageTokenSourceUpstream; r.Usage.InputTokens = 10 }},
		{"measured output", func(r *ForwardResult) { r.OutputTokensSource = UsageTokenSourceUpstream; r.Usage.OutputTokens = 10 }},
		{"cache read only", func(r *ForwardResult) { r.Usage.CacheReadInputTokens = 10 }},
		{"cache write only", func(r *ForwardResult) { r.Usage.CacheCreationInputTokens = 10 }},
		{"cache five minutes", func(r *ForwardResult) { r.Usage.CacheCreation5mTokens = 10 }},
		{"cache one hour", func(r *ForwardResult) { r.Usage.CacheCreation1hTokens = 10 }},
		{"image tokens", func(r *ForwardResult) { r.Usage.ImageOutputTokens = 10 }},
		{"measured candidates", func(r *ForwardResult) { r.GeminiUsageFallback.MeasuredCandidates = 10 }},
		{"measured thoughts", func(r *ForwardResult) { r.GeminiUsageFallback.MeasuredThoughts = 10 }},
		{"received candidates", func(r *ForwardResult) { r.GeminiUsageFallback.CandidateTokens = 10 }},
		{"received thoughts", func(r *ForwardResult) { r.GeminiUsageFallback.ThoughtTokens = 10 }},
		{"delivered tool", func(r *ForwardResult) { r.Outcome.HasToolCall = true }},
		{"delivered output counter", func(r *ForwardResult) { tokens := 10; r.DeliveredOutputTokens = &tokens }},
		{"truncated semantic capture", func(r *ForwardResult) { r.GeminiUsageFallback.OutputTruncated = true }},
		{"media", func(r *ForwardResult) { r.ImageCount = 1 }},
		{"audio", func(r *ForwardResult) { r.AudioUsage = &AudioUsage{Mode: "tts", DurationOrUnits: 1} }},
		{"search", func(r *ForwardResult) { r.SearchCount = 1 }},
		{"cache storage", func(r *ForwardResult) { r.CacheStorageTokenHours = 1 }},
		{"earlier continuation evidence unavailable", func(r *ForwardResult) { r.AntiTruncationRounds = 1 }},
		{"missing outcome", func(r *ForwardResult) { r.Outcome = nil }},
		{"legacy outcome", func(r *ForwardResult) { r.Outcome.CollectorVersion = 0 }},
		{"missing fallback", func(r *ForwardResult) { r.GeminiUsageFallback = nil }},
		{"other protocol", func(r *ForwardResult) { r.RequireMeasuredUsage = false }},
		{"successful explicit zero", func(r *ForwardResult) {
			r.MeasuredUsageComplete = true
			r.InputTokensSource, r.OutputTokensSource = UsageTokenSourceUpstream, UsageTokenSourceUpstream
			r.Outcome = &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true, CollectorVersion: ResponseOutcomeCollectorVersion}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, logs, bills := newGeminiInputUsageRegressionBilling()
			input := pureGeminiUsageFailureInput(false)
			tc.edit(input.Result)
			svc.FinalizeGeminiUsageFallback(context.Background(), input)
			require.NoError(t, svc.RecordUsage(context.Background(), input))
			require.Equal(t, 1, bills.calls)
			require.Equal(t, 1, logs.calls)
		})
	}
}
