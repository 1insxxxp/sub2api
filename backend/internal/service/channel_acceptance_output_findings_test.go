package service

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestAcceptanceComparableOutputUsage(t *testing.T) {
	n := func(v int64) *int64 { return &v }
	split := func(output int64, thoughts *int64) AcceptanceUsage {
		return AcceptanceUsage{Output: n(output), Thoughts: thoughts}
	}
	gross := func(total int64) AcceptanceUsage {
		return AcceptanceUsage{Output: n(total), OutputIncludesThoughts: true}
	}
	for _, tc := range []struct {
		name         string
		raw, client  AcceptanceUsage
		code, status string
	}{
		{"both positive, client lower", gross(11), gross(9), "client_output_usage_mismatch", "anomaly"},
		{"both positive, client higher", gross(9), gross(11), "client_output_usage_mismatch", "anomaly"},
		{"candidate plus thoughts equals completion", split(9, n(2)), gross(11), "", "passed"},
		{"gross compared to known split", gross(11), split(8, n(2)), "client_output_usage_mismatch", "anomaly"},
		{"same gross, different reported breakdown", split(9, n(2)), split(10, n(1)), "client_output_usage_mismatch", "anomaly"},
		{"reasoning only survives conversion", split(0, n(6)), gross(6), "", "passed"},
		{"missing thought split on both candidate counts", split(9, nil), split(7, nil), "client_output_usage_mismatch", "anomaly"},
		{"unknown thoughts allow higher gross", split(9, nil), gross(12), "output_usage_not_comparable", "passed"},
		{"equal numbers on different bases", split(9, nil), gross(9), "output_usage_not_comparable", "passed"},
		{"unknown client thoughts allow higher upstream gross", gross(12), split(9, nil), "output_usage_not_comparable", "passed"},
		{"client total below known ordinary component", split(9, nil), gross(8), "client_output_usage_mismatch", "anomaly"},
		{"upstream total below client ordinary component", gross(8), split(9, nil), "client_output_usage_mismatch", "anomaly"},
		{"zero cannot include positive ordinary output", split(9, nil), gross(0), "client_output_usage_lost", "anomaly"},
		{"explicit zero thoughts", split(9, n(0)), gross(9), "", "passed"},
		{"positive to explicit zero", split(9, n(0)), gross(0), "client_output_usage_lost", "anomaly"},
		{"zero to positive", gross(0), gross(2), "client_output_usage_mismatch", "anomaly"},
		{"sum overflow", split(math.MaxInt64, n(1)), gross(1), "output_usage_not_comparable", "inconclusive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := &acceptanceEvidenceParser{completed: true, usageFinal: true, usage: tc.raw}
			client := &acceptanceEvidenceParser{completed: true, usageFinal: true, usage: tc.client}
			r := &AcceptanceResult{Status: "passed"}
			acceptanceCheckOutputUsage(r, raw, client)
			if r.Status != tc.status || tc.code != "" && !acceptanceHasFinding(r, tc.code) || tc.code == "" && len(r.Findings) != 0 {
				t.Fatalf("status=%s findings=%+v", r.Status, r.Findings)
			}
		})
	}
}

func TestAcceptanceOutputFindingsRequireCompleteWireEvidence(t *testing.T) {
	const upstream = `{"candidates":[{"finishReason":"STOP","content":{"parts":[{"text":"ok"}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":9,"thoughtsTokenCount":2}}`
	const complete = `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":11}}`
	parse := func(body string) *acceptanceEvidenceParser {
		p := newAcceptanceEvidenceParser()
		p.Feed([]byte(body))
		p.Finish()
		return p.snapshot()
	}
	for _, tc := range []struct {
		name, raw, client, code, status string
		truncateRaw, truncateClient     bool
	}{
		{"completed without client count", upstream, `{"choices":[{"finish_reason":"stop"}]}`, "client_output_usage_lost", "anomaly", false, false},
		{"correct complete reply", upstream, complete, "", "passed", false, false},
		{"client never finished", upstream, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":0}}\n\n", "", "inconclusive", false, false},
		{"client preliminary zero then finish", upstream, "data: {\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":0}}\n\ndata: {\"choices\":[{\"finish_reason\":\"stop\"}]}\n\n", "client_output_usage_not_final", "inconclusive", false, false},
		{"partial breakdown without an output count", upstream, acceptanceStreamFixture(
			`{"usage":{"prompt_tokens":10,"completion_tokens_details":{"reasoning_tokens":3}}}`,
			`{"choices":[{"finish_reason":"stop"}]}`,
		), "client_output_usage_not_final", "inconclusive", false, false},
		{"upstream interim zero with text", strings.ReplaceAll(strings.ReplaceAll(upstream, `"finishReason":"STOP",`, ""), `"candidatesTokenCount":9`, `"candidatesTokenCount":0`), complete, "", "inconclusive", false, false},
		{"downward correction after preliminary Chat usage", upstream, acceptanceStreamFixture(
			`{"usage":{"prompt_tokens":10,"completion_tokens":32}}`,
			`{"choices":[{"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":11}}`,
		), "output_usage_snapshot_unconfirmed", "inconclusive", false, false},
		{"truncated upstream", upstream, complete, "", "inconclusive", true, false},
		{"truncated downstream", upstream, complete, "", "inconclusive", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, client := parse(tc.raw), parse(tc.client)
			raw.truncated, client.truncated = tc.truncateRaw, tc.truncateClient
			r := &AcceptanceResult{Status: "inconclusive"}
			if raw.usageFinal && client.completed {
				r.Status = "passed"
			}
			acceptanceCheckOutputUsage(r, raw, client)
			if r.Status != tc.status || tc.code != "" && !acceptanceHasFinding(r, tc.code) || tc.code == "" && len(r.Findings) != 0 {
				t.Fatalf("status=%s findings=%+v", r.Status, r.Findings)
			}
		})
	}
}

func TestAcceptanceExecutorReportsOutputProjectionDiscrepancy(t *testing.T) {
	account := &Account{ID: 7, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
	body := `{"candidates":[{"content":{"parts":[{"text":"acceptance-ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":0,"thoughtsTokenCount":0,"totalTokenCount":10}}`
	upstream := &acceptanceExecutorHTTP{status: 200, contentType: "application/json", body: body}
	executor := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: account}, nil, &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: upstream}, nil, nil, nil)
	r, err := executor.Execute(context.Background(), acceptanceExecutorSample(account, "chat", false))
	if err != nil {
		t.Fatal(err)
	}
	if upstream.calls != 1 || r.Status != "anomaly" || !acceptanceHasFinding(r, "client_output_usage_mismatch") || !acceptanceHasFinding(r, "upstream_zero_output_with_content") {
		t.Fatalf("calls=%d result=%+v", upstream.calls, r)
	}
	if r.RawUsage.Output == nil || *r.RawUsage.Output != 0 || r.ClientUsage.Output == nil || *r.ClientUsage.Output <= 0 {
		t.Fatalf("raw and client observations must survive comparison: %+v", r)
	}
}

func TestAcceptanceToolChallengeWithIncomparableOutputUsage(t *testing.T) {
	const body = `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"acceptance_echo","args":{"value":"nonce-1"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":9}}`
	for _, tc := range []struct {
		expected, status string
		mismatch         bool
	}{
		{"nonce-1", "passed", false},
		{"nonce-2", "anomaly", true},
	} {
		t.Run(tc.expected, func(t *testing.T) {
			a := &Account{ID: 7, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
			u := &acceptanceExecutorHTTP{status: 200, contentType: "application/json", body: body}
			e := NewAccountAcceptanceExecutor(&acceptanceExecutorAccounts{a: a}, nil, &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: u}, nil, nil, nil)
			s := acceptanceExecutorSample(a, "chat", false)
			s.Category, s.Expected = "tools", tc.expected
			r, err := e.Execute(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			if u.calls != 1 || !r.Completed || !r.HasTool || r.Status != tc.status || acceptanceHasFinding(r, "challenge_mismatch") != tc.mismatch {
				t.Fatalf("calls=%d result=%+v", u.calls, r)
			}
			if !acceptanceHasFinding(r, "output_usage_not_comparable") || acceptanceHasFinding(r, "client_output_usage_mismatch") || acceptanceHasFinding(r, "client_output_usage_lost") {
				t.Fatalf("usage findings=%+v", r.Findings)
			}
		})
	}
}
