package service

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/typesafe"
	"github.com/tiktoken-go/tokenizer"
)

// EstimateCustomerInputTokenProtection counts only the original customer payload.
// Call before routing, history reconstruction or prompt/tool injection. A partial
// text count cannot establish a limit when any model input is opaque.
func EstimateCustomerInputTokenProtection(path string, body []byte) *InputTokenProtection {
	pending := &InputTokenProtection{Pending: true}
	if len(body) == 0 || len(body) > geminiInputCountMaxRequestBytes || !utf8.Valid(body) {
		return pending
	}
	path, _, _ = strings.Cut(path, "?")
	path = strings.TrimRight(path, "/")
	if path == openAIImagesEditsEndpoint || path == "/images/edits" {
		return pending
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	budget := 32 << 10
	value, ok := decodeCustomerInput(dec, 0, &budget)
	obj, isObject := value.(map[string]any)
	if !ok || !isObject {
		return pending
	}
	if _, err := dec.Token(); err != io.EOF {
		return pending
	}
	responses := strings.HasSuffix(path, "/responses") || strings.HasSuffix(path, "/responses/compact") ||
		strings.HasSuffix(path, "/responses/input_tokens") || path == "response.create" ||
		(path == "" && obj["type"] == "response.create")
	// For image generation, prompt is inline text. Its field allowlist below
	// rejects references and any other unrecognized source of model input.
	imageGeneration := path == openAIImagesGenerationsEndpoint || path == "/images/generations"
	if !imageGeneration && hasCustomerContextReference(obj) {
		return pending
	}
	if responses && obj["type"] != nil && obj["type"] != "response.create" {
		return pending
	}
	if responses && obj["type"] == "response.create" && obj["response"] != nil {
		// A nested realtime frame carries the request inside response. The
		// Responses WS transport also accepts flattened response.create frames.
		if obj["input"] != nil || obj["instructions"] != nil || obj["tools"] != nil {
			return pending
		}
		obj, ok = obj["response"].(map[string]any)
		if !ok || hasCustomerContextReference(obj) {
			return pending
		}
	}
	model, _ := obj["model"].(string)
	codec, err := openAIInputTokensCodecForModel(model)
	if err != nil {
		return pending
	}
	c := customerInputCounter{codec: codec}
	switch {
	case imageGeneration:
		ok = c.imagePrompt(obj)
	case path == typesafe.SystemOnePath:
		_, err := typesafe.ValidateSystemOneRequest(body)
		ok = err == nil && c.systemOne(obj)
	case strings.HasSuffix(path, "/embeddings"):
		input := obj["input"]
		if _, batch := input.([]any); batch {
			ok = everyCustomerInput(input, c.text)
		} else {
			ok = c.text(input)
		}
	case responses:
		ok = c.content(obj["input"]) && c.optionalContent(obj["instructions"]) &&
			c.tools(obj["tools"]) && c.outputSchema(obj["text"], "format")
	case strings.HasSuffix(path, "/chat/completions"):
		ok = everyCustomerInput(obj["messages"], c.message) && c.tools(obj["tools"]) &&
			c.tools(obj["functions"]) && c.outputSchema(obj, "response_format")
	case strings.HasSuffix(path, "/messages"), strings.HasSuffix(path, "/messages/count_tokens"):
		ok = everyCustomerInput(obj["messages"], c.message) && c.optionalContent(obj["system"]) && c.tools(obj["tools"]) &&
			c.outputSchema(obj["output_config"], "format")
	case path == "/v1beta/cachedContents":
		ok = c.geminiInputs(obj, false)
	case strings.HasSuffix(path, "/v1beta/models"), strings.HasSuffix(path, "/models/*modelAction"),
		strings.Contains(path, "/models/") && (strings.HasSuffix(path, ":generateContent") ||
			strings.HasSuffix(path, ":streamGenerateContent") || strings.HasSuffix(path, ":countTokens")):
		ok = c.geminiInputs(obj, true)
		for _, key := range []string{"generationConfig", "generation_config"} {
			cfg, _ := obj[key].(map[string]any)
			for _, schema := range []string{"responseSchema", "responseJsonSchema", "response_schema", "response_json_schema"} {
				if cfg[schema] != nil {
					ok = ok && c.data(cfg[schema])
				}
			}
		}
	default:
		return pending
	}
	if !ok || c.tokens == 0 {
		return pending
	}
	// Framing is an estimate. Many empty blocks must not manufacture an
	// unbounded allowance. The cap applies only to wrappers, never to text.
	return &InputTokenProtection{Limit: c.tokens + min(c.overhead, 256)}
}

// Limit depth and total values before allocating maps/slices. A byte cap
// alone still permits millions of empty containers in a small request.
func decodeCustomerInput(d *json.Decoder, depth int, remaining *int) (any, bool) {
	if depth > 64 || *remaining <= 0 {
		return nil, false
	}
	*remaining -= 1
	token, err := d.Token()
	if err != nil {
		return nil, false
	}
	switch token {
	case json.Delim('{'):
		obj := make(map[string]any)
		for d.More() {
			keyToken, err := d.Token()
			key, ok := keyToken.(string)
			if err != nil || !ok {
				return nil, false
			}
			if _, duplicate := obj[key]; duplicate {
				return nil, false
			}
			value, ok := decodeCustomerInput(d, depth+1, remaining)
			if !ok {
				return nil, false
			}
			obj[key] = value
		}
		end, err := d.Token()
		return obj, err == nil && end == json.Delim('}')
	case json.Delim('['):
		var items []any
		for d.More() {
			value, ok := decodeCustomerInput(d, depth+1, remaining)
			if !ok {
				return nil, false
			}
			items = append(items, value)
		}
		end, err := d.Token()
		return items, err == nil && end == json.Delim(']')
	default:
		return token, true
	}
}

func hasCustomerContextReference(obj map[string]any) bool {
	for _, key := range []string{"previous_response_id", "cachedContent", "cached_content", "conversation", "prompt", "container", "mcp_servers", "attachments"} {
		switch value := obj[key].(type) {
		case nil:
			continue
		case string:
			if value == "" {
				continue
			}
		case []any:
			if len(value) == 0 {
				continue
			}
		}
		return true
	}
	return false
}

func everyCustomerInput(value any, visit func(any) bool) bool {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return false
	}
	for _, item := range items {
		if !visit(item) {
			return false
		}
	}
	return true
}

type customerInputCounter struct {
	codec            tokenizer.Codec
	tokens, overhead int
}

func (c *customerInputCounter) text(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	n, err := countInflightText(c.codec, text)
	c.tokens += n
	return err == nil
}

func (c *customerInputCounter) data(value any) bool {
	if _, text := value.(string); text {
		return c.text(value)
	}
	n, err := countInflightJSON(c.codec, value)
	c.tokens += n
	return err == nil
}

// Provider-specific extensions can add image inputs or additional prompts.
// Only certify a prompt when all other fields describe known request options.
func (c *customerInputCounter) imagePrompt(obj map[string]any) bool {
	prompt, ok := obj["prompt"].(string)
	if !ok || strings.TrimSpace(prompt) == "" {
		return false
	}
	for field, value := range obj {
		switch field {
		case "prompt":
		case "model", "size", "quality", "style", "background", "moderation", "response_format",
			"output_format", "input_fidelity", "complexity", "aspect_ratio", "resolution", "user":
			if _, ok := value.(string); !ok {
				return false
			}
		case "n", "output_compression", "partial_images", "frame_count", "frame_duration_ms", "seed":
			if _, ok := value.(json.Number); !ok {
				return false
			}
		case "stream":
			if _, ok := value.(bool); !ok {
				return false
			}
		default:
			return false
		}
	}
	return c.text(prompt)
}

func (c *customerInputCounter) optionalContent(value any) bool {
	return value == nil || c.content(value)
}

func (c *customerInputCounter) message(value any) bool {
	msg, ok := value.(map[string]any)
	role, _ := msg["role"].(string)
	if !ok || role == "" || msg["audio"] != nil || msg["attachments"] != nil {
		return false
	}
	c.overhead += openAIResponsesInputItemTokenOverhead
	for _, key := range []string{"name", "reasoning", "reasoning_content", "refusal"} {
		if msg[key] != nil && !c.text(msg[key]) {
			return false
		}
	}
	if calls := msg["tool_calls"]; calls != nil {
		items, ok := calls.([]any)
		if !ok {
			return false
		}
		for _, call := range items {
			if !c.chatCall(call) {
				return false
			}
		}
	}
	if f := msg["function_call"]; f != nil && !c.function(f) {
		return false
	}
	return (msg["content"] != nil || msg["tool_calls"] != nil || msg["function_call"] != nil || msg["refusal"] != nil) &&
		c.optionalContent(msg["content"])
}

func (c *customerInputCounter) chatCall(value any) bool {
	call, ok := value.(map[string]any)
	return ok && c.function(call["function"])
}

func (c *customerInputCounter) function(value any) bool {
	fn, ok := value.(map[string]any)
	return ok && c.text(fn["name"]) && fn["arguments"] != nil && c.data(fn["arguments"])
}

func (c *customerInputCounter) content(value any) bool {
	switch item := value.(type) {
	case string:
		return c.text(item)
	case []any:
		return everyCustomerInput(item, c.content)
	case map[string]any:
		c.overhead += openAIResponsesContentPartOverhead
		switch item["type"] {
		case nil, "message":
			return c.message(item)
		case "text", "input_text", "output_text", "summary_text", "reasoning_text":
			return c.text(item["text"])
		case "thinking":
			return c.text(item["thinking"])
		case "refusal":
			return c.text(item["refusal"])
		case "reasoning":
			return item["encrypted_content"] == nil && (item["summary"] != nil || item["content"] != nil) &&
				c.optionalContent(item["summary"]) && c.optionalContent(item["content"])
		case "function_call", "custom_tool_call", "tool_use":
			args := item["arguments"]
			if args == nil {
				args = item["input"]
			}
			return c.text(item["name"]) && args != nil && c.data(args)
		case "function_call_output", "custom_tool_call_output":
			return c.toolOutput(item["output"])
		case "tool_result":
			return c.toolOutput(item["content"])
		case "document":
			source, _ := item["source"].(map[string]any)
			for _, key := range []string{"title", "context"} {
				if item[key] != nil && !c.text(item[key]) {
					return false
				}
			}
			switch source["type"] {
			case "text":
				return c.text(source["data"])
			case "content":
				return c.content(source["content"])
			}
		}
	}
	return false
}

func (c *customerInputCounter) toolOutput(value any) bool {
	if value == nil {
		return false
	}
	// A list of typed output parts can contain media or external references.
	// Arbitrary JSON tool results retain their keys as well as their values.
	if items, ok := value.([]any); ok {
		for _, entry := range items {
			if part, ok := entry.(map[string]any); ok && part["type"] != nil {
				return everyCustomerInput(items, c.content)
			}
		}
	}
	if part, ok := value.(map[string]any); ok {
		// A structured tool result can have its own "type" discriminator.
		// Only interpret known protocol blocks as content.
		if isInflightContentBlockList([]any{part}) {
			return c.content(part)
		}
	}
	return c.data(value)
}

// A cache can contain just a system instruction or tool declarations.
// Generation requests require at least one content entry.
func (c *customerInputCounter) geminiInputs(obj map[string]any, requireContents bool) bool {
	contents, ok := obj["contents"].([]any)
	if (obj["contents"] != nil && !ok) || (requireContents && len(contents) == 0) {
		return false
	}
	for _, content := range contents {
		if !c.geminiContent(content) {
			return false
		}
	}
	for _, key := range []string{"systemInstruction", "system_instruction"} {
		if obj[key] != nil && !c.geminiContent(obj[key]) {
			return false
		}
	}
	return c.tools(obj["tools"])
}

func (c *customerInputCounter) geminiContent(value any) bool {
	obj, ok := value.(map[string]any)
	c.overhead += geminiInputContentOverhead
	return ok && everyCustomerInput(obj["parts"], func(value any) bool {
		part, ok := value.(map[string]any)
		if !ok {
			return false
		}
		c.overhead += geminiInputPartOverhead
		seen := false
		for key, value := range part {
			switch key {
			case "text":
				if !c.text(value) {
					return false
				}
			case "functionCall", "function_call":
				fn, ok := value.(map[string]any)
				if !ok || !c.text(fn["name"]) || fn["partialArgs"] != nil || fn["partial_args"] != nil ||
					(fn["args"] != nil && !c.data(fn["args"])) {
					return false
				}
			case "functionResponse", "function_response":
				fn, ok := value.(map[string]any)
				if !ok || fn["parts"] != nil || !c.text(fn["name"]) || fn["response"] == nil || !c.data(fn["response"]) {
					return false
				}
			case "executableCode", "executable_code", "codeExecutionResult", "code_execution_result":
				payload, ok := value.(map[string]any)
				if !ok || !c.data(payload) {
					return false
				}
			case "thought", "thoughtSignature", "thought_signature", "metadata":
				continue
			default:
				return false
			}
			seen = true
		}
		return seen
	})
}

func (c *customerInputCounter) tools(value any) bool {
	if value == nil {
		return true
	}
	tools, ok := value.([]any)
	if !ok {
		return false
	}
	for _, tool := range tools {
		projected, ok := customerToolDefinition(tool)
		if !ok || !c.data(projected) {
			return false
		}
	}
	return true
}

// Tool wrappers carry transport metadata; schemas and grammar definitions are
// model input in their entirety, including application-defined property names.
func customerToolDefinition(value any) (any, bool) {
	switch v := value.(type) {
	case string: // Codex shorthand for a named custom tool.
		return v, v != ""
	case []any:
		defs := make([]any, 0, len(v))
		for _, item := range v {
			def, ok := customerToolDefinition(item)
			if !ok {
				return nil, false
			}
			defs = append(defs, def)
		}
		return defs, len(defs) > 0
	case map[string]any:
		switch v["type"] {
		case "mcp", "file_search", "retrieval":
			return nil, false
		}
		projected := make(map[string]any)
		for k, field := range v {
			switch k {
			case "function", "functionDeclarations", "function_declarations", "tools", "children":
				def, ok := customerToolDefinition(field)
				if !ok {
					return nil, false
				}
				projected[k] = def
			case "type", "name", "description", "strict", "parameters", "input_schema", "format", "execution",
				"parametersJsonSchema", "parameters_json_schema", "response", "responseJsonSchema", "response_json_schema",
				"allowed_x_handles", "excluded_x_handles", "from_date", "to_date", "enable_image_understanding", "enable_video_understanding",
				"googleSearch", "google_search", "codeExecution", "code_execution", "urlContext", "url_context":
				projected[k] = field
			case "metadata", "client_metadata", "cache_control", "_meta":
				// Transport only.
			default:
				// An unfamiliar tool option may refer to external data or change
				// the visible definition. Do not certify a partial count.
				return nil, false
			}
		}
		return projected, len(projected) > 0
	}
	return nil, false
}

func (c *customerInputCounter) outputSchema(parent any, key string) bool {
	obj, _ := parent.(map[string]any)
	format, _ := obj[key].(map[string]any)
	if format["type"] != "json_schema" {
		return true
	}
	if nested, ok := format["json_schema"].(map[string]any); ok {
		format = nested
	}
	return format["schema"] != nil && c.data(projectInflightTool(format))
}

// System One forwards the raw envelope. The validator permits vendor
// extensions; only the documented state and questions establish local evidence.
func (c *customerInputCounter) systemOne(obj map[string]any) bool {
	for key := range obj {
		switch key {
		case "model", "state", "questions", "stream", "metadata":
		default:
			return false
		}
	}
	if !localSystemOneValue(obj["state"]) || !c.data(obj["state"]) {
		return false
	}
	questions, _ := obj["questions"].(map[string]any)
	for label, raw := range questions {
		q, _ := raw.(map[string]any)
		if !c.text(label) || !c.text(q["type"]) {
			return false
		}
		c.overhead += openAIResponsesInputItemTokenOverhead
		for k, value := range q {
			switch k {
			case "type":
			case "instructions", "criteria":
				if value != nil && (!localSystemOneValue(value) || !c.data(value)) {
					return false
				}
			default:
				return false
			}
		}
	}
	return len(questions) > 0
}

// A System One state or criterion may be arbitrary JSON. Preserve domain
// keys and values, but do not tokenize a structured attachment as text.
func localSystemOneValue(value any) bool {
	switch v := value.(type) {
	case string:
		return !strings.HasPrefix(strings.TrimSpace(v), "data:")
	case []any:
		for _, child := range v {
			if !localSystemOneValue(child) {
				return false
			}
		}
	case map[string]any:
		switch v["type"] {
		case "image", "audio", "video", "file", "input_image", "input_file", "input_audio", "base64", "item_reference":
			return false
		}
		for key, child := range v {
			switch key {
			case "fileData", "file_data", "file_id", "inlineData", "inline_data", "image_url", "previous_response_id",
				"cachedContent", "cached_content", "encrypted_content":
				return false
			}
			if !localSystemOneValue(child) {
				return false
			}
		}
	}
	return true
}
