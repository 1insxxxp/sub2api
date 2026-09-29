package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountGetImageProtocolDefaultsToOpenAI(t *testing.T) {
	account := &Account{}
	require.Equal(t, ImageProtocolOpenAI, account.GetImageProtocol())

	account.Credentials = map[string]any{"image_protocol": "novelai"}
	require.Equal(t, ImageProtocolNovelAI, account.GetImageProtocol())

	account.Credentials["image_protocol"] = "unexpected"
	require.Equal(t, ImageProtocolOpenAI, account.GetImageProtocol())
}

func TestBuildNovelAIImageURLNormalizesBaseURL(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{name: "host", base: "https://nai.example.test", want: "https://nai.example.test/ai/generate-image"},
		{name: "v1 suffix", base: "https://nai.example.test/v1/", want: "https://nai.example.test/ai/generate-image"},
		{name: "native suffix", base: "https://nai.example.test/ai/generate-image", want: "https://nai.example.test/ai/generate-image"},
		{name: "ai suffix", base: "https://nai.example.test/ai/", want: "https://nai.example.test/ai/generate-image"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildNovelAIImageURL(tt.base)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBuildNovelAIImageURLRejectsInvalidBaseURL(t *testing.T) {
	_, err := buildNovelAIImageURL("not a url")
	require.Error(t, err)
}

func TestForwardNovelAIUsesNativeEndpointAndPreservesBody(t *testing.T) {
	body := []byte(`{"input":"1girl","model":"nai-diffusion-5-full","action":"generate","parameters":{"steps":28,"n_samples":1,"future_option":{"enabled":true}}}`)
	c, _ := newOpenAIImagesTestContext(t, body)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/zip"}, "X-Request-Id": []string{"nai-req"}},
		Body:       io.NopCloser(strings.NewReader("zip-bytes")),
	}}
	svc := newOpenAIImagesTestService(upstream)
	account := &Account{
		ID:       31,
		Name:     "nai-api-key",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":        "ynai-test",
			"base_url":       "https://nai.example.test/v1",
			"image_protocol": "novelai",
		},
	}
	parsed, err := ParseNovelAIImageRequest(body)
	require.NoError(t, err)

	result, err := svc.ForwardNovelAI(context.Background(), c, account, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, []byte("zip-bytes"), result.Body)
	require.Equal(t, "nai-req", result.RequestID)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "/ai/generate-image", upstream.lastReq.URL.Path)
	require.Equal(t, "Bearer ynai-test", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Content-Type"))
	forwarded, readErr := io.ReadAll(upstream.lastReq.Body)
	require.NoError(t, readErr)
	require.JSONEq(t, string(body), string(forwarded))
}
