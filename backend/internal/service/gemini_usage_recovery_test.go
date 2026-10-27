//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

const geminiUsageRecoveryLedgerFixture = `{"success":true,"data":[{"type":2,"request_id":"provider-request-1","model_name":"gemini-2.5-flash","prompt_tokens":21916,"completion_tokens":4055,"other":"{\"cache_tokens\":0,\"cache_input_includes_cache\":true}"}]}`

func geminiUsageRecoveryResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func newGeminiUsageRecoveryFixture() (*RecordUsageInput, *httpUpstreamRecorder) {
	account := &Account{ID: 5957, Platform: PlatformGemini, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"base_url": "https://ledger.example.com/gateway/v1", "api_key": "local-ledger-key"}}
	return &RecordUsageInput{Account: account, Result: &ForwardResult{
		Model: geminiInputUsageRegressionModel, UpstreamModel: geminiInputUsageRegressionModel,
		RequireMeasuredUsage: true, InputTokensSource: UsageTokenSourceUnreported, OutputTokensSource: UsageTokenSourceUnreported,
		UsageLedgerRequestID: "provider-request-1", UsageLedgerIdentityHash: GeminiUsageLedgerIdentity(account.GetCredential("base_url"), account.GetCredential("api_key")),
	}}, &httpUpstreamRecorder{resp: geminiUsageRecoveryResponse(200, geminiUsageRecoveryLedgerFixture)}
}

func TestGeminiUsageRecovery_ExactLedgerUsageSurvivesCanceledContextWithoutBilling(t *testing.T) {
	input, upstream := newGeminiUsageRecoveryFixture()
	svc, logs, billing := newGeminiInputUsageRegressionBilling()
	svc.httpUpstream = upstream
	svc.cfg = &config.Config{}
	input.Result.Usage.OutputTokens = 5071
	input.Result.EstimatedInputTokens = 90000
	input.Result.InputTokensEstimated = true
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	recovered, err := svc.RecoverGeminiUsage(ctx, input)
	require.NoError(t, err)
	require.True(t, recovered)
	require.Equal(t, 21916, input.Result.Usage.InputTokens)
	require.Equal(t, 4055, input.Result.Usage.OutputTokens)
	require.Equal(t, UsageTokenSourceUpstream, input.Result.InputTokensSource)
	require.Equal(t, UsageTokenSourceUpstream, input.Result.OutputTokensSource)
	require.True(t, input.Result.MeasuredUsageComplete)
	require.Zero(t, input.Result.EstimatedInputTokens)
	require.False(t, input.Result.InputTokensEstimated)
	require.Zero(t, billing.calls)
	require.Nil(t, logs.lastLog, "recovery itself may not write usage or bill")
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://ledger.example.com/api/log/token", upstream.lastReq.URL.String())
	require.Equal(t, http.MethodGet, upstream.lastReq.Method)
	require.Equal(t, "Bearer local-ledger-key", upstream.lastReq.Header.Get("Authorization"))
	require.True(t, HTTPUpstreamRedirectsDisabled(upstream.lastReq.Context()))
	require.Empty(t, upstream.lastReq.URL.RawQuery, "credential/request ID must never enter the URL")
}

func TestGeminiUsageRecovery_IneligibleOrChangedIdentityNeverFetches(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*RecordUsageInput)
	}{
		{"missing_request_id", func(in *RecordUsageInput) { in.Result.UsageLedgerRequestID = "" }},
		{"missing_identity", func(in *RecordUsageInput) { in.Result.UsageLedgerIdentityHash = "" }},
		{"changed_key", func(in *RecordUsageInput) { in.Account.Credentials["api_key"] = "different-key" }},
		{"changed_origin", func(in *RecordUsageInput) { in.Account.Credentials["base_url"] = "https://changed.example.com" }},
		{"missing_key", func(in *RecordUsageInput) { in.Account.Credentials["api_key"] = "" }},
		{"already_complete", func(in *RecordUsageInput) { in.Result.MeasuredUsageComplete = true }},
		{"image_policy", func(in *RecordUsageInput) { in.Result.RequireMeasuredUsage = false }},
		{"other_platform", func(in *RecordUsageInput) { in.Account.Platform = PlatformOpenAI }},
		{"oauth", func(in *RecordUsageInput) { in.Account.Type = AccountTypeOAuth }},
		{"service_account", func(in *RecordUsageInput) { in.Account.Type = AccountTypeServiceAccount }},
		{"missing_proxy", func(in *RecordUsageInput) { id := int64(12); in.Account.ProxyID = &id }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, upstream := newGeminiUsageRecoveryFixture()
			tc.edit(input)
			before := input.Result.Usage
			svc := &GatewayService{httpUpstream: upstream, cfg: &config.Config{}}
			recovered, _ := svc.RecoverGeminiUsage(context.Background(), input)
			require.False(t, recovered)
			require.Empty(t, upstream.requests)
			require.Equal(t, before, input.Result.Usage)
		})
	}
}

func TestGeminiUsageRecovery_RetriesOnlyMissingLedgerEntry(t *testing.T) {
	input, upstream := newGeminiUsageRecoveryFixture()
	upstream.responses = []*http.Response{
		geminiUsageRecoveryResponse(200, `{"success":true,"data":[]}`),
		geminiUsageRecoveryResponse(200, geminiUsageRecoveryLedgerFixture),
	}
	svc := &GatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	recovered, err := svc.RecoverGeminiUsage(context.Background(), input)
	require.NoError(t, err)
	require.True(t, recovered)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, 21916, input.Result.Usage.InputTokens)
}

func TestGeminiUsageRecovery_NoMatchLeavesReviewAndPreventsEstimatedBilling(t *testing.T) {
	input, upstream := newGeminiUsageRecoveryFixture()
	upstream.responses = make([]*http.Response, 4)
	for i := range upstream.responses {
		upstream.responses[i] = geminiUsageRecoveryResponse(200, `{"success":true,"data":[]}`)
	}
	svc, logs, billing := newGeminiInputUsageRegressionBilling()
	svc.httpUpstream, svc.cfg = upstream, &config.Config{}
	input.APIKey, input.User = &APIKey{ID: 501}, &User{ID: 601}
	input.Result.EstimatedInputTokens = 90000
	recovered, err := svc.RecoverGeminiUsage(context.Background(), input)
	require.NoError(t, err)
	require.False(t, recovered)
	require.True(t, NeedsUsageMeasurementReview(input.Result))
	require.Zero(t, input.Result.Usage.InputTokens)
	require.Len(t, upstream.requests, 4)
	require.ErrorIs(t, svc.RecordUsage(context.Background(), input), ErrUsageMeasurementIncomplete)
	require.Zero(t, billing.calls)
	require.Nil(t, logs.lastLog)
}

func TestGeminiUsageRecovery_FailureIsSanitizedAndNotRetried(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response *http.Response
		err      error
	}{
		{"http_error", geminiUsageRecoveryResponse(403, "sensitive-provider-body"), nil},
		{"malformed", geminiUsageRecoveryResponse(200, "sensitive-provider-body"), nil},
		{"transport", nil, errors.New("https://sensitive-host?key=sensitive-key")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, upstream := newGeminiUsageRecoveryFixture()
			upstream.resp, upstream.err = tc.response, tc.err
			recovered, err := (&GatewayService{httpUpstream: upstream, cfg: &config.Config{}}).RecoverGeminiUsage(context.Background(), input)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "sensitive")
			require.False(t, recovered)
			require.Len(t, upstream.requests, 1)
			require.True(t, NeedsUsageMeasurementReview(input.Result))
		})
	}
}

func TestGeminiUsageRecovery_RealHTTPRedirectNeverReceivesCredential(t *testing.T) {
	var redirected atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1); w.WriteHeader(200) }))
	t.Cleanup(target.Close)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/collect", http.StatusFound)
	}))
	t.Cleanup(origin.Close)
	input, _ := newGeminiUsageRecoveryFixture()
	input.Account.Credentials["base_url"] = origin.URL
	input.Result.UsageLedgerIdentityHash = GeminiUsageLedgerIdentity(origin.URL, input.Account.GetCredential("api_key"))
	u, err := url.Parse(origin.URL)
	require.NoError(t, err)
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	svc := &GatewayService{cfg: cfg, httpUpstream: &geminiRealHTTPUsageUpstream{client: origin.Client(), target: u}}
	recovered, err := svc.RecoverGeminiUsage(context.Background(), input)
	require.Error(t, err)
	require.False(t, recovered)
	require.Zero(t, redirected.Load())
}

func TestGeminiUsageRecovery_RealHTTPHasAbsoluteTimeout(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	t.Cleanup(origin.Close)
	input, _ := newGeminiUsageRecoveryFixture()
	input.Account.Credentials["base_url"] = origin.URL
	input.Result.UsageLedgerIdentityHash = GeminiUsageLedgerIdentity(origin.URL, input.Account.GetCredential("api_key"))
	u, err := url.Parse(origin.URL)
	require.NoError(t, err)
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	svc := &GatewayService{cfg: cfg, httpUpstream: &geminiRealHTTPUsageUpstream{client: origin.Client(), target: u}}
	started := time.Now()
	recovered, err := svc.RecoverGeminiUsage(context.Background(), input)
	require.Error(t, err)
	require.False(t, recovered)
	require.Less(t, time.Since(started), 5*time.Second)
}
