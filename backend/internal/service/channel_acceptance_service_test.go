package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

type acceptanceAccountsStub struct {
	AccountRepository
	account *Account
}

func TestAcceptanceShadowCannotBorrowParentCredentials(t *testing.T) {
	parent := int64(12)
	a := &Account{ID: 13, Platform: PlatformOpenAI, Type: AccountTypeOAuth, ParentAccountID: &parent}
	s := NewChannelAcceptanceService(nil, &acceptanceAccountsStub{account: a}, nil, nil, nil)
	targets, err := s.Targets(context.Background(), []int64{13})
	if err != nil || len(targets) != 1 || !strings.Contains(targets[0].UnavailableReason, "母账号") || len(targets[0].Models) != 0 {
		t.Fatal(targets, err)
	}
	if _, err = s.targetSnapshot(context.Background(), AcceptanceTargetRequest{AccountID: 13, Model: "gpt-5.3-codex-spark"}); !errors.Is(err, ErrAcceptanceInvalid) {
		t.Fatal(err)
	}
}

func TestAcceptanceOpenAIGroupMessagesGateAndUnknownPrice(t *testing.T) {
	a := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	for _, enabled := range []bool{false, true} {
		group := &Group{Platform: PlatformComposite, AllowMessagesDispatch: enabled}
		model := acceptanceModelPlan(a, "gpt-5.4", group)
		if slices.Contains(model.Protocols, "messages") != enabled || !slices.Contains(model.Protocols, "responses") {
			t.Fatalf("group gate %t: %+v", enabled, model)
		}
	}
	a.ID = 4
	s := NewChannelAcceptanceService(&acceptanceRepoStub{}, &acceptanceAccountsStub{account: a}, nil, nil, nil)
	p, err := s.Preview(context.Background(), 1, AcceptancePreviewRequest{Targets: []AcceptanceTargetRequest{{AccountID: 4, Model: "unpriced-exact-model"}}})
	if err != nil || p.CanStart || p.GenerationCalls == 0 || p.CountCalls != 0 || !strings.Contains(strings.Join(p.Issues, ";"), "仅发送生成请求") {
		t.Fatal(p, err)
	}
}

func (a *acceptanceAccountsStub) GetByID(_ context.Context, id int64) (*Account, error) {
	if a.account != nil && id == a.account.ID {
		return a.account, nil
	}
	return nil, ErrAcceptanceNotFound
}

type acceptanceRepoStub struct {
	ChannelAcceptanceRepository
	preview *AcceptancePreview
	created int
	run     *AcceptanceRun
}

func (r *acceptanceRepoStub) FindRunByPreview(_ context.Context, _ string, _ int64) (*AcceptanceRun, error) {
	return r.run, nil
}

func (r *acceptanceRepoStub) SavePreview(_ context.Context, p *AcceptancePreview) error {
	r.preview = p
	return nil
}
func (r *acceptanceRepoStub) GetPreview(_ context.Context, id string, actor int64) (*AcceptancePreview, error) {
	if r.preview == nil || r.preview.ID != id || r.preview.ActorID != actor {
		return nil, ErrAcceptanceNotFound
	}
	return r.preview, nil
}
func (r *acceptanceRepoStub) CreateRun(_ context.Context, p *AcceptancePreview) (*AcceptanceRun, error) {
	r.created++
	r.run = &AcceptanceRun{ID: "run", PreviewID: p.ID}
	return r.run, nil
}

func TestAcceptancePricePresenceAndUnknownThoughtCost(t *testing.T) {
	for _, raw := range []string{`{"source":"manual"}`, `{"source":"manual","input_per_million":null,"output_per_million":0,"cache_per_million":0}`} {
		var price AcceptancePrice
		if json.Unmarshal([]byte(raw), &price) == nil {
			t.Fatal("missing manual rate accepted")
		}
	}
	var p AcceptancePrice
	if err := json.Unmarshal([]byte(`{"source":"manual","input_per_million":0,"output_per_million":0,"cache_per_million":0}`), &p); err != nil {
		t.Fatal(err)
	}
	in, out, total := int64(100), int64(10), int64(200)
	p = AcceptancePrice{Source: "manual", InputPerMillion: 1, OutputPerMillion: 5}
	if AcceptanceCalculatedCost(p, AcceptanceUsage{Input: &in, Output: &out, Total: &total, Source: "upstream_gemini_reported"}) != nil {
		t.Fatal("unknown thought cost released budget")
	}
	total = 110
	if cost := AcceptanceCalculatedCost(p, AcceptanceUsage{Input: &in, Output: &out, Total: &total, Source: "upstream_gemini_reported"}); cost == nil || *cost != .00015 {
		t.Fatalf("%v", cost)
	}
}

func TestAcceptanceIdempotentStartAfterExpiryAndStableGroupCounters(t *testing.T) {
	r := &acceptanceRepoStub{preview: &AcceptancePreview{ID: "p", ActorID: 7, ExpiresAt: time.Now().Add(-time.Hour)}, run: &AcceptanceRun{ID: "existing"}}
	s := NewChannelAcceptanceService(r, nil, nil, nil, nil)
	got, err := s.Start(context.Background(), 7, "p")
	if err != nil || got.ID != "existing" {
		t.Fatalf("%v %+v", err, got)
	}
	a := &Account{ID: 1}
	g := &Group{ID: 2, RateMultiplier: 1, AccountCount: 2}
	h := AcceptanceAccountFingerprint(a, g)
	g.AccountCount = 3
	g.RateLimitedAccountCount = 1
	if h != AcceptanceAccountFingerprint(a, g) {
		t.Fatal("live group counts invalidated settings")
	}
	g.GeminiInputCountingEnabled = true
	if h == AcceptanceAccountFingerprint(a, g) {
		t.Fatal("discount not bound")
	}
}

func TestAcceptanceInjectedPromptReservesKnownInput(t *testing.T) {
	a := &acceptanceAccountsStub{account: &Account{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey, ModelSystemPrompts: map[string]string{"gemini-test": "injected instructions"}}}
	s := NewChannelAcceptanceService(&acceptanceRepoStub{}, a, nil, nil, nil)
	snapshot, err := s.targetSnapshot(context.Background(), AcceptanceTargetRequest{AccountID: 1, Model: "gemini-test", Price: &AcceptancePrice{Source: "manual", InputPerMillion: 1}})
	if err != nil || snapshot.AdditionalInputEstimate == 0 {
		t.Fatalf("%v %+v", err, snapshot)
	}
	with := acceptanceManifest(snapshot, nil, "x")
	snapshot.AdditionalInputEstimate = 0
	without := acceptanceManifest(snapshot, nil, "x")
	if with[0].EstimatedCostUSD <= without[0].EstimatedCostUSD {
		t.Fatal("injected prompt not reserved")
	}
}

func TestAcceptancePreviewRequiresPriceAndDoesNotExecute(t *testing.T) {
	a := &acceptanceAccountsStub{account: &Account{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"gemini-test": "gemini-test"}}}}
	r := &acceptanceRepoStub{}
	s := NewChannelAcceptanceService(r, a, nil, nil, nil)
	p, err := s.Preview(context.Background(), 7, AcceptancePreviewRequest{Targets: []AcceptanceTargetRequest{{AccountID: 1, Model: "gemini-test"}}})
	if err != nil {
		t.Fatal(err)
	}
	if p.CanStart || len(p.Issues) == 0 || r.created != 0 {
		t.Fatal("unpriced preview must not start")
	}
	if _, err = s.Start(context.Background(), 7, p.ID); !errors.Is(err, ErrAcceptancePriceRequired) {
		t.Fatal(err)
	}
}
func TestAcceptanceStartChecksActorExpiryAndSettings(t *testing.T) {
	a := &acceptanceAccountsStub{account: &Account{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"gemini-test": "gemini-test"}}}}
	r := &acceptanceRepoStub{}
	s := NewChannelAcceptanceService(r, a, nil, nil, nil)
	p, err := s.Preview(context.Background(), 7, AcceptancePreviewRequest{Targets: []AcceptanceTargetRequest{{AccountID: 1, Model: "gemini-test", Price: &AcceptancePrice{Source: "manual", InputPerMillion: 1, OutputPerMillion: 5}}}})
	if err != nil || !p.CanStart {
		t.Fatalf("%v %+v", err, p)
	}
	if _, err = s.Start(context.Background(), 8, p.ID); !errors.Is(err, ErrAcceptanceNotFound) {
		t.Fatal(err)
	}
	a.account.ModelSystemPrompts = map[string]string{"gemini-test": "changed"}
	if _, err = s.Start(context.Background(), 7, p.ID); !errors.Is(err, ErrAcceptanceStale) {
		t.Fatal(err)
	}
	if r.created != 0 {
		t.Fatal("stale config executed")
	}
	a.account.ModelSystemPrompts = nil
	p.ExpiresAt = time.Now().Add(-time.Second)
	if _, err = s.Start(context.Background(), 7, p.ID); !errors.Is(err, ErrAcceptanceStale) {
		t.Fatal(err)
	}
}
func TestAcceptanceFingerprintIgnoresRefreshButBindsPolicies(t *testing.T) {
	a := &Account{ID: 1, Platform: PlatformGemini, Credentials: map[string]any{"api_key": "secret", "access_token": "old"}}
	before := AcceptanceAccountFingerprint(a, nil)
	a.Credentials["access_token"] = "new"
	if before != AcceptanceAccountFingerprint(a, nil) {
		t.Fatal("token refresh invalidated preview")
	}
	a.Credentials["api_key"] = "changed"
	if before == AcceptanceAccountFingerprint(a, nil) {
		t.Fatal("credential changes not bound")
	}
}

func TestAcceptanceOAuthSnapshotMatchesDispatchedModel(t *testing.T) {
	a := &acceptanceAccountsStub{account: &Account{ID: 1, Platform: PlatformGemini, Type: AccountTypeOAuth,
		Credentials:        map[string]any{"model_mapping": map[string]any{"gemini-2.5-pro": "gemini-2.5-flash"}},
		ModelSystemPrompts: map[string]string{"gemini-2.5-pro": "actual injected prompt"}}}
	s := NewChannelAcceptanceService(&acceptanceRepoStub{}, a, nil, nil, nil)
	snapshot, err := s.targetSnapshot(context.Background(), AcceptanceTargetRequest{AccountID: 1, Model: "gemini-2.5-pro"})
	if err != nil || snapshot.UpstreamModel != "gemini-2.5-pro" || snapshot.AdditionalInputEstimate == 0 {
		t.Fatalf("OAuth adapter does not map API-key aliases: %v %+v", err, snapshot)
	}
	targets, err := s.Targets(context.Background(), []int64{1})
	if err != nil || len(targets[0].Models) != 1 || targets[0].Models[0].UpstreamModel != snapshot.UpstreamModel {
		t.Fatalf("target preview differs from dispatch: %v %+v", err, targets)
	}
	a.account.Type = AccountTypeAPIKey
	snapshot, err = s.targetSnapshot(context.Background(), AcceptanceTargetRequest{AccountID: 1, Model: "gemini-2.5-pro"})
	if err != nil || snapshot.UpstreamModel != "gemini-2.5-flash" {
		t.Fatalf("API-key mapping lost: %v %+v", err, snapshot)
	}
}

func TestAcceptanceFailedCounterStopsOnlySameSource(t *testing.T) {
	next := AcceptanceSample{Ordinal: 2, Protocol: "count_tokens", Snapshot: AcceptanceSnapshot{AccountID: 1, Model: "gemini-test"}}
	previous := next
	previous.Ordinal = 1
	previous.Result = &AcceptanceResult{Status: "anomaly", RawUsage: AcceptanceUsage{Source: "upstream_gemini_reported"}}
	run := &AcceptanceRun{Samples: []AcceptanceSample{previous}}
	if !acceptanceCountSourceRejected(run, next) {
		t.Fatal("invalid counter retried")
	}
	next.Snapshot.AccountID = 2
	if acceptanceCountSourceRejected(run, next) {
		t.Fatal("other source incorrectly suppressed")
	}
	next.Snapshot.AccountID = 1
	run.Samples[0].Result.Status = "passed"
	run.Samples[0].Result.RawUsage.Source = "upstream_count_tokens"
	if acceptanceCountSourceRejected(run, next) {
		t.Fatal("valid counter suppressed")
	}
}
