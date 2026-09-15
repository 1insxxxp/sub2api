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
