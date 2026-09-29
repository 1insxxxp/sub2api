package service

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseNovelAIImageRequestBodyExtractsMultipartRequestPart(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("request", "blob")
	require.NoError(t, err)
	_, err = part.Write([]byte(`{"input":"cat","model":"nai-diffusion-5-full","parameters":{"n_samples":1}}`))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request, err := ParseNovelAIImageRequestBody(body.Bytes(), writer.FormDataContentType())
	require.NoError(t, err)
	require.Equal(t, "cat", request.Input)
	require.Equal(t, "nai-diffusion-5-full", request.Model)
}

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

func TestParseNovelAIImageRequestAllowsNativeGenerationActions(t *testing.T) {
	parsed, err := ParseNovelAIImageRequest([]byte(`{"input":"cat","model":"nai-diffusion-5-full","action":"img2img","parameters":{"strength":0.7}}`))
	require.NoError(t, err)
	require.Equal(t, "img2img", parsed.Action)
}

func TestBuildOpenAIImageRoutingBodyKeepsNativeModelAndPrompt(t *testing.T) {
	request, err := ParseNovelAIImageRequest([]byte(`{"input":"cat","model":"nai-diffusion-5-full","parameters":{"width":832,"height":1216,"n_samples":2}}`))
	require.NoError(t, err)
	body, err := BuildOpenAIImageRoutingBody(request)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"nai-diffusion-5-full","prompt":"cat","n":2,"size":"832x1216","response_format":"b64_json"}`, string(body))
}

func TestNovelAIImageModelsAreAcceptedByImageValidation(t *testing.T) {
	require.True(t, IsNovelAIImageGenerationModel("nai-diffusion-4-5-full"))
	require.True(t, IsNovelAIImageGenerationModel("NAI-DIFFUSION-5-FULL"))
	require.NoError(t, validateCompatibleImagesModel("nai-diffusion-5-full"))
	require.False(t, IsNovelAIImageGenerationModel("nai-text-model"))
}
