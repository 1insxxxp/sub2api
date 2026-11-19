package service

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
)

func acceptanceGPTCompletionLimit(model string) bool {
	id := strings.ToLower(lastOpenAIModelSegment(model))
	family, _, _ := strings.Cut(id, "-")
	return isOpenAICodexReasoningGPTModel(id) || family == "o1" || family == "o3" || family == "o4"
}

func acceptanceLowEffort(model string) bool {
	return isOpenAICodexReasoningGPTModel(lastOpenAIModelSegment(model)) || slices.Contains(claude.EffortLevelsForModel(model), "low")
}

// System One evaluates structured questions. Each case must yield a
// verifiable choice; only the synthetic reference text changes between cases.
func acceptanceSystemOneBody(s AcceptanceSample) ([]byte, error) {
	if s.Stream || s.Expected == "" || s.Category == "tools" || s.Category == "audio" || s.Category == "image" {
		return nil, ErrAcceptanceInvalid
	}
	return json.Marshal(map[string]any{
		"model": s.Snapshot.UpstreamModel, "state": s.Prompt,
		"questions": map[string]any{"code": map[string]any{
			"type": "choice", "instructions": "Select the verification code requested in the supplied state.",
			"criteria": map[string]string{s.Expected: "The requested code is this value.", "missing": "The state contains no verification code."},
		}},
	})
}

func acceptanceToolSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"value": map[string]string{"type": "string"}},
		"required":   []string{"value"},
	}
}

func acceptanceMessagesBody(s AcceptanceSample, maxTokens int) ([]byte, error) {
	content := []map[string]any{{"type": "text", "text": s.Prompt}}
	switch s.Category {
	case "cache":
		content[0]["cache_control"] = map[string]string{"type": "ephemeral"}
	case "image":
		content = append(content, map[string]any{"type": "image", "source": map[string]string{
			"type": "base64", "media_type": "image/png", "data": acceptanceImageFixture(),
		}})
	case "audio":
		return nil, ErrAcceptanceInvalid
	}
	body := map[string]any{"model": s.Snapshot.Model, "messages": []map[string]any{{"role": "user", "content": content}}}
	if s.Protocol != "count_tokens" {
		body["max_tokens"] = maxTokens
		body["stream"] = s.Stream
		if s.Category != "tools" && slices.Contains(claude.EffortLevelsForModel(s.Snapshot.UpstreamModel), "low") {
			body["output_config"] = map[string]string{"effort": "low"}
		}
	}
	if s.Category == "tools" {
		body["tools"] = []map[string]any{{"name": "acceptance_echo", "description": "Echo the supplied test value", "input_schema": acceptanceToolSchema()}}
		body["tool_choice"] = map[string]string{"type": "tool", "name": "acceptance_echo"}
	}
	return json.Marshal(body)
}

func acceptanceResponsesBody(s AcceptanceSample, maxTokens int) ([]byte, error) {
	content := []map[string]any{{"type": "input_text", "text": s.Prompt}}
	switch s.Category {
	case "image":
		content = append(content, map[string]any{"type": "input_image", "image_url": "data:image/png;base64," + acceptanceImageFixture()})
	case "audio":
		return nil, ErrAcceptanceInvalid
	}
	body := map[string]any{
		"model": s.Snapshot.Model, "input": []map[string]any{{"role": "user", "content": content}},
		"max_output_tokens": maxTokens, "stream": s.Stream, "store": false,
	}
	if s.Category != "tools" && acceptanceLowEffort(s.Snapshot.UpstreamModel) {
		body["reasoning"] = map[string]string{"effort": "low"}
	}
	if s.Category == "tools" {
		body["tools"] = []map[string]any{{"type": "function", "name": "acceptance_echo", "description": "Echo the supplied test value", "parameters": acceptanceToolSchema()}}
		body["tool_choice"] = map[string]string{"type": "function", "name": "acceptance_echo"}
	}
	return json.Marshal(body)
}
