//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Exercise the public Messages forwarding entry point and its real conversion,
// policy and response handling. Only the upstream HTTP boundary is stubbed.
func TestForwardAsAnthropic_ForceChatCompletionsForcedServiceTier(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		model          string
		forceFast      bool
		forceUltrafast bool
		clientTier     string
		policyTier     string
		policyAction   string
		wantTier       string
		wantBlocked    bool
	}{
		{
			name: "astra ultrafast without client tier", model: "gpt-6-astra",
			forceUltrafast: true, wantTier: "ultrafast",
		},
		{
			name: "sol ultrafast without client tier", model: "gpt-6.1-sol",
			forceUltrafast: true, wantTier: "ultrafast",
		},
		{
			name: "astra ultrafast wins over fast and client priority", model: "gpt-6-astra",
			forceFast: true, forceUltrafast: true, clientTier: "priority", wantTier: "ultrafast",
		},
		{
			name: "sol ultrafast wins over fast and client priority", model: "gpt-6.1-sol",
			forceFast: true, forceUltrafast: true, clientTier: "priority", wantTier: "ultrafast",
		},
		{
			name: "fast alone injects priority", model: "gpt-6.1-sol",
			forceFast: true, wantTier: "priority",
		},
		{
			name: "unsupported ultrafast model falls back to fast", model: "gpt-5.5",
			forceFast: true, forceUltrafast: true, wantTier: "priority",
		},
		{
			name: "ultrafast alone leaves unsupported model unchanged", model: "gpt-5.5",
			forceUltrafast: true,
		},
		{
			name: "global filter removes injected ultrafast", model: "gpt-6-astra",
			forceFast: true, forceUltrafast: true,
			policyTier: "ultrafast", policyAction: BetaPolicyActionFilter,
		},
		{
			name: "global filter clears original ultrafast billing tier", model: "gpt-6.1-sol",
			forceFast: true, forceUltrafast: true, clientTier: "ultrafast",
			policyTier: "ultrafast", policyAction: BetaPolicyActionFilter,
		},
		{
			name: "global filter clears original priority billing tier", model: "gpt-6-astra",
			forceFast: true, clientTier: "priority",
			policyTier: "priority", policyAction: BetaPolicyActionFilter,
		},
		{
			name: "global force priority overrides forced ultrafast", model: "gpt-6.1-sol",
			forceUltrafast: true, clientTier: "ultrafast",
			policyTier: "ultrafast", policyAction: OpenAIFastPolicyActionForcePriority, wantTier: "priority",
		},
		{
			name: "priority filter does not remove forced ultrafast", model: "gpt-6-astra",
			forceFast: true, forceUltrafast: true,
			policyTier: "priority", policyAction: BetaPolicyActionFilter, wantTier: "ultrafast",
		},
		{
			name: "global block rejects injected ultrafast", model: "gpt-6.1-sol",
			forceFast: true, forceUltrafast: true,
			policyTier: "ultrafast", policyAction: BetaPolicyActionBlock, wantBlocked: true,
		},
		{
			name: "global block rejects injected priority", model: "gpt-6-astra",
			forceFast:  true,
			policyTier: "priority", policyAction: BetaPolicyActionBlock, wantBlocked: true,
		},
	}

	for _, tt := range tests {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", tt.name, stream), func(t *testing.T) {
				request := map[string]any{
					"model": tt.model, "max_tokens": 32, "stream": stream,
					"messages":      []map[string]string{{"role": "user", "content": "hello"}},
					"output_config": map[string]string{"effort": "max"},
				}
				if tt.clientTier != "" {
					// A raw client tier must never override the final outbound policy
					// in usage metadata, even if the Messages converter drops it.
					request["service_tier"] = tt.clientTier
				}
				body, err := json.Marshal(request)
				require.NoError(t, err)
				ctx := context.WithValue(context.Background(), ctxkey.Group, &Group{
					ID: 7, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true,
					ForceOpenAIFast: tt.forceFast, ForceOpenAIUltrafast: tt.forceUltrafast,
				})
				// Keep the custom max-effort conversion and policy cap in this path.
				ctx = WithOpenAIReasoningEffortPolicy(ctx, "high", nil, "")
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body)).WithContext(ctx)
				c.Request.Header.Set("Content-Type", "application/json")

				settings := DefaultOpenAIFastPolicySettings()
				const blockedMessage = "forced tier disabled by global policy"
				if tt.policyAction != "" {
					settings.Rules = []OpenAIFastPolicyRule{{
						ServiceTier: tt.policyTier, Action: tt.policyAction,
						Scope: BetaPolicyScopeAll, ErrorMessage: blockedMessage,
					}}
				}
				upstream := &httpUpstreamRecorder{resp: messagesChatForcedTierResponse(tt.model, stream)}
				svc := newOpenAIGatewayServiceWithSettings(t, settings)
				svc.cfg = rawChatCompletionsTestConfig()
				svc.httpUpstream = upstream

				result, err := svc.ForwardAsAnthropic(ctx, c, forceChatMessagesFallbackAccount(), body, "", "")
				if tt.wantBlocked {
					var blocked *OpenAIFastBlockedError
					if assert.ErrorAs(t, err, &blocked) {
						assert.Equal(t, blockedMessage, blocked.Message)
					}
					assert.Nil(t, result)
					assert.Empty(t, upstream.requests, "global block must stop the request before upstream HTTP")
					assert.Equal(t, http.StatusForbidden, rec.Code)
					assert.Equal(t, "error", gjson.Get(rec.Body.String(), "type").String())
					assert.Equal(t, "forbidden_error", gjson.Get(rec.Body.String(), "error.type").String())
					assert.Equal(t, blockedMessage, gjson.Get(rec.Body.String(), "error.message").String())
					return
				}
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Len(t, upstream.requests, 1)
				assert.Equal(t, "/v1/chat/completions", upstream.lastReq.URL.Path)
				assert.Equal(t, tt.model, gjson.GetBytes(upstream.lastBody, "model").String())
				assert.Equal(t, "hello", gjson.GetBytes(upstream.lastBody, "messages.0.content").String())
				assert.False(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
				assert.Equal(t, stream, gjson.GetBytes(upstream.lastBody, "stream").Bool())
				outboundTier := gjson.GetBytes(upstream.lastBody, "service_tier")
				assert.Equal(t, tt.wantTier, outboundTier.String(), "forced tier must pass through global policy before sending")
				assert.Equal(t, tt.wantTier != "", outboundTier.Exists(), "filter must remove the field entirely")
				if tt.wantTier == "" {
					assert.Nil(t, result.ServiceTier, "filtered requests must not retain a billing tier")
				} else {
					assert.Equal(t, &tt.wantTier, result.ServiceTier, "billing tier must match final outbound tier")
				}
				assert.Equal(t, "default", result.UpstreamResponseServiceTier, "observed tier remains separate from outbound billing tier")
				assert.Equal(t, "high", gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
				wantEffort := "high"
				assert.Equal(t, &wantEffort, result.ReasoningEffort)
				assert.Equal(t, stream, result.Stream)
				assert.Equal(t, http.StatusOK, rec.Code)
				if stream {
					assert.Contains(t, rec.Body.String(), "event: message_stop")
				} else {
					assert.Equal(t, "ok", gjson.Get(rec.Body.String(), "content.0.text").String())
				}
			})
		}
	}
}

// Returning a different observed tier prevents the fixture from masking stale
// request-tier billing metadata. No HTTP server or network client is used.
func messagesChatForcedTierResponse(model string, stream bool) *http.Response {
	contentType := "application/json"
	body := fmt.Sprintf(`{"id":"chatcmpl_tier","object":"chat.completion","model":%q,"service_tier":"default","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`, model)
	if stream {
		contentType = "text/event-stream"
		body = fmt.Sprintf("data: "+`{"id":"chatcmpl_tier","object":"chat.completion.chunk","model":%q,"service_tier":"default","choices":[{"index":0,"delta":{"role":"assistant","content":"ok"},"finish_reason":null}]}`+"\n\n"+
			"data: "+`{"id":"chatcmpl_tier","object":"chat.completion.chunk","model":%q,"service_tier":"default","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`+"\n\ndata: [DONE]\n\n", model, model)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
