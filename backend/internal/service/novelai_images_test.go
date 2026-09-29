package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseNovelAIImageRequestPreservesNativeParametersAndUnknownFields(t *testing.T) {
	body := []byte(`{
		"input":"1girl, blue hair",
		"model":"nai-diffusion-4-5-full",
		"action":"generate",
		"parameters":{
			"width":832,
			"height":1216,
			"steps":28,
			"sampler":"k_euler_ancestral",
			"n_samples":2,
			"reference_image_multiple":["data:image/png;base64,abc"]
		},
		"future_option":{"enabled":true}
	}`)

	parsed, err := ParseNovelAIImageRequest(body)
	require.NoError(t, err)
	require.Equal(t, "1girl, blue hair", parsed.Input)
	require.Equal(t, "nai-diffusion-4-5-full", parsed.Model)
	require.Equal(t, "generate", parsed.Action)
	require.Equal(t, 2, parsed.Samples)
	require.Equal(t, "832x1216", parsed.Size())
	require.JSONEq(t, `{"width":832,"height":1216,"steps":28,"sampler":"k_euler_ancestral","n_samples":2,"reference_image_multiple":["data:image/png;base64,abc"]}`, string(parsed.Parameters))

	var preserved map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(parsed.RawBody, &preserved))
	require.JSONEq(t, `{"enabled":true}`, string(preserved["future_option"]))
}

func TestParseNovelAIImageRequestDefaultsActionAndSamples(t *testing.T) {
	parsed, err := ParseNovelAIImageRequest([]byte(`{"input":"cat","model":"nai-diffusion-5-full","parameters":{}}`))
	require.NoError(t, err)
	require.Equal(t, "generate", parsed.Action)
	require.Equal(t, 1, parsed.Samples)
	require.Empty(t, parsed.Size())
}

func TestParseNovelAIImageRequestRejectsInvalidPayload(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "malformed json", body: `{`, want: "invalid JSON"},
		{name: "missing input", body: `{"model":"nai-diffusion-5-full"}`, want: "input"},
		{name: "missing model", body: `{"input":"cat"}`, want: "model"},
		{name: "unsupported action", body: `{"input":"cat","model":"nai-diffusion-5-full","action":"generate-prompt"}`, want: "action"},
		{name: "parameters must be object", body: `{"input":"cat","model":"nai-diffusion-5-full","parameters":[]}`, want: "parameters"},
		{name: "invalid sample count", body: `{"input":"cat","model":"nai-diffusion-5-full","parameters":{"n_samples":0}}`, want: "n_samples"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseNovelAIImageRequest([]byte(tt.body))
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.want)
		})
	}
}
