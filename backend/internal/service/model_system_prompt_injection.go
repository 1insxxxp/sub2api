package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type ModelSystemPromptProtocol string

const (
	ModelSystemPromptOpenAIResponses ModelSystemPromptProtocol = "openai_responses"
	ModelSystemPromptOpenAIChat      ModelSystemPromptProtocol = "openai_chat"
	ModelSystemPromptClaude          ModelSystemPromptProtocol = "claude"
	ModelSystemPromptGemini          ModelSystemPromptProtocol = "gemini"
)

func (a *Account) ResolveModelSystemPrompt(mappedModel string) (string, bool) {
	if a == nil || a.ModelSystemPrompts == nil {
		return "", false
	}
	prompt, ok := a.ModelSystemPrompts[mappedModel]
	prompt = strings.TrimSpace(prompt)
	return prompt, ok && prompt != ""
}

func ApplyAccountModelSystemPrompt(body []byte, account *Account, mappedModel string, protocol ModelSystemPromptProtocol) ([]byte, bool, error) {
	prompt, ok := account.ResolveModelSystemPrompt(mappedModel)
	if !ok {
		return body, false, nil
	}
	updated, err := PrependModelSystemPrompt(body, protocol, prompt)
	if err != nil {
		return nil, false, err
	}
	return updated, true, nil
}

func PrependModelSystemPrompt(body []byte, protocol ModelSystemPromptProtocol, prompt string) ([]byte, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return bytes.Clone(body), nil
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("parse request for model system prompt injection: %w", err)
	}
	if root == nil {
		return nil, fmt.Errorf("request body must be a JSON object")
	}

	var err error
	switch protocol {
	case ModelSystemPromptOpenAIResponses:
		err = prependJSONStringField(root, "instructions", prompt)
	case ModelSystemPromptOpenAIChat:
		err = prependOpenAIChatSystemMessage(root, prompt)
	case ModelSystemPromptClaude:
		err = prependClaudeSystem(root, prompt)
	case ModelSystemPromptGemini:
		err = prependGeminiSystemInstruction(root, prompt)
	default:
		return nil, fmt.Errorf("unsupported model system prompt protocol %q", protocol)
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(root)
}

func prependJSONStringField(root map[string]json.RawMessage, field, prompt string) error {
	raw, exists := root[field]
	if !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		root[field], _ = json.Marshal(prompt)
		return nil
	}
	var existing string
	if err := json.Unmarshal(raw, &existing); err != nil {
		return fmt.Errorf("%s must be a string: %w", field, err)
	}
	value := prompt
	if existing != "" {
		value += "\n\n" + existing
	}
	root[field], _ = json.Marshal(value)
	return nil
}

func prependOpenAIChatSystemMessage(root map[string]json.RawMessage, prompt string) error {
	var messages []json.RawMessage
	if err := json.Unmarshal(root["messages"], &messages); err != nil {
		return fmt.Errorf("messages must be an array: %w", err)
	}
	message, _ := json.Marshal(map[string]any{"role": "system", "content": prompt})
	messages = append([]json.RawMessage{message}, messages...)
	root["messages"], _ = json.Marshal(messages)
	return nil
}

func prependClaudeSystem(root map[string]json.RawMessage, prompt string) error {
	raw, exists := root["system"]
	if !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		root["system"], _ = json.Marshal(prompt)
		return nil
	}

	var existing string
	if err := json.Unmarshal(raw, &existing); err == nil {
		value := prompt
		if existing != "" {
			value += "\n\n" + existing
		}
		root["system"], _ = json.Marshal(value)
		return nil
	}

	var blocks []json.RawMessage
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return fmt.Errorf("system must be a string or array: %w", err)
	}
	block, _ := json.Marshal(map[string]any{"type": "text", "text": prompt})
	blocks = append([]json.RawMessage{block}, blocks...)
	root["system"], _ = json.Marshal(blocks)
	return nil
}

func prependGeminiSystemInstruction(root map[string]json.RawMessage, prompt string) error {
	system := make(map[string]json.RawMessage)
	if raw, exists := root["systemInstruction"]; exists && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if err := json.Unmarshal(raw, &system); err != nil {
			return fmt.Errorf("systemInstruction must be an object: %w", err)
		}
	}

	var parts []json.RawMessage
	if raw, exists := system["parts"]; exists && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if err := json.Unmarshal(raw, &parts); err != nil {
			return fmt.Errorf("systemInstruction.parts must be an array: %w", err)
		}
	}
	part, _ := json.Marshal(map[string]any{"text": prompt})
	parts = append([]json.RawMessage{part}, parts...)
	system["parts"], _ = json.Marshal(parts)
	root["systemInstruction"], _ = json.Marshal(system)
	return nil
}
