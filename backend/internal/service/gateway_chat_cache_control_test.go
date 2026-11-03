//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestForwardAsChatCompletions_PreservesExplicitCacheControl(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name, fields, path, cache string
		absent                    []string
	}{
		{
			name:   "system block boundary and TTL",
			fields: `"messages":[{"role":"system","content":[{"type":"text","text":"stable prefix","cache_control":{"type":"ephemeral","ttl":"1h"}},{"type":"text","text":"changing suffix"}]},{"role":"user","content":"hello"}]`,
			path:   "system.0.cache_control", cache: `{"type":"ephemeral","ttl":"1h"}`,
			absent: []string{"system.1.cache_control", "messages.0.content.0.cache_control"},
		},
		{
			name:   "developer block",
			fields: `"messages":[{"role":"system","content":"first"},{"role":"developer","content":[{"type":"text","text":"stable developer prefix","cache_control":{"type":"ephemeral"}}]},{"role":"user","content":"hello"}]`,
			path:   "system.1.cache_control", cache: `{"type":"ephemeral"}`,
			absent: []string{"system.0.cache_control"},
		},
		{
			name:   "merged user messages",
			fields: `"messages":[{"role":"user","content":[{"type":"text","text":"stable prefix","cache_control":{"type":"ephemeral","ttl":"5m"}}]},{"role":"user","content":"new question"}]`,
			path:   "messages.0.content.0.cache_control", cache: `{"type":"ephemeral","ttl":"5m"}`,
			absent: []string{"messages.0.content.1.cache_control"},
		},
		{
			name:   "assistant history boundary",
			fields: `"messages":[{"role":"user","content":"question"},{"role":"assistant","content":[{"type":"text","text":"stable answer","cache_control":{"type":"ephemeral"}},{"type":"text","text":"later answer"}]},{"role":"user","content":"continue"}]`,
			path:   "messages.1.content.0.cache_control", cache: `{"type":"ephemeral"}`,
			absent: []string{"messages.1.content.1.cache_control"},
		},
		{
			name:   "function definition",
			fields: `"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}},"cache_control":{"type":"ephemeral","ttl":"1h"}}],"messages":[{"role":"user","content":"hello"}]`,
			path:   "tools.0.cache_control", cache: `{"type":"ephemeral","ttl":"1h"}`,
		},
		{
			name:   "explicit automatic caching",
			fields: `"cache_control":{"type":"ephemeral","ttl":"5m"},"messages":[{"role":"user","content":"hello"}]`,
			path:   "cache_control", cache: `{"type":"ephemeral","ttl":"5m"}`,
		},
		{
			name:   "image block after skipped part",
			fields: `"messages":[{"role":"user","content":[{"type":"unknown","cache_control":{"type":"ephemeral","ttl":"1h"}},{"type":"image_url","image_url":{"url":"data:image/png;base64,aGVsbG8="},"cache_control":{"type":"ephemeral","ttl":"5m"}},{"type":"text","text":"describe"}]}]`,
			path:   "messages.0.content.0.cache_control", cache: `{"type":"ephemeral","ttl":"5m"}`,
			absent: []string{"messages.0.content.1.cache_control"},
		},
		{
			name:   "document block",
			fields: `"messages":[{"role":"user","content":[{"type":"file","file":{"filename":"test.pdf","file_data":"data:application/pdf;base64,aGVsbG8="},"cache_control":{"type":"ephemeral"}},{"type":"text","text":"summarize"}]}]`,
			path:   "messages.0.content.0.cache_control", cache: `{"type":"ephemeral"}`,
			absent: []string{"messages.0.content.1.cache_control"},
		},
		{
			name:   "no implicit caching",
			fields: `"messages":[{"role":"system","content":"stable prefix"},{"role":"user","content":"hello"}]`,
		},
	}
	for _, tc := range cases {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%v", tc.name, stream), func(t *testing.T) {
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
				upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
					StatusCode: http.StatusOK, Header: http.Header{},
					Body: io.NopCloser(strings.NewReader(namespaceToolAnthropicStream())),
				}}
				svc := &GatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}}
				body := []byte(fmt.Sprintf(`{"model":"claude-opus-4-6","max_tokens":100,"stream":%v,%s}`, stream, tc.fields))
				_, err := svc.ForwardAsChatCompletions(context.Background(), c, newAnthropicAPIKeyAccountForTest(), body, nil)
				require.NoError(t, err)
				if tc.path == "" {
					require.NotContains(t, string(upstream.lastBody), `"cache_control"`)
					require.Equal(t, "stable prefix", gjson.GetBytes(upstream.lastBody, "system").String())
					return
				}
				require.JSONEq(t, tc.cache, gjson.GetBytes(upstream.lastBody, tc.path).Raw, "%s", upstream.lastBody)
				for _, path := range tc.absent {
					require.False(t, gjson.GetBytes(upstream.lastBody, path).Exists(), path)
				}
			})
		}
	}
}

func TestForwardAsResponses_PreservesExplicitCacheControl(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%v", stream), func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK, Header: http.Header{},
				Body: io.NopCloser(strings.NewReader(namespaceToolAnthropicStream())),
			}}
			svc := &GatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}}
			body := []byte(fmt.Sprintf(`{"model":"claude-opus-4-6","max_output_tokens":100,"stream":%v,
				"cache_control":{"type":"ephemeral","ttl":"5m"},
				"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"},"cache_control":{"type":"ephemeral","ttl":"1h"}}],
				"input":[{"role":"system","content":[{"type":"input_text","text":"stable prefix","cache_control":{"type":"ephemeral","ttl":"1h"}},{"type":"input_text","text":"changing suffix"}]},{"role":"user","content":"hello"}]}`, stream))
			_, err := svc.ForwardAsResponses(context.Background(), c, newAnthropicAPIKeyAccountForTest(), body, nil)
			require.NoError(t, err)
			for _, path := range []string{"system.0.cache_control", "tools.0.cache_control"} {
				require.JSONEq(t, `{"type":"ephemeral","ttl":"1h"}`, gjson.GetBytes(upstream.lastBody, path).Raw, path)
			}
			require.JSONEq(t, `{"type":"ephemeral","ttl":"5m"}`, gjson.GetBytes(upstream.lastBody, "cache_control").Raw)
			require.False(t, gjson.GetBytes(upstream.lastBody, "system.1.cache_control").Exists())
		})
	}
}

func TestNativeAnthropicChat_PreservesExplicitCacheControl(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{resp: nativeAnthropicStreamResponse()}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	body := []byte(`{"model":"k3","max_tokens":100,"cache_control":{"type":"ephemeral","ttl":"5m"},"messages":[{"role":"system","content":[{"type":"text","text":"prefix","cache_control":{"type":"ephemeral","ttl":"1h"}},{"type":"text","text":"suffix"}]},{"role":"user","content":"hello"}]}`)
	_, err := svc.forwardChatCompletionsViaNativeAnthropic(context.Background(), adaptiveProtocolTestContext("/v1/chat/completions", body), nativeAnthropicTestAccount(), body, "")
	require.NoError(t, err)
	require.Equal(t, "ephemeral", gjson.GetBytes(upstream.lastBody, "cache_control.type").String())
	require.Equal(t, "1h", gjson.GetBytes(upstream.lastBody, "system.0.cache_control.ttl").String())
	require.Equal(t, "suffix", gjson.GetBytes(upstream.lastBody, "system.1.text").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "system.1.cache_control").Exists())
}
