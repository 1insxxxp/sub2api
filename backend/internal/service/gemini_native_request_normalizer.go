package service

import (
	"encoding/json"
	"strings"
)

// normalizeGeminiNativeRequest applies compatibility fixes to requests that are
// already in Gemini native format before they are forwarded to an API-key
// upstream. These upstreams often implement the Gemini protocol with stricter
// validation than the official API.
func normalizeGeminiNativeRequest(body []byte) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	normalizeGeminiNativeThinkingLevel(payload)
	normalizeGeminiNativeMixedTools(payload)

	if contents, ok := payload["contents"].([]any); ok {
		contents = mergeConsecutiveGeminiContents(contents)
		contents = normalizeGeminiFunctionCallHistory(contents)
		contents = alignGeminiFunctionResponsesByOrder(contents)
		contents = appendGeminiUserTurnAfterModelTurn(contents)
		payload["contents"] = contents
	}

	return json.Marshal(payload)
}

func normalizeGeminiNativeThinkingLevel(payload map[string]any) {
	generationConfig, ok := payload["generationConfig"].(map[string]any)
	if !ok {
		return
	}
	thinkingConfig, ok := generationConfig["thinkingConfig"].(map[string]any)
	if !ok {
		return
	}
	level, ok := thinkingConfig["thinkingLevel"].(string)
	if !ok {
		return
	}
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "minimal":
		thinkingConfig["thinkingLevel"] = "low"
	}
}

func normalizeGeminiNativeMixedTools(payload map[string]any) {
	tools, ok := payload["tools"].([]any)
	if !ok {
		return
	}

	hasFunctions := false
	hasBuiltIn := false
	for _, rawTool := range tools {
		tool, ok := rawTool.(map[string]any)
		if !ok {
			continue
		}
		if declarations, ok := tool["functionDeclarations"].([]any); ok && len(declarations) > 0 {
			hasFunctions = true
		}
		if _, ok := tool["googleSearch"]; ok {
			hasBuiltIn = true
		}
		if _, ok := tool["google_search"]; ok {
			hasBuiltIn = true
		}
		if _, ok := tool["codeExecution"]; ok {
			hasBuiltIn = true
		}
	}
	if !hasFunctions || !hasBuiltIn {
		return
	}

	toolConfig, ok := payload["toolConfig"].(map[string]any)
	if !ok {
		toolConfig = make(map[string]any)
		payload["toolConfig"] = toolConfig
	}
	if _, exists := toolConfig["includeServerSideToolInvocations"]; !exists {
		toolConfig["includeServerSideToolInvocations"] = true
	}
}

func alignGeminiFunctionResponsesByOrder(contents []any) []any {
	for i := 0; i+1 < len(contents); i++ {
		modelContent, ok := contents[i].(map[string]any)
		if !ok || modelContent["role"] != "model" {
			continue
		}
		userContent, ok := contents[i+1].(map[string]any)
		if !ok || userContent["role"] != "user" {
			continue
		}

		calls := geminiNativeFunctionCalls(modelContent["parts"])
		responses := geminiNativeFunctionResponses(userContent["parts"])
		if len(calls) == 0 || len(responses) == 0 || len(calls) != len(responses) {
			continue
		}

		// Some clients use provider-specific aliases for response names. Pairing
		// by order keeps the otherwise invalid request forwardable.
		needsAlignment := false
		for j, response := range responses {
			if response["name"] != calls[j]["name"] {
				needsAlignment = true
				break
			}
		}
		if !needsAlignment {
			continue
		}

		for j, response := range responses {
			response["name"] = calls[j]["name"]
		}
	}
	return contents
}

func geminiNativeFunctionCalls(rawParts any) []map[string]any {
	parts, ok := rawParts.([]any)
	if !ok {
		return nil
	}
	calls := make([]map[string]any, 0, len(parts))
	for _, rawPart := range parts {
		part, ok := rawPart.(map[string]any)
		if !ok {
			continue
		}
		call, ok := part["functionCall"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := call["name"].(string)
		if name == "" {
			continue
		}
		calls = append(calls, call)
	}
	return calls
}

func geminiNativeFunctionResponses(rawParts any) []map[string]any {
	parts, ok := rawParts.([]any)
	if !ok {
		return nil
	}
	responses := make([]map[string]any, 0, len(parts))
	for _, rawPart := range parts {
		part, ok := rawPart.(map[string]any)
		if !ok {
			continue
		}
		response, ok := part["functionResponse"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := response["name"].(string)
		if name == "" {
			continue
		}
		responses = append(responses, response)
	}
	return responses
}

func appendGeminiUserTurnAfterModelTurn(contents []any) []any {
	if len(contents) == 0 {
		return contents
	}
	last, ok := contents[len(contents)-1].(map[string]any)
	if !ok || last["role"] != "model" {
		return contents
	}
	return append(contents, map[string]any{
		"role": "user",
		"parts": []any{map[string]any{
			"text": "Continue the assistant response from where it stopped without repeating prior text.",
		}},
	})
}
