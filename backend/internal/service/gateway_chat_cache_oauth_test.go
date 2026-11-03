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

func TestForwardAsChatCompletions_OAuthSystemCacheBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name, systemContent, wantContent string
	}{
		{
			name:          "marked prefix with unmarked suffix",
			systemContent: `[{"type":"text","text":"stable prefix","cache_control":{"type":"ephemeral","ttl":"1h"}},{"type":"text","text":"changing suffix"}]`,
			wantContent:   `[{"type":"text","text":"[System Instructions]\nstable prefix","cache_control":{"type":"ephemeral","ttl":"1h"}},{"type":"text","text":"changing suffix"}]`,
		},
		{
			name: "multiple markers with original TTLs",
			systemContent: `[{"type":"text","text":"intro"},
				{"type":"text","text":"long lived prefix","cache_control":{"type":"ephemeral","ttl":"1h"}},
				{"type":"text","text":"unmarked middle"},
				{"type":"text","text":"short lived prefix","cache_control":{"type":"ephemeral","ttl":"5m"}},
				{"type":"text","text":"default TTL prefix","cache_control":{"type":"ephemeral"}},
				{"type":"text","text":"changing suffix"}]`,
			wantContent: `[{"type":"text","text":"[System Instructions]\nintro"},
				{"type":"text","text":"long lived prefix","cache_control":{"type":"ephemeral","ttl":"1h"}},
				{"type":"text","text":"unmarked middle"},
				{"type":"text","text":"short lived prefix","cache_control":{"type":"ephemeral","ttl":"5m"}},
				{"type":"text","text":"default TTL prefix","cache_control":{"type":"ephemeral"}},
				{"type":"text","text":"changing suffix"}]`,
		},
		{
			name:          "unmarked blocks retain joined representation",
			systemContent: `[{"type":"text","text":"first"},{"type":"text","text":"second"}]`,
			wantContent:   `[{"type":"text","text":"[System Instructions]\nfirst\n\nsecond"}]`,
		},
		{
			name:          "unmarked string retains trimming",
			systemContent: `"  original instructions  "`,
			wantContent:   `[{"type":"text","text":"[System Instructions]\noriginal instructions"}]`,
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
				body := []byte(fmt.Sprintf(`{"model":"claude-opus-4-6","max_tokens":100,"stream":%v,"messages":[{"role":"system","content":%s},{"role":"user","content":"hello"}]}`, stream, tc.systemContent))
				_, err := svc.ForwardAsChatCompletions(context.Background(), c, newAnthropicOAuthAccountForPartialUsageTest(), body, nil)
				require.NoError(t, err)
				require.NotNil(t, upstream.lastReq)
				require.Equal(t, []string{"Bearer oauth-token"}, upstream.lastReq.Header["authorization"])
				require.JSONEq(t, tc.wantContent, gjson.GetBytes(upstream.lastBody, "messages.0.content").Raw)
				require.Len(t, gjson.GetBytes(upstream.lastBody, "messages").Array(), 3)
				require.Equal(t, "user", gjson.GetBytes(upstream.lastBody, "messages.0.role").String())
				require.Equal(t, "assistant", gjson.GetBytes(upstream.lastBody, "messages.1.role").String())
				require.Equal(t, "Understood. I will follow these instructions.", gjson.GetBytes(upstream.lastBody, "messages.1.content.0.text").String())
				require.Equal(t, "user", gjson.GetBytes(upstream.lastBody, "messages.2.role").String())
				require.Contains(t, gjson.GetBytes(upstream.lastBody, "messages.2.content").Raw, "hello")
				require.Len(t, gjson.GetBytes(upstream.lastBody, "system").Array(), 3)
				require.Equal(t, "ephemeral", gjson.GetBytes(upstream.lastBody, "system.2.cache_control.type").String())
				requireCacheTTLOrderAccepted(t, upstream.lastBody)
				_, messagePaths, toolPaths, systemPaths := collectCacheControlPaths(upstream.lastBody)
				require.LessOrEqual(t, len(messagePaths)+len(toolPaths)+len(systemPaths), maxCacheControlBlocks)
			})
		}
	}
}

func TestForwardAsChatCompletions_OAuthNestedToolCacheTTL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{},
		Body: io.NopCloser(strings.NewReader(namespaceToolAnthropicStream())),
	}}
	svc := &GatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}}
	body := []byte(`{"model":"claude-opus-4-6","max_tokens":100,"messages":[
		{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"call_1","content":[{"type":"text","text":"stable result","cache_control":{"type":"ephemeral","ttl":"1h"}}]}
	]}`)
	_, err := svc.ForwardAsChatCompletions(context.Background(), c, newAnthropicOAuthAccountForPartialUsageTest(), body, nil)
	require.NoError(t, err)
	require.Equal(t, "1h", gjson.GetBytes(upstream.lastBody, "messages.1.content.0.content.0.cache_control.ttl").String())
	require.Equal(t, "1h", gjson.GetBytes(upstream.lastBody, "system.2.cache_control.ttl").String(), "injected system breakpoint must precede nested 1h breakpoint with a compatible TTL")
}

func TestNestedToolResultCachePolicies(t *testing.T) {
	nested := `{"type":"tool_result","tool_use_id":"call_1","content":[{"type":"text","text":"result","cache_control":{"type":"ephemeral","ttl":"1h"}}],"cache_control":{"type":"ephemeral","ttl":"5m"}}`
	body := []byte(`{"messages":[{"role":"user","content":[` + nested + `]}]}`)
	t.Run("TTL hint sees nested marker", func(t *testing.T) {
		target, ok := cacheTTLTargetFromAnthropicRequestBody(body)
		require.True(t, ok)
		require.Equal(t, cacheTTLTarget1h, target)
	})
	t.Run("nested breakpoint precedes outer result breakpoint", func(t *testing.T) {
		_, paths, _, _ := collectCacheControlPaths(body)
		require.Equal(t, []string{"messages.0.content.0.content.0.cache_control", "messages.0.content.0.cache_control"}, paths)
		require.Equal(t, string(body), string(normalizeCacheControlTTLOrder(body)), "1h inner prefix followed by 5m outer prefix is already valid")
	})
	t.Run("forced TTL includes nested markers", func(t *testing.T) {
		out := forceEphemeralCacheControlTTL(body, cacheTTLTarget5m)
		require.Equal(t, "5m", gjson.GetBytes(out, "messages.0.content.0.content.0.cache_control.ttl").String())
	})
	t.Run("opt in rewrite strips nested markers", func(t *testing.T) {
		out := stripMessageCacheControl(body)
		require.NotContains(t, string(out), `"cache_control"`)
		require.Equal(t, "result", gjson.GetBytes(out, "messages.0.content.0.content.0.text").String())
	})
	t.Run("nested markers count toward four block limit", func(t *testing.T) {
		parts := make([]string, 5)
		for i := range parts {
			parts[i] = fmt.Sprintf(`{"type":"text","text":"result %d","cache_control":{"type":"ephemeral"}}`, i)
		}
		request := []byte(`{"system":[{"type":"text","text":"instructions","cache_control":{"type":"ephemeral"}}],"tools":[{"name":"lookup","input_schema":{},"cache_control":{"type":"ephemeral"}}],"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","content":[` + strings.Join(parts, ",") + `]}]}]}`)
		out := enforceCacheControlLimit(request)
		require.Equal(t, 4, strings.Count(string(out), `"cache_control"`))
		require.False(t, gjson.GetBytes(out, "tools.0.cache_control").Exists())
		require.False(t, gjson.GetBytes(out, "messages.0.content.0.content.0.cache_control").Exists())
		require.True(t, gjson.GetBytes(out, "messages.0.content.0.content.4.cache_control").Exists())
		require.Equal(t, int64(5), gjson.GetBytes(out, "messages.0.content.0.content.#").Int())
	})
	t.Run("tool input data is not cache metadata", func(t *testing.T) {
		request := []byte(`{"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"call_1","name":"lookup","input":{"cache_control":{"type":"ephemeral","ttl":"1h"},"content":[{"type":"text","cache_control":{"type":"ephemeral"}}]}}]}]}`)
		_, paths, _, _ := collectCacheControlPaths(request)
		require.Empty(t, paths)
		_, ok := cacheTTLTargetFromAnthropicRequestBody(request)
		require.False(t, ok)
		require.Equal(t, string(request), string(stripMessageCacheControl(request)))
		require.Equal(t, string(request), string(forceEphemeralCacheControlTTL(request, cacheTTLTarget5m)))
	})
}
