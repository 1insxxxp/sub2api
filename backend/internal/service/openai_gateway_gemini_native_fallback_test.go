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
)

type geminiNativeFallbackStub struct {
	calls  int
	body   []byte
	result *ForwardResult
	err    error
}

func (s *geminiNativeFallbackStub) ForwardAsChatCompletions(_ context.Context, _ *gin.Context, _ *Account, body []byte) (*ForwardResult, error) {
	s.calls++
	s.body = append([]byte(nil), body...)
	return s.result, s.err
}

func TestForwardAsChatCompletions_FallsBackToGeminiNativeOnContentsRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"按量vet/gemini-3.7-flash","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"contents is required (request id: test)"}}`)),
	}}
	fallback := &geminiNativeFallbackStub{
		result: &ForwardResult{
			Model:         "按量vet/gemini-3.7-flash",
			UpstreamModel: "gemini-3.7-flash",
			Usage: ClaudeUsage{
				InputTokens:  2,
				OutputTokens: 1,
			},
		},
	}
	svc := &OpenAIGatewayService{
		cfg:                   &config.Config{},
		httpUpstream:          upstream,
		geminiCompatForwarder: fallback,
	}
	account := &Account{
		ID:       5555,
		Name:     "vet gemini 1",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://kpzhu.com",
			"model_mapping": map[string]any{
				"按量vet/gemini-3.7-flash": "gemini-3.7-flash",
			},
		},
		Extra: map[string]any{
			"openai_responses_supported": true,
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, fallback.calls)
	require.Equal(t, body, fallback.body)
	require.Equal(t, "按量vet/gemini-3.7-flash", result.Model)
	require.Equal(t, "gemini-3.7-flash", result.UpstreamModel)
}

func TestForwardAsChatCompletions_DoesNotGeminiFallbackForOtherErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"按量vet/gemini-3.7-flash","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"unknown parameter"}}`)),
	}}
	fallback := &geminiNativeFallbackStub{result: &ForwardResult{}}
	svc := &OpenAIGatewayService{
		cfg:                   &config.Config{},
		httpUpstream:          upstream,
		geminiCompatForwarder: fallback,
	}
	account := &Account{
		ID:       5555,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://kpzhu.com",
			"model_mapping": map[string]any{
				"按量vet/gemini-3.7-flash": "gemini-3.7-flash",
			},
		},
		Extra: map[string]any{
			"openai_responses_supported": true,
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")

	require.Error(t, err)
	require.Nil(t, result)
	require.Zero(t, fallback.calls)
}

func TestForwardAsChatCompletions_RawPathFallsBackToGeminiNative(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"按量vet/gemini-2.5-flash","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"contents is required (request id: test)"}}`)),
	}}
	fallback := &geminiNativeFallbackStub{
		result: &ForwardResult{
			Model:         "按量vet/gemini-2.5-flash",
			UpstreamModel: "gemini-2.5-flash",
			Usage: ClaudeUsage{
				InputTokens:  2,
				OutputTokens: 1,
			},
		},
	}
	svc := &OpenAIGatewayService{
		cfg:                   &config.Config{},
		httpUpstream:          upstream,
		geminiCompatForwarder: fallback,
	}
	account := &Account{
		ID:       5555,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://kpzhu.com",
			"model_mapping": map[string]any{
				"按量vet/gemini-2.5-flash": "gemini-2.5-flash",
			},
		},
		Extra: map[string]any{
			"openai_responses_supported": false,
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, fallback.calls)
	require.Equal(t, body, fallback.body)
}
