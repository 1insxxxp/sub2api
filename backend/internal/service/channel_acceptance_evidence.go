package service

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

const acceptanceFrameLimit = 256 << 10
const acceptanceExcerptLimit = 2048

// Parsers retain only known numeric usage and a bounded synthetic answer. They
// never retain response headers, URLs with queries, or arbitrary usage fields.
type acceptanceEvidenceParser struct {
	mu                                              sync.Mutex
	pending, event                                  []byte
	mode                                            int // 0 undecided; 1 JSON; 2 SSE
	dropping, discardEvent, truncated               bool
	eventType                                       string
	usage                                           AcceptanceUsage
	frames                                          []json.RawMessage
	excerpt, finishReason, responseModel            string
	hasText, hasReasoning, hasTool, completed       bool
	usageFinal                                      bool
	terminalInput, terminalOutput, terminalThoughts bool
	terminalCache, terminalCreate, terminalTotal    bool
	chatOutput                                      *int64
	messages                                        acceptanceMessagesState
	responses                                       acceptanceResponsesState
	protocolEnded                                   bool
	tools                                           map[string]*acceptanceToolEvidence
}

func newAcceptanceEvidenceParser() *acceptanceEvidenceParser { return &acceptanceEvidenceParser{} }
func (p *acceptanceEvidenceParser) Feed(b []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(b) > 0 {
		n := bytes.IndexByte(b, '\n')
		if n < 0 {
			p.appendLine(b)
			return
		}
		p.appendLine(b[:n])
		p.line()
		b = b[n+1:]
	}
}
func (p *acceptanceEvidenceParser) appendLine(b []byte) {
	if p.dropping {
		return
	}
	if len(p.pending)+len(b) > acceptanceFrameLimit {
		if p.mode == 0 {
			prefix := append(p.pending, b[:min(len(b), 8)]...)
			p.detectMode(prefix)
		}
		p.discardEvent = true
		p.pending = nil
		p.event = nil
		p.dropping = true
		p.truncated = true
		return
	}
	p.pending = append(p.pending, b...)
}
func (p *acceptanceEvidenceParser) line() {
	line := bytes.TrimSuffix(p.pending, []byte{'\r'})
	if p.dropping {
		p.pending = nil
		p.dropping = false
		return
	}
	p.detectMode(line)
	if p.mode == 1 {
		if p.discardEvent {
			p.pending = nil
			return
		}
		if len(p.event)+len(line)+1 > acceptanceFrameLimit {
			p.event = nil
			p.truncated = true
			p.discardEvent = true
		} else {
			p.event = append(p.event, '\n')
			p.event = append(p.event, line...)
		}
		p.pending = nil
		return
	}
	if len(line) == 0 {
		if !p.discardEvent {
			p.flushEvent()
		}
		p.event = nil
		p.eventType = ""
		p.discardEvent = false
	} else if p.discardEvent {
		// Discard all remaining lines of an oversized event.
	} else if name, ok := bytes.CutPrefix(line, []byte("event:")); ok {
		p.eventType = string(bytes.TrimSpace(name))
	} else if bytes.HasPrefix(line, []byte("data:")) {
		line = bytes.TrimPrefix(line, []byte("data:"))
		line = bytes.TrimPrefix(line, []byte(" "))
		if len(p.event)+len(line)+1 > acceptanceFrameLimit {
			p.event = nil
			p.truncated = true
			p.discardEvent = true
		} else {
			if len(p.event) > 0 {
				p.event = append(p.event, '\n')
			}
			p.event = append(p.event, line...)
		}
	}
	p.pending = nil
}
func (p *acceptanceEvidenceParser) detectMode(line []byte) {
	if p.mode != 0 || len(bytes.TrimSpace(line)) == 0 {
		return
	}
	p.mode = 1
	for _, prefix := range []string{":", "data:", "event:", "retry:", "id:"} {
		if bytes.HasPrefix(line, []byte(prefix)) {
			p.mode = 2
			return
		}
	}
}

func (p *acceptanceEvidenceParser) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.pending) > 0 {
		p.line()
	}
	if p.mode == 1 && !p.discardEvent {
		p.flushEvent()
	}
	p.finishResponseText()
}
func (p *acceptanceEvidenceParser) flushEvent() {
	b := bytes.TrimSpace(p.event)
	p.event = nil
	if len(b) == 0 || bytes.Equal(b, []byte("[DONE]")) {
		return
	}
	var v any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if d.Decode(&v) != nil || d.Decode(new(any)) != io.EOF {
		return
	}
	// Named SSE events occasionally omit type in their JSON payload. A
	// conflicting name and payload cannot establish terminal evidence.
	if p.mode == 2 && p.eventType != "" && p.eventType != "message" {
		if m := acceptanceObject(v); m != nil {
			if typ := acceptanceString(m["type"]); typ == "" {
				m["type"] = p.eventType
			} else if typ != p.eventType {
				return
			}
		}
	}
	// A Messages count is an unwrapped JSON object with one field. The same
	// counter in message usage (or in an SSE fragment) is generation evidence.
	if m := acceptanceObject(v); p.mode == 1 && len(m) == 1 {
		if n := acceptanceNumber(m["input_tokens"]); n != nil {
			p.usage.Input, p.usage.Source, p.usageFinal = n, "upstream_count_tokens", true
			return
		}
	}
	if m := acceptanceObject(v); m != nil && m["answers"] != nil {
		if p.mode == 1 {
			p.consumeSystemOne(m)
		}
		return
	}
	p.consume(v)
}
func acceptanceNumber(v any) *int64 {
	n, ok := v.(json.Number)
	if !ok {
		return nil
	}
	i, err := n.Int64()
	if err != nil || i < 0 {
		return nil
	}
	return &i
}
func acceptanceMax(dst **int64, n *int64) {
	if n != nil && (*dst == nil || **dst < *n) {
		v := *n
		*dst = &v
	}
}
func acceptanceObject(v any) map[string]any { m, _ := v.(map[string]any); return m }
func acceptanceString(v any) string         { s, _ := v.(string); return s }
func (p *acceptanceEvidenceParser) text(s string, thought bool) {
	if s == "" {
		return
	}
	if thought {
		p.hasReasoning = true
		return
	}
	p.hasText = true
	p.excerpt += acceptanceBoundedText(s, acceptanceExcerptLimit-len(p.excerpt))
}

func acceptanceBoundedText(s string, remaining int) string {
	if len(s) > remaining {
		s = s[:remaining]
		for !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
	}
	return s
}
func (p *acceptanceEvidenceParser) consume(v any) {
	if a, ok := v.([]any); ok {
		for _, x := range a {
			p.consume(x)
		}
		return
	}
	m := acceptanceObject(v)
	if m == nil {
		return
	}
	if p.protocolEnded || p.consumeMessages(m) || p.consumeResponses(m) {
		return
	}
	if r := acceptanceObject(m["response"]); r != nil {
		p.consume(r)
	}
	if model := acceptanceString(m["modelVersion"]); model != "" {
		p.responseModel = model
	} else if model = acceptanceString(m["model"]); model != "" {
		p.responseModel = model
	}
	if n := acceptanceNumber(m["totalTokens"]); n != nil {
		acceptanceMax(&p.usage.Input, n)
		p.usage.Source = "upstream_count_tokens"
		p.usageFinal = true // countTokens is a complete, non-generating result.
	}
	if choices, ok := m["choices"].([]any); ok {
		for _, item := range choices {
			c := acceptanceObject(item)
			if reason := acceptanceString(c["finish_reason"]); reason != "" {
				p.completed = true
				p.finishReason = reason
			}
			delta := acceptanceObject(c["delta"])
			if delta == nil {
				delta = acceptanceObject(c["message"])
			}
			p.text(acceptanceString(delta["content"]), false)
			p.text(acceptanceString(delta["reasoning_content"]), true)
			p.text(acceptanceString(delta["reasoning"]), true)
			if a, ok := delta["tool_calls"].([]any); ok && len(a) > 0 {
				p.hasTool = true
				for index, item := range a {
					call := acceptanceObject(item)
					key := strconv.Itoa(index)
					if n := acceptanceNumber(call["index"]); n != nil {
						key = strconv.FormatInt(*n, 10)
					}
					f := acceptanceObject(call["function"])
					p.toolFragment(key, acceptanceString(f["name"]), acceptanceString(f["arguments"]))
				}
			}
		}
	}
	if candidates, ok := m["candidates"].([]any); ok {
		for _, item := range candidates {
			c := acceptanceObject(item)
			if reason := acceptanceString(c["finishReason"]); reason != "" {
				p.completed = true
				p.finishReason = reason
			}
			content := acceptanceObject(c["content"])
			parts, _ := content["parts"].([]any)
			for _, part := range parts {
				o := acceptanceObject(part)
				thought, _ := o["thought"].(bool)
				p.text(acceptanceString(o["text"]), thought)
				if f := acceptanceObject(o["functionCall"]); f != nil {
					p.hasTool = true
					args, _ := json.Marshal(f["args"])
					p.toolFragment("native:"+acceptanceString(f["name"]), acceptanceString(f["name"]), string(args))
				}
			}
		}
	}
	// Completion and usage may coexist on this frame, or usage may arrive in a
	// separate tail frame. Earlier cumulative usage alone is never final, even
	// if its arithmetic is consistent or a later frame only contains a finish.
	if u := acceptanceObject(m["usageMetadata"]); u != nil {
		p.captureUsage(u, "upstream_gemini_reported")
	}
	if u := acceptanceObject(m["usage"]); u != nil {
		p.captureUsage(u, "upstream_chat_reported")
	}
}

// capture copies only the first reported numeric alias into the retained frame.
// Preserve the wire field name and explicit zero, including in nested breakdowns.
type acceptanceUsageFrame map[string]any

func (f acceptanceUsageFrame) capture(u map[string]any, paths ...string) *int64 {
	for _, path := range paths {
		field, child, nested := strings.Cut(path, ".")
		value := u[field]
		if nested {
			value = acceptanceObject(value)[child]
		}
		n := acceptanceNumber(value)
		if n == nil {
			continue
		}
		if nested {
			details := acceptanceObject(f[field])
			if details == nil {
				details = make(map[string]any)
				f[field] = details
			}
			details[child] = *n
		} else {
			f[field] = *n
		}
		return n
	}
	return nil
}

func (p *acceptanceEvidenceParser) captureUsage(u map[string]any, source string) {
	if len(u) == 0 {
		return
	}
	frame := make(acceptanceUsageFrame)
	record := func(dst **int64, terminal *bool, paths ...string) {
		if n := frame.capture(u, paths...); n != nil {
			if p.completed {
				*terminal = true
			}
			if source == "upstream_responses_reported" && p.completed {
				// The terminal snapshot can correct preliminary counts downward.
				*dst = n
			} else {
				acceptanceMax(dst, n)
			}
		}
	}
	if source == "upstream_gemini_reported" {
		record(&p.usage.Input, &p.terminalInput, "promptTokenCount")
		record(&p.usage.Output, &p.terminalOutput, "candidatesTokenCount")
		record(&p.usage.Thoughts, &p.terminalThoughts, "thoughtsTokenCount")
		record(&p.usage.CacheRead, &p.terminalCache, "cachedContentTokenCount")
		record(&p.usage.Total, &p.terminalTotal, "totalTokenCount")
	} else {
		record(&p.usage.Input, &p.terminalInput, "input_tokens", "prompt_tokens")
		record(&p.chatOutput, &p.terminalOutput, "output_tokens", "completion_tokens")
		record(&p.usage.Total, &p.terminalTotal, "total_tokens")
		record(&p.usage.CacheRead, &p.terminalCache, "input_tokens_details.cached_tokens", "prompt_tokens_details.cached_tokens",
			"cache_read_input_tokens", "cache_read_tokens", "cached_tokens", "prompt_cache_hit_tokens")
		record(&p.usage.CacheCreate, &p.terminalCreate, "input_tokens_details.cache_write_tokens", "prompt_tokens_details.cache_write_tokens",
			"input_tokens_details.cache_creation_tokens", "prompt_tokens_details.cache_creation_tokens",
			"cache_write_tokens", "cache_creation_input_tokens", "cache_write_input_tokens", "cache_creation_tokens")
		record(&p.usage.Thoughts, &p.terminalThoughts, "output_tokens_details.reasoning_tokens", "completion_tokens_details.reasoning_tokens")
		if p.chatOutput != nil {
			p.usage.OutputIncludesThoughts = p.usage.Thoughts == nil
			n := *p.chatOutput
			if p.usage.Thoughts != nil {
				n -= *p.usage.Thoughts
			}
			p.usage.Output = nil
			if n >= 0 {
				p.usage.Output = &n
			}
		}
	}
	if len(frame) == 0 {
		return
	}
	p.recordUsageFrame(frame, source)
	// A finish without updated counters cannot promote an earlier estimate,
	// including optional counters used to split output or price cached input.
	p.usageFinal = p.completed && p.terminalInput && p.terminalOutput && p.usage.Output != nil &&
		(p.usage.CacheCreate == nil || p.terminalCreate) &&
		(p.usage.Thoughts == nil || p.terminalThoughts) &&
		(p.usage.CacheRead == nil || p.terminalCache) &&
		(p.usage.Total == nil || p.terminalTotal)
}

func (p *acceptanceEvidenceParser) recordUsageFrame(safe map[string]any, source string) {
	b, _ := json.Marshal(safe)
	if len(p.frames) == 16 {
		copy(p.frames, p.frames[1:])
		p.frames = p.frames[:15]
	}
	p.frames = append(p.frames, b)
	p.usage.Source = source
}

func acceptanceSourceFromRequest(r *http.Request, adapter, requested, mapped string) AcceptanceSource {
	s := AcceptanceSource{Kind: "unverified", Channel: "unknown", Adapter: adapter, RequestedModel: requested, UpstreamModel: mapped}
	if r == nil || r.URL == nil {
		return s
	}
	host := strings.TrimSuffix(strings.ToLower(r.URL.Hostname()), ".")
	endpoint := r.URL.Scheme + "://" + r.URL.Host
	s.Host = r.URL.Host
	switch {
	case getHeaderRaw(r.Header, "x-api-key") != "" || getHeaderRaw(r.Header, "x-goog-api-key") != "" || r.URL.Query().Get("key") != "":
		s.AuthKind = "api_key"
	case strings.HasPrefix(getHeaderRaw(r.Header, "authorization"), "AWS4-HMAC-SHA256 "):
		s.AuthKind = "aws_sigv4"
	case getHeaderRaw(r.Header, "authorization") != "":
		s.AuthKind = "bearer"
	default:
		s.AuthKind = "unknown"
	}
	// Retain the recognized route template, not arbitrary path segments which can
	// themselves contain tenant IDs, keys, project names or signed credentials.
	path := r.URL.Path
	switch {
	case strings.HasSuffix(path, ":streamGenerateContent"):
		s.Path = "/models/{model}:streamGenerateContent"
	case strings.HasSuffix(path, ":generateContent"):
		s.Path = "/models/{model}:generateContent"
	case strings.HasSuffix(path, ":countTokens"):
		s.Path = "/models/{model}:countTokens"
	case strings.HasSuffix(path, "/v1/systemone"):
		s.Path = "/v1/systemone"
	case strings.HasSuffix(path, ":streamRawPredict"):
		s.Path = "/models/{model}:streamRawPredict"
	case strings.HasSuffix(path, ":rawPredict"):
		s.Path = "/models/{model}:rawPredict"
	case strings.HasPrefix(path, "/model/") && strings.HasSuffix(path, "/invoke"):
		s.Path = "/model/{model}/invoke"
	case strings.HasPrefix(path, "/model/") && strings.HasSuffix(path, "/invoke-with-response-stream"):
		s.Path = "/model/{model}/invoke-with-response-stream"
	case strings.HasSuffix(path, "/messages/count_tokens"):
		s.Path = "/v1/messages/count_tokens"
	case strings.HasSuffix(path, "/messages"):
		s.Path = "/v1/messages"
	case strings.HasSuffix(path, "/responses/input_tokens"):
		s.Path = "/v1/responses/input_tokens"
	case path == "/backend-api/codex/responses":
		s.Path = path
	case strings.HasSuffix(path, "/chat/completions"):
		s.Path = "/v1/chat/completions"
	case strings.HasSuffix(path, "/responses"):
		s.Path = "/v1/responses"
	default:
		s.Path = "/{redacted}"
	}
	for _, action := range []string{"generateContent", "streamGenerateContent", "countTokens"} {
		if strings.HasSuffix(path, "/v1internal:"+action) {
			s.Path = "/v1internal:" + action
		}
	}
	if i := strings.LastIndex(path, "/models/"); i >= 0 {
		modelAction := path[i+len("/models/"):]
		if j := strings.LastIndex(modelAction, ":"); j > 0 {
			if model, err := url.PathUnescape(modelAction[:j]); err == nil && len(model) < 256 {
				s.UpstreamModel = model
			}
		}
	}

	if r.URL.Scheme != "https" {
		return s
	}
	switch {
	case strings.HasPrefix(host, "bedrock-runtime.") && strings.HasSuffix(host, ".amazonaws.com") && strings.Count(host, ".") == 3:
		s.Kind, s.Channel = "direct", "bedrock"
	case host == "api.anthropic.com":
		s.Kind, s.Channel = "direct", "anthropic"
	case isOfficialOpenAIModelsBaseURL(endpoint):
		s.Kind, s.Channel = "direct", "openai"
	case host == "chatgpt.com" && s.Path == "/backend-api/codex/responses":
		s.Kind, s.Channel = "direct", "openai_codex"
	case isOfficialGrokCodexBaseURL(endpoint):
		s.Kind, s.Channel = "direct", PlatformGrok
	case isOfficialOpenCodeHost(endpoint):
		s.Kind, s.Channel = "direct", PlatformOpenCodeGo
	case isOfficialCommandCodeHost(endpoint):
		s.Kind, s.Channel = "direct", PlatformCommandCode
	case isOfficialClineHost(endpoint):
		s.Kind, s.Channel = "direct", PlatformCline
	case host == "api.kimi.com" || host == "api.moonshot.cn" || host == "api.moonshot.ai":
		s.Kind, s.Channel = "direct", PlatformKimi
	case host == "open.bigmodel.cn" || host == "api.z.ai":
		s.Kind, s.Channel = "direct", PlatformZhipu
	case host == "api.deepseek.com":
		s.Kind, s.Channel = "direct", PlatformDeepseek
	case host == "api.minimaxi.com" || host == "api.minimax.io" || host == "api.minimax.com":
		s.Kind, s.Channel = "direct", PlatformMiniMax
	case host == "api.typesafe.ai":
		s.Kind, s.Channel = "direct", PlatformTypeSafe
	case host == "generativelanguage.googleapis.com":
		s.Kind = "direct"
		s.Channel = "google_developer_api"
	case host == "aiplatform.googleapis.com" || host == "aiplatform.us.rep.googleapis.com" || host == "aiplatform.eu.rep.googleapis.com" || strings.HasSuffix(host, "-aiplatform.googleapis.com"):
		s.Kind = "direct"
		s.Channel = "vertex"
	case host == "cloudcode-pa.googleapis.com" || host == "daily-cloudcode-pa.googleapis.com" || host == "daily-cloudcode-pa.sandbox.googleapis.com":
		s.Kind = "direct"
		s.Channel = "google_code_assist"
	}
	return s
}

type acceptanceToolEvidence struct {
	name, args string
	truncated  bool
}

func (p *acceptanceEvidenceParser) toolFragment(key, name, args string) {
	if p.tools == nil {
		p.tools = make(map[string]*acceptanceToolEvidence)
	}
	t := p.tools[key]
	if t == nil {
		if len(p.tools) >= 16 {
			return
		}
		t = &acceptanceToolEvidence{}
		p.tools[key] = t
	}
	if name != "" && len(name) <= 128 {
		t.name = name
	}
	if len(t.args)+len(args) > 8192 {
		t.truncated = true
	} else if !t.truncated {
		t.args += args
	}
}
func (p *acceptanceEvidenceParser) toolMatches(expected string) bool {
	for _, t := range p.tools {
		if t.truncated || t.name != "acceptance_echo" {
			continue
		}
		var args map[string]any
		if json.Unmarshal([]byte(t.args), &args) == nil && acceptanceString(args["value"]) == expected {
			return true
		}
	}
	return false
}

func (p *acceptanceEvidenceParser) snapshot() *acceptanceEvidenceParser {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := &acceptanceEvidenceParser{usage: p.usage, usageFinal: p.usageFinal, frames: append([]json.RawMessage(nil), p.frames...), excerpt: p.excerpt, finishReason: p.finishReason, responseModel: p.responseModel, hasText: p.hasText, hasReasoning: p.hasReasoning, hasTool: p.hasTool, completed: p.completed, truncated: p.truncated, tools: map[string]*acceptanceToolEvidence{}}
	for k, v := range p.tools {
		copy := *v
		out.tools[k] = &copy
	}
	return out
}
