package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

type acceptanceExecutorAccounts struct {
	AccountRepository
	a *Account
}

type acceptanceExecutorGroups struct {
	GroupRepository
	group *Group
}

func (r *acceptanceExecutorGroups) GetByID(context.Context, int64) (*Group, error) {
	return r.group, nil
}

func TestAcceptanceExecutorDiscountKeepsThreeUsageLayers(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, protocol := range []string{"gemini", "chat"} {
			a := &Account{ID: 7, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
			g := &Group{ID: 8, Platform: PlatformGemini, Status: StatusActive, Hydrated: true, GeminiInputCountingEnabled: enabled, RateMultiplier: 2}
			u := &acceptanceExecutorHTTP{status: 200, contentType: "application/json", body: `{"candidates":[{"content":{"parts":[{"text":"acceptance-ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1000,"candidatesTokenCount":9,"thoughtsTokenCount":2,"cachedContentTokenCount":0,"totalTokenCount":1011}}`}
			e := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: a}, &acceptanceExecutorGroups{group: g}, &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: u}, nil, nil, nil)
			sample := acceptanceExecutorSample(a, protocol, false)
			sample.Snapshot.GroupID = &g.ID
			sample.Snapshot.Fingerprint = AcceptanceAccountFingerprint(a, g)
			r, err := e.Execute(context.Background(), sample)
			if err != nil || r.RawUsage.Input == nil || *r.RawUsage.Input != 1000 || r.ClientUsage.Input == nil || r.BillingUsage.Input == nil {
				t.Fatalf("lost usage: %v %+v", err, r)
			}
			if *r.ClientUsage.Input != *r.BillingUsage.Input || enabled && (*r.ClientUsage.Input >= 1000 || *r.ClientUsage.Input <= 0) || !enabled && *r.ClientUsage.Input != 1000 {
				t.Fatalf("discount projection differs: %+v", r)
			}
			if enabled && r.EffectiveParameters["input_counting_policy"] != GeminiInputCountingVertexLikeDiscountV1 {
				t.Fatal("discount rule hidden")
			}
		}
	}
}

func (r *acceptanceExecutorAccounts) GetByID(context.Context, int64) (*Account, error) {
	return r.a, nil
}

// Every other method on the embedded nil interface panics: any attempted write
// to this real repository fails the tests instead of silently passing.
type acceptanceExecutorHTTP struct {
	calls             int
	status            int
	body, contentType string
	request           *http.Request
	fn                func(*http.Request)
}

func (u *acceptanceExecutorHTTP) Do(r *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.calls++
	u.request = r
	if u.fn != nil {
		u.fn(r)
	}
	return &http.Response{StatusCode: u.status, Header: http.Header{"Content-Type": []string{u.contentType}}, Body: io.NopCloser(strings.NewReader(u.body)), Request: r}, nil
}
func (u *acceptanceExecutorHTTP) DoWithTLS(r *http.Request, p string, id int64, n int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(r, p, id, n)
}
func acceptanceExecutorSample(a *Account, protocol string, stream bool) AcceptanceSample {
	return AcceptanceSample{Snapshot: AcceptanceSnapshot{AccountID: a.ID, Model: "gemini-3.8-flash", UpstreamModel: "gemini-3.8-flash", Fingerprint: AcceptanceAccountFingerprint(a, nil)}, Protocol: protocol, Category: "protocol", Prompt: "Reply acceptance-ok", Expected: "acceptance-ok", Stream: stream, MaxOutputTokens: 2048, TimeoutSeconds: 5}
}
func TestAcceptanceExecutorProductionGeminiMatrix(t *testing.T) {
	for _, kind := range []string{AccountTypeAPIKey, AccountTypeServiceAccount} {
		for _, protocol := range []string{"gemini", "chat"} {
			for _, stream := range []bool{false, true} {
				t.Run(kind+protocol+map[bool]string{false: "JSON", true: "SSE"}[stream], func(t *testing.T) {
					a := &Account{ID: 7, Platform: PlatformGemini, Type: kind, Credentials: map[string]any{"api_key": "secret-acceptance-key", "auth_mode": "apikey"}}
					body := `{"candidates":[{"content":{"parts":[{"text":"acceptance-ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":9,"thoughtsTokenCount":2,"cachedContentTokenCount":10,"totalTokenCount":111}}`
					contentType := "application/json"
					if stream {
						body = "data: " + body + "\n\n"
						contentType = "text/event-stream"
					}
					u := &acceptanceExecutorHTTP{status: 200, body: body, contentType: contentType}
					svc := &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: u}
					e := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: a}, nil, svc, nil, nil, nil)
					result, err := e.Execute(context.Background(), acceptanceExecutorSample(a, protocol, stream))
					if err != nil {
						t.Fatal(err)
					}
					if u.calls != 1 || result.Status != "passed" || !result.UpstreamUsageFinal || result.RawUsage.Output == nil || *result.RawUsage.Output != 9 || *result.RawUsage.Thoughts != 2 || *result.RawUsage.Input != 100 {
						t.Fatalf("bad result %+v calls=%d", result, u.calls)
					}
					clientOutput := int64(9)
					if protocol == "chat" && result.ClientUsage.Thoughts == nil {
						clientOutput = 11
					}
					if result.ClientUsage.Output == nil || *result.ClientUsage.Output != clientOutput {
						t.Fatalf("client output %+v", result.ClientUsage)
					}
					if result.BillingUsage.Input == nil || *result.BillingUsage.Input != 100 || result.BillingUsage.Output == nil || *result.BillingUsage.Output != 9 {
						t.Fatalf("billing %+v", result.BillingUsage)
					}
					raw, _ := json.Marshal(result)
					if strings.Contains(string(raw), "secret-acceptance-key") {
						t.Fatal("secret leaked")
					}
				})
			}
		}
	}
}
func TestAcceptanceExecutorRawUsageSurvivesClientProjection(t *testing.T) {
	a := &Account{ID: 7, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
	u := &acceptanceExecutorHTTP{status: 200, contentType: "application/json", body: `{"candidates":[{"content":{"parts":[{"text":"acceptance-ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":0,"cachedContentTokenCount":0}}`}
	e := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: a}, nil, &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: u}, nil, nil, nil)
	r, err := e.Execute(context.Background(), acceptanceExecutorSample(a, "chat", false))
	if err != nil {
		t.Fatal(err)
	}
	if r.RawUsage.Output == nil || *r.RawUsage.Output != 0 || r.ClientUsage.Output == nil || *r.ClientUsage.Output == 0 {
		t.Fatalf("raw/client not independent %+v", r)
	}
}
func TestAcceptanceExecutorNoHealthMutationOnFailure(t *testing.T) {
	for _, platform := range []string{PlatformGemini, PlatformAntigravity} {
		for _, status := range []int{400, 401, 404, 429, 500} {
			t.Run(platform+http.StatusText(status), func(t *testing.T) {
				a := &Account{ID: 7, Platform: platform, Type: AccountTypeAPIKey, Status: StatusActive, Credentials: map[string]any{"api_key": "test", "access_token": "token", "project_id": "project", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)}}
				if platform == PlatformAntigravity {
					a.Type = AccountTypeOAuth
				}
				repo := &acceptanceExecutorAccounts{a: a}
				u := &acceptanceExecutorHTTP{status: status, contentType: "application/json", body: `{"error":{"message":"upstream rejected test","status":"RESOURCE_EXHAUSTED"}}`}
				e := NewAccountAcceptanceExecutor(repo, nil, &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: u}, &AntigravityGatewayService{httpUpstream: u, tokenProvider: &AntigravityTokenProvider{}}, nil, nil)
				r, err := e.Execute(context.Background(), acceptanceExecutorSample(a, "gemini", false))
				if err != nil {
					t.Fatal(err)
				}
				if r.Status != "error" || u.calls != 1 || a.Status != StatusActive || a.RateLimitResetAt != nil {
					t.Fatalf("mutated/retried %+v calls=%d", r, u.calls)
				}
			})
		}
	}
}
func TestAcceptanceExecutorCountFallbackIsNotMeasured(t *testing.T) {
	a := &Account{ID: 7, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
	for _, status := range []int{200, 403} {
		u := &acceptanceExecutorHTTP{status: status, contentType: "application/json", body: `{"totalTokens":123}`}
		if status == 403 {
			u.body = `{"error":{"message":"insufficient authentication scopes","status":"PERMISSION_DENIED"}}`
		}
		e := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: a}, nil, &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: u}, nil, nil, nil)
		r, err := e.Execute(context.Background(), acceptanceExecutorSample(a, "count_tokens", false))
		if err != nil {
			t.Fatal(err)
		}
		if status == 200 && (r.Status != "passed" || r.RawUsage.Input == nil || *r.RawUsage.Input != 123) {
			t.Fatalf("%+v", r)
		}
		if status == 403 && (r.Status != "unsupported" || r.RawUsage.Input != nil) {
			t.Fatalf("%+v", r)
		}
		if r.CostUSD == nil || *r.CostUSD != 0 {
			t.Fatal("count cost unclassified")
		}
	}
}

func TestAcceptanceExecutorCountEndpointCannotSilentlyGenerate(t *testing.T) {
	a := &Account{ID: 7, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
	u := &acceptanceExecutorHTTP{status: 200, contentType: "application/json", body: `{"candidates":[{"content":{"parts":[{"text":"generated reply"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":31,"totalTokenCount":131}}`}
	e := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: a}, nil, &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: u}, nil, nil, nil)
	r, err := e.Execute(context.Background(), acceptanceExecutorSample(a, "count_tokens", false))
	if err != nil || r.Status != "anomaly" || r.CostUSD != nil || !acceptanceHasFinding(r, "count_returned_generation") || !r.UpstreamUsageFinal {
		t.Fatalf("generation was accepted as a free counter: %v %+v", err, r)
	}
}
func TestAcceptanceExecutorToolResultUsesArguments(t *testing.T) {
	a := &Account{ID: 7, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
	for _, protocol := range []string{"gemini", "chat"} {
		u := &acceptanceExecutorHTTP{status: 200, contentType: "application/json", body: `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"acceptance_echo","args":{"value":"nonce-1"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":9}}`}
		e := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: a}, nil, &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: u}, nil, nil, nil)
		s := acceptanceExecutorSample(a, protocol, false)
		s.Category = "tools"
		s.Expected = "nonce-1"
		r, err := e.Execute(context.Background(), s)
		if err != nil || r.Status != "passed" || !r.HasTool {
			t.Fatalf("%v %+v", err, r)
		}
	}
}
func TestAcceptanceExecutorOpenAIChat(t *testing.T) {
	a := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test", "base_url": "https://compat.example.test"}, Extra: map[string]any{"api_protocol": "chat_completions", "openai_responses_supported": false}}
	u := &acceptanceExecutorHTTP{status: 200, contentType: "application/json", body: `{"choices":[{"message":{"role":"assistant","content":"acceptance-ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":11,"completion_tokens_details":{"reasoning_tokens":2}}}`}
	e := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: a}, nil, nil, nil, &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: u}, nil)
	r, err := e.Execute(context.Background(), acceptanceExecutorSample(a, "chat", false))
	if err != nil || r.Status != "passed" || u.calls != 1 || r.RawUsage.Output == nil || *r.RawUsage.Output != 9 {
		t.Fatalf("%v %+v calls=%d", err, r, u.calls)
	}
}

func TestAcceptanceEffectiveInputFingerprintMatchesActualInputOnly(t *testing.T) {
	capture := func(body string) *AcceptanceResult {
		r, _ := http.NewRequest(http.MethodPost, "https://provider.test/v1beta/models/gemini-3.8-flash:generateContent", strings.NewReader(body))
		result := &AcceptanceResult{EffectiveParameters: map[string]any{}}
		acceptanceCaptureParameters(r, result)
		return result
	}
	a := capture(`{"contents":[{"role":"user","parts":[{"text":"synthetic"}]}],"systemInstruction":{"parts":[{"text":"injected"}]},"generationConfig":{"maxOutputTokens":100}}`)
	b := capture(`{"generationConfig":{"maxOutputTokens":2048},"systemInstruction":{"parts":[{"text":"injected"}]},"contents":[{"role":"user","parts":[{"text":"synthetic"}]}]}`)
	c := capture(`{"contents":[{"role":"user","parts":[{"text":"synthetic"}]}]}`)
	if a.EffectiveParameters["input_fingerprint"] == nil || a.EffectiveParameters["input_fingerprint"] != b.EffectiveParameters["input_fingerprint"] || a.EffectiveParameters["input_fingerprint"] == c.EffectiveParameters["input_fingerprint"] {
		t.Fatalf("fingerprints do not bind injected inputs: %+v %+v %+v", a, b, c)
	}
}
func TestAcceptanceSingleTransportAttempt(t *testing.T) {
	base := &acceptanceExecutorHTTP{status: 200, contentType: "application/json", body: `{}`}
	u := &acceptanceHTTPUpstream{base: base, accountID: 7, parser: newAcceptanceEvidenceParser(), result: &AcceptanceResult{EffectiveParameters: map[string]any{}}, sample: AcceptanceSample{}}
	r, _ := http.NewRequest(http.MethodPost, "https://provider.test/v1/chat/completions", strings.NewReader(`{"messages":[]}`))
	resp, err := u.Do(r, "", 7, 1)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if _, err = u.Do(r, "", 7, 1); err == nil || base.calls != 1 {
		t.Fatal("audit retry reached upstream")
	}
}
func TestAcceptanceBillingDoesNotInventMissingZero(t *testing.T) {
	u := acceptanceBillingProjection(&ForwardResult{InputTokensSource: UsageTokenSourceUnreported, OutputTokensSource: UsageTokenSourceUnknown}, nil, AcceptanceUsage{})
	if u.Input != nil || u.Output != nil || u.Total != nil {
		t.Fatalf("missing was converted to zero %+v", u)
	}
}
func TestAcceptanceCacheThresholdRequiresKnownModel(t *testing.T) {
	for model, want := range map[string]int{
		"gemini-2.5-pro": 2048, "gemini-2.5-flash": 2048,
		"gemini-3.1-pro-preview": 4096, "gemini-3.8-flash": 4096,
		"gemini-4-argon": 4096, "gemini-4-carbon": 4096,
		"gemini-future": 0, "gemini-2.5-flash-lite": 0, "gemini-2.5-flash-lite-preview-09-2025": 0,
		"gemini-2.5-flash-image": 0, "gemini-3.8-flash-tts": 0, "gemini-4-argon-third-party": 0,
	} {
		if got := acceptanceCacheMinimum(model); got != want {
			t.Fatalf("cache threshold %s=%d, want %d", model, got, want)
		}
	}
}

func TestAcceptanceTransportDisablesCredentialRedirects(t *testing.T) {
	for _, tls := range []bool{false, true} {
		base := &acceptanceExecutorHTTP{status: 307, body: `{}`, fn: func(r *http.Request) {
			if !HTTPUpstreamRedirectsDisabled(r.Context()) {
				t.Fatal("credential-bearing audit may redirect")
			}
		}}
		u := &acceptanceHTTPUpstream{base: base, accountID: 7, parser: newAcceptanceEvidenceParser(), result: &AcceptanceResult{EffectiveParameters: map[string]any{}}}
		r, _ := http.NewRequest(http.MethodPost, "https://provider.test/v1/chat/completions", strings.NewReader(`{}`))
		var resp *http.Response
		var err error
		if tls {
			resp, err = u.DoWithTLS(r, "", 7, 1, &tlsfingerprint.Profile{})
		} else {
			resp, err = u.Do(r, "", 7, 1)
		}
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}
}
func TestAcceptanceOpenAIBillingInputAlreadyIncludesCache(t *testing.T) {
	input, output, cache := int64(100), int64(10), int64(30)
	got := acceptanceBillingProjection(nil, &OpenAIForwardResult{Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 10, CacheReadInputTokens: 30}}, AcceptanceUsage{Input: &input, Output: &output, CacheRead: &cache})
	if got.Input == nil || *got.Input != 100 || *got.CacheRead != 30 {
		t.Fatalf("cache counted twice: %+v", got)
	}
}
func TestAcceptanceFingerprintIncludesResponsesInstructions(t *testing.T) {
	capture := func(instructions string) any {
		r, _ := http.NewRequest(http.MethodPost, "https://provider.test/v1/responses", strings.NewReader(`{"input":"synthetic","instructions":"`+instructions+`","model":"gemini-model"}`))
		result := &AcceptanceResult{EffectiveParameters: map[string]any{}}
		acceptanceCaptureParameters(r, result)
		return result.EffectiveParameters["input_fingerprint"]
	}
	if capture("one") == nil || capture("one") == capture("two") {
		t.Fatal("Responses instruction not bound to input")
	}
}
func TestAcceptanceAuditCannotEnterSettlement(t *testing.T) {
	ctx := context.WithValue(context.Background(), channelAcceptanceAuditKey{}, true)
	r := &ForwardResult{RequireMeasuredUsage: true, MeasuredUsageComplete: true, InputTokensSource: UsageTokenSourceUpstream}
	(&GatewayService{}).FinalizeGeminiUsageFallback(ctx, &RecordUsageInput{Result: r})
	if r.GeminiUsageFinalized {
		t.Fatal("audit entered Gemini settlement")
	}
	o := &OpenAIForwardResult{ChatUsage: &OpenAIChatUsageEvidence{}}
	(&OpenAIGatewayService{}).FinalizeOpenAIChatUsage(ctx, &OpenAIRecordUsageInput{Result: o})
	if o.ChatUsage.Finalized {
		t.Fatal("audit entered OpenAI settlement")
	}
}

func acceptanceHasFinding(r *AcceptanceResult, code string) bool {
	for _, finding := range r.Findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func TestAcceptanceExecutorIncompleteStreamRetainsPartialUsage(t *testing.T) {
	for _, protocol := range []string{"gemini", "chat"} {
		t.Run(protocol, func(t *testing.T) {
			a := &Account{ID: 7, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
			u := &acceptanceExecutorHTTP{status: 200, contentType: "text/event-stream", body: "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"acceptance-ok\"}]}}],\"usageMetadata\":{\"promptTokenCount\":100,\"candidatesTokenCount\":5,\"thoughtsTokenCount\":0,\"totalTokenCount\":105}}\n\n"}
			e := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: a}, nil, &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: u}, nil, nil, nil)
			r, err := e.Execute(context.Background(), acceptanceExecutorSample(a, protocol, true))
			if err != nil {
				t.Fatal(err)
			}
			if r.Status == "passed" || r.UpstreamUsageFinal || r.RawUsage.Output == nil || *r.RawUsage.Output != 5 || !acceptanceHasFinding(r, "upstream_usage_not_final") {
				t.Fatalf("partial usage accepted as final: %+v", r)
			}
			if !r.Completed && !acceptanceHasFinding(r, "stream_incomplete") {
				t.Fatalf("missing explicit stream finding: %+v", r)
			}
		})
	}
}

func TestAcceptanceExecutorReasoningOnlyIsInconclusive(t *testing.T) {
	for _, stream := range []bool{false, true} {
		a := &Account{ID: 7, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
		body := `{"candidates":[{"content":{"parts":[{"text":"synthetic thinking","thought":true}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":0,"thoughtsTokenCount":5,"totalTokenCount":105}}`
		contentType := "application/json"
		if stream {
			body, contentType = "data: "+body+"\n\n", "text/event-stream"
		}
		u := &acceptanceExecutorHTTP{status: 200, contentType: contentType, body: body}
		e := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: a}, nil, &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: u}, nil, nil, nil)
		r, err := e.Execute(context.Background(), acceptanceExecutorSample(a, "gemini", stream))
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != "inconclusive" || !r.HasReasoning || r.HasText || r.HasTool || !acceptanceHasFinding(r, "reasoning_only") || acceptanceHasFinding(r, "challenge_mismatch") {
			t.Fatalf("reasoning-only response misclassified: %+v", r)
		}
	}
}

func TestAcceptanceNativeThinkingUsesKnownSupportedRanges(t *testing.T) {
	for _, tc := range []struct {
		model  string
		budget float64
		level  string
	}{
		{"gemini-2.5-flash-lite", 512, ""}, {"gemini-2.5-flash-lite-preview-09-2025", 512, ""},
		{"gemini-2.5-flash", 128, ""}, {"gemini-2.5-pro", 128, ""},
		{"gemini-3.1-pro-preview", 0, "low"}, {"gemini-3.8-flash", 0, "low"},
		{"gemini-4-argon", 0, "low"}, {"gemini-4-carbon", 0, "low"},
		{"gemini-2.0-flash", 0, ""}, {"gemini-future", 0, ""}, {"thirdparty-gemini-4-argon", 0, ""},
	} {
		t.Run(tc.model, func(t *testing.T) {
			body, err := acceptanceRequestBody(AcceptanceSample{Protocol: "gemini", Snapshot: AcceptanceSnapshot{Model: tc.model, UpstreamModel: tc.model}, MaxOutputTokens: 2048})
			if err != nil {
				t.Fatal(err)
			}
			var request map[string]any
			if err := json.Unmarshal(body, &request); err != nil {
				t.Fatal(err)
			}
			thinking := acceptanceObject(acceptanceObject(request["generationConfig"])["thinkingConfig"])
			if tc.budget != 0 && thinking["thinkingBudget"] != tc.budget || tc.level != "" && thinking["thinkingLevel"] != tc.level || tc.budget == 0 && tc.level == "" && thinking != nil {
				t.Fatalf("unsupported thinking parameters: %+v", thinking)
			}
		})
	}
}

func TestAcceptanceBillingOriginsDescribeProjectionOnly(t *testing.T) {
	for _, openai := range []bool{false, true} {
		r := &AcceptanceResult{EffectiveParameters: map[string]any{}}
		if openai {
			acceptanceCaptureBillingOrigins(r, nil, &OpenAIForwardResult{InputTokensSource: UsageTokenSourceUpstream, OutputTokensSource: UsageTokenSourceEstimated})
		} else {
			acceptanceCaptureBillingOrigins(r, &ForwardResult{InputTokensSource: UsageTokenSourceUpstream, OutputTokensSource: UsageTokenSourceEstimated}, nil)
		}
		projection := acceptanceObject(r.EffectiveParameters["billing_projection"])
		if r.EffectiveParameters["input_tokens_source"] != "upstream" || r.EffectiveParameters["output_tokens_source"] != "estimated" || projection["customer_charged"] != false || projection["settlement_performed"] != false || projection["kind"] != "simulation" {
			t.Fatalf("projection provenance missing: %+v", r.EffectiveParameters)
		}
	}
}

func TestAcceptanceAuditCannotTrainFirstOutputRouting(t *testing.T) {
	const accountID int64 = 9223372036854775700
	model := "acceptance-isolated-routing-test"
	key := geminiFirstOutputKey(accountID, model)
	t.Cleanup(func() {
		defaultGeminiFirstOutputTracker.mu.Lock()
		delete(defaultGeminiFirstOutputTracker.data, key)
		defaultGeminiFirstOutputTracker.mu.Unlock()
	})
	ctx := context.WithValue(context.Background(), channelAcceptanceAuditKey{}, true)
	recordGeminiFirstOutput(ctx, accountID, model, time.Second, false)
	recordGeminiFirstOutput(ctx, accountID, model, 0, true)
	if stats, exists := defaultGeminiFirstOutputTracker.snapshotAt(accountID, model, time.Now()); exists || stats.SampleCount != 0 {
		t.Fatalf("synthetic traffic trained production routing: %+v", stats)
	}
	recordGeminiFirstOutput(context.Background(), accountID, model, time.Second, false)
	if stats, exists := defaultGeminiFirstOutputTracker.snapshotAt(accountID, model, time.Now()); !exists || stats.SampleCount != 1 {
		t.Fatalf("ordinary routing recording regressed: %+v", stats)
	}
}
