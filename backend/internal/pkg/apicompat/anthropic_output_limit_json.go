package apicompat

import "encoding/json"

// MarshalJSON preserves an explicitly supplied zero while leaving an unknown
// or optional output limit absent. Positive values in existing struct literals
// continue to serialize without requiring the presence flag.
func (r AnthropicRequest) MarshalJSON() ([]byte, error) {
	type request AnthropicRequest
	var limit *int
	if r.MaxTokensPresent || r.MaxTokens != 0 {
		limit = &r.MaxTokens
	}
	return json.Marshal(struct {
		MaxTokens *int `json:"max_tokens,omitempty"`
		*request
	}{MaxTokens: limit, request: (*request)(&r)})
}

func (r *AnthropicRequest) UnmarshalJSON(data []byte) error {
	type request AnthropicRequest
	wire := struct {
		MaxTokens *int `json:"max_tokens"`
		*request
	}{request: (*request)(r)}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	r.MaxTokens, r.MaxTokensPresent = 0, wire.MaxTokens != nil
	if wire.MaxTokens != nil {
		r.MaxTokens = *wire.MaxTokens
	}
	return nil
}
