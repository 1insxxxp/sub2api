//go:build unit

package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesToAnthropicRequest_PreservesCacheControl(t *testing.T) {
	var req ResponsesRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"model":"claude-opus-4-6","max_output_tokens":100,
		"instructions":"base instructions",
		"cache_control":{"type":"ephemeral","ttl":"5m"},
		"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"},"cache_control":{"type":"ephemeral","ttl":"1h"}}],
		"input":[
			{"role":"developer","content":[{"type":"input_text","text":"stable","cache_control":{"type":"ephemeral","ttl":"1h"}},{"type":"input_text","text":"changing"}]},
			{"role":"user","content":[{"type":"input_text","text":"first","cache_control":{"type":"ephemeral","ttl":"5m"}},{"type":"input_text","text":"second"}]},
			{"role":"assistant","content":[{"type":"output_text","text":"answer","cache_control":{"type":"ephemeral"}},{"type":"output_text","text":"suffix"}]}
		]
	}`), &req))
	out, err := ResponsesToAnthropicRequest(&req)
	require.NoError(t, err)
	wire, err := json.Marshal(out)
	require.NoError(t, err)
	for path, value := range map[string]string{
		"cache_control":                      `{"type":"ephemeral","ttl":"5m"}`,
		"system.1.cache_control":             `{"type":"ephemeral","ttl":"1h"}`,
		"tools.0.cache_control":              `{"type":"ephemeral","ttl":"1h"}`,
		"messages.0.content.0.cache_control": `{"type":"ephemeral","ttl":"5m"}`,
		"messages.1.content.0.cache_control": `{"type":"ephemeral"}`,
	} {
		require.JSONEq(t, value, gjson.GetBytes(wire, path).Raw, path)
	}
	for _, path := range []string{"system.0.cache_control", "system.2.cache_control", "messages.0.content.1.cache_control", "messages.1.content.1.cache_control"} {
		require.False(t, gjson.GetBytes(wire, path).Exists(), path)
	}
}

func TestChatCompletionsToResponses_DoesNotForwardAnthropicCacheControl(t *testing.T) {
	var req ChatCompletionsRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"model":"gpt-5","cache_control":{"type":"ephemeral"},
		"messages":[{"role":"system","content":[{"type":"text","text":"prefix","cache_control":{"type":"ephemeral"}}]},{"role":"user","content":"hello"}],
		"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}},"cache_control":{"type":"ephemeral"}}]
	}`), &req))
	out, err := ChatCompletionsToResponses(&req)
	require.NoError(t, err)
	wire, err := json.Marshal(out)
	require.NoError(t, err)
	require.NotContains(t, string(wire), `"cache_control"`)
}

func TestChatAnthropicCacheControl_DoesNotMoveSkippedBreakpoints(t *testing.T) {
	var req ChatCompletionsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"model":"claude-opus-4-6","messages":[
		{"role":"system","content":[{"type":"unknown","cache_control":{"type":"ephemeral"}},{"type":"text","text":"system"}]},
		{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,"},"cache_control":{"type":"ephemeral"}},{"type":"file","file":{"file_id":"not-downloadable"},"cache_control":{"type":"ephemeral"}},{"type":"text","text":"question"}]},
		{"role":"assistant","content":[{"type":"text","text":"","cache_control":{"type":"ephemeral"}},{"type":"text","text":"answer"}]}
	]}`), &req))
	intermediate, err := ChatCompletionsToResponsesForAnthropic(&req)
	require.NoError(t, err)
	out, err := ResponsesToAnthropicRequest(intermediate)
	require.NoError(t, err)
	wire, err := json.Marshal(out)
	require.NoError(t, err)
	require.NotContains(t, string(wire), `"cache_control"`)
}

func TestChatAnthropicCacheControl_PreservesAssistantReasoningAndToolPairing(t *testing.T) {
	var req ChatCompletionsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"model":"claude-opus-4-6","messages":[
		{"role":"user","content":"question"},
		{"role":"assistant","reasoning_content":"reasoning","content":[{"type":"text","text":"prefix","cache_control":{"type":"ephemeral"}},{"type":"text","text":"suffix"}],"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"call_1","content":"result"},
		{"role":"user","content":"continue"}
	]}`), &req))
	intermediate, err := ChatCompletionsToResponsesForAnthropic(&req)
	require.NoError(t, err)
	out, err := ResponsesToAnthropicRequest(intermediate)
	require.NoError(t, err)
	wire, err := json.Marshal(out)
	require.NoError(t, err)
	require.Equal(t, "<thinking>reasoning</thinking>\nprefix", gjson.GetBytes(wire, "messages.1.content.0.text").String())
	require.Equal(t, "suffix", gjson.GetBytes(wire, "messages.1.content.1.text").String())
	require.Equal(t, "ephemeral", gjson.GetBytes(wire, "messages.1.content.0.cache_control.type").String())
	require.False(t, gjson.GetBytes(wire, "messages.1.content.1.cache_control").Exists())
	require.Equal(t, "call_1", gjson.GetBytes(wire, "messages.1.content.2.id").String())
	require.Equal(t, "call_1", gjson.GetBytes(wire, "messages.2.content.0.tool_use_id").String())
}

func TestResponsesAnthropicCacheControl_RejectsMalformedMarkersWithoutDroppingContent(t *testing.T) {
	for _, role := range []string{"system", "developer", "user", "assistant"} {
		t.Run(role, func(t *testing.T) {
			partType := "input_text"
			if role == "assistant" {
				partType = "output_text"
			}
			input, err := json.Marshal([]map[string]any{{"role": role, "content": []map[string]any{{"type": partType, "text": "Do not lose this content", "cache_control": false}}}})
			require.NoError(t, err)
			_, err = ResponsesToAnthropicRequest(&ResponsesRequest{Model: "claude-opus-4-6", Input: input})
			require.Error(t, err, "malformed metadata must not silently drop instructions or forward untranslated content")
		})
	}
}

func TestChatAnthropicCacheControl_PreservesToolResultBlocks(t *testing.T) {
	var req ChatCompletionsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"model":"claude-opus-4-6","messages":[
		{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"call_1","content":[{"type":"text","text":"stable result","cache_control":{"type":"ephemeral","ttl":"1h"}},{"type":"text","text":"changing suffix"}]}
	]}`), &req))
	intermediate, err := ChatCompletionsToResponsesForAnthropic(&req)
	require.NoError(t, err)
	out, err := ResponsesToAnthropicRequest(intermediate)
	require.NoError(t, err)
	wire, err := json.Marshal(out)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"ephemeral","ttl":"1h"}`, gjson.GetBytes(wire, "messages.1.content.0.content.0.cache_control").Raw)
	require.Equal(t, "changing suffix", gjson.GetBytes(wire, "messages.1.content.0.content.1.text").String())
	require.False(t, gjson.GetBytes(wire, "messages.1.content.0.content.1.cache_control").Exists())
}

func TestChatAnthropicCacheControl_DoesNotEmitBlankAssistantBlocks(t *testing.T) {
	var req ChatCompletionsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"model":"claude-opus-4-6","messages":[{"role":"assistant","content":[
		{"type":"text","text":"prefix","cache_control":{"type":"ephemeral"}},
		{"type":"text","text":"\n"},{"type":"text","text":"suffix"}
	]}]}`), &req))
	intermediate, err := ChatCompletionsToResponsesForAnthropic(&req)
	require.NoError(t, err)
	out, err := ResponsesToAnthropicRequest(intermediate)
	require.NoError(t, err)
	wire, err := json.Marshal(out)
	require.NoError(t, err)
	require.Equal(t, int64(2), gjson.GetBytes(wire, "messages.0.content.#").Int())
	require.Equal(t, "prefix", gjson.GetBytes(wire, "messages.0.content.0.text").String())
	require.Equal(t, "\nsuffix", gjson.GetBytes(wire, "messages.0.content.1.text").String())
	require.Equal(t, "ephemeral", gjson.GetBytes(wire, "messages.0.content.0.cache_control.type").String())
	require.False(t, gjson.GetBytes(wire, "messages.0.content.1.cache_control").Exists())
}

func TestChatAnthropicCacheControl_DoesNotEmitBlankUserBlocks(t *testing.T) {
	var req ChatCompletionsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"model":"claude-opus-4-6","messages":[{"role":"user","content":[
		{"type":"text","text":"prefix","cache_control":{"type":"ephemeral"}},
		{"type":"text","text":"\n"},{"type":"text","text":"suffix"}
	]}]}`), &req))
	intermediate, err := ChatCompletionsToResponsesForAnthropic(&req)
	require.NoError(t, err)
	out, err := ResponsesToAnthropicRequest(intermediate)
	require.NoError(t, err)
	wire, err := json.Marshal(out)
	require.NoError(t, err)
	require.Equal(t, int64(2), gjson.GetBytes(wire, "messages.0.content.#").Int())
	require.Equal(t, "prefix", gjson.GetBytes(wire, "messages.0.content.0.text").String())
	require.Equal(t, "\nsuffix", gjson.GetBytes(wire, "messages.0.content.1.text").String())
	require.Equal(t, "ephemeral", gjson.GetBytes(wire, "messages.0.content.0.cache_control.type").String())
	require.False(t, gjson.GetBytes(wire, "messages.0.content.1.cache_control").Exists())
}
