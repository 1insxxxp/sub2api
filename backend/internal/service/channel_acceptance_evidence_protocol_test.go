package service

import (
	"net/http"
	"strings"
	"testing"
)

func acceptanceParseFixture(body string) *acceptanceEvidenceParser {
	p := newAcceptanceEvidenceParser()
	for len(body) > 0 {
		n := min(7, len(body))
		p.Feed([]byte(body[:n]))
		body = body[n:]
	}
	p.Finish()
	return p.snapshot()
}

func acceptanceStreamFixture(frames ...string) string {
	var b strings.Builder
	for _, frame := range frames {
		b.WriteString("data: " + frame + "\r\n\r\n")
	}
	return b.String()
}

func TestAcceptanceEvidenceMessages(t *testing.T) {
	start := `{"type":"message_start","message":{"model":"m","usage":{"input_tokens":10,"cache_read_input_tokens":0,"cache_creation_input_tokens":5,"output_tokens":1}}}`
	delta := `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":7}}`
	stop := `{"type":"message_stop"}`
	for _, body := range []string{
		`{"type":"message","model":"m","stop_reason":"tool_use","usage":{"input_tokens":10,"cache_read_input_tokens":0,"cache_creation_input_tokens":5,"output_tokens":7},"content":[{"type":"thinking","thinking":"private"},{"type":"text","text":"answer"},{"type":"tool_use","name":"acceptance_echo","input":{"value":"nonce"}}]}`,
		acceptanceStreamFixture(start,
			`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"private"}}`,
			`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":"an"}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"swer"}}`,
			`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","name":"acceptance_echo","input":{}}}`,
			`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"value\":"}}`,
			`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\"nonce\"}"}}`, delta, stop),
	} {
		p := acceptanceParseFixture(body)
		if !p.completed || !p.usageFinal || p.finishReason != "tool_use" || p.responseModel != "m" || !p.hasText || !p.hasReasoning || !p.hasTool || p.excerpt != "answer" || !p.toolMatches("nonce") {
			t.Fatalf("messages evidence %+v", p)
		}
		u := p.usage
		if u.Source != "upstream_anthropic_reported" || u.Input == nil || *u.Input != 15 || u.CacheRead == nil || *u.CacheRead != 0 || u.Output == nil || *u.Output != 7 || !u.OutputIncludesThoughts || u.Total != nil || u.Thoughts != nil || !strings.Contains(string(p.frames[0]), `"cache_creation_input_tokens":5`) {
			t.Fatalf("messages usage %+v frames=%s", u, p.frames)
		}
	}
	for _, tc := range []struct {
		body      string
		completed bool
	}{
		{acceptanceStreamFixture(start, delta), false},
		{acceptanceStreamFixture(start, stop), false},
		{acceptanceStreamFixture(start, `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`, stop), true},
		{acceptanceStreamFixture(start, delta, `{"type":"error","error":{"type":"overloaded_error"}}`, stop), false},
	} {
		p := acceptanceParseFixture(tc.body)
		if p.completed != tc.completed || p.usageFinal {
			t.Fatalf("invalid final proof %+v", p)
		}
	}
}

func TestAcceptanceEvidenceAnthropicCount(t *testing.T) {
	p := acceptanceParseFixture(`{"input_tokens":0}`)
	if !p.usageFinal || p.usage.Input == nil || *p.usage.Input != 0 || p.usage.Output != nil || p.usage.Source != "upstream_count_tokens" {
		t.Fatalf("missing count %+v", p)
	}
	for _, b := range []string{`{"input_tokens":7,"content":[]}`, `{"usage":{"input_tokens":7}}`, `{"input_tokens":7}{}`, `[{"input_tokens":7}]`, `{"response":{"input_tokens":7}}`, acceptanceStreamFixture(`{"input_tokens":7}`)} {
		if p := acceptanceParseFixture(b); p.usage.Source == "upstream_count_tokens" || p.usageFinal {
			t.Fatalf("ambiguous count %s", b)
		}
	}
}

func TestAcceptanceEvidenceResponses(t *testing.T) {
	usage := `{"input_tokens":9,"input_tokens_details":{"cached_tokens":0},"output_tokens":8,"output_tokens_details":{"reasoning_tokens":3},"total_tokens":17}`
	output := `[{"type":"reasoning","summary":[{"type":"summary_text","text":"private"}]},{"type":"message","content":[{"type":"output_text","text":"one"},{"type":"output_text","text":"two"}]},{"type":"function_call","name":"acceptance_echo","arguments":"{\"value\":\"nonce\"}"}]`
	response := `{"object":"response","model":"m","status":"completed","output":` + output + `,"usage":` + usage + `}`
	start := `{"type":"response.created","response":{"model":"m","status":"in_progress"}}`
	events := []string{start,
		`{"type":"response.reasoning_summary_text.delta","delta":"private"}`,
		`{"type":"response.output_text.delta","output_index":1,"content_index":0,"delta":"one"}`,
		`{"type":"response.output_text.done","output_index":1,"content_index":0,"text":"one"}`,
		`{"type":"response.content_part.done","output_index":1,"content_index":1,"part":{"type":"output_text","text":"two"}}`,
		`{"type":"response.output_item.added","output_index":2,"item":{"id":"fc","type":"function_call","name":"acceptance_echo","arguments":""}}`,
		`{"type":"response.function_call_arguments.delta","item_id":"fc","output_index":2,"delta":"{\"value\":"}`,
		`{"type":"response.function_call_arguments.delta","item_id":"fc","output_index":2,"delta":"\"nonce\"}"}`,
	}
	for _, b := range []string{response, acceptanceStreamFixture(append(events, `{"type":"response.completed","response":`+response+`}`)...), acceptanceStreamFixture(append(events, `{"type":"response.completed","response":{"status":"completed","usage":`+usage+`}}`)...)} {
		p := acceptanceParseFixture(b)
		if !p.completed || !p.usageFinal || p.finishReason != "completed" || p.excerpt != "onetwo" || !p.hasReasoning || !p.hasTool || !p.toolMatches("nonce") || p.responseModel != "m" {
			t.Fatalf("responses evidence %+v", p)
		}
		u := p.usage
		if u.Source != "upstream_responses_reported" || u.OutputIncludesThoughts || u.Output == nil || *u.Output != 5 || u.Input == nil || *u.Input != 9 || u.CacheRead == nil || *u.CacheRead != 0 || u.Thoughts == nil || *u.Thoughts != 3 || u.Total == nil || *u.Total != 17 {
			t.Fatalf("responses usage %+v", u)
		}
	}
	for _, b := range []string{
		`{"type":"response.output_item.done","item":{"type":"message","status":"completed"}}`,
		`{"type":"response.failed","response":` + response + `}`,
		`{"type":"response.completed","response":{"status":"incomplete","usage":` + usage + `}}`,
		acceptanceStreamFixture(`{"type":"response.in_progress","response":`+response+`}`, "[DONE]"),
	} {
		if p := acceptanceParseFixture(b); p.completed || p.usageFinal {
			t.Fatalf("false terminal %s", b)
		}
	}
}

func TestAcceptanceSystemOneEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, body, excerpt string
		completed, final    bool
	}{
		{"choice", `{"answers":{"q":{"type":"choice","choice":"a","extra":"ignore"}},"usage":{"input_tokens":123,"output_tokens":7,"secret":"ignore"},"model":"jev-1"}`, "a", true, true},
		{"missing output", `{"answers":{"q":{"type":"choice","choice":"b"}},"usage":{"input_tokens":0}}`, "b", true, false},
		{"empty answers", `{"answers":{},"usage":{"input_tokens":0,"output_tokens":0}}`, "", false, false},
		{"wrong field", `{"answers":{"q":{"type":"choice","answer":"a"}},"usage":{"input_tokens":0,"output_tokens":0}}`, "", false, false},
		{"incomplete body", `{"answers":{"q":{"type":"choice","choice":"a"}}`, "", false, false},
		{"sse", acceptanceStreamFixture(`{"answers":{"q":{"type":"choice","choice":"a"}},"usage":{"input_tokens":0,"output_tokens":0}}`), "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := acceptanceParseFixture(tc.body)
			if p.excerpt != tc.excerpt || p.completed != tc.completed || p.usageFinal != tc.final {
				t.Fatalf("systemone evidence %+v", p)
			}
			if tc.final && (p.usage.Source != "upstream_systemone_reported" || p.responseModel != "jev-1" || p.usage.Input == nil || *p.usage.Input != 123 || p.usage.Output == nil || *p.usage.Output != 7 || !p.usage.OutputIncludesThoughts || p.usage.Thoughts != nil || p.usage.Total != nil || strings.Contains(string(p.frames[0]), "ignore")) {
				t.Fatalf("systemone usage %+v", p.usage)
			}
			if tc.name == "missing output" && (p.usage.Input == nil || *p.usage.Input != 0 || p.usage.Output != nil) {
				t.Fatalf("systemone missing counters %+v", p.usage)
			}
		})
	}
}

func TestAcceptanceMessagesInclusiveInputAndCachePresence(t *testing.T) {
	for _, tc := range []struct {
		usage               string
		input, read, create int64 // -1 means absent
	}{
		{`{"input_tokens":9,"output_tokens":0}`, 9, -1, -1},
		{`{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}`, 0, 0, 0},
		{`{"prompt_tokens":25,"input_tokens":25,"output_tokens":0,"prompt_tokens_details":{"cached_tokens":20}}`, 25, 20, -1},
		{`{"prompt_cache_hit_tokens":20,"prompt_cache_miss_tokens":5,"output_tokens":0}`, 25, 20, -1},
		{`{"input_tokens":1,"output_tokens":0,"cache_creation":{"ephemeral_5m_input_tokens":2,"ephemeral_1h_input_tokens":3}}`, 6, -1, 5},
		{`{"cache_read_input_tokens":2,"output_tokens":0}`, -1, 2, -1},
		{`{"input_tokens":9223372036854775807,"cache_read_input_tokens":1,"output_tokens":0}`, -1, 1, -1},
	} {
		p := acceptanceParseFixture(`{"type":"message","stop_reason":"end_turn","usage":` + tc.usage + `}`)
		matches := func(n *int64, want int64) bool {
			if n == nil {
				return want == -1
			}
			return *n == want
		}
		if !matches(p.usage.Input, tc.input) || !matches(p.usage.CacheRead, tc.read) || !matches(p.usage.CacheCreate, tc.create) || !matches(p.usage.Output, 0) || p.usageFinal != (tc.input >= 0) {
			t.Fatalf("wrong usage for %s: %+v", tc.usage, p.usage)
		}
	}
}

func TestAcceptanceResponsesFinalSnapshotUsage(t *testing.T) {
	created := `{"type":"response.created","response":{"usage":{"input_tokens":12,"output_tokens":11,"input_tokens_details":{"cached_tokens":10,"cache_write_tokens":1}}}}`
	for _, tc := range []struct {
		usage string
		final bool
	}{
		{`{"input_tokens":9,"output_tokens":0,"input_tokens_details":{"cached_tokens":0,"cache_write_tokens":0}}`, true},
		{`{"input_tokens":9,"output_tokens":0,"input_tokens_details":{"cache_write_tokens":0}}`, false},
		{`{"input_tokens":9,"output_tokens":0,"input_tokens_details":{"cached_tokens":0}}`, false},
	} {
		p := acceptanceParseFixture(acceptanceStreamFixture(created, `{"type":"response.completed","response":{"status":"completed","usage":`+tc.usage+`}}`))
		if p.usageFinal != tc.final || p.usage.Input == nil || *p.usage.Input != 9 || p.usage.Output == nil || *p.usage.Output != 0 || !p.usage.OutputIncludesThoughts || p.usage.Thoughts != nil {
			t.Fatalf("wrong final evidence: %+v", p)
		}
	}
}

func TestAcceptanceCanonicalSources(t *testing.T) {
	for _, tc := range []struct{ url, channel, path string }{
		{"https://api.anthropic.com/v1/messages", "anthropic", "/v1/messages"},
		{"https://api.anthropic.com/v1/messages/count_tokens", "anthropic", "/v1/messages/count_tokens"},
		{"https://api.openai.com/v1/responses", "openai", "/v1/responses"},
		{"https://chatgpt.com/backend-api/codex/responses", "openai_codex", "/backend-api/codex/responses"},
		{"https://api.typesafe.ai/v1/systemone", "typesafe", "/v1/systemone"},
		{"https://api.kimi.com/coding/v1/messages", PlatformKimi, "/v1/messages"},
		{"https://api.cline.bot/api/v1/chat/completions", PlatformCline, "/v1/chat/completions"},
		{"https://api.commandcode.ai/provider/v1/responses", PlatformCommandCode, "/v1/responses"},
		{"https://opencode.ai/zen/go/v1/messages", PlatformOpenCodeGo, "/v1/messages"},
		{"https://api.x.ai/v1/responses", PlatformGrok, "/v1/responses"},
		{"https://api.anthropic.com.evil.test/tenant-secret/v1/messages", "unknown", "/v1/messages"},
		{"http://api.openai.com/v1/responses", "unknown", "/v1/responses"},
	} {
		r, err := http.NewRequest("POST", tc.url+"?private=secret", nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Header["x-api-key"] = []string{"sensitive"}
		s := acceptanceSourceFromRequest(r, "", "", "")
		if s.Channel != tc.channel || s.Path != tc.path || s.AuthKind != "api_key" || (s.Kind == "direct") != (tc.channel != "unknown") {
			t.Fatalf("bad attribution %+v", s)
		}
	}
}

func TestAcceptanceEvidenceSSEFraming(t *testing.T) {
	counts := `{"response":{"usage":{"input_tokens":2,"output_tokens":1}}}`
	p := acceptanceParseFixture("id: 12\r\nevent: response.completed\r\ndata: " + counts + "\r\n\r\n")
	if !p.completed || !p.usageFinal {
		t.Fatal("named SSE event discarded")
	}
	for _, body := range []string{
		"event: response.failed\ndata: " + `{"type":"response.completed","response":{"usage":{"input_tokens":2,"output_tokens":1}}}` + "\n\n",
		"event: response.completed\ndata: " + counts + "\n",
		"event: response.completed\ndata: " + strings.Repeat("x", acceptanceFrameLimit) + "\ndata: " + counts + "\n\n",
		strings.Repeat("x", acceptanceFrameLimit+1) + "\n" + `{"input_tokens":1}`,
	} {
		if p := acceptanceParseFixture(body); p.completed || p.usageFinal {
			t.Fatalf("malformed or interrupted framing yielded final evidence")
		}
	}
}

func TestAcceptanceResponsesParallelCallsByItemID(t *testing.T) {
	p := acceptanceParseFixture(acceptanceStreamFixture(
		`{"type":"response.output_item.added","output_index":1,"item":{"type":"function_call","id":"fc_a","name":"acceptance_echo"}}`,
		`{"type":"response.output_item.added","output_index":2,"item":{"type":"function_call","id":"fc_b","name":"other_tool"}}`,
		`{"type":"response.function_call_arguments.done","item_id":"fc_b","arguments":"{\"value\":\"wrong\"}"}`,
		`{"type":"response.function_call_arguments.delta","item_id":"fc_a","delta":"{\"value\":"}`,
		`{"type":"response.function_call_arguments.done","item_id":"fc_a","arguments":"{\"value\":\"right\"}"}`,
		`{"type":"response.completed","response":{"status":"completed"}}`,
	))
	if !p.toolMatches("right") || p.toolMatches("wrong") || p.hasText {
		t.Fatal("interleaved tool evidence assigned to wrong call")
	}
}
