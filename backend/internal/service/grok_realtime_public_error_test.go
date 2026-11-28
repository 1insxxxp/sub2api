package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrokRealtimePublicErrorPreservesTerminalEnvelope(t *testing.T) {
	payload := []byte(`{"type":"response.done","event_id":"evt_done","response_id":"resp_1","private":"private.invalid","response":{"id":"resp_1","object":"realtime.response","status":"failed","conversation_id":"conv_1","output":[{"text":"private.invalid"}],"metadata":{"secret":"private.invalid"},"status_details":{"type":"failed","reason":"private.invalid","error":{"code":"rate_limit_exceeded","status_code":429,"message":"private.invalid"}}}}`)
	safe, handled := publicGrokRealtimeErrorEvent(context.Background(), payload)
	require.True(t, handled)
	var got map[string]any
	require.NoError(t, json.Unmarshal(safe, &got))
	require.Equal(t, map[string]any{
		"type": "response.done", "event_id": "evt_done", "response_id": "resp_1",
		"response": map[string]any{
			"id": "resp_1", "object": "realtime.response", "status": "failed", "conversation_id": "conv_1",
			"output": []any{},
			"status_details": map[string]any{"type": "failed", "error": map[string]any{
				"type": "rate_limit_error", "code": "rate_limit_exceeded", "message": "上游限流",
			}},
		},
	}, got)
	require.NotContains(t, string(safe), "private.invalid")
}

func TestGrokRealtimePublicErrorPreservesErrorCorrelation(t *testing.T) {
	payload := []byte(`{"type":"error","event_id":"evt_error","item_id":"item_1","output_index":0,"content_index":2,"sequence_number":3,"error":{"event_id":"evt_request","code":"unknown","message":"private.invalid","param":"private.invalid","debug":{"secret":true}}}`)
	safe, handled := publicGrokRealtimeErrorEvent(context.Background(), payload)
	require.True(t, handled)
	var got map[string]any
	require.NoError(t, json.Unmarshal(safe, &got))
	require.Equal(t, map[string]any{
		"type": "error", "event_id": "evt_error", "item_id": "item_1",
		"output_index": float64(0), "content_index": float64(2), "sequence_number": float64(3),
		"error": map[string]any{
			"event_id": "evt_request", "type": "api_error", "code": "request_failed",
			"message": DefaultPublicGatewayErrorSettings().Fallback.Message,
		},
	}, got)
	require.NotContains(t, string(safe), "private.invalid")
}

func TestGrokRealtimePublicErrorPreservesNumericUsage(t *testing.T) {
	for _, fields := range []struct{ input, output string }{
		{"input_token_details", "output_token_details"},
		{"input_tokens_details", "output_tokens_details"},
	} {
		t.Run(fields.input, func(t *testing.T) {
			usage := map[string]any{
				"input_tokens": 100, "output_tokens": 20, "total_tokens": 120,
				"debug": "private.invalid", "unknown_tokens": 999,
				fields.input: map[string]any{
					"cached_tokens": 40, "text_tokens": 60, "audio_tokens": 30, "image_tokens": 10,
					"debug": "private.invalid", "unknown_tokens": 999,
					"cached_tokens_details": map[string]any{
						"text_tokens": 20, "audio_tokens": 15, "image_tokens": 5, "debug": "private.invalid",
					},
				},
				fields.output: map[string]any{
					"text_tokens": 0, "audio_tokens": 12, "image_tokens": 3, "reasoning_tokens": 5,
					"accepted_prediction_tokens": 2, "rejected_prediction_tokens": 1, "debug": "private.invalid",
				},
			}
			payload, err := json.Marshal(map[string]any{
				"type":     "response.done",
				"response": map[string]any{"id": "resp_usage", "status": "failed", "usage": usage},
			})
			require.NoError(t, err)
			safe, handled := publicGrokRealtimeErrorEvent(context.Background(), payload)
			require.True(t, handled)
			var got struct {
				Response struct {
					Usage map[string]any `json:"usage"`
				} `json:"response"`
			}
			require.NoError(t, json.Unmarshal(safe, &got))
			require.Equal(t, map[string]any{
				"input_tokens": float64(100), "output_tokens": float64(20), "total_tokens": float64(120),
				fields.input: map[string]any{
					"cached_tokens": float64(40), "text_tokens": float64(60), "audio_tokens": float64(30), "image_tokens": float64(10),
					"cached_tokens_details": map[string]any{"text_tokens": float64(20), "audio_tokens": float64(15), "image_tokens": float64(5)},
				},
				fields.output: map[string]any{
					"text_tokens": float64(0), "audio_tokens": float64(12), "image_tokens": float64(3), "reasoning_tokens": float64(5),
					"accepted_prediction_tokens": float64(2), "rejected_prediction_tokens": float64(1),
				},
			}, got.Response.Usage)
			require.NotContains(t, string(safe), "private.invalid")
			require.NotContains(t, string(safe), "unknown_tokens")
		})
	}
}

func TestGrokRealtimePublicErrorRejectsNonnumericUsage(t *testing.T) {
	payload := []byte(`{"type":"response.done","response":{"status":"failed","usage":{"input_tokens":"private.invalid","output_tokens":{"secret":"private.invalid"},"total_tokens":null,"input_token_details":{"cached_tokens":"private.invalid","audio_tokens":false,"cached_tokens_details":{"text_tokens":"private.invalid"}},"output_token_details":{"text_tokens":["private.invalid"]}}}}`)
	safe, handled := publicGrokRealtimeErrorEvent(context.Background(), payload)
	require.True(t, handled)
	require.NotContains(t, string(safe), "private.invalid")
	require.NotContains(t, string(safe), `"usage"`)
}

func TestGrokRealtimePublicErrorLeavesSuccessfulEvents(t *testing.T) {
	for _, payload := range []string{
		`{"type":"response.done","response":{"id":"resp_1","status":"completed","output":[]}}`,
		`{"type":"response.audio.delta","delta":"audio"}`,
		`{"type":"session.created","session":{"id":"session_1"}}`,
	} {
		safe, handled := publicGrokRealtimeErrorEvent(context.Background(), []byte(payload))
		require.False(t, handled)
		require.Equal(t, payload, string(safe))
	}
}

func TestGrokRealtimePublicErrorUsesContextPolicyAndRejectsStructuredCorrelation(t *testing.T) {
	settings := DefaultPublicGatewayErrorSettings()
	settings.Fallback = PublicGatewayError{Status: 503, Code: "retry_later", Message: "Please retry."}
	policy, err := CompilePublicGatewayErrorSettings(settings)
	require.NoError(t, err)
	snapshot := &publicGatewaySettingsSnapshot{policy: policy}
	snapshot.once.Do(func() {})
	ctx := context.WithValue(context.Background(), publicGatewaySettingsContextKey{}, snapshot)
	payload := []byte(`{"type":"response.done","event_id":{"secret":"private.invalid"},"sequence_number":{"secret":"private.invalid"},"response":{"id":"resp_2","object":"realtime.response","status":"failed","error":{"event_id":{"secret":"private.invalid"},"message":"private.invalid"}}}`)
	safe, handled := publicGrokRealtimeErrorEvent(ctx, payload)
	require.True(t, handled)
	var got map[string]any
	require.NoError(t, json.Unmarshal(safe, &got))
	require.Equal(t, map[string]any{
		"type": "response.done",
		"response": map[string]any{
			"id": "resp_2", "object": "realtime.response", "status": "failed", "output": []any{},
			"error": map[string]any{"type": "overloaded_error", "code": "retry_later", "message": "Please retry."},
		},
	}, got)
	require.NotContains(t, string(safe), "private.invalid")
}
