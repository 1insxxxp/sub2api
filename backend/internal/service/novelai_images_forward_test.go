package service

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
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
	contentType := upstream.lastReq.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	require.NotEmpty(t, params["boundary"])
	forwarded, readErr := io.ReadAll(upstream.lastReq.Body)
	require.NoError(t, readErr)
	reader := multipart.NewReader(strings.NewReader(string(forwarded)), params["boundary"])
	part, readErr := reader.NextPart()
	require.NoError(t, readErr)
	require.Equal(t, "request", part.FormName())
	require.Equal(t, "blob", part.FileName())
	require.Equal(t, "application/json", part.Header.Get("Content-Type"))
	requestPart, readErr := io.ReadAll(part)
	require.NoError(t, readErr)
	require.JSONEq(t, string(body), string(requestPart))
	_, readErr = reader.NextPart()
	require.ErrorIs(t, readErr, io.EOF)
}

func TestForwardNovelAIReturnsFailoverForRetryableUpstream(t *testing.T) {
	body := []byte(`{"input":"1girl","model":"nai-diffusion-5-full","parameters":{}}`)
	c, _ := newOpenAIImagesTestContext(t, body)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"temporarily unavailable"}}`)),
	}}
	svc := newOpenAIImagesTestService(upstream)
	account := &Account{
		ID:       32,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":        "ynai-test",
			"base_url":       "https://nai.example.test",
			"image_protocol": "novelai",
		},
	}
	parsed, err := ParseNovelAIImageRequest(body)
	require.NoError(t, err)

	_, err = svc.ForwardNovelAI(context.Background(), c, account, parsed, "")
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusServiceUnavailable, failoverErr.StatusCode)
}
