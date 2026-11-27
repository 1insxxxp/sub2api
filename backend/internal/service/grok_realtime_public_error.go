package service

import (
	"context"
	"encoding/json"

	"github.com/tidwall/gjson"
)

// Realtime uses response.done even for failed turns. Rebuild error events with
// only protocol identity, correlation and numeric usage fields plus public errors.
func publicGrokRealtimeErrorEvent(ctx context.Context, payload []byte) ([]byte, bool) {
	root := gjson.ParseBytes(payload)
	kind := root.Get("type").String()
	errorValue := root.Get("error")
	if kind != "error" && kind != "response.failed" && root.Get("response.status").String() != "failed" &&
		(!errorValue.Exists() || errorValue.Type == gjson.Null) {
		return payload, false
	}
	source := root.Get("response.status_details.error")
	if !source.IsObject() {
		source = root.Get("response.error")
	}
	if !source.IsObject() {
		source = errorValue
	}
	if !source.IsObject() {
		source = root
	}
	status := int(source.Get("status_code").Int())
	if status == 0 {
		status = int(source.Get("code").Int())
	}
	if status == 0 {
		status = int(root.Get("status_code").Int())
	}
	diagnostic := source.Get("code").String() + " " + source.Get("type").String() + " " + ExtractUpstreamErrorMessage([]byte(source.Raw))
	public := ClassifyPublicGatewayErrorContext(ctx, status, diagnostic)
	publicError := map[string]any{"type": public.Type(), "code": public.Code, "message": public.Message}
	copyGrokRealtimeCorrelation(publicError, source, []string{"event_id"}, nil)
	if kind == "" {
		kind = "error"
	}
	result := map[string]any{"type": kind}
	copyGrokRealtimeCorrelation(result, root,
		[]string{"event_id", "response_id", "item_id", "conversation_id"},
		[]string{"output_index", "content_index", "sequence_number"})
	response := root.Get("response")
	if response.IsObject() {
		safeResponse := map[string]any{"output": []any{}}
		copyGrokRealtimeCorrelation(safeResponse, response, []string{"id", "object", "status", "conversation_id"}, nil)
		if usage := publicGrokRealtimeUsage(response.Get("usage")); len(usage) > 0 {
			safeResponse["usage"] = usage
		}
		if response.Get("status_details").IsObject() {
			safeResponse["status_details"] = map[string]any{"type": "failed", "error": publicError}
		}
		if response.Get("error").Exists() || !response.Get("status_details").IsObject() {
			safeResponse["error"] = publicError
		}
		result["response"] = safeResponse
	} else {
		result["error"] = publicError
	}
	safe, _ := json.Marshal(result)
	return safe, true
}

func publicGrokRealtimeUsage(source gjson.Result) map[string]any {
	usage := make(map[string]any)
	copyGrokRealtimeCorrelation(usage, source, nil, []string{"input_tokens", "output_tokens", "total_tokens"})
	for _, field := range []struct {
		name   string
		tokens []string
	}{
		{"input_token_details", []string{"cached_tokens", "text_tokens", "audio_tokens", "image_tokens"}},
		{"input_tokens_details", []string{"cached_tokens", "text_tokens", "audio_tokens", "image_tokens"}},
		{"output_token_details", []string{"text_tokens", "audio_tokens", "image_tokens", "reasoning_tokens", "accepted_prediction_tokens", "rejected_prediction_tokens"}},
		{"output_tokens_details", []string{"text_tokens", "audio_tokens", "image_tokens", "reasoning_tokens", "accepted_prediction_tokens", "rejected_prediction_tokens"}},
	} {
		detailsSource := source.Get(field.name)
		details := make(map[string]any)
		copyGrokRealtimeCorrelation(details, detailsSource, nil, field.tokens)
		cached := make(map[string]any)
		copyGrokRealtimeCorrelation(cached, detailsSource.Get("cached_tokens_details"), nil, []string{"text_tokens", "audio_tokens", "image_tokens"})
		if len(cached) > 0 {
			details["cached_tokens_details"] = cached
		}
		if len(details) > 0 {
			usage[field.name] = details
		}
	}
	return usage
}

func copyGrokRealtimeCorrelation(dst map[string]any, source gjson.Result, stringFields, numberFields []string) {
	for _, key := range stringFields {
		if value := source.Get(key); value.Type == gjson.String {
			dst[key] = value.String()
		}
	}
	for _, key := range numberFields {
		if value := source.Get(key); value.Type == gjson.Number {
			dst[key] = value.Int()
		}
	}
}
