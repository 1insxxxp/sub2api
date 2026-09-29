package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type novelAIImageRequestContextKey struct{}

// NovelAIImageRequest is the client-facing native NovelAI image request.
// RawBody and Parameters intentionally retain fields that this gateway does
// not interpret so newer NovelAI clients can be forwarded without loss.
type NovelAIImageRequest struct {
	RawBody    []byte
	Input      string
	Model      string
	Action     string
	Parameters json.RawMessage
	Samples    int
	Width      int
	Height     int
}

// ParseNovelAIImageRequest validates the native NovelAI image contract while
// preserving the original JSON body and complete parameters object.
func ParseNovelAIImageRequest(body []byte) (*NovelAIImageRequest, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, fmt.Errorf("invalid JSON: request body is empty")
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if payload == nil {
		return nil, fmt.Errorf("invalid JSON: request must be an object")
	}

	input, err := requiredStringField(payload, "input")
	if err != nil {
		return nil, err
	}
	model, err := requiredStringField(payload, "model")
	if err != nil {
		return nil, err
	}

	action := "generate"
	if raw, ok := payload["action"]; ok {
		if err := json.Unmarshal(raw, &action); err != nil {
			return nil, fmt.Errorf("action must be a string")
		}
		action = strings.TrimSpace(action)
		if action == "" {
			action = "generate"
		}
	}
	// NovelAI uses the same endpoint for several generation modes (for example
	// img2img and inpainting). Keep the action opaque so newer native modes and
	// their parameter sets pass through unchanged.

	parameters := json.RawMessage(`{}`)
	if raw, ok := payload["parameters"]; ok {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil || object == nil {
			return nil, fmt.Errorf("parameters must be an object")
		}
		parameters = append(parameters[:0], bytes.TrimSpace(raw)...)
	}

	request := &NovelAIImageRequest{
		RawBody:    append([]byte(nil), body...),
		Input:      input,
		Model:      model,
		Action:     action,
		Parameters: parameters,
		Samples:    1,
	}
	var parameterValues map[string]json.RawMessage
	if err := json.Unmarshal(parameters, &parameterValues); err != nil {
		return nil, fmt.Errorf("parameters must be an object")
	}
	if raw, ok := parameterValues["n_samples"]; ok {
		samples, err := positiveInt(raw, "n_samples")
		if err != nil {
			return nil, err
		}
		request.Samples = samples
	}
	if raw, ok := parameterValues["width"]; ok {
		request.Width, err = positiveInt(raw, "width")
		if err != nil {
			return nil, err
		}
	}
	if raw, ok := parameterValues["height"]; ok {
		request.Height, err = positiveInt(raw, "height")
		if err != nil {
			return nil, err
		}
	}
	return request, nil
}

func requiredStringField(payload map[string]json.RawMessage, name string) (string, error) {
	raw, ok := payload[name]
	if !ok {
		return "", fmt.Errorf("%s is required", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required and must be a non-empty string", name)
	}
	return strings.TrimSpace(value), nil
}

func positiveInt(raw json.RawMessage, name string) (int, error) {
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err != nil {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	value, err := strconv.Atoi(number.String())
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}

// Size returns a normalized OpenAI-style size only when both dimensions are
// present. Native forwarding still uses RawBody and Parameters unchanged.
func (r *NovelAIImageRequest) Size() string {
	if r == nil || r.Width <= 0 || r.Height <= 0 {
		return ""
	}
	return fmt.Sprintf("%dx%d", r.Width, r.Height)
}

// WithNovelAIImageRequest carries the native request through the shared image
// scheduling handler while its routing view uses an OpenAI-shaped body.
func WithNovelAIImageRequest(ctx context.Context, request *NovelAIImageRequest) context.Context {
	return context.WithValue(ctx, novelAIImageRequestContextKey{}, request)
}

func NovelAIImageRequestFromContext(ctx context.Context) (*NovelAIImageRequest, bool) {
	if ctx == nil {
		return nil, false
	}
	request, ok := ctx.Value(novelAIImageRequestContextKey{}).(*NovelAIImageRequest)
	return request, ok && request != nil
}

// BuildOpenAIImageRoutingBody creates the small routing view consumed by the
// existing image scheduler. The native body remains available in context and
// is what the NovelAI forwarder sends upstream.
func BuildOpenAIImageRoutingBody(request *NovelAIImageRequest) ([]byte, error) {
	if request == nil {
		return nil, fmt.Errorf("NovelAI request is required")
	}
	payload := map[string]any{
		"model":           request.Model,
		"prompt":          request.Input,
		"n":               request.Samples,
		"response_format": "b64_json",
	}
	if size := request.Size(); size != "" {
		payload["size"] = size
	}
	return json.Marshal(payload)
}
