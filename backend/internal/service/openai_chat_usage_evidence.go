package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tiktoken-go/tokenizer"
)

// OpenAIChatUsageEvidence is scoped to one selected raw Chat request. Rounds are
// separate provider calls; they must be resolved separately before aggregation.
// No prompt, credentials or generated text survives in this settlement snapshot.
type OpenAIChatUsageEvidence struct {
	Rounds     []*OpenAIChatUsageRound
	Finalized  bool
	ExtraUsage OpenAIUsage
	// Retained across finalization and accounting retries after the downstream
	// token fallback is cleared in favor of finalized provider usage.
	HadDeliveredOutput bool
}

type OpenAIChatUsageRound struct {
	Usage                                                                OpenAIUsage
	InputReported, OutputReported, CacheReadReported, CacheWriteReported bool
	ReasoningReported, TotalReported                                     bool
	ReasoningTokens, TotalTokens                                         int
	CompletionTokens                                                     int
	InputFallback                                                        *GeminiInputUsageFallback `json:"-"`
	ReceivedReasoningTokens                                              int
	ReceivedOutputTokens                                                 int
	OutputTruncated                                                      bool
	UpstreamAccepted                                                     bool
	UpstreamRequestID, IdentityHash, UpstreamModel                       string
	AccountID                                                            int64
	Outcome                                                              ResponseOutcome
	InputTokensSource, OutputTokensSource                                UsageTokenSource
}

type openAIChatCaptureKey struct{}
type openAIChatUsageCapture struct {
	evidence      OpenAIChatUsageEvidence
	current       *OpenAIChatUsageRound
	received      *openAIChatSemanticOutput
	delivered     *ResponseOutcomeCollector
	outcome       *ResponseOutcomeCollector
	opsContext    *gin.Context
	opsFirstEvent int
	opsAttempt    int
	opsStarted    time.Time
	opsRecorded   bool
}

func newOpenAIChatUsageCapture(ctx context.Context) (context.Context, *openAIChatUsageCapture) {
	capture := &openAIChatUsageCapture{outcome: NewResponseOutcomeCollector(200, 200)}
	return context.WithValue(ctx, openAIChatCaptureKey{}, capture), capture
}
func openAIChatCaptureFromContext(ctx context.Context) *openAIChatUsageCapture {
	if ctx == nil {
		return nil
	}
	c, _ := ctx.Value(openAIChatCaptureKey{}).(*openAIChatUsageCapture)
	return c
}
func (c *openAIChatUsageCapture) begin(req *http.Request, account *Account) {
	c.finishRound()
	if c.opsContext != nil {
		c.opsFirstEvent = len(geminiUpstreamAttemptEvents(c.opsContext))
		c.opsAttempt = c.opsContext.GetInt(openAIChatOpsAttemptSequenceKey) + 1
		c.opsContext.Set(openAIChatOpsAttemptSequenceKey, c.opsAttempt)
		c.opsStarted = time.Now()
	}
	round := &OpenAIChatUsageRound{InputTokensSource: UsageTokenSourceUnreported, OutputTokensSource: UsageTokenSourceUnreported}
	if account != nil {
		round.AccountID = account.ID
	}
	if req != nil {
		round.IdentityHash = openAIChatRequestIdentity(req)
		if req.GetBody != nil {
			reader, err := req.GetBody()
			if err == nil {
				raw, readErr := io.ReadAll(io.LimitReader(reader, geminiFallbackMaxSemanticBytes+1))
				_ = reader.Close()
				if readErr == nil && len(raw) <= geminiFallbackMaxSemanticBytes {
					round.UpstreamModel = gjson.GetBytes(raw, "model").String()
				}
			}
		}
		round.InputFallback = prepareOpenAIChatInputUsageFallback(req, round.AccountID, round.UpstreamModel)
	}
	c.current = round
	c.received = &openAIChatSemanticOutput{}
	c.delivered = NewResponseOutcomeCollector(200, 0)
	c.evidence.Rounds = append(c.evidence.Rounds, round)
}
func (c *openAIChatUsageCapture) response(resp *http.Response) {
	if c == nil || c.current == nil || resp == nil {
		return
	}
	r := c.current
	r.UpstreamRequestID = ""
	r.UpstreamAccepted = resp.StatusCode >= 200 && resp.StatusCode < 300
	c.delivered.SetStatuses(200, resp.StatusCode)
	c.outcome.SetStatuses(200, resp.StatusCode)
	if resp.Request != nil && openAIChatRequestIdentity(resp.Request) != r.IdentityHash {
		r.IdentityHash = ""
		if r.InputFallback != nil {
			r.InputFallback.CountRequest = nil
			r.InputFallback.CalibrationKey = ""
		}
		return
	}
	id := strings.TrimSpace(resp.Header.Get("X-Oneapi-Request-Id"))
	if len(id) <= maxUsageUpstreamRequestIDLen && !strings.ContainsAny(id, "\r\n\t") {
		r.UpstreamRequestID = id
	}
}
func (c *openAIChatUsageCapture) observe(payload string) {
	if c == nil || c.current == nil || !gjson.Valid(payload) {
		return
	}
	c.current.mergeUsage(gjson.Get(payload, "usage"))
	c.received.observe(gjson.Parse(payload))
	if c.received.bytes > 0 {
		c.current.UpstreamAccepted = true
	}
}
func openAIChatHasError(payload string) bool {
	value := gjson.Get(payload, "error")
	return value.Exists() && value.Type != gjson.Null
}
func (c *openAIChatUsageCapture) rejectEmptyAttempt() {
	// An error envelope can follow paid generation without any answer text.
	// Positive provider usage still proves this attempt was accepted.
	if c != nil && c.current != nil && (c.received == nil || c.received.bytes == 0) &&
		!openAIChatUsageHasTokens(c.current.Usage) && (!c.current.ReasoningReported || c.current.ReasoningTokens <= 0) {
		c.current.UpstreamAccepted = false
	}
}
func (c *openAIChatUsageCapture) deliveredLine(line string) {
	if c == nil || c.delivered == nil {
		return
	}
	if payload, ok := extractOpenAISSEDataLine(line); ok {
		c.delivered.ObserveOpenAISSEData(payload)
		c.outcome.ObserveOpenAISSEData(payload)
		observeOpenAIChatReasoningAlias(c.delivered, payload)
		observeOpenAIChatReasoningAlias(c.outcome, payload)
	}
}
func (c *openAIChatUsageCapture) finishRound() {
	if c == nil || c.current == nil {
		return
	}
	if c.received != nil {
		c.current.ReceivedOutputTokens = c.received.count()
		c.current.ReceivedReasoningTokens = c.received.reasoningTokens
		c.current.OutputTruncated = c.received.truncated
		c.received = nil
	}
	if c.delivered != nil {
		c.current.Outcome = c.delivered.Snapshot()
	}
}
func (c *openAIChatUsageCapture) snapshot() *OpenAIChatUsageEvidence {
	c.finishRound()
	return &c.evidence
}
func (c *openAIChatUsageCapture) markCompleted(reason string) {
	if c != nil {
		c.delivered.MarkCompleted(reason)
		c.outcome.MarkCompleted(reason)
	}
}
func (c *openAIChatUsageCapture) markError(err error, client bool, server bool) {
	if c == nil || err == nil {
		return
	}
	c.delivered.MarkStreamError(err, client)
	c.outcome.MarkStreamError(err, client)
	// MarkCompleted may have observed a terminal frame before a later write/read
	// failed. Completion requires both the terminal and successful delivery.
	for _, collector := range []*ResponseOutcomeCollector{c.delivered, c.outcome} {
		collector.mu.Lock()
		collector.outcome.StreamCompleted = false
		if server && !client {
			collector.outcome.DisconnectSource = DisconnectSourceServer
		}
		collector.mu.Unlock()
	}
}
func ensureOpenAIChatUsageCapture(c *gin.Context, resp *http.Response, account *Account) *openAIChatUsageCapture {
	capture := openAIChatCaptureFromContext(c.Request.Context())
	if capture == nil {
		ctx, created := newOpenAIChatUsageCapture(c.Request.Context())
		c.Request = c.Request.WithContext(ctx)
		capture = created
		capture.begin(nil, account)
		capture.response(resp)
	}
	return capture
}

func openAIChatReportedCount(root gjson.Result, paths ...string) (int, bool) {
	for _, path := range paths {
		value := root.Get(path)
		if !value.Exists() {
			continue
		}
		if value.Type != gjson.Number {
			return 0, false
		}
		n, err := strconv.ParseInt(value.Raw, 10, 64)
		if err != nil || n < 0 || n > 2147483647 {
			return 0, false
		}
		return int(n), true
	}
	return 0, false
}
func (r *OpenAIChatUsageRound) mergeUsage(root gjson.Result) {
	if r == nil || !root.IsObject() {
		return
	}
	set := func(target *int, present *bool, paths ...string) {
		if n, ok := openAIChatReportedCount(root, paths...); ok {
			*target = n
			*present = true
		}
	}
	set(&r.Usage.InputTokens, &r.InputReported, "input_tokens", "prompt_tokens")
	set(&r.CompletionTokens, &r.OutputReported, "output_tokens", "completion_tokens")
	set(&r.Usage.CacheReadInputTokens, &r.CacheReadReported, "input_tokens_details.cached_tokens", "prompt_tokens_details.cached_tokens", "cache_read_input_tokens", "cache_read_tokens", "cached_tokens")
	set(&r.Usage.CacheCreationInputTokens, &r.CacheWriteReported, "input_tokens_details.cache_write_tokens", "prompt_tokens_details.cache_write_tokens", "input_tokens_details.cache_creation_tokens", "prompt_tokens_details.cache_creation_tokens", "cache_write_tokens", "cache_creation_input_tokens", "cache_write_input_tokens", "cache_creation_tokens")
	set(&r.ReasoningTokens, &r.ReasoningReported, "completion_tokens_details.reasoning_tokens", "output_tokens_details.reasoning_tokens")
	set(&r.TotalTokens, &r.TotalReported, "total_tokens")
	r.Usage.OutputTokens = r.CompletionTokens
	if r.OutputReported && r.ReasoningReported {
		r.Usage.OutputTokens = int(xai.IncludeIndependentReasoningTokens(int64(r.Usage.InputTokens), int64(r.CompletionTokens), int64(r.TotalTokens), int64(r.ReasoningTokens)))
	}
	for _, field := range []struct {
		dst   *int
		paths []string
	}{{&r.Usage.ImageInputTokens, []string{"input_tokens_details.image_tokens", "prompt_tokens_details.image_tokens"}}, {&r.Usage.ImageOutputTokens, []string{"output_tokens_details.image_tokens", "completion_tokens_details.image_tokens"}}} {
		if n, ok := openAIChatReportedCount(root, field.paths...); ok {
			*field.dst = n
		}
	}
	if r.InputReported {
		r.InputTokensSource = UsageTokenSourceUpstream
	}
	if r.OutputReported {
		r.OutputTokensSource = UsageTokenSourceUpstream
	}
}

// Text is bounded on the live read loop and tokenized only once per round.
// delta is incremental; message is a cumulative snapshot. Never deduplicate a
// repeated delta, because repeating words can be legitimate generated output.
type openAIChatSemanticOutput struct {
	parts           map[string]*strings.Builder
	bytes           int
	truncated       bool
	reasoningTokens int
}

func (o *openAIChatSemanticOutput) add(key, value string, snapshot bool) {
	if value == "" {
		return
	}
	if o.parts == nil {
		o.parts = make(map[string]*strings.Builder)
	}
	b := o.parts[key]
	if b == nil {
		if len(o.parts) >= geminiFallbackMaxStreams {
			o.truncated = true
			return
		}
		b = &strings.Builder{}
		o.parts[key] = b
	}
	if snapshot {
		if strings.HasPrefix(value, b.String()) {
			value = value[b.Len():]
		} else if strings.HasPrefix(b.String(), value) {
			return
		} else {
			o.bytes -= b.Len()
			b.Reset()
		}
	}
	remaining := geminiFallbackMaxSemanticBytes - o.bytes
	if len(value) > remaining {
		o.truncated = true
		value = value[:max(remaining, 0)]
		for len(value) > 0 && !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	_, _ = b.WriteString(value)
	o.bytes += len(value)
}
func (o *openAIChatSemanticOutput) observe(root gjson.Result) {
	root.Get("choices").ForEach(func(pos, choice gjson.Result) bool {
		index := pos.Int()
		if n, ok := openAIChatReportedCount(choice, "index"); ok {
			index = int64(n)
		}
		msg := choice.Get("delta")
		snapshot := false
		if !msg.IsObject() {
			msg = choice.Get("message")
			snapshot = true
		}
		prefix := strconv.FormatInt(index, 10) + ":"
		for _, field := range []string{"content", "reasoning_content", "reasoning", "refusal"} {
			v := msg.Get(field)
			if v.Type == gjson.String {
				o.add(prefix+field, v.String(), snapshot)
			}
		}
		msg.Get("tool_calls").ForEach(func(pos, call gjson.Result) bool {
			idx := pos.Int()
			if n, ok := openAIChatReportedCount(call, "index"); ok {
				idx = int64(n)
			}
			base := prefix + "tool:" + strconv.FormatInt(idx, 10)
			o.add(base+":name", call.Get("function.name").String(), true)
			o.add(base+":args", call.Get("function.arguments").String(), snapshot)
			return true
		})
		o.add(prefix+"function:name", msg.Get("function_call.name").String(), true)
		o.add(prefix+"function:args", msg.Get("function_call.arguments").String(), snapshot)
		return true
	})
}
func (o *openAIChatSemanticOutput) count() int {
	codec, err := tokenizer.Get(tokenizer.O200kBase)
	if err != nil {
		return 0
	}
	total := 0
	for key, part := range o.parts {
		n, err := codec.Count(part.String())
		if err == nil {
			total += n
			if strings.HasSuffix(key, ":reasoning_content") || strings.HasSuffix(key, ":reasoning") {
				o.reasoningTokens += n
			}
		}
	}
	o.parts = nil
	return total
}
func openAIChatHasSemanticOutput(payload string) bool {
	if !gjson.Valid(payload) {
		return false
	}
	collector := NewResponseOutcomeCollector(200, 200)
	_ = collector.ObserveJSONPayload(ResponseOutcomeProtocolOpenAI, []byte(payload))
	observeOpenAIChatReasoningAlias(collector, payload)
	return collector.Snapshot().HasEffectiveOutput()
}

func observeOpenAIChatReasoningAlias(collector *ResponseOutcomeCollector, payload string) {
	gjson.Get(payload, "choices").ForEach(func(_, choice gjson.Result) bool {
		collector.ObserveReasoning(choice.Get("delta.reasoning").String())
		collector.ObserveReasoning(choice.Get("message.reasoning").String())
		return true
	})
}

func prepareOpenAIChatInputUsageFallback(req *http.Request, accountID int64, model string) *GeminiInputUsageFallback {
	fallback := &GeminiInputUsageFallback{}
	if req == nil || req.GetBody == nil {
		return fallback
	}
	reader, err := req.GetBody()
	if err != nil {
		return fallback
	}
	raw, err := io.ReadAll(io.LimitReader(reader, geminiFallbackMaxSemanticBytes+1))
	_ = reader.Close()
	if err != nil || len(raw) > geminiFallbackMaxSemanticBytes {
		return fallback
	}
	var body map[string]json.RawMessage
	if json.Unmarshal(raw, &body) != nil {
		return fallback
	}
	codec, err := tokenizer.Get(tokenizer.O200kBase)
	if err != nil {
		return fallback
	}
	counter := geminiInputSemanticCounter{codec: codec, textOnly: true}
	var messages []map[string]json.RawMessage
	if json.Unmarshal(body["messages"], &messages) != nil {
		return fallback
	}
	for _, msg := range messages {
		counter.tokens += 4
		for _, key := range []string{"role", "name", "reasoning_content", "reasoning"} {
			openAIChatCountString(&counter, msg[key])
		}
		openAIChatCountInputContent(&counter, msg["content"])
		openAIChatCountInputFunction(&counter, msg["function_call"])
		var calls []map[string]json.RawMessage
		if json.Unmarshal(msg["tool_calls"], &calls) == nil {
			for _, call := range calls {
				openAIChatCountInputFunction(&counter, call["function"])
			}
		}
		if len(msg["audio"]) > 0 {
			counter.textOnly = false
		}
	}
	for _, key := range []string{"tools", "functions", "response_format"} {
		if value := body[key]; len(value) > 0 && string(value) != "null" {
			counter.tools = true
			counter.semanticJSON(value)
		}
	}
	if counter.err != nil {
		return fallback
	}
	fallback.Tokens = max(counter.tokens, 0)
	fallback.Known = true
	// A Chat path alone does not prove support for a native countTokens endpoint.
	// Keep CountRequest nil rather than guessing a second provider protocol.
	if counter.textOnly && len(body["cachedContent"]) == 0 && len(body["cached_content"]) == 0 && accountID > 0 {
		identity := openAIChatRequestIdentity(req)
		if identity != "" {
			shape, language := "plain", "latin"
			if counter.tools {
				shape = "tools"
			}
			if counter.runes > 0 && counter.cjkRunes*5 >= counter.runes {
				language = "cjk"
			}
			bucket := 0
			for length := counter.tokens; length >= 128; length /= 4 {
				bucket++
			}
			digest := sha256.Sum256([]byte(fmt.Sprintf("cc\x00%d\x00%s\x00%s\x00%s\x00%s\x00%d", accountID, identity, model, shape, language, bucket)))
			fallback.CalibrationKey = hex.EncodeToString(digest[:])
		}
	}
	return fallback
}

func openAIChatRequestIdentity(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}
	credential := strings.Fields(req.Header.Get("Authorization"))
	if len(credential) != 2 || !strings.EqualFold(credential[0], "Bearer") {
		return ""
	}
	return GeminiUsageLedgerIdentity(req.URL.String(), credential[1])
}

func openAIChatCountString(counter *geminiInputSemanticCounter, raw json.RawMessage) {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		counter.text(value)
	}
}

func openAIChatCountInputFunction(counter *geminiInputSemanticCounter, raw json.RawMessage) {
	var function map[string]json.RawMessage
	if json.Unmarshal(raw, &function) != nil || function == nil {
		return
	}
	counter.tools = true
	openAIChatCountString(counter, function["name"])
	if arguments := function["arguments"]; len(arguments) > 0 {
		var text string
		if json.Unmarshal(arguments, &text) == nil {
			counter.text(text)
		} else {
			counter.semanticJSON(arguments)
		}
	}
}

func openAIChatCountInputContent(counter *geminiInputSemanticCounter, raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		counter.text(text)
		return
	}
	var parts []map[string]json.RawMessage
	if json.Unmarshal(raw, &parts) != nil {
		counter.textOnly = false
		return
	}
	for _, part := range parts {
		var kind string
		_ = json.Unmarshal(part["type"], &kind)
		switch kind {
		case "text", "input_text", "output_text":
			openAIChatCountString(counter, part["text"])
		case "refusal":
			openAIChatCountString(counter, part["refusal"])
		default:
			// URLs, encoded images/audio/files and opaque signatures are not
			// text tokens. Their presence also disqualifies text calibration.
			counter.textOnly = false
		}
	}
}
