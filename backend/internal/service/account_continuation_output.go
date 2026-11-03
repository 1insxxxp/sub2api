package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Capture limits only disable continuation. They never impose an output token
// limit on an otherwise valid response.
const accountContinuationCaptureLimit = 16 << 20

type AccountContinuationUsage struct {
	InputTokens              int
	OutputTokens             int
	CacheReadInputTokens     int
	CacheCreationInputTokens int
	ReasoningTokens          int
}

type AccountContinuationRound struct {
	Text         string
	Truncated    bool
	Unsafe       bool
	Terminal     bool
	FinishReason string
	ResponseID   string
	HTTPStatus   int
}

type accountContinuationEvent struct {
	name string
	data map[string]any
	done bool
}

// AccountContinuationOutput gives all internal rounds one client lifecycle.
// Text deltas are emitted immediately; only small terminal frames and the text
// needed to reconstruct a safe continuation are retained.
type AccountContinuationOutput struct {
	base            gin.ResponseWriter
	protocol        AccountContinuationProtocol
	stream          bool
	current         *AccountContinuationRoundWriter
	rounds          int
	logicalID       string
	sequence        int
	nextBlock       int
	nextOutput      int
	text            strings.Builder
	responseOutput  []any
	completed       bool
	err             error
	reasoningTokens int
}

type AccountContinuationRoundWriter struct {
	gin.ResponseWriter
	output          *AccountContinuationOutput
	number          int
	status          int
	size            int
	pending         []byte
	buffer          bytes.Buffer
	text            strings.Builder
	terminals       []accountContinuationEvent
	round           AccountContinuationRound
	blocks          map[int]int
	items           map[string]string
	outputOffset    int
	maxOutput       int
	buffered        map[string]any
	responseItems   []any
	finished        bool
	passthrough     bool
	reasoningTokens int
}

func NewAccountContinuationOutput(base gin.ResponseWriter, protocol AccountContinuationProtocol, stream bool) *AccountContinuationOutput {
	return &AccountContinuationOutput{base: base, protocol: protocol, stream: stream}
}

func (o *AccountContinuationOutput) PublicResponseID() string { return o.logicalID }

func (o *AccountContinuationOutput) BeginRound() *AccountContinuationRoundWriter {
	w := &AccountContinuationRoundWriter{ResponseWriter: o.base, output: o, number: o.rounds, status: http.StatusOK, size: -1, blocks: make(map[int]int), items: make(map[string]string), outputOffset: o.nextOutput, maxOutput: -1}
	o.rounds++
	o.current = w
	return w
}

func (w *AccountContinuationRoundWriter) WriteHeader(status int) {
	if status < 100 || status > 999 {
		return
	}
	w.status = status
	if !w.output.base.Written() {
		w.output.base.WriteHeader(status)
	}
}
func (w *AccountContinuationRoundWriter) WriteHeaderNow() {
	if !w.output.base.Written() {
		w.output.base.WriteHeaderNow()
	}
}
func (w *AccountContinuationRoundWriter) Status() int { return w.status }
func (w *AccountContinuationRoundWriter) Size() int   { return w.size }
func (w *AccountContinuationRoundWriter) Written() bool {
	return w.size >= 0 || w.output.base.Written()
}
func (w *AccountContinuationRoundWriter) Flush() { w.output.base.Flush() }
func (w *AccountContinuationRoundWriter) WriteString(value string) (int, error) {
	return w.Write([]byte(value))
}

func (w *AccountContinuationRoundWriter) Write(value []byte) (int, error) {
	if w.output.err != nil {
		return 0, w.output.err
	}
	if w.finished {
		return 0, errors.New("continuation round is already finished")
	}
	if w.size < 0 {
		w.size = 0
	}
	w.size += len(value)
	if !w.output.stream || w.status >= http.StatusBadRequest {
		if w.buffer.Len()+len(value) > accountContinuationCaptureLimit {
			// Buffered responses already require the upstream body in memory.
			// Disable another replay, but let the current valid JSON finish.
			w.round.Unsafe = true
		}
		if w.passthrough {
			_, w.output.err = w.output.base.Write(value)
		} else {
			_, _ = w.buffer.Write(value)
		}
		return len(value), w.output.err
	}
	if w.passthrough {
		_, w.output.err = w.output.base.Write(value)
		return len(value), w.output.err
	}
	w.pending = append(w.pending, value...)
	for {
		end, separator := bytes.Index(w.pending, []byte("\n\n")), 2
		if cr := bytes.Index(w.pending, []byte("\r\n\r\n")); cr >= 0 && (end < 0 || cr < end) {
			end, separator = cr, 4
		}
		if end < 0 {
			break
		}
		frame := bytes.Clone(w.pending[:end])
		w.pending = w.pending[end+separator:]
		if err := w.processFrame(frame); err != nil {
			w.output.err = err
			return 0, err
		}
	}
	if len(w.pending) > accountContinuationCaptureLimit {
		w.round.Unsafe = true
		_, w.output.err = w.output.base.Write(w.pending)
		w.pending = nil
		w.passthrough = true
	}
	return len(value), w.output.err
}

func (w *AccountContinuationRoundWriter) capture(text string) {
	if w.output.stream && (w.text.Len()+len(text) > accountContinuationCaptureLimit || w.output.text.Len()+w.text.Len()+len(text) > accountContinuationCaptureLimit) {
		w.round.Unsafe = true
		return
	}
	_, _ = w.text.WriteString(text)
}

func (w *AccountContinuationRoundWriter) processFrame(frame []byte) error {
	event := accountContinuationEvent{}
	var data []string
	for _, line := range strings.Split(strings.ReplaceAll(string(frame), "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "event:") {
			event.name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		}
		if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if len(data) == 0 {
		_, err := w.output.base.Write(append(frame, '\n', '\n'))
		return err
	}
	payload := strings.Join(data, "\n")
	if strings.TrimSpace(payload) == "[DONE]" {
		w.round.Terminal = true
		w.queueTerminal(accountContinuationEvent{done: true})
		return nil
	}
	if json.Unmarshal([]byte(payload), &event.data) != nil || event.data == nil {
		w.round.Unsafe = true
		_, err := w.output.base.Write(append(frame, '\n', '\n'))
		return err
	}
	w.observeReasoningUsage(event.data)
	if event.data["error"] != nil || event.name == "error" {
		w.round.Unsafe = true
		w.round.Terminal = true
		return w.emit(event)
	}
	switch w.output.protocol {
	case AccountContinuationChat:
		return w.processChat(event)
	case AccountContinuationMessages:
		return w.processMessages(event)
	case AccountContinuationGemini:
		return w.processGemini(event)
	case AccountContinuationResponses:
		return w.processResponses(event)
	default:
		w.round.Unsafe = true
		return w.emit(event)
	}
}

func (w *AccountContinuationRoundWriter) setResponseID(id string) {
	if id == "" {
		return
	}
	w.round.ResponseID = id
	if w.output.logicalID == "" {
		w.output.logicalID = id
	}
}

func (w *AccountContinuationRoundWriter) processChat(event accountContinuationEvent) error {
	data := event.data
	if id, _ := data["id"].(string); id != "" {
		w.setResponseID(id)
		data["id"] = w.output.logicalID
	}
	choices, _ := data["choices"].([]any)
	if len(choices) > 1 {
		w.round.Unsafe = true
	}
	terminal := false
	for _, value := range choices {
		choice, ok := value.(map[string]any)
		if !ok {
			w.round.Unsafe = true
			continue
		}
		if index, _ := choice["index"].(float64); index != 0 {
			w.round.Unsafe = true
		}
		delta, _ := choice["delta"].(map[string]any)
		if delta["tool_calls"] != nil || delta["function_call"] != nil || delta["reasoning_content"] != nil || delta["reasoning"] != nil {
			w.round.Unsafe = true
		}
		if text, ok := delta["content"].(string); ok {
			w.capture(text)
		}
		if reason, _ := choice["finish_reason"].(string); reason != "" {
			w.round.FinishReason = reason
			w.round.Terminal = true
			terminal = true
		}
	}
	if terminal {
		content := accountContinuationClone(data)
		delete(content, "usage")
		hasContent := false
		for _, value := range accountContinuationArray(content["choices"]) {
			choice, ok := value.(map[string]any)
			if !ok {
				continue
			}
			choice["finish_reason"] = nil
			delta, _ := choice["delta"].(map[string]any)
			if text, _ := delta["content"].(string); text != "" {
				hasContent = true
			}
		}
		if hasContent {
			if err := w.emit(accountContinuationEvent{name: event.name, data: content}); err != nil {
				return err
			}
		}
		for _, value := range choices {
			choice, _ := value.(map[string]any)
			delta, _ := choice["delta"].(map[string]any)
			delete(delta, "content")
			delete(delta, "role")
		}
		w.queueTerminal(event)
		return nil
	}
	if data["usage"] != nil {
		if len(choices) == 0 {
			w.queueTerminal(event)
			return nil
		}
		// Some compatible providers repeat cumulative usage on every text
		// chunk. Retain only the usage for final aggregation, while letting
		// each content delta reach the client immediately and exactly once.
		usage := accountContinuationClone(data)
		usage["choices"] = []any{}
		w.queueTerminal(accountContinuationEvent{name: event.name, data: usage})
		delete(data, "usage")
	}
	if w.number > 0 && len(choices) == 1 {
		choice, _ := choices[0].(map[string]any)
		delta, _ := choice["delta"].(map[string]any)
		delete(delta, "role")
		if len(delta) == 0 {
			return nil
		}
	}
	return w.emit(event)
}

func (w *AccountContinuationRoundWriter) processMessages(event accountContinuationEvent) error {
	data := event.data
	kind, _ := data["type"].(string)
	if kind == "" {
		kind = event.name
	}
	switch kind {
	case "message_start":
		message, _ := data["message"].(map[string]any)
		if message == nil {
			w.round.Unsafe = true
			return w.emit(event)
		}
		id, _ := message["id"].(string)
		w.setResponseID(id)
		if w.number > 0 {
			return nil
		}
		message["id"] = w.output.logicalID
		message["usage"] = map[string]any{"input_tokens": 0, "output_tokens": 0}
	case "content_block_start":
		index := accountContinuationInt(data["index"])
		w.blocks[index] = w.output.nextBlock
		w.output.nextBlock++
		data["index"] = w.blocks[index]
		block, _ := data["content_block"].(map[string]any)
		if block["type"] != "text" {
			w.round.Unsafe = true
		}
		if text, _ := block["text"].(string); text != "" {
			w.capture(text)
		}
	case "content_block_delta":
		w.remapBlock(data)
		delta, _ := data["delta"].(map[string]any)
		if delta["type"] != "text_delta" {
			w.round.Unsafe = true
		}
		if text, _ := delta["text"].(string); text != "" {
			w.capture(text)
		}
	case "content_block_stop":
		w.remapBlock(data)
	case "message_delta":
		delta, _ := data["delta"].(map[string]any)
		if reason, _ := delta["stop_reason"].(string); reason != "" {
			w.round.FinishReason = reason
			w.round.Terminal = true
		}
		w.queueTerminal(event)
		return nil
	case "message_stop":
		w.round.Terminal = true
		w.queueTerminal(event)
		return nil
	case "error":
		w.round.Unsafe = true
		w.round.Terminal = true
	}
	return w.emit(event)
}

func (w *AccountContinuationRoundWriter) remapBlock(data map[string]any) {
	index := accountContinuationInt(data["index"])
	if mapped, ok := w.blocks[index]; ok {
		data["index"] = mapped
	} else {
		w.round.Unsafe = true
	}
}

func (w *AccountContinuationRoundWriter) processGemini(event accountContinuationEvent) error {
	data := event.data
	if response, ok := data["response"].(map[string]any); ok {
		data = response
		event.data = response
	}
	candidates, _ := data["candidates"].([]any)
	if len(candidates) > 1 {
		w.round.Unsafe = true
	}
	terminal := data["usageMetadata"] != nil
	hasText := false
	for _, value := range candidates {
		candidate, ok := value.(map[string]any)
		if !ok {
			w.round.Unsafe = true
			continue
		}
		if index, _ := candidate["index"].(float64); index != 0 {
			w.round.Unsafe = true
		}
		content, _ := candidate["content"].(map[string]any)
		parts, _ := content["parts"].([]any)
		for _, value := range parts {
			part, ok := value.(map[string]any)
			if !ok {
				w.round.Unsafe = true
				continue
			}
			if len(part) != 1 {
				w.round.Unsafe = true
			}
			if text, ok := part["text"].(string); ok {
				hasText = hasText || text != ""
				w.capture(text)
			} else {
				w.round.Unsafe = true
			}
		}
		if reason, _ := candidate["finishReason"].(string); reason != "" {
			w.round.FinishReason = reason
			w.round.Terminal = true
			terminal = true
		}
	}
	if terminal {
		if hasText {
			content := accountContinuationClone(data)
			delete(content, "usageMetadata")
			for _, value := range accountContinuationArray(content["candidates"]) {
				candidate, ok := value.(map[string]any)
				if !ok {
					continue
				}
				delete(candidate, "finishReason")
				delete(candidate, "finishMessage")
			}
			if err := w.emit(accountContinuationEvent{name: event.name, data: content}); err != nil {
				return err
			}
		}
		for _, value := range candidates {
			candidate, _ := value.(map[string]any)
			delete(candidate, "content")
		}
		w.queueTerminal(event)
		return nil
	}
	return w.emit(event)
}

func (w *AccountContinuationRoundWriter) responseItemID(id string) string {
	if id == "" {
		return id
	}
	if mapped, ok := w.items[id]; ok {
		return mapped
	}
	mapped := id
	if w.number > 0 {
		mapped = fmt.Sprintf("%s_cont_%d", id, w.number)
	}
	w.items[id] = mapped
	return mapped
}

func (w *AccountContinuationRoundWriter) processResponses(event accountContinuationEvent) error {
	data := event.data
	kind, _ := data["type"].(string)
	if kind == "" {
		kind = event.name
	}
	if response, ok := data["response"].(map[string]any); ok {
		id, _ := response["id"].(string)
		w.setResponseID(id)
		response["id"] = w.output.logicalID
	}
	if _, ok := data["response_id"]; ok {
		data["response_id"] = w.output.logicalID
	}
	if index, ok := data["output_index"]; ok {
		raw := accountContinuationInt(index)
		data["output_index"] = raw + w.outputOffset
		if raw > w.maxOutput {
			w.maxOutput = raw
		}
	}
	if id, ok := data["item_id"].(string); ok {
		data["item_id"] = w.responseItemID(id)
	}
	if item, ok := data["item"].(map[string]any); ok {
		if id, ok := item["id"].(string); ok {
			item["id"] = w.responseItemID(id)
		}
		if item["type"] != "message" {
			w.round.Unsafe = true
		}
	}
	switch kind {
	case "response.created", "response.in_progress":
		if w.number > 0 {
			return nil
		}
	case "response.output_text.delta":
		if text, _ := data["delta"].(string); text != "" {
			w.capture(text)
		}
	case "response.content_part.added":
		part, _ := data["part"].(map[string]any)
		if part["type"] != "output_text" {
			w.round.Unsafe = true
		}
	case "response.completed", "response.done", "response.incomplete", "response.failed", "response.cancelled", "response.canceled":
		w.round.Terminal = true
		response, _ := data["response"].(map[string]any)
		if response == nil {
			w.round.Unsafe = true
			w.queueTerminal(event)
			return nil
		}
		status, _ := response["status"].(string)
		if kind == "response.incomplete" || status == "incomplete" {
			details, _ := response["incomplete_details"].(map[string]any)
			w.round.FinishReason, _ = details["reason"].(string)
		} else if kind == "response.failed" || kind == "response.cancelled" || kind == "response.canceled" {
			w.round.Unsafe = true
			w.round.FinishReason = status
		} else {
			w.round.FinishReason = "stop"
		}
		if items, ok := response["output"].([]any); ok {
			for _, value := range items {
				item, ok := value.(map[string]any)
				if !ok {
					w.round.Unsafe = true
					continue
				}
				if id, ok := item["id"].(string); ok {
					item["id"] = w.responseItemID(id)
				}
				if !accountContinuationResponseItemSafe(item) {
					w.round.Unsafe = true
				}
				w.responseItems = append(w.responseItems, item)
			}
		}
		w.queueTerminal(event)
		return nil
	default:
		if strings.Contains(kind, "reasoning") || strings.Contains(kind, "function_call") || strings.Contains(kind, "refusal") || strings.Contains(kind, "audio") || strings.Contains(kind, "image") {
			w.round.Unsafe = true
		}
	}
	return w.emit(event)
}

func accountContinuationResponseItemSafe(item map[string]any) bool {
	if item["type"] != "message" {
		return false
	}
	parts, ok := item["content"].([]any)
	if !ok {
		return false
	}
	for _, value := range parts {
		part, ok := value.(map[string]any)
		if !ok || part["type"] != "output_text" {
			return false
		}
	}
	return true
}

func (w *AccountContinuationRoundWriter) observeReasoningUsage(data map[string]any) {
	if response, ok := data["response"].(map[string]any); ok {
		data = response
	}
	if usage, ok := data["usage"].(map[string]any); ok {
		for _, key := range []string{"output_tokens_details", "completion_tokens_details"} {
			if details, ok := usage[key].(map[string]any); ok {
				if n := accountContinuationInt(details["reasoning_tokens"]); n > w.reasoningTokens {
					w.reasoningTokens = n
				}
			}
		}
	}
	if usage, ok := data["usageMetadata"].(map[string]any); ok {
		if n := accountContinuationInt(usage["thoughtsTokenCount"]); n > w.reasoningTokens {
			w.reasoningTokens = n
		}
	}
}

func (w *AccountContinuationRoundWriter) queueTerminal(event accountContinuationEvent) {
	key := w.terminalKey(event)
	for i, existing := range w.terminals {
		if w.terminalKey(existing) == key {
			w.terminals[i] = event
			return
		}
	}
	w.terminals = append(w.terminals, event)
}

func (w *AccountContinuationRoundWriter) terminalKey(event accountContinuationEvent) string {
	if event.done {
		return "done"
	}
	switch w.output.protocol {
	case AccountContinuationChat:
		if choices, _ := event.data["choices"].([]any); len(choices) > 0 {
			return "finish"
		}
		return "usage"
	case AccountContinuationMessages:
		kind, _ := event.data["type"].(string)
		return kind
	case AccountContinuationGemini:
		for _, value := range accountContinuationArray(event.data["candidates"]) {
			candidate, _ := value.(map[string]any)
			if candidate["finishReason"] != nil {
				return "finish"
			}
		}
		return "usage"
	case AccountContinuationResponses:
		return "response-terminal"
	}
	return "terminal"
}

func (w *AccountContinuationRoundWriter) emit(event accountContinuationEvent) error {
	if w.output.err != nil {
		return w.output.err
	}
	var frame []byte
	if event.done {
		frame = []byte("data: [DONE]\n\n")
	} else {
		if w.output.protocol == AccountContinuationResponses {
			event.data["sequence_number"] = w.output.sequence
			w.output.sequence++
		}
		payload, err := json.Marshal(event.data)
		if err != nil {
			return err
		}
		if event.name != "" {
			frame = append(frame, []byte("event: "+event.name+"\n")...)
		}
		frame = append(frame, []byte("data: ")...)
		frame = append(frame, payload...)
		frame = append(frame, '\n', '\n')
	}
	_, err := w.output.base.Write(frame)
	if err == nil {
		w.output.base.Flush()
	}
	w.output.err = err
	return err
}

func (w *AccountContinuationRoundWriter) Finish() (AccountContinuationRound, error) {
	if w.finished {
		return w.round, w.output.err
	}
	w.finished = true
	if w.output.stream && w.status < http.StatusBadRequest && !w.passthrough {
		if len(bytes.TrimSpace(w.pending)) > 0 {
			if err := w.processFrame(w.pending); err != nil {
				w.output.err = err
			}
		}
		w.pending = nil
	} else if !w.passthrough && w.buffer.Len() > 0 {
		if json.Unmarshal(w.buffer.Bytes(), &w.buffered) != nil || w.buffered == nil {
			w.round.Unsafe = true
		} else {
			w.observeBuffered()
		}
	}
	if w.status >= http.StatusBadRequest {
		w.round.Unsafe = true
		w.round.Terminal = true
	}
	w.round.HTTPStatus = w.status
	w.round.Text = w.text.String()
	w.round.Truncated = accountContinuationReasonIsLimit(w.round.FinishReason) || (!w.round.Terminal && w.round.Text != "" && !w.round.Unsafe)
	if w.round.FinishReason != "" && !accountContinuationReasonIsLimit(w.round.FinishReason) {
		w.round.Truncated = false
	}
	if !w.output.stream || w.output.text.Len()+w.text.Len() <= accountContinuationCaptureLimit {
		_, _ = w.output.text.WriteString(w.round.Text)
	} else {
		w.round.Unsafe = true
	}
	if w.maxOutput >= 0 {
		w.output.nextOutput = w.outputOffset + w.maxOutput + 1
	}
	w.output.reasoningTokens += w.reasoningTokens
	w.output.responseOutput = append(w.output.responseOutput, w.responseItems...)
	return w.round, w.output.err
}

func (w *AccountContinuationRoundWriter) observeBuffered() {
	data := w.buffered
	w.observeReasoningUsage(data)
	if data["error"] != nil {
		w.round.Unsafe = true
		w.round.Terminal = true
		return
	}
	if id, _ := data["id"].(string); id != "" {
		w.setResponseID(id)
	}
	switch w.output.protocol {
	case AccountContinuationChat:
		choices, ok := data["choices"].([]any)
		if !ok || len(choices) != 1 {
			w.round.Unsafe = true
			return
		}
		choice, _ := choices[0].(map[string]any)
		message, _ := choice["message"].(map[string]any)
		if message["tool_calls"] != nil || message["function_call"] != nil || message["reasoning_content"] != nil || message["reasoning"] != nil {
			w.round.Unsafe = true
		}
		if text, ok := message["content"].(string); ok {
			w.capture(text)
		} else {
			w.round.Unsafe = true
		}
		w.round.FinishReason, _ = choice["finish_reason"].(string)
		w.round.Terminal = w.round.FinishReason != ""
	case AccountContinuationMessages:
		parts, _ := data["content"].([]any)
		for _, value := range parts {
			part, ok := value.(map[string]any)
			if !ok || part["type"] != "text" {
				w.round.Unsafe = true
				continue
			}
			text, _ := part["text"].(string)
			w.capture(text)
		}
		w.round.FinishReason, _ = data["stop_reason"].(string)
		w.round.Terminal = w.round.FinishReason != ""
	case AccountContinuationGemini:
		if response, ok := data["response"].(map[string]any); ok {
			data = response
			w.buffered = response
		}
		candidates, ok := data["candidates"].([]any)
		if !ok || len(candidates) != 1 {
			w.round.Unsafe = true
			return
		}
		candidate, _ := candidates[0].(map[string]any)
		content, _ := candidate["content"].(map[string]any)
		parts, _ := content["parts"].([]any)
		for _, value := range parts {
			part, ok := value.(map[string]any)
			if !ok || len(part) != 1 {
				w.round.Unsafe = true
				continue
			}
			text, ok := part["text"].(string)
			if !ok {
				w.round.Unsafe = true
			}
			w.capture(text)
		}
		w.round.FinishReason, _ = candidate["finishReason"].(string)
		w.round.Terminal = w.round.FinishReason != ""
	case AccountContinuationResponses:
		items, _ := data["output"].([]any)
		for _, value := range items {
			item, ok := value.(map[string]any)
			if !ok || !accountContinuationResponseItemSafe(item) {
				w.round.Unsafe = true
				continue
			}
			if id, ok := item["id"].(string); ok {
				item["id"] = w.responseItemID(id)
			}
			for _, value := range accountContinuationArray(item["content"]) {
				part, _ := value.(map[string]any)
				text, _ := part["text"].(string)
				w.capture(text)
			}
			w.responseItems = append(w.responseItems, item)
		}
		status, _ := data["status"].(string)
		w.round.Terminal = status != ""
		switch status {
		case "incomplete":
			details, _ := data["incomplete_details"].(map[string]any)
			w.round.FinishReason, _ = details["reason"].(string)
		case "completed":
			w.round.FinishReason = "stop"
		default:
			w.round.Unsafe = true
			w.round.FinishReason = status
		}
	}
}

func (o *AccountContinuationOutput) Complete(usage AccountContinuationUsage) error {
	if o.completed {
		return o.err
	}
	o.completed = true
	if o.current == nil {
		return o.err
	}
	if o.reasoningTokens > usage.ReasoningTokens {
		usage.ReasoningTokens = o.reasoningTokens
	}
	w := o.current
	if !w.finished {
		if _, err := w.Finish(); err != nil {
			return err
		}
	}
	if o.reasoningTokens > usage.ReasoningTokens {
		usage.ReasoningTokens = o.reasoningTokens
	}
	if o.err != nil {
		return o.err
	}
	if w.passthrough {
		return nil
	}
	if !o.stream {
		if w.buffer.Len() == 0 && w.buffered == nil {
			return nil
		}
		if w.buffered == nil || (w.round.Unsafe && o.rounds == 1) {
			_, o.err = o.base.Write(w.buffer.Bytes())
			return o.err
		}
		data := w.buffered
		if o.logicalID != "" && data["id"] != nil {
			data["id"] = o.logicalID
		}
		switch o.protocol {
		case AccountContinuationChat:
			choices, _ := data["choices"].([]any)
			if len(choices) == 1 {
				choice, _ := choices[0].(map[string]any)
				message, _ := choice["message"].(map[string]any)
				if message != nil {
					message["content"] = o.text.String()
				}
			}
		case AccountContinuationMessages:
			data["content"] = []any{map[string]any{"type": "text", "text": o.text.String()}}
		case AccountContinuationGemini:
			candidates, _ := data["candidates"].([]any)
			if len(candidates) == 1 {
				candidate, _ := candidates[0].(map[string]any)
				candidate["content"] = map[string]any{"role": "model", "parts": []any{map[string]any{"text": o.text.String()}}}
			}
		case AccountContinuationResponses:
			data["output"] = o.responseOutput
		}
		o.applyUsage(data, usage)
		payload, err := json.Marshal(data)
		if err != nil {
			return err
		}
		_, o.err = o.base.Write(payload)
		return o.err
	}
	if w.status >= http.StatusBadRequest {
		if o.rounds == 1 {
			_, o.err = o.base.Write(w.buffer.Bytes())
			return o.err
		}
		// A later HTTP failure cannot replace an already emitted stream with
		// plain JSON or a successful terminal. Emit one protocol error instead.
		failure := map[string]any{"type": "error", "error": map[string]any{"type": "server_error", "message": "Upstream continuation failed"}}
		return w.emit(accountContinuationEvent{name: "error", data: failure})
	}
	if len(w.terminals) == 0 {
		return nil
	}
	if o.protocol == AccountContinuationGemini {
		var final accountContinuationEvent
		for _, event := range w.terminals {
			if event.done {
				continue
			}
			if final.data == nil || w.terminalKey(event) == "finish" {
				final = event
			}
		}
		if final.data != nil {
			final.data["usageMetadata"] = accountContinuationGeminiUsage(usage)
			return w.emit(final)
		}
		return nil
	}
	sawUsage, sawDone := false, false
	for _, event := range w.terminals {
		if event.done {
			if o.protocol == AccountContinuationChat && !sawUsage {
				if err := w.emit(accountContinuationEvent{data: map[string]any{"choices": []any{}, "usage": accountContinuationChatUsage(usage)}}); err != nil {
					return err
				}
				sawUsage = true
			}
			sawDone = true
		} else {
			switch o.protocol {
			case AccountContinuationChat:
				if event.data["usage"] != nil {
					event.data["usage"] = accountContinuationChatUsage(usage)
					sawUsage = true
				}
			case AccountContinuationMessages:
				if event.data["type"] == "message_delta" {
					event.data["usage"] = accountContinuationMessagesUsage(usage)
					sawUsage = true
				}
			case AccountContinuationGemini:
				if event.data["usageMetadata"] != nil {
					if sawUsage {
						continue
					}
					event.data["usageMetadata"] = accountContinuationGeminiUsage(usage)
					sawUsage = true
				}
			case AccountContinuationResponses:
				if response, ok := event.data["response"].(map[string]any); ok {
					response["output"] = o.responseOutput
					response["usage"] = accountContinuationResponsesUsage(usage)
					if response["status"] == "completed" {
						delete(response, "incomplete_details")
					}
					sawUsage = true
				}
			}
		}
		if err := w.emit(event); err != nil {
			return err
		}
	}
	if o.protocol == AccountContinuationChat && !sawDone {
		return w.emit(accountContinuationEvent{done: true})
	}
	return o.err
}

func (o *AccountContinuationOutput) applyUsage(data map[string]any, usage AccountContinuationUsage) {
	switch o.protocol {
	case AccountContinuationChat:
		data["usage"] = accountContinuationChatUsage(usage)
	case AccountContinuationMessages:
		data["usage"] = accountContinuationMessagesUsage(usage)
	case AccountContinuationGemini:
		data["usageMetadata"] = accountContinuationGeminiUsage(usage)
	case AccountContinuationResponses:
		data["usage"] = accountContinuationResponsesUsage(usage)
	}
}

func accountContinuationChatUsage(u AccountContinuationUsage) map[string]any {
	return map[string]any{"prompt_tokens": u.InputTokens, "completion_tokens": u.OutputTokens, "total_tokens": u.InputTokens + u.OutputTokens, "prompt_tokens_details": map[string]any{"cached_tokens": u.CacheReadInputTokens}, "completion_tokens_details": map[string]any{"reasoning_tokens": u.ReasoningTokens}}
}
func accountContinuationMessagesUsage(u AccountContinuationUsage) map[string]any {
	return map[string]any{"input_tokens": u.InputTokens, "output_tokens": u.OutputTokens, "cache_read_input_tokens": u.CacheReadInputTokens, "cache_creation_input_tokens": u.CacheCreationInputTokens}
}
func accountContinuationGeminiUsage(u AccountContinuationUsage) map[string]any {
	return map[string]any{"promptTokenCount": u.InputTokens, "candidatesTokenCount": u.OutputTokens, "totalTokenCount": u.InputTokens + u.OutputTokens, "cachedContentTokenCount": u.CacheReadInputTokens, "thoughtsTokenCount": u.ReasoningTokens}
}
func accountContinuationResponsesUsage(u AccountContinuationUsage) map[string]any {
	return map[string]any{"input_tokens": u.InputTokens, "output_tokens": u.OutputTokens, "total_tokens": u.InputTokens + u.OutputTokens, "input_tokens_details": map[string]any{"cached_tokens": u.CacheReadInputTokens}, "output_tokens_details": map[string]any{"reasoning_tokens": u.ReasoningTokens}}
}
func accountContinuationClone(value map[string]any) map[string]any {
	payload, _ := json.Marshal(value)
	var clone map[string]any
	_ = json.Unmarshal(payload, &clone)
	return clone
}
func accountContinuationInt(value any) int {
	switch n := value.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

func accountContinuationArray(value any) []any { values, _ := value.([]any); return values }
