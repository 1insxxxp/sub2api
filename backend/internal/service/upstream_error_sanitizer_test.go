package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSanitizeUpstreamErrorBodyMasksUpstreamAddresses(t *testing.T) {
	body := []byte(`{
		"error":{
			"code":"not_found",
			"message":"[openclawroot.com] 请求的模型或接口不存在，请检查模型名称和请求路径 https://openclawroot.com/v1/embeddings?key=secret",
			"type":"api_error"
		}
	}`)

	sanitized := sanitizeUpstreamErrorBody(body)

	require.NotContains(t, string(sanitized), "openclawroot.com")
	require.Contains(t, string(sanitized), "[upstream]")
	require.Contains(t, string(sanitized), "[upstream-url]")
	require.NotContains(t, string(sanitized), "key=secret")
	require.Equal(t, "not_found", gjson.GetBytes(sanitized, "error.code").String())
}

func TestSanitizeUpstreamErrorBodyPreservesUnchangedJSONBytes(t *testing.T) {
	body := []byte(`{"type":"response.failed","response":{"status":"failed","error":{"code":"content_policy","message":"request blocked by policy"},"usage":{"input_tokens":6,"output_tokens":0,"total_tokens":6}}}`)

	require.Equal(t, string(body), string(sanitizeUpstreamErrorBody(body)))
}

func TestSanitizeUpstreamErrorBodyMasksNestedAndPlainTextAddresses(t *testing.T) {
	body := []byte(`{
		"error": {
			"message": "request failed at https://relay.example:8443/v1/responses?access_token=secret",
			"details": [{"endpoint": "[10.20.30.40:9443]"}]
		},
		"message": "see https://relay.example:8443/debug"
	}`)

	sanitized := sanitizeUpstreamErrorBody(body)

	require.NotContains(t, string(sanitized), "relay.example")
	require.NotContains(t, string(sanitized), "10.20.30.40")
	require.NotContains(t, string(sanitized), "access_token=secret")
	require.Contains(t, string(sanitized), "[upstream-url]")
	require.Contains(t, string(sanitized), "[upstream]")

	plainText := sanitizeUpstreamErrorBody([]byte("upstream https://relay.example:8443/v1 failed"))
	require.NotContains(t, string(plainText), "relay.example")
	require.Contains(t, string(plainText), "[upstream-url]")
}

func TestSanitizeErrorBodyForStorageMasksUpstreamAddresses(t *testing.T) {
	raw := `{"error":{"message":"provider failed at https://relay.example:8443/v1","api_key":"secret"}}`

	sanitized, _ := sanitizeErrorBodyForStorage(raw, 4096)

	require.NotContains(t, sanitized, "relay.example")
	require.NotContains(t, sanitized, "secret")
	require.Contains(t, sanitized, "[upstream-url]")
}

func TestSanitizeOpenAIResponseFailedEventForClientMasksAddress(t *testing.T) {
	payload := []byte(`{"type":"response.failed","response":{"status":"failed","error":{"code":"upstream_error","message":"blocked by https://relay.example:8443/policy"}}}`)

	sanitized, changed := sanitizeOpenAIResponseFailedEventForClient(payload, "response.failed", false)

	require.True(t, changed)
	require.NotContains(t, string(sanitized), "relay.example")
	require.Contains(t, string(sanitized), "[upstream-url]")
}

func TestForwardEmbeddingsSanitizesRawUpstreamErrorBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"按次反重力1/gemini-2.5-pro","input":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"code":"not_found","message":"[openclawroot.com] 请求的模型或接口不存在，请检查模型名称和请求路径","type":"api_error"}}`,
		)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       5439,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://openclawroot.com",
			"model_mapping": map[string]any{
				"按次反重力1/gemini-2.5-pro": "gemini-2.5-pro",
			},
		},
	}

	result, err := svc.ForwardEmbeddings(context.Background(), c, account, body, "")

	require.Error(t, err)
	require.Nil(t, result)
	require.NotContains(t, rec.Body.String(), "openclawroot.com")
	require.Contains(t, rec.Body.String(), "[upstream]")
}
