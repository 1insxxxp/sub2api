package service

import (
	"encoding/json"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const anthropicChatMinimumThinkingBudget = 1024

// Resolve native controls before converting legacy aliases into thinking.
// Only 5.5 converters need an effort to select their native thinking mode.
// The bool removes their default when native thinking left effort omitted.
func prepareChatAnthropicReasoning(clientBody []byte, req *apicompat.ResponsesRequest, group *Group) (bool, error) {
	effort, err := chatAnthropicNativeEffort(clientBody)
	if err != nil {
		return false, err
	}
	if effort != "" {
		req.Reasoning = nil
		if isClaude55SignedThinkingModel(req.Model) {
			req.Reasoning = &apicompat.ResponsesReasoning{Effort: effort}
		}
		return false, nil
	}
	if gjson.GetBytes(clientBody, "thinking.type").Exists() {
		if req.Reasoning != nil && req.Reasoning.Effort != "" && req.Reasoning.Effort != "none" {
			return false, nil
		}
		req.Reasoning = nil
		return isClaude55SignedThinkingModel(req.Model), nil
	}
	applyOpus55GroupReasoningDefault(req, group)
	return false, nil
}

// Restore Anthropic extensions at the final protocol boundary, where the
// Responses intermediate cannot discard explicit client controls.
func applyChatCompletionsAnthropicControls(clientBody, anthropicBody []byte) ([]byte, error) {
	body, err := restoreChatAnthropicThinking(clientBody, anthropicBody)
	if err != nil {
		return nil, err
	}
	body, err = restoreChatAnthropicEffort(clientBody, body)
	if err != nil {
		return nil, err
	}
	stops, err := parseChatAnthropicStops(gjson.GetBytes(clientBody, "stop"))
	if err != nil {
		return nil, err
	}
	if len(stops) == 0 {
		return body, nil
	}
	return sjson.SetBytes(body, "stop_sequences", stops)
}

func restoreChatAnthropicThinking(clientBody, anthropicBody []byte) ([]byte, error) {
	thinking := gjson.GetBytes(clientBody, "thinking")
	if !thinking.Exists() || thinking.Type == gjson.Null {
		return anthropicBody, nil
	}
	maxTokens := gjson.GetBytes(anthropicBody, "max_tokens").Int()
	typ, err := validateChatAnthropicThinking(thinking, maxTokens, gjson.GetBytes(anthropicBody, "model").String())
	if err != nil {
		return nil, err
	}
	if typ != "disabled" {
		// Keep native subfields such as display and block_binding verbatim.
		return sjson.SetRawBytes(anthropicBody, "thinking", []byte(thinking.Raw))
	}
	// Preserve the existing explicit opt-out precedence over reasoning aliases.
	body, err := sjson.SetRawBytes(anthropicBody, "thinking", []byte(`{"type":"disabled"}`))
	if err != nil {
		return nil, err
	}
	return sjson.DeleteBytes(body, "output_config")
}

func validateChatAnthropicThinking(raw gjson.Result, maxTokens int64, model string) (string, error) {
	if !raw.IsObject() {
		return "", fmt.Errorf("thinking must be an object")
	}
	typ := raw.Get("type")
	if typ.Type != gjson.String {
		return "", fmt.Errorf("thinking.type must be a string")
	}
	switch typ.Str {
	case "enabled":
		if err := validateChatAnthropicManualBudget(raw, maxTokens, model); err != nil {
			return "", err
		}
	case "adaptive", "disabled", "between_tools":
	default:
		return "", fmt.Errorf("thinking.type must be enabled, adaptive, disabled or between_tools")
	}
	return typ.Str, nil
}

func validateChatAnthropicManualBudget(raw gjson.Result, maxTokens int64, model string) error {
	if ResolveThinkingProtocol(model) != ThinkingProtocolAnthropicStrict {
		// Compatible providers may enable native thinking without a Claude budget.
		return nil
	}
	var thinking apicompat.AnthropicThinking
	if err := json.Unmarshal([]byte(raw.Raw), &thinking); err != nil {
		return fmt.Errorf("invalid thinking controls: %w", err)
	}
	if thinking.BudgetTokens < anthropicChatMinimumThinkingBudget {
		return fmt.Errorf("thinking.budget_tokens must be at least %d", anthropicChatMinimumThinkingBudget)
	}
	if int64(thinking.BudgetTokens) >= maxTokens {
		return fmt.Errorf("thinking.budget_tokens must be less than max_tokens")
	}
	return nil
}

func restoreChatAnthropicEffort(clientBody, anthropicBody []byte) ([]byte, error) {
	effort, err := chatAnthropicNativeEffort(clientBody)
	if err != nil {
		return nil, err
	}
	if effort == "" || gjson.GetBytes(anthropicBody, "thinking.type").String() == "disabled" {
		return anthropicBody, nil
	}
	return sjson.SetBytes(anthropicBody, "output_config.effort", effort)
}

func chatAnthropicNativeEffort(clientBody []byte) (string, error) {
	config := gjson.GetBytes(clientBody, "output_config")
	if !config.Exists() || config.Type == gjson.Null {
		return "", nil
	}
	if !config.IsObject() {
		return "", fmt.Errorf("output_config must be an object")
	}
	effort := config.Get("effort")
	if !effort.Exists() || effort.Type == gjson.Null {
		return "", nil
	}
	normalized := NormalizeClaudeOutputEffort(effort.String())
	if effort.Type != gjson.String || normalized == nil || *normalized != effort.Str {
		return "", fmt.Errorf("output_config.effort must be low, medium, high, xhigh or max")
	}
	return effort.Str, nil
}

func parseChatAnthropicStops(raw gjson.Result) ([]string, error) {
	if !raw.Exists() || raw.Type == gjson.Null {
		return nil, nil
	}
	var stops []string
	if raw.Type == gjson.String {
		stops = []string{raw.Str}
	} else if raw.IsArray() {
		if err := json.Unmarshal([]byte(raw.Raw), &stops); err != nil {
			return nil, fmt.Errorf("stop must be a string or an array of strings")
		}
	} else {
		return nil, fmt.Errorf("stop must be a string or an array of strings")
	}
	for _, value := range stops {
		if value == "" {
			return nil, fmt.Errorf("stop sequences must be non-empty strings")
		}
	}
	return stops, nil
}
