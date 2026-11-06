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

func TestForwardClaude55LegacyThinking(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, model, fields, wantType, wantEffort, wantDisplay string
	}{
		{"opus enabled", "claude-opus-5-5", `"thinking":{"type":"enabled","budget_tokens":4096,"display":"summarized"}`, "adaptive", "medium", "summarized"},
		{"sonnet enabled", "claude-sonnet-5-5", `"thinking":{"type":"enabled","budget_tokens":1024}`, "adaptive", "low", ""},
		{"explicit effort", "claude-opus-5-5", `"thinking":{"type":"enabled","budget_tokens":1024},"output_config":{"effort":"max"}`, "adaptive", "max", ""},
		{"opus disabled", "claude-opus-5-5", `"thinking":{"type":"disabled","budget_tokens":16384},"output_config":{"effort":"max"}`, "adaptive", "low", "omitted"},
		{"sonnet disabled", "claude-sonnet-5-5", `"thinking":{"type":"disabled","budget_tokens":8192,"display":"omitted","block_binding":"strict"},"output_config":{"effort":"max"}`, "between_tools", "high", ""},
		{"sonnet disabled low", "claude-sonnet-5-5", `"thinking":{"type":"disabled"},"output_config":{"effort":"low"}`, "between_tools", "low", ""},
	} {
		for _, passthrough := range []bool{false, true} {
			for _, mode := range []string{"buffered", "stream", "count"} {
				t.Run(fmt.Sprintf("%s/passthrough=%t/%s", tc.name, passthrough, mode), func(t *testing.T) {
					body := []byte(fmt.Sprintf(`{"model":"public-claude","stream":%t,"max_tokens":256,"system":[{"type":"text","text":"fixture","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"","signature":"signed-history"},{"type":"text","text":"earlier"}]},{"role":"user","content":"hello"}],%s}`, mode == "stream", tc.fields))
					original, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
					require.NoError(t, err)
					parsed, err := original.CloneForBody(body)
					require.NoError(t, err)
					response := nativeAnthropicBufferedResponse()
					if mode == "stream" {
						response = nativeAnthropicStreamResponse()
					} else if mode == "count" {
						response = &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"input_tokens":18}`))}
					}
					upstream := &anthropicHTTPUpstreamRecorder{resp: response}
					svc := newForwardPartialUsageServiceForTest(upstream)
					account := newAnthropicAPIKeyAccountForTest()
					account.Extra["anthropic_passthrough"] = passthrough
					account.Credentials["model_mapping"] = map[string]any{"public-claude": tc.model}
					rec := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(rec)
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
					if mode == "count" {
						err = svc.ForwardCountTokens(context.Background(), c, account, parsed)
					} else {
						_, err = svc.Forward(context.Background(), c, account, parsed)
					}
					require.NoError(t, err)
					// The Messages handler reads the refreshed attempt metadata
					// when stamping effort on usage records.
					require.Equal(t, tc.wantEffort, parsed.OutputEffort)
					require.True(t, parsed.ThinkingEnabled)
					require.Equal(t, http.StatusOK, rec.Code)
					require.Equal(t, tc.model, gjson.GetBytes(upstream.lastBody, "model").String())
					require.Equal(t, tc.wantType, gjson.GetBytes(upstream.lastBody, "thinking.type").String())
					require.Equal(t, tc.wantEffort, gjson.GetBytes(upstream.lastBody, "output_config.effort").String())
					require.Equal(t, tc.wantDisplay, gjson.GetBytes(upstream.lastBody, "thinking.display").String())
					require.False(t, gjson.GetBytes(upstream.lastBody, "thinking.budget_tokens").Exists())
					require.JSONEq(t, gjson.GetBytes(body, "messages").Raw, gjson.GetBytes(upstream.lastBody, "messages").Raw)
					require.JSONEq(t, gjson.GetBytes(body, "system").Raw, gjson.GetBytes(upstream.lastBody, "system").Raw)
					if mode != "count" {
						require.Equal(t, int64(256), gjson.GetBytes(upstream.lastBody, "max_tokens").Int())
					}
					require.Equal(t, string(body), string(original.Body.Bytes()), "failover must still start from the client body")
				})
			}
		}
	}
}

func TestClaude55GroupDefaultNeverAddsLegacyThinking(t *testing.T) {
	for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5"} {
		t.Run(model, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"model":%q,"max_tokens":256,"messages":[]}`, model))
			out, changed := ApplyGroupDefaultReasoningEffort(body, &Group{Platform: PlatformAnthropic, DefaultReasoningEffort: "high"})
			require.True(t, changed)
			require.Equal(t, "adaptive", gjson.GetBytes(out, "thinking.type").String())
			require.Equal(t, "high", gjson.GetBytes(out, "output_config.effort").String())
			require.False(t, gjson.GetBytes(out, "thinking.budget_tokens").Exists())
			require.Equal(t, int64(256), gjson.GetBytes(out, "max_tokens").Int())
			require.NoError(t, validateClaude55Request(out, model))
		})
	}
}

func TestNormalizeClaude55LegacyThinkingBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, model, fields, wantType, wantEffort string
		unchanged                                 bool
	}{
		{name: "legacy model untouched", model: "claude-opus-4-6", fields: `"thinking":{"type":"enabled","budget_tokens":4096}`, unchanged: true},
		{name: "third party untouched", model: "kimi-k3", fields: `"thinking":{"type":"disabled"}`, unchanged: true},
		{name: "haiku remains strict", model: "claude-haiku-5-5", fields: `"thinking":{"type":"enabled","budget_tokens":4096}`, unchanged: true},
		{name: "adaptive untouched", model: "claude-opus-5-5", fields: `"thinking":{"type":"adaptive","display":"summarized"}`, unchanged: true},
		{name: "native between tools remains strict", model: "claude-sonnet-5-5", fields: `"thinking":{"type":"between_tools","budget_tokens":1024},"output_config":{"effort":"max"}`, unchanged: true},
		{name: "missing thinking untouched", model: "claude-opus-5-5", fields: `"metadata":{"user_id":"fixture"}`, unchanged: true},
		{name: "malformed budget untouched", model: "claude-opus-5-5", fields: `"thinking":{"type":"enabled","budget_tokens":1.5}`, unchanged: true},
		{name: "negative budget untouched", model: "claude-opus-5-5", fields: `"thinking":{"type":"enabled","budget_tokens":-1}`, unchanged: true},
		{name: "null budget untouched", model: "claude-opus-5-5", fields: `"thinking":{"type":"enabled","budget_tokens":null}`, unchanged: true},
		{name: "provider alias", model: "anthropic/claude-opus-5.5", fields: `"thinking":{"type":"enabled","budget_tokens":1024}`, wantType: "adaptive", wantEffort: "low"},
		{name: "bedrock ID", model: "global.anthropic.claude-sonnet-5-5", fields: `"thinking":{"type":"disabled"}`, wantType: "between_tools"},
		{name: "budget maps high", model: "claude-opus-5-5", fields: `"thinking":{"type":"enabled","budget_tokens":8192}`, wantType: "adaptive", wantEffort: "high"},
		{name: "budget maps max", model: "claude-opus-5-5", fields: `"thinking":{"type":"enabled","budget_tokens":32768}`, wantType: "adaptive", wantEffort: "max"},
		{name: "missing budget uses upstream default", model: "claude-opus-5-5", fields: `"thinking":{"type":"enabled"}`, wantType: "adaptive"},
		{name: "sonnet disabled caps xhigh", model: "claude-sonnet-5-5", fields: `"thinking":{"type":"disabled"},"output_config":{"effort":"xhigh"}`, wantType: "between_tools", wantEffort: "high"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`{"model":"public-alias","max_tokens":0,"messages":[],` + tc.fields + `}`)
			saved := string(body)
			out, changed := normalizeClaude55LegacyThinking(body, tc.model)
			require.Equal(t, saved, string(body), "never mutate shared request bytes")
			require.Equal(t, !tc.unchanged, changed)
			if tc.unchanged {
				require.Equal(t, saved, string(out))
				return
			}
			require.Equal(t, "public-alias", gjson.GetBytes(out, "model").String())
			require.Equal(t, "0", gjson.GetBytes(out, "max_tokens").Raw)
			require.Equal(t, tc.wantType, gjson.GetBytes(out, "thinking.type").String())
			require.Equal(t, tc.wantEffort, gjson.GetBytes(out, "output_config.effort").String())
			require.NoError(t, validateClaude55Request(out, tc.model))
			again, changed := normalizeClaude55LegacyThinking(out, tc.model)
			require.False(t, changed)
			require.Equal(t, string(out), string(again), "normalization is idempotent")
		})
	}
}

func TestForwardClaude55LegacyThinkingEffortPolicy(t *testing.T) {
	for _, count := range []bool{false, true} {
		for _, deny := range []bool{false, true} {
			t.Run(fmt.Sprintf("count=%t/deny=%t", count, deny), func(t *testing.T) {
				body := []byte(`{"model":"public-claude","max_tokens":256,"messages":[{"role":"user","content":"hello"}],"thinking":{"type":"enabled","budget_tokens":32768}}`)
				parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
				require.NoError(t, err)
				upstream := &anthropicHTTPUpstreamRecorder{resp: nativeAnthropicBufferedResponse()}
				if count {
					upstream.resp.Body = io.NopCloser(strings.NewReader(`{"input_tokens":18}`))
				}
				svc := newForwardPartialUsageServiceForTest(upstream)
				group := &Group{ID: 55, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true, MaxReasoningEffort: "medium"}
				if deny {
					group.MaxReasoningEffortOverLimit = ReasoningEffortOverLimitDeny
				}
				parsed.GroupID = &group.ID
				account := newAnthropicAPIKeyAccountForTest()
				account.Credentials["model_mapping"] = map[string]any{"public-claude": "claude-opus-5-5"}
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
				ctx := svc.withGroupContext(context.Background(), group)
				if count {
					err = svc.ForwardCountTokens(ctx, c, account, parsed)
				} else {
					_, err = svc.Forward(ctx, c, account, parsed)
				}
				if deny {
					require.True(t, IsReasoningEffortPolicyDenied(err))
					require.Equal(t, http.StatusForbidden, rec.Code)
					require.Nil(t, upstream.lastReq)
					return
				}
				require.NoError(t, err)
				require.Equal(t, "medium", gjson.GetBytes(upstream.lastBody, "output_config.effort").String())
			})
		}
	}
}

func TestForwardClaude55LegacyThinkingRequestedEffortAudit(t *testing.T) {
	body := []byte(`{"model":"claude-opus-5-5","max_tokens":256,"messages":[{"role":"user","content":"hello"}],"thinking":{"type":"disabled"},"output_config":{"effort":"high"}}`)
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
	require.NoError(t, err)
	upstream := &anthropicHTTPUpstreamRecorder{resp: nativeAnthropicBufferedResponse()}
	svc := newForwardPartialUsageServiceForTest(upstream)
	account := newAnthropicAPIKeyAccountForTest()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	// The ingress group policy already mapped the original max to high.
	result, err := svc.Forward(WithRequestedReasoningEffort(context.Background(), "max"), c, account, parsed)
	require.NoError(t, err)
	require.NotNil(t, result.RequestedReasoningEffort)
	require.Equal(t, "max", *result.RequestedReasoningEffort)
	require.Equal(t, "low", parsed.OutputEffort)
}

func TestClaude55ContinuationBillsNormalizedEffort(t *testing.T) {
	for _, tc := range []struct{ model, fields, want string }{
		{"claude-opus-5-5", `"thinking":{"type":"disabled"},"output_config":{"effort":"max"}`, "low"},
		{"claude-sonnet-5-5", `"thinking":{"type":"disabled"},"output_config":{"effort":"max"}`, "high"},
		{"claude-opus-5-5", `"thinking":{"type":"enabled","budget_tokens":4096}`, "medium"},
	} {
		for _, enabled := range []bool{false, true} {
			for _, stream := range []bool{false, true} {
				for _, passthrough := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/enabled=%t/stream=%t/passthrough=%t", tc.model, tc.want, enabled, stream, passthrough), func(t *testing.T) {
						body := []byte(fmt.Sprintf(`{"model":"long-text","max_tokens":256,"stream":%t,"messages":[{"role":"user","content":"write"}],%s}`, stream, tc.fields))
						parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
						require.NoError(t, err)
						var responses []*http.Response
						for i, reason := range []string{"max_tokens", "end_turn"} {
							payload, contentType := anthropicTextStream("hello", reason, 100, 5), "text/event-stream"
							if !stream {
								payload = fmt.Sprintf(`{"id":"msg-%d","type":"message","role":"assistant","model":%q,"content":[{"type":"text","text":"hello"}],"stop_reason":%q,"usage":{"input_tokens":100,"output_tokens":5}}`, i, tc.model, reason)
								contentType = "application/json"
							}
							responses = append(responses, &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(payload))})
						}
						upstream := &httpUpstreamRecorder{responses: responses}
						svc := &GatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}, rateLimitService: &RateLimitService{}, deferredService: &DeferredService{}}
						account := newAnthropicAPIKeyAccountForTest()
						account.Extra["anthropic_passthrough"] = passthrough
						account.Extra[AccountExtraAntiTruncation] = accountContinuationTestAccount(enabled).Extra[AccountExtraAntiTruncation]
						account.Credentials["model_mapping"] = map[string]any{"long-text": tc.model}
						c, _ := newGeminiOutputLimitContext(body)
						ctx := context.Background()
						if parsed.OutputEffort != "" {
							ctx = WithRequestedReasoningEffort(ctx, parsed.OutputEffort)
						}
						result, err := svc.Forward(ctx, c, account, parsed)
						require.NoError(t, err)
						require.NotNil(t, result)
						require.Equal(t, tc.want, optionalStringValue(result.ReasoningEffort))
						if parsed.OutputEffort != "" {
							require.Equal(t, "max", optionalStringValue(result.RequestedReasoningEffort))
						}
						leaves := []*ForwardResult{result}
						if enabled {
							require.Len(t, result.ContinuationResults, 2)
							leaves = result.ContinuationResults
						}
						require.Len(t, upstream.bodies, len(leaves))
						billing := NewBillingService(rawChatCompletionsTestConfig(), nil)
						group := &Group{ID: 1, Platform: PlatformAnthropic, ModelPricing: []ChannelModelPricing{{Models: []string{tc.model}, BillingMode: BillingModeToken, InputPrice: testPtrFloat64(1e-6), OutputPrice: testPtrFloat64(0), ReasoningEffortMultipliers: map[string]float64{"low": 1, "medium": 2, "high": 3, "max": 7}}}}
						for i, leaf := range leaves {
							require.Equal(t, tc.want, optionalStringValue(leaf.ReasoningEffort))
							require.Equal(t, tc.want, gjson.GetBytes(upstream.bodies[i], "output_config.effort").String())
							cost, costErr := billing.CalculateTokenCostForRequest(TokenCostRequest{Ctx: context.Background(), Model: tc.model, Group: group, Tokens: UsageTokens{InputTokens: leaf.Usage.InputTokens, OutputTokens: leaf.Usage.OutputTokens}, ReasoningEffort: optionalStringValue(leaf.ReasoningEffort), RateMultiplier: 1, Resolver: NewModelPricingResolver(nil, billing)})
							require.NoError(t, costErr)
							require.InDelta(t, 100e-6*group.ModelPricing[0].ReasoningEffortMultipliers[tc.want], cost.ActualCost, 1e-12)
						}
					})
				}
			}
		}
	}
}

func TestClaude55LegacyThinkingBedrockPreservesEffectiveEffort(t *testing.T) {
	body := []byte(`{"model":"claude-opus-5-5","max_tokens":256,"messages":[{"role":"user","content":"hello"}],"thinking":{"type":"disabled"}}`)
	normalized, changed := normalizeClaude55LegacyThinking(body, "anthropic.claude-opus-5-5")
	require.True(t, changed)
	wire, err := PrepareBedrockRequestBodyWithTokens(normalized, "anthropic.claude-opus-5-5", nil, false)
	require.NoError(t, err)
	require.Equal(t, "low", gjson.GetBytes(wire, "output_config.effort").String())
	require.Equal(t, int64(256), gjson.GetBytes(wire, "max_tokens").Int())
}

func TestClaude55CountTokensVertexDoesNotSendCredentialsToAnthropic(t *testing.T) {
	body := []byte(`{"model":"public-claude","messages":[{"role":"user","content":"hello"}],"thinking":{"type":"enabled","budget_tokens":4096}}`)
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
	require.NoError(t, err)
	account := &Account{Platform: PlatformAnthropic, Type: AccountTypeServiceAccount, Credentials: map[string]any{"model_mapping": map[string]any{"public-claude": "claude-opus-5-5"}}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)
	err = (&GatewayService{}).ForwardCountTokens(context.Background(), c, account, parsed)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, string(body), string(parsed.Body.Bytes()))
}

func TestClaude55LegacyDerivedEffortMappings(t *testing.T) {
	for _, tc := range []struct {
		name, extra, ceiling, target, want string
		deny                               bool
	}{
		{name: "map derived max once", target: "high", want: "high"},
		{name: "explicit already mapped effort is not mapped twice", extra: `,"output_config":{"effort":"high"}`, target: "high", want: "high"},
		{name: "deny derived max", target: "deny", deny: true},
		{name: "composite minimal uses Claude low", ceiling: "minimal", target: "high", want: "low"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`{"model":"public-claude","max_tokens":256,"messages":[],"thinking":{"type":"enabled","budget_tokens":32768}` + tc.extra + `}`)
			parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
			require.NoError(t, err)
			group := &Group{ID: 55, Platform: PlatformComposite, Status: StatusActive, Hydrated: true, MaxReasoningEffort: tc.ceiling,
				ReasoningEffortMappings: []ReasoningEffortMapping{{From: "max", To: tc.target}, {From: "high", To: "low"}}}
			parsed.GroupID = &group.ID
			svc := &GatewayService{}
			_, err = svc.normalizeClaude55LegacyRequest(svc.withGroupContext(context.Background(), group), parsed, "claude-opus-5-5")
			if tc.deny {
				require.True(t, IsReasoningEffortPolicyDenied(err))
				require.Equal(t, string(body), string(parsed.Body.Bytes()))
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, parsed.OutputEffort)
		})
	}
}

func TestForwardClaude55LegacyThinkingOAuth(t *testing.T) {
	for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5-5"} {
		t.Run(model, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"model":%q,"max_tokens":256,"messages":[{"role":"user","content":"hello"}],"thinking":{"type":"disabled"}}`, model))
			parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
			require.NoError(t, err)
			upstream := &anthropicHTTPUpstreamRecorder{resp: nativeAnthropicBufferedResponse()}
			svc := newForwardPartialUsageServiceForTest(upstream)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			_, err = svc.Forward(context.Background(), c, newAnthropicOAuthAccountForPartialUsageTest(), parsed)
			require.NoError(t, err)
			require.NoError(t, validateClaude55Request(upstream.lastBody, model))
			wantType := "between_tools"
			if model == "claude-opus-5-5" {
				wantType = "adaptive"
				require.Equal(t, "low", gjson.GetBytes(upstream.lastBody, "output_config.effort").String())
			}
			require.Equal(t, wantType, gjson.GetBytes(upstream.lastBody, "thinking.type").String())
			require.Equal(t, int64(256), gjson.GetBytes(upstream.lastBody, "max_tokens").Int())
		})
	}
}

func TestClaude55LegacyPolicyPreservesIngressModel(t *testing.T) {
	body := []byte(`{"model":"public-claude","max_tokens":256,"messages":[],"thinking":{"type":"enabled","budget_tokens":32768}}`)
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
	require.NoError(t, err)
	// A group/channel mapping rewrites the model before account selection.
	attempt, err := parsed.CloneForBody([]byte(strings.ReplaceAll(string(body), "public-claude", "internal-claude")))
	require.NoError(t, err)
	group := &Group{ID: 55, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true,
		ReasoningEffortMappings: []ReasoningEffortMapping{{From: "max", To: "deny", MatchType: "exact", Model: "public-claude"}}}
	attempt.GroupID = &group.ID
	svc := &GatewayService{}
	ctx := WithGatewayPublicModel(svc.withGroupContext(context.Background(), group), "public-claude")
	_, err = svc.normalizeClaude55LegacyRequest(ctx, attempt, "claude-opus-5-5")
	require.True(t, IsReasoningEffortPolicyDenied(err))
	require.Equal(t, "internal-claude", attempt.Model)
}
