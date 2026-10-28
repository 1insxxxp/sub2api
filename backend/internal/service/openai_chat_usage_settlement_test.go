//go:build unit

package service

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func openAIChatSettlementFixture(rounds ...*OpenAIChatUsageRound) *OpenAIRecordUsageInput {
	return &OpenAIRecordUsageInput{
		Account: &Account{ID: 301, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
			Credentials: map[string]any{"base_url": "https://ledger.example.com/v1", "api_key": "test-ledger-key"}},
		Result: &OpenAIForwardResult{RequestID: "client-raw-cc", Model: "gpt-5.1", Stream: true,
			ChatUsage: &OpenAIChatUsageEvidence{Rounds: rounds}},
	}
}

func TestOpenAIChatSettlementFillsOnlyMissingFields(t *testing.T) {
	for _, tc := range []struct {
		name                                    string
		inputReported, outputReported, accepted bool
		input, output, wantInput, wantOutput    int
	}{
		{"missing all", false, false, true, 0, 0, 21916, 4055},
		{"prompt only", true, false, true, 17005, 0, 17005, 4055},
		{"explicit zero prompt", true, false, true, 0, 0, 0, 4055},
		{"completion only", false, true, true, 0, 679, 21916, 679},
		{"explicit zero completion", false, true, true, 0, 0, 21916, 0},
		{"complete usage", true, true, true, 17, 9, 17, 9},
		{"not accepted", false, false, false, 0, 0, 0, 0},
		{"failed with measured usage", true, true, false, 200, 13, 200, 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			round := &OpenAIChatUsageRound{AccountID: 301, UpstreamModel: "gpt-5.1", UpstreamAccepted: tc.accepted,
				Usage:         OpenAIUsage{InputTokens: tc.input, OutputTokens: tc.output},
				InputReported: tc.inputReported, OutputReported: tc.outputReported,
				InputFallback: &GeminiInputUsageFallback{Tokens: 21916, Known: true}, ReceivedOutputTokens: 4055}
			input := openAIChatSettlementFixture(round)
			oldDelivered := 1
			input.Result.DeliveredOutputTokens = &oldDelivered
			input.Result.ClientDisconnect = true
			(&OpenAIGatewayService{}).FinalizeOpenAIChatUsage(context.Background(), input)
			require.True(t, input.Result.ChatUsage.Finalized)
			require.Equal(t, tc.wantInput, input.Result.Usage.InputTokens)
			require.Equal(t, tc.wantOutput, input.Result.Usage.OutputTokens)
			require.Equal(t, tc.wantOutput, recordedOpenAIOutputTokens(input.Result), "disconnect may not replace finalized usage with delivered estimate")
			if tc.inputReported {
				require.Equal(t, UsageTokenSourceUpstream, round.InputTokensSource)
			}
			if tc.outputReported {
				require.Equal(t, UsageTokenSourceUpstream, round.OutputTokensSource)
			}
		})
	}
}

func TestOpenAIChatSettlementExactLedgerPreservesReportedZeroAndCache(t *testing.T) {
	round := &OpenAIChatUsageRound{AccountID: 301, UpstreamModel: "gpt-5.1", UpstreamAccepted: true,
		UpstreamRequestID: "exact-round", OutputReported: true, CacheReadReported: true,
		Usage:         OpenAIUsage{OutputTokens: 0, CacheReadInputTokens: 0},
		InputFallback: &GeminiInputUsageFallback{Tokens: 90000, Known: true}, ReceivedOutputTokens: 999}
	input := openAIChatSettlementFixture(round)
	round.IdentityHash = GeminiUsageLedgerIdentity(input.Account.GetCredential("base_url"), input.Account.GetCredential("api_key"))
	upstream := &httpUpstreamRecorder{resp: geminiUsageRecoveryResponse(200, `{"success":true,"data":[{"type":2,"request_id":"exact-round","model_name":"gpt-5.1","prompt_tokens":1000,"completion_tokens":800,"other":"{\"cache_tokens\":200,\"cache_input_includes_cache\":true}"}]}`)}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	svc.FinalizeOpenAIChatUsage(context.Background(), input)
	require.Equal(t, 1000, input.Result.Usage.InputTokens)
	require.Zero(t, input.Result.Usage.OutputTokens)
	require.Zero(t, input.Result.Usage.CacheReadInputTokens)
	require.Equal(t, UsageTokenSourceUpstream, round.InputTokensSource)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://ledger.example.com/api/log/token", upstream.lastReq.URL.String())
	require.True(t, HTTPUpstreamRedirectsDisabled(upstream.lastReq.Context()))
	require.Equal(t, http.MethodGet, upstream.lastReq.Method)
}

func TestOpenAIChatSettlementEachRoundHasIndependentLedgerIdentity(t *testing.T) {
	rounds := []*OpenAIChatUsageRound{
		{AccountID: 301, UpstreamModel: "gpt-5.1", UpstreamRequestID: "first", UpstreamAccepted: true, InputFallback: &GeminiInputUsageFallback{Tokens: 10, Known: true}},
		{AccountID: 301, UpstreamModel: "gpt-5.1", UpstreamRequestID: "second", UpstreamAccepted: true, InputFallback: &GeminiInputUsageFallback{Tokens: 100, Known: true}},
	}
	input := openAIChatSettlementFixture(rounds...)
	for _, round := range rounds {
		round.IdentityHash = GeminiUsageLedgerIdentity(input.Account.GetCredential("base_url"), input.Account.GetCredential("api_key"))
	}
	upstream := &httpUpstreamRecorder{resp: geminiUsageRecoveryResponse(200, `{"success":true,"data":[{"type":2,"request_id":"first","model_name":"gpt-5.1","prompt_tokens":21,"completion_tokens":4},{"type":2,"request_id":"second","model_name":"gpt-5.1","prompt_tokens":62,"completion_tokens":17}]}`)}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	svc.FinalizeOpenAIChatUsage(context.Background(), input)
	require.Equal(t, 83, input.Result.Usage.InputTokens)
	require.Equal(t, 21, input.Result.Usage.OutputTokens)
	require.Equal(t, 21, rounds[0].Usage.InputTokens)
	require.Equal(t, 62, rounds[1].Usage.InputTokens)
	require.Len(t, upstream.requests, 1, "one credential ledger snapshot can match multiple exact round IDs")
	svc.FinalizeOpenAIChatUsage(context.Background(), input)
	require.Len(t, upstream.requests, 1, "finalization is idempotent")
	require.Equal(t, 83, input.Result.Usage.InputTokens)
}

func TestOpenAIChatSettlementWrongIdentityNeverLooksUpLedger(t *testing.T) {
	for _, edit := range []struct {
		name  string
		apply func(*OpenAIChatUsageRound)
	}{
		{"missing ID", func(r *OpenAIChatUsageRound) { r.UpstreamRequestID = "" }},
		{"changed credential", func(r *OpenAIChatUsageRound) { r.IdentityHash = "other" }},
		{"different account", func(r *OpenAIChatUsageRound) { r.AccountID = 999 }},
	} {
		t.Run(edit.name, func(t *testing.T) {
			round := &OpenAIChatUsageRound{AccountID: 301, UpstreamModel: "gpt-5.1", UpstreamAccepted: true, UpstreamRequestID: "exact",
				InputFallback: &GeminiInputUsageFallback{Tokens: 51, Known: true}, ReceivedOutputTokens: 13}
			input := openAIChatSettlementFixture(round)
			round.IdentityHash = GeminiUsageLedgerIdentity(input.Account.GetCredential("base_url"), input.Account.GetCredential("api_key"))
			edit.apply(round)
			upstream := &httpUpstreamRecorder{}
			(&OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}).FinalizeOpenAIChatUsage(context.Background(), input)
			require.Empty(t, upstream.requests)
			require.Equal(t, 51, input.Result.Usage.InputTokens)
			require.Equal(t, 13, input.Result.Usage.OutputTokens)
		})
	}
}

func TestOpenAIChatSettlementRecoveryHonorsTaskDeadline(t *testing.T) {
	round := &OpenAIChatUsageRound{AccountID: 301, UpstreamModel: "gpt-5.1", UpstreamAccepted: true, UpstreamRequestID: "not-yet-logged",
		InputFallback: &GeminiInputUsageFallback{Tokens: 51, Known: true}, ReceivedOutputTokens: 13}
	input := openAIChatSettlementFixture(round)
	round.IdentityHash = GeminiUsageLedgerIdentity(input.Account.GetCredential("base_url"), input.Account.GetCredential("api_key"))
	upstream := &httpUpstreamRecorder{resp: geminiUsageRecoveryResponse(200, `{"success":true,"data":[]}`)}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	start := time.Now()
	(&OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}).FinalizeOpenAIChatUsage(ctx, input)
	require.Less(t, time.Since(start), time.Second, "recovery must not detach from usage-task deadline")
	require.Equal(t, 51, input.Result.Usage.InputTokens)
	require.Equal(t, 13, input.Result.Usage.OutputTokens)
	require.Empty(t, upstream.requests, "reserve the remaining task budget for billing instead of starting a lookup")
	require.NoError(t, ctx.Err(), "recovery may not consume the billing deadline")
}

func TestOpenAIChatSettlementUsesValidatedCalibrationAndReleasesCountBody(t *testing.T) {
	key := fmt.Sprintf("cc-test-%s", t.Name())
	for i := 0; i < 16; i++ {
		round := &OpenAIChatUsageRound{Usage: OpenAIUsage{InputTokens: 512}, InputReported: true, OutputReported: true,
			InputFallback: &GeminiInputUsageFallback{Tokens: 1024, Known: true, CalibrationKey: key}}
		(&OpenAIGatewayService{}).FinalizeOpenAIChatUsage(context.Background(), openAIChatSettlementFixture(round))
	}
	round := &OpenAIChatUsageRound{UpstreamAccepted: true, InputFallback: &GeminiInputUsageFallback{Tokens: 1024, Known: true, CalibrationKey: key,
		CountRequest: &geminiInputCountRequest{Body: []byte("private request")}}}
	input := openAIChatSettlementFixture(round)
	(&OpenAIGatewayService{}).FinalizeOpenAIChatUsage(context.Background(), input)
	require.Equal(t, 512, input.Result.Usage.InputTokens)
	require.Nil(t, round.InputFallback.CountRequest)
	require.Equal(t, UsageTokenSourceEstimated, round.InputTokensSource)
}

func TestOpenAIChatSettlementRecordUsagePersistsOneBillWithProvenance(t *testing.T) {
	logs := &openAIRecordUsageLogRepoStub{inserted: true}
	bills := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, bills, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	input := openAIChatSettlementFixture(
		&OpenAIChatUsageRound{UpstreamAccepted: true, InputFallback: &GeminiInputUsageFallback{Tokens: 1000, Known: true}, ReceivedOutputTokens: 55},
		&OpenAIChatUsageRound{InputReported: true, OutputReported: true, Usage: OpenAIUsage{InputTokens: 900, OutputTokens: 23}},
	)
	input.APIKey, input.User = &APIKey{ID: 1000, Quota: 100, Group: &Group{RateMultiplier: 1}}, &User{ID: 2000}
	input.APIKeyService = &openAIRecordUsageAPIKeyQuotaStub{}
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Equal(t, 1, bills.calls)
	require.Equal(t, 1, logs.calls)
	require.Equal(t, 1900, logs.lastLog.InputTokens)
	require.Equal(t, 78, logs.lastLog.OutputTokens)
	require.Equal(t, UsageTokenSourceEstimated, logs.lastLog.InputTokensSource)
	require.Equal(t, UsageTokenSourceEstimated, logs.lastLog.OutputTokensSource)
	require.Positive(t, logs.lastLog.ActualCost)
	require.InDelta(t, logs.lastLog.ActualCost, bills.lastCmd.BalanceCost, 1e-12)
}

func TestOpenAIChatSettlementPerRequestPriceDoesNotDependOnTokenCount(t *testing.T) {
	for _, accepted := range []bool{true, false} {
		t.Run(fmt.Sprintf("accepted_%t", accepted), func(t *testing.T) {
			logs := &openAIRecordUsageLogRepoStub{inserted: true}
			bills := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, bills, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			svc.resolver = newOpenAITextChannelPricingResolverForTest(t, 123, "gpt-5.1", 0.05)
			outcome := pureUsageFailureOutcome(false)
			if accepted {
				outcome = ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true, CollectorVersion: ResponseOutcomeCollectorVersion}
			}
			input := openAIChatSettlementFixture(&OpenAIChatUsageRound{UpstreamAccepted: accepted, InputReported: accepted, OutputReported: accepted, Outcome: outcome})
			input.Result.Outcome = &outcome
			input.APIKey, input.User = &APIKey{ID: 1000, GroupID: i64p(123), Group: &Group{ID: 123, RateMultiplier: 1}}, &User{ID: 2000}
			require.NoError(t, svc.RecordUsage(context.Background(), input))
			if !accepted {
				require.Nil(t, logs.lastLog, "an explicitly failed unaccepted request creates no usage record")
				require.Zero(t, bills.calls)
				return
			}
			require.NotNil(t, logs.lastLog)
			require.Zero(t, logs.lastLog.InputTokens)
			require.Zero(t, logs.lastLog.OutputTokens)
			require.InDelta(t, 0.05, logs.lastLog.ActualCost, 1e-12)
			require.Equal(t, string(BillingModePerRequest), *logs.lastLog.BillingMode)
		})
	}
}

func TestOpenAIChatSettlementLedgerCacheUsesExclusiveBillingBuckets(t *testing.T) {
	round := &OpenAIChatUsageRound{AccountID: 301, UpstreamModel: "gpt-5.1", UpstreamAccepted: true, UpstreamRequestID: "cache-round"}
	input := openAIChatSettlementFixture(round)
	round.IdentityHash = GeminiUsageLedgerIdentity(input.Account.GetCredential("base_url"), input.Account.GetCredential("api_key"))
	upstream := &httpUpstreamRecorder{resp: geminiUsageRecoveryResponse(200, `{"success":true,"data":[{"type":2,"request_id":"cache-round","model_name":"gpt-5.1","prompt_tokens":1000,"completion_tokens":15,"other":"{\"cache_tokens\":800,\"cache_input_includes_cache\":true}"}]}`)}
	logs := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(logs, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.httpUpstream = upstream
	input.APIKey, input.User = &APIKey{ID: 1000, Group: &Group{RateMultiplier: 1}}, &User{ID: 2000}
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Equal(t, 1000, input.Result.Usage.InputTokens)
	require.Equal(t, 200, logs.lastLog.InputTokens)
	require.Equal(t, 800, logs.lastLog.CacheReadTokens)
	require.Equal(t, 15, logs.lastLog.OutputTokens)
	require.Equal(t, UsageTokenSourceUpstream, logs.lastLog.InputTokensSource)
	require.NotNil(t, logs.lastLog.UpstreamRequestID)
	require.Equal(t, "cache-round", *logs.lastLog.UpstreamRequestID, "retain the direct ledger ID without requiring a generic header override")
}

func TestOpenAIChatSettlementInvalidLedgerFallsBackWithoutReplacingAnyReportedField(t *testing.T) {
	for _, body := range []string{
		`{"success":true,"data":[{"type":2,"request_id":"exact-round","model_name":"another-model","prompt_tokens":20000,"completion_tokens":900}]}`,
		`{"success":true,"data":[{"type":2,"request_id":"exact-round","model_name":"gpt-5.1","prompt_tokens":null,"completion_tokens":900}]}`,
		`{"success":true,"data":[{"type":2,"request_id":"exact-round","model_name":"gpt-5.1","prompt_tokens":20000,"completion_tokens":900},{"type":2,"request_id":"exact-round","model_name":"gpt-5.1","prompt_tokens":20000,"completion_tokens":900}]}`,
	} {
		round := &OpenAIChatUsageRound{AccountID: 301, UpstreamModel: "gpt-5.1", UpstreamAccepted: true, UpstreamRequestID: "exact-round",
			OutputReported: true, Usage: OpenAIUsage{OutputTokens: 37}, InputFallback: &GeminiInputUsageFallback{Tokens: 51, Known: true}}
		input := openAIChatSettlementFixture(round)
		round.IdentityHash = GeminiUsageLedgerIdentity(input.Account.GetCredential("base_url"), input.Account.GetCredential("api_key"))
		upstream := &httpUpstreamRecorder{resp: geminiUsageRecoveryResponse(200, body)}
		(&OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}).FinalizeOpenAIChatUsage(context.Background(), input)
		require.Equal(t, 51, input.Result.Usage.InputTokens)
		require.Equal(t, 37, input.Result.Usage.OutputTokens)
		require.Len(t, upstream.requests, 1)
	}
}

func TestOpenAIChatSettlementMeasuredReasoningReplacesOnlyLocalReasoning(t *testing.T) {
	round := &OpenAIChatUsageRound{UpstreamAccepted: true, ReasoningReported: true, ReasoningTokens: 200,
		ReceivedOutputTokens: 80, ReceivedReasoningTokens: 30}
	input := openAIChatSettlementFixture(round)
	(&OpenAIGatewayService{}).FinalizeOpenAIChatUsage(context.Background(), input)
	require.Equal(t, 250, input.Result.Usage.OutputTokens, "50 received answer tokens plus 200 measured reasoning; do not also add 30 local reasoning")
	require.Equal(t, UsageTokenSourceEstimated, input.Result.OutputTokensSource)
}

func TestOpenAIChatSettlementLedgerRejectsRedirectedResponse(t *testing.T) {
	round := &OpenAIChatUsageRound{AccountID: 301, UpstreamModel: "gpt-5.1", UpstreamAccepted: true, UpstreamRequestID: "exact-round",
		InputFallback: &GeminiInputUsageFallback{Tokens: 51, Known: true}, ReceivedOutputTokens: 13}
	input := openAIChatSettlementFixture(round)
	round.IdentityHash = GeminiUsageLedgerIdentity(input.Account.GetCredential("base_url"), input.Account.GetCredential("api_key"))
	response := geminiUsageRecoveryResponse(200, `{"success":true,"data":[{"type":2,"request_id":"exact-round","model_name":"gpt-5.1","prompt_tokens":20000,"completion_tokens":900}]}`)
	response.Request = &http.Request{URL: &url.URL{Scheme: "https", Host: "other.example.com", Path: "/api/log/token"}}
	upstream := &httpUpstreamRecorder{resp: response}
	(&OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}).FinalizeOpenAIChatUsage(context.Background(), input)
	require.Equal(t, 51, input.Result.Usage.InputTokens)
	require.Equal(t, 13, input.Result.Usage.OutputTokens)
	require.Len(t, upstream.requests, 1)
}

func TestOpenAIChatSettlementPreservesMeasuredImageBridgeUsage(t *testing.T) {
	input := openAIChatSettlementFixture(&OpenAIChatUsageRound{InputReported: true, OutputReported: true,
		Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 25}})
	input.Result.ChatUsage.ExtraUsage = OpenAIUsage{InputTokens: 200, OutputTokens: 5, CacheReadInputTokens: 50,
		ImageInputTokens: 150, ImageCacheReadTokens: 20, ImageOutputTokens: 30}
	(&OpenAIGatewayService{}).FinalizeOpenAIChatUsage(context.Background(), input)
	require.Equal(t, OpenAIUsage{InputTokens: 300, OutputTokens: 30, CacheReadInputTokens: 50,
		ImageInputTokens: 150, ImageCacheReadTokens: 20, ImageOutputTokens: 30}, input.Result.Usage)
	require.Equal(t, UsageTokenSourceUpstream, input.Result.InputTokensSource)
	require.Equal(t, UsageTokenSourceUpstream, input.Result.OutputTokensSource)
}

func TestOpenAIChatSettlementOAuthShadowKeepsLogicalAccountAndNeverProbesTokenLedger(t *testing.T) {
	logs := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(logs, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	parent := &Account{ID: 302, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "oauth-token", "base_url": "https://ledger.example.com"}}
	svc.accountRepo = &openAIRecordUsageAccountRepoStub{account: parent}
	upstream := &httpUpstreamRecorder{}
	svc.httpUpstream = upstream
	input := openAIChatSettlementFixture(&OpenAIChatUsageRound{AccountID: 301, UpstreamModel: "gpt-5.1", UpstreamAccepted: true,
		UpstreamRequestID: "shadow-round", IdentityHash: GeminiUsageLedgerIdentity("https://ledger.example.com", "oauth-token"),
		InputFallback: &GeminiInputUsageFallback{Tokens: 1200, Known: true}, ReceivedOutputTokens: 47})
	input.Account.Type, input.Account.ParentAccountID = AccountTypeOAuth, &parent.ID
	input.Account.Credentials = nil
	input.APIKey, input.User = &APIKey{ID: 1000, Group: &Group{RateMultiplier: 1}}, &User{ID: 2000}
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Empty(t, upstream.requests, "OAuth credentials are not token-ledger API keys")
	require.Equal(t, int64(301), logs.lastLog.AccountID)
	require.Equal(t, 1200, logs.lastLog.InputTokens)
	require.Equal(t, 47, logs.lastLog.OutputTokens)
	require.Positive(t, logs.lastLog.ActualCost)
}

func TestOpenAIChatSettlementRejectsUnpersistableAggregateBeforeBilling(t *testing.T) {
	for _, tc := range []struct {
		name   string
		rounds []*OpenAIChatUsageRound
		extra  OpenAIUsage
	}{
		{"input rounds overflow", []*OpenAIChatUsageRound{
			{InputReported: true, OutputReported: true, Usage: OpenAIUsage{InputTokens: math.MaxInt32}},
			{InputReported: true, OutputReported: true, Usage: OpenAIUsage{InputTokens: 1}},
		}, OpenAIUsage{}},
		{"bridge output overflow", []*OpenAIChatUsageRound{{InputReported: true, OutputReported: true, Usage: OpenAIUsage{OutputTokens: 1}}}, OpenAIUsage{OutputTokens: math.MaxInt32}},
		{"reported reasoning merge overflow", []*OpenAIChatUsageRound{{InputReported: true, OutputReported: true, Usage: OpenAIUsage{OutputTokens: int(math.MaxInt32) + 1}}}, OpenAIUsage{}},
		{"cache overflow", []*OpenAIChatUsageRound{{InputReported: true, OutputReported: true, Usage: OpenAIUsage{CacheReadInputTokens: 1}}}, OpenAIUsage{CacheReadInputTokens: math.MaxInt32}},
		{"machine integer overflow", []*OpenAIChatUsageRound{{InputReported: true, OutputReported: true, Usage: OpenAIUsage{InputTokens: 1}}}, OpenAIUsage{InputTokens: int(^uint(0) >> 1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs := &openAIRecordUsageLogRepoStub{inserted: true}
			bills := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, bills, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			input := openAIChatSettlementFixture(tc.rounds...)
			input.Result.ChatUsage.ExtraUsage = tc.extra
			input.APIKey, input.User = &APIKey{ID: 1000, Group: &Group{RateMultiplier: 1}}, &User{ID: 2000}
			err := svc.RecordUsage(context.Background(), input)
			require.ErrorContains(t, err, "token count is outside the supported range")
			require.Zero(t, bills.calls, "invalid detail must not enter wallet/dedup/outbox")
			require.Zero(t, logs.calls)
		})
	}
}

func TestOpenAIChatSettlementAcceptsPersistableIntegerBoundary(t *testing.T) {
	logs := &openAIRecordUsageLogRepoStub{inserted: true}
	bills := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, bills, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	input := openAIChatSettlementFixture(
		&OpenAIChatUsageRound{InputReported: true, OutputReported: true, Usage: OpenAIUsage{InputTokens: math.MaxInt32 - 1}},
		&OpenAIChatUsageRound{InputReported: true, OutputReported: true, Usage: OpenAIUsage{InputTokens: 1}},
	)
	input.APIKey, input.User = &APIKey{ID: 1000, Group: &Group{RateMultiplier: 1}}, &User{ID: 2000}
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Equal(t, math.MaxInt32, logs.lastLog.InputTokens)
	require.Equal(t, 1, bills.calls)
}

func TestOpenAIChatSettlementCacheCannotOffsetAnotherRoundsUncachedInput(t *testing.T) {
	logs := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(logs, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	input := openAIChatSettlementFixture(
		&OpenAIChatUsageRound{UpstreamAccepted: true, CacheReadReported: true, OutputReported: true,
			Usage: OpenAIUsage{CacheReadInputTokens: 300}, InputFallback: &GeminiInputUsageFallback{Tokens: 200, Known: true}},
		&OpenAIChatUsageRound{InputReported: true, OutputReported: true, Usage: OpenAIUsage{InputTokens: 100}},
	)
	input.APIKey, input.User = &APIKey{ID: 1000, Group: &Group{RateMultiplier: 1}}, &User{ID: 2000}
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Equal(t, 100, logs.lastLog.InputTokens, "first round has zero uncached input, never negative input that offsets round two")
	require.Equal(t, 300, logs.lastLog.CacheReadTokens)
	expected := expectedOpenAICost(t, svc, "gpt-5.1", OpenAIUsage{InputTokens: 400, CacheReadInputTokens: 300}, svc.cfg.Default.RateMultiplier)
	require.InDelta(t, expected.ActualCost, logs.lastLog.ActualCost, 1e-12)
}

func TestOpenAIChatSettlementRejectedEmptyRoundDoesNotHideMeasuredUsage(t *testing.T) {
	measured := func() *OpenAIChatUsageRound {
		return &OpenAIChatUsageRound{UpstreamAccepted: true, InputReported: true, OutputReported: true,
			Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 10}}
	}
	rejected := func() *OpenAIChatUsageRound {
		return &OpenAIChatUsageRound{Outcome: ResponseOutcome{UpstreamStatus: http.StatusServiceUnavailable,
			DisconnectSource: DisconnectSourceUpstream, UpstreamErrorKind: UpstreamErrorHTTP5xx, CollectorVersion: ResponseOutcomeCollectorVersion}}
	}
	for _, tc := range []struct {
		name                      string
		rounds                    []*OpenAIChatUsageRound
		extra                     OpenAIUsage
		want                      OpenAIUsage
		inputSource, outputSource UsageTokenSource
	}{
		{"rejected continuation", []*OpenAIChatUsageRound{measured(), rejected()}, OpenAIUsage{}, OpenAIUsage{InputTokens: 100, OutputTokens: 10}, UsageTokenSourceUpstream, UsageTokenSourceUpstream},
		{"rejected earlier round", []*OpenAIChatUsageRound{rejected(), measured()}, OpenAIUsage{}, OpenAIUsage{InputTokens: 100, OutputTokens: 10}, UsageTokenSourceUpstream, UsageTokenSourceUpstream},
		{"bridge then rejected chat", []*OpenAIChatUsageRound{rejected()}, OpenAIUsage{InputTokens: 50, OutputTokens: 2}, OpenAIUsage{InputTokens: 50, OutputTokens: 2}, UsageTokenSourceUpstream, UsageTokenSourceUpstream},
		{"all rounds rejected", []*OpenAIChatUsageRound{rejected(), rejected()}, OpenAIUsage{}, OpenAIUsage{}, UsageTokenSourceUnreported, UsageTokenSourceUnreported},
		{"accepted unknown round remains relevant", []*OpenAIChatUsageRound{measured(), {UpstreamAccepted: true}}, OpenAIUsage{}, OpenAIUsage{InputTokens: 100, OutputTokens: 10}, UsageTokenSourceUnreported, UsageTokenSourceEstimated},
		{"partial explicit zero remains relevant", []*OpenAIChatUsageRound{measured(), {InputReported: true}}, OpenAIUsage{}, OpenAIUsage{InputTokens: 100, OutputTokens: 10}, UsageTokenSourceUpstream, UsageTokenSourceUnreported},
		{"semantic output without count remains relevant", []*OpenAIChatUsageRound{measured(), {Outcome: ResponseOutcome{HasToolCall: true}}}, OpenAIUsage{}, OpenAIUsage{InputTokens: 100, OutputTokens: 10}, UsageTokenSourceUnreported, UsageTokenSourceUnreported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs := &openAIRecordUsageLogRepoStub{inserted: true}
			bills := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, bills, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			input := openAIChatSettlementFixture(tc.rounds...)
			input.Result.ChatUsage.ExtraUsage = tc.extra
			outcome := &ResponseOutcome{HTTPStatus: http.StatusOK, UpstreamStatus: http.StatusServiceUnavailable,
				DisconnectSource: DisconnectSourceUpstream, UpstreamErrorKind: UpstreamErrorHTTP5xx, HasText: tc.want.OutputTokens > 0,
				CollectorVersion: ResponseOutcomeCollectorVersion}
			input.Result.Outcome = outcome
			input.APIKey, input.User = &APIKey{ID: 1000, Group: &Group{RateMultiplier: 1}}, &User{ID: 2000}
			require.NoError(t, svc.RecordUsage(context.Background(), input))
			require.Equal(t, tc.want, input.Result.Usage)
			if tc.name == "all rounds rejected" {
				require.Equal(t, tc.inputSource, input.Result.InputTokensSource)
				require.Equal(t, tc.outputSource, input.Result.OutputTokensSource)
				require.Equal(t, *outcome, *input.Result.Outcome)
				require.Zero(t, bills.calls)
				require.Nil(t, logs.lastLog)
				return
			}
			require.Equal(t, tc.inputSource, logs.lastLog.InputTokensSource, "a rejected zero round may not hide a previous precise input count")
			require.Equal(t, tc.outputSource, logs.lastLog.OutputTokensSource)
			require.Equal(t, tc.want.InputTokens, logs.lastLog.InputTokens)
			require.Equal(t, tc.want.OutputTokens, logs.lastLog.OutputTokens)
			require.Equal(t, *outcome, *logs.lastLog.Outcome, "source aggregation must retain the final request failure")
			require.Equal(t, 1, bills.calls, "source correction may not create another settlement")
		})
	}
}
