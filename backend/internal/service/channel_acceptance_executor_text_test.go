package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestAcceptanceExecutor6178AnthropicMessages(t *testing.T) {
	account := &Account{ID: 6178, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "fixture-only", "base_url": "https://fixture.example.test"}}
	upstream := &acceptanceExecutorHTTP{status: 200, contentType: "application/json", body: `{"type":"message","id":"msg_fixture","model":"claude-sonnet-4-5","role":"assistant","content":[{"type":"text","text":"acceptance-ok"}],"stop_reason":"end_turn","usage":{"input_tokens":31,"output_tokens":6}}`}
	executor := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: account}, nil, nil, nil, nil, &GatewayService{cfg: &config.Config{}, httpUpstream: upstream})
	sample := acceptanceExecutorSample(account, "messages", false)
	sample.Snapshot.Model, sample.Snapshot.UpstreamModel, sample.Snapshot.Platform = "claude-sonnet-4-5", "claude-sonnet-4-5", PlatformAnthropic
	result, err := executor.Execute(context.Background(), sample)
	if err != nil || upstream.calls != 1 || upstream.request.URL.Path != "/v1/messages" || result.RawUsage.Output == nil || *result.RawUsage.Output != 6 || !result.HasText || result.Status != "passed" {
		t.Fatalf("messages evidence: calls=%d result=%+v err=%v", upstream.calls, result, err)
	}
	sample.Protocol, sample.Category = "count_tokens", "count_tokens"
	upstream.body = `{"input_tokens":31}`
	result, err = executor.Execute(context.Background(), sample)
	if err != nil || upstream.calls != 2 || upstream.request.URL.Path != "/v1/messages/count_tokens" || result.Status != "passed" || result.RawUsage.Input == nil || *result.RawUsage.Input != 31 {
		t.Fatalf("count evidence: calls=%d result=%+v err=%v", upstream.calls, result, err)
	}
	sample.Category, sample.Protocol = "audio", "messages"
	result, err = executor.Execute(context.Background(), sample)
	if err != nil || result.Status != "unsupported" || upstream.calls != 2 {
		t.Fatalf("audio sent: %+v %v", result, err)
	}
}

func TestAcceptanceExecutorTextAdaptersRejectOnce(t *testing.T) {
	for _, tc := range []struct{ platform, kind, model, protocol, suffix string }{
		{PlatformOpenAI, AccountTypeOAuth, "gpt-5", "responses", "/responses"},
		{PlatformDeepseek, AccountTypeAPIKey, "deepseek-chat", "chat", "/chat/completions"},
		{PlatformGrok, AccountTypeAPIKey, "grok-4", "chat", "/chat/completions"},
		{PlatformAntigravity, AccountTypeUpstream, "claude-sonnet-4-5", "messages", "/v1/messages"},
		{PlatformAnthropic, AccountTypeBedrock, "claude-sonnet-4-5", "messages", "/invoke"},
		{PlatformTypeSafe, AccountTypeAPIKey, "jev-latest", "systemone", "/v1/systemone"},
	} {
		t.Run(tc.platform+"/"+tc.kind, func(t *testing.T) {
			account := &Account{ID: 6178, Platform: tc.platform, Type: tc.kind, Credentials: map[string]any{"api_key": "fixture", "auth_mode": "apikey", "access_token": "fixture", "chatgpt_account_id": "test-account", "base_url": "https://fixture.example.test"}, Status: StatusActive}
			upstream := &acceptanceExecutorHTTP{status: 422, contentType: "application/json", body: `{"error":{"message":"invalid fixture"}}`}
			cfg := &config.Config{}
			executor := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: account}, nil, nil, &AntigravityGatewayService{httpUpstream: upstream}, &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}, &GatewayService{cfg: cfg, httpUpstream: upstream})
			sample := acceptanceExecutorSample(account, tc.protocol, false)
			sample.Snapshot.Platform = tc.platform
			sample.Snapshot.Model = tc.model
			sample.Snapshot.UpstreamModel = tc.model
			result, err := executor.Execute(context.Background(), sample)
			if err != nil || upstream.calls != 1 || !strings.HasSuffix(upstream.request.URL.Path, tc.suffix) || result.Status != "error" || result.HTTPStatus != 422 || account.Status != StatusActive {
				t.Fatalf("calls=%d request=%v result=%+v err=%v", upstream.calls, upstream.request != nil, result, err)
			}
		})
	}
}

func TestAcceptanceExecutorSystemOneReturnsClientEvidence(t *testing.T) {
	account := &Account{ID: 17, Platform: PlatformTypeSafe, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "fixture"}}
	upstream := &acceptanceExecutorHTTP{status: 200, contentType: "application/json", body: `{"model":"jev-fixture","answers":{"code":{"type":"choice","choice":"acceptance-ok"}},"usage":{"input_tokens":22,"output_tokens":3}}`}
	executor := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: account}, nil, nil, nil, nil, &GatewayService{cfg: &config.Config{}, httpUpstream: upstream})
	sample := acceptanceExecutorSample(account, "systemone", false)
	sample.Snapshot.Model, sample.Snapshot.UpstreamModel = "jev-latest", "jev-latest"
	result, err := executor.Execute(context.Background(), sample)
	if err != nil || upstream.calls != 1 || result.Status != "passed" || result.RawUsage.Input == nil || *result.RawUsage.Input != 22 || result.ClientUsage.Output == nil || *result.ClientUsage.Output != 3 || !result.HasText {
		t.Fatalf("calls=%d result=%+v err=%v", upstream.calls, result, err)
	}
	sample.Stream = true
	result, err = executor.Execute(context.Background(), sample)
	if err != nil || upstream.calls != 1 || result.Status != "unsupported" {
		t.Fatalf("stream was sent: calls=%d result=%+v err=%v", upstream.calls, result, err)
	}
}
