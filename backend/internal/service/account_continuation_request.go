package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

type AccountContinuationProtocol string

const (
	AccountContinuationChat      AccountContinuationProtocol = "chat"
	AccountContinuationMessages  AccountContinuationProtocol = "messages"
	AccountContinuationGemini    AccountContinuationProtocol = "gemini"
	AccountContinuationResponses AccountContinuationProtocol = "responses"
	accountContinuationPrompt                                = "Continue the response from where you left off. Do not repeat any earlier text."
)

// AccountContinuationRequestAllowed restricts transparent replay to transcripts
// whose contents can be reconstructed without tool, media or opaque state.
func AccountContinuationRequestAllowed(protocol AccountContinuationProtocol, body []byte) bool {
	var request map[string]any
	if json.Unmarshal(body, &request) != nil || request == nil {
		return false
	}
	if accountContinuationHasOpaqueState(request) {
		return false
	}
	if request["audio"] != nil || !accountContinuationTextModalitiesSafe(request["modalities"]) {
		return false
	}
	for _, key := range []string{"response_format", "guided_json", "guided_regex", "grammar", "json_schema", "previous_response_id", "conversation", "cachedContent", "cached_content"} {
		if value, ok := request[key]; ok && value != nil {
			return false
		}
	}
	for _, key := range []string{"system", "instructions"} {
		if value, exists := request[key]; exists && value != nil && !accountContinuationSystemSafe(value) {
			return false
		}
	}
	for _, key := range []string{"systemInstruction", "system_instruction"} {
		if value, exists := request[key]; exists && value != nil {
			system, ok := value.(map[string]any)
			if !ok || !accountContinuationGeminiPartsSafe(system["parts"]) {
				return false
			}
		}
	}
	for _, key := range []string{"tools", "functions"} {
		if value, ok := request[key]; ok && value != nil {
			items, valid := value.([]any)
			if !valid || len(items) != 0 {
				return false
			}
		}
	}
	if n, ok := request["n"].(float64); ok && n > 1 {
		return false
	}
	if text, ok := request["text"].(map[string]any); ok && text["format"] != nil {
		return false
	}
	switch protocol {
	case AccountContinuationChat, AccountContinuationMessages:
		messages, ok := request["messages"].([]any)
		if !ok {
			return false
		}
		return accountContinuationMessagesSafe(messages, false)
	case AccountContinuationGemini:
		if generation, ok := request["generationConfig"].(map[string]any); ok {
			if generation["speechConfig"] != nil || generation["imageConfig"] != nil || !accountContinuationTextModalitiesSafe(generation["responseModalities"]) {
				return false
			}
			if n, ok := generation["candidateCount"].(float64); ok && n > 1 {
				return false
			}
			if mime, ok := generation["responseMimeType"].(string); ok && mime != "" && mime != "text/plain" {
				return false
			}
			if generation["responseSchema"] != nil || generation["responseJsonSchema"] != nil {
				return false
			}
		}
		contents, ok := request["contents"].([]any)
		if !ok {
			return false
		}
		for _, value := range contents {
			message, ok := value.(map[string]any)
			if !ok {
				return false
			}
			role, _ := message["role"].(string)
			if role != "" && role != "user" && role != "model" {
				return false
			}
			parts, ok := message["parts"].([]any)
			if !ok {
				return false
			}
			for _, part := range parts {
				object, ok := part.(map[string]any)
				if !ok {
					return false
				}
				if _, ok := object["text"].(string); !ok || len(object) != 1 {
					return false
				}
			}
		}
		return true
	case AccountContinuationResponses:
		if _, ok := request["input"].(string); ok {
			return true
		}
		messages, ok := request["input"].([]any)
		return ok && accountContinuationMessagesSafe(messages, true)
	}
	return false
}

func accountContinuationTextModalitiesSafe(value any) bool {
	if value == nil {
		return true
	}
	modalities, ok := value.([]any)
	if !ok {
		return false
	}
	for _, value := range modalities {
		modality, ok := value.(string)
		if !ok || !strings.EqualFold(modality, "text") {
			return false
		}
	}
	return true
}

func accountContinuationHasOpaqueState(value any) bool {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			switch key {
			case "signature", "thoughtSignature", "thought_signature", "encrypted_content", "reasoning_content":
				return true
			}
			if accountContinuationHasOpaqueState(child) {
				return true
			}
		}
	case []any:
		for _, child := range value {
			if accountContinuationHasOpaqueState(child) {
				return true
			}
		}
	}
	return false
}

func accountContinuationGeminiPartsSafe(value any) bool {
	parts, ok := value.([]any)
	if !ok {
		return false
	}
	for _, value := range parts {
		part, ok := value.(map[string]any)
		if !ok || len(part) != 1 {
			return false
		}
		if _, ok := part["text"].(string); !ok {
			return false
		}
	}
	return true
}
func accountContinuationSystemSafe(value any) bool {
	if _, ok := value.(string); ok {
		return true
	}
	parts, ok := value.([]any)
	if !ok {
		return false
	}
	for _, value := range parts {
		part, ok := value.(map[string]any)
		if !ok || part["type"] != "text" {
			return false
		}
		if _, ok := part["text"].(string); !ok {
			return false
		}
		for key := range part {
			if key != "type" && key != "text" && key != "cache_control" {
				return false
			}
		}
	}
	return true
}

func accountContinuationMessagesSafe(messages []any, responses bool) bool {
	for _, value := range messages {
		message, ok := value.(map[string]any)
		if !ok {
			return false
		}
		if kind, ok := message["type"].(string); ok && kind != "message" {
			return false
		}
		role, _ := message["role"].(string)
		if role != "user" && role != "assistant" && role != "system" && role != "developer" {
			return false
		}
		for _, key := range []string{"tool_calls", "function_call", "tool_call_id", "reasoning", "reasoning_content"} {
			if _, exists := message[key]; exists {
				return false
			}
		}
		content := message["content"]
		if content == nil {
			continue
		}
		if _, ok := content.(string); ok {
			continue
		}
		parts, ok := content.([]any)
		if !ok {
			return false
		}
		for _, part := range parts {
			object, ok := part.(map[string]any)
			if !ok {
				return false
			}
			kind, _ := object["type"].(string)
			if kind != "text" && (!responses || (kind != "input_text" && kind != "output_text")) {
				return false
			}
			if _, ok := object["text"].(string); !ok {
				return false
			}
		}
	}
	return true
}

// BuildAccountContinuationBody always starts from the original client body.
// assistantText contains all confirmed text from prior internal rounds.
func BuildAccountContinuationBody(protocol AccountContinuationProtocol, body []byte, assistantText string, remaining int) ([]byte, error) {
	if !AccountContinuationRequestAllowed(protocol, body) {
		return nil, errors.New("request is not safe for text continuation")
	}
	var request map[string]any
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, err
	}
	var path string
	switch protocol {
	case AccountContinuationChat, AccountContinuationMessages:
		messages, ok := request["messages"].([]any)
		if !ok {
			return nil, errors.New("continuation requires a messages array")
		}
		request["messages"] = append(messages, map[string]any{"role": "assistant", "content": assistantText}, map[string]any{"role": "user", "content": accountContinuationPrompt})
		path = "max_tokens"
		if _, ok := request["max_completion_tokens"]; ok && protocol == AccountContinuationChat {
			path = "max_completion_tokens"
		}
	case AccountContinuationGemini:
		contents, ok := request["contents"].([]any)
		if !ok {
			return nil, errors.New("continuation requires a contents array")
		}
		request["contents"] = append(contents, map[string]any{"role": "model", "parts": []any{map[string]any{"text": assistantText}}}, map[string]any{"role": "user", "parts": []any{map[string]any{"text": accountContinuationPrompt}}})
		if remaining > 0 {
			config, _ := request["generationConfig"].(map[string]any)
			if config == nil {
				config = make(map[string]any)
			}
			accountContinuationClamp(config, "maxOutputTokens", remaining)
			request["generationConfig"] = config
		}
	case AccountContinuationResponses:
		var input []any
		if prompt, ok := request["input"].(string); ok {
			input = []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": prompt}}}}
		} else {
			var valid bool
			input, valid = request["input"].([]any)
			if !valid {
				return nil, errors.New("continuation requires text input")
			}
		}
		request["input"] = append(input, map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": assistantText}}}, map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": accountContinuationPrompt}}})
		path = "max_output_tokens"
	}
	if path != "" && remaining > 0 {
		accountContinuationClamp(request, path, remaining)
	}
	return json.Marshal(request)
}

func accountContinuationClamp(object map[string]any, key string, remaining int) {
	if current, ok := object[key].(float64); ok && current > 0 && current <= float64(remaining) {
		return
	}
	object[key] = remaining
}

type AccountContinuationBudget struct {
	policy      domain.AntiTruncationPolicy
	outputLimit int
	started     time.Time
}

func NewAccountContinuationBudget(policy domain.AntiTruncationPolicy, protocol AccountContinuationProtocol, body []byte) *AccountContinuationBudget {
	budget := &AccountContinuationBudget{policy: policy, outputLimit: policy.MaxOutputTokens}
	var request map[string]any
	if json.Unmarshal(body, &request) != nil {
		return budget
	}
	keys := []string{"max_tokens", "max_completion_tokens"}
	if protocol == AccountContinuationResponses {
		keys = []string{"max_output_tokens"}
	}
	if protocol == AccountContinuationGemini {
		request, _ = request["generationConfig"].(map[string]any)
		keys = []string{"maxOutputTokens"}
	}
	for _, key := range keys {
		if limit, ok := request[key].(float64); ok && limit > 0 && limit <= float64(int(^uint(0)>>1)) {
			if budget.outputLimit <= 0 || int(limit) < budget.outputLimit {
				budget.outputLimit = int(limit)
			}
		}
	}
	return budget
}

func (b *AccountContinuationBudget) OutputLimit() int { return b.outputLimit }

func (b *AccountContinuationBudget) RemainingOutputTokens(total int) int {
	if b.outputLimit <= 0 {
		return 0
	}
	if total >= b.outputLimit {
		return 0
	}
	return b.outputLimit - total
}

// CanContinue only decides whether to start another round. It deliberately
// provides no deadline context that could cancel an ordinary long response.
func (b *AccountContinuationBudget) CanContinue(now time.Time, completedRounds, totalOutput int) bool {
	if !b.policy.Enabled {
		return false
	}
	maxRounds := b.policy.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 1
	}
	if completedRounds >= maxRounds || (b.outputLimit > 0 && totalOutput >= b.outputLimit) {
		return false
	}
	duration := time.Duration(b.policy.MaxDurationSeconds) * time.Second
	if duration <= 0 {
		duration = 180 * time.Second
	}
	if b.started.IsZero() {
		b.started = now
	}
	return now.Sub(b.started) < duration
}

func accountContinuationReasonIsLimit(reason string) bool {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "length", "max_tokens", "max_output_tokens":
		return true
	}
	return false
}
