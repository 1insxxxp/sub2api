//go:build unit

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestAcceptanceExecutorBedrockStreamEvidence(t *testing.T) {
	var stream strings.Builder
	for _, event := range []string{
		`{"type":"message_start","message":{"id":"msg_fixture","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[],"usage":{"input_tokens":11,"output_tokens":0}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"acceptance-ok"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":4}}`,
		`{"type":"message_stop"}`,
	} {
		stream.Write(bedrockChunkFrame(event))
	}
	account := &Account{ID: 6178, Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{"auth_mode": "apikey", "api_key": "fixture"}}
	upstream := &acceptanceExecutorHTTP{status: 200, contentType: "application/vnd.amazon.eventstream", body: stream.String()}
	executor := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: account}, nil, nil, nil, nil, &GatewayService{cfg: &config.Config{}, httpUpstream: upstream})
	sample := acceptanceExecutorSample(account, "messages", true)
	sample.Snapshot.Model = "claude-sonnet-4-5"
	sample.Snapshot.UpstreamModel, _ = ResolveBedrockModelID(account, sample.Snapshot.Model)
	result, err := executor.Execute(context.Background(), sample)
	if err != nil || upstream.calls != 1 || result.Status != "passed" || result.RawUsage.Output == nil || *result.RawUsage.Output != 4 || !result.UpstreamUsageFinal || !result.HasText {
		t.Fatalf("calls=%d result=%+v err=%v", upstream.calls, result, err)
	}
}
