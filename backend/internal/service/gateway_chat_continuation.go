package service

import (
	"encoding/json"
	"strconv"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func buildAnthropicChatContinuationBody(body []byte, assistantText string, remainingOutputTokens int) ([]byte, error) {
	messages := gjson.GetBytes(body, "messages").Array()
	assistant, err := json.Marshal(map[string]any{"role": "assistant", "content": assistantText})
	if err != nil {
		return nil, err
	}
	updated, err := sjson.SetRawBytes(body, "messages."+strconv.Itoa(len(messages)), assistant)
	if err != nil {
		return nil, err
	}
	user, err := json.Marshal(map[string]any{
		"role":    "user",
		"content": "Continue the response from where you left off. Do not repeat any earlier text.",
	})
	if err != nil {
		return nil, err
	}
	updated, err = sjson.SetRawBytes(updated, "messages."+strconv.Itoa(len(messages)+1), user)
	if err != nil {
		return nil, err
	}
	if remainingOutputTokens > 0 {
		updated, err = sjson.SetBytes(updated, "max_tokens", continuationRoundOutputLimit(body, "max_tokens", remainingOutputTokens))
	}
	return updated, err
}

func anthropicChatContinuationAllowed(body []byte) bool {
	if !rawChatContinuationAllowed(body) || len(gjson.GetBytes(body, "tools").Array()) > 0 || len(gjson.GetBytes(body, "functions").Array()) > 0 {
		return false
	}
	for _, message := range gjson.GetBytes(body, "messages").Array() {
		content := message.Get("content")
		if content.Type == gjson.String || !content.Exists() || content.Type == gjson.Null {
			continue
		}
		if !content.IsArray() {
			return false
		}
		for _, part := range content.Array() {
			if part.Get("type").String() != "text" {
				return false
			}
		}
	}
	return true
}

func clampAnthropicChatOutputTokens(body []byte, maxOutputTokens int) ([]byte, error) {
	if maxOutputTokens <= 0 {
		return body, nil
	}
	current := gjson.GetBytes(body, "max_tokens")
	if current.Exists() && current.Int() <= int64(maxOutputTokens) {
		return body, nil
	}
	return sjson.SetBytes(body, "max_tokens", maxOutputTokens)
}

func addClaudeUsage(dst *ClaudeUsage, src ClaudeUsage) {
	if dst == nil {
		return
	}
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.CacheCreationInputTokens += src.CacheCreationInputTokens
	dst.CacheReadInputTokens += src.CacheReadInputTokens
	dst.CacheCreation5mTokens += src.CacheCreation5mTokens
	dst.CacheCreation1hTokens += src.CacheCreation1hTokens
	dst.ImageOutputTokens += src.ImageOutputTokens
}
