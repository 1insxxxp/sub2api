//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAnthropicBridgeDefaultOutputUsesSelectedAccountModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, adapter := range []string{"gateway", "native"} {
		for _, endpoint := range []string{"chat/completions", "responses"} {
			for _, stream := range []bool{false, true} {
				for _, tc := range []struct {
					name, upstreamModel string
					metadata            map[string]UpstreamModelMetadata
					limitField          string
					limit, want         int
					explicit            bool
				}{
					{name: "known model maximum", upstreamModel: "claude-opus-4-6", want: 128000},
					{name: "older model maximum", upstreamModel: "claude-sonnet-4-5", want: 64000},
					{name: "selected account metadata wins", upstreamModel: "claude-opus-4-6", metadata: map[string]UpstreamModelMetadata{"claude-opus-4-6": {ID: "claude-opus-4-6", MaxOutputTokens: 96000}}, want: 96000},
					{name: "public alias metadata does not win", upstreamModel: "claude-opus-4-6", metadata: map[string]UpstreamModelMetadata{"public-alias": {ID: "public-alias", MaxOutputTokens: 512}}, want: 128000},
					{name: "unknown compatible model metadata", upstreamModel: "compatible-private-model", metadata: map[string]UpstreamModelMetadata{"compatible-private-model": {ID: "compatible-private-model", MaxOutputTokens: 48000}}, want: 48000},
					{name: "unknown compatible model leaves limit omitted", upstreamModel: "compatible-private-model"},
					{name: "thinking suffix known maximum", upstreamModel: "claude-opus-4-6-thinking", want: 128000},
					{name: "namespace known maximum", upstreamModel: "a/claude-opus-4-6", want: 128000},
					{name: "anti prefix known maximum", upstreamModel: "anti-claude-opus-4-6", want: 128000},
					{name: "dotted version known maximum", upstreamModel: "claude-opus-4.6", want: 128000},
					{name: "cursor wrapper known maximum", upstreamModel: "[次cursor]claude-opus-4-6", want: 128000},
					{name: "cursor suffix known maximum", upstreamModel: "claude-opus-4-6[次cursor]", want: 128000},
					{name: "cu suffix known maximum", upstreamModel: "claude-opus-4-6-cu", want: 128000},
					{name: "combined provider modifiers", upstreamModel: "a/anti-claude-opus-4.6-thinking-cu[次cursor]", want: 128000},
					{name: "older decorated model maximum", upstreamModel: "anti-claude-sonnet-4.5-thinking-cu", want: 64000},
					{name: "bedrock decorated model maximum", upstreamModel: "us.anthropic.claude-opus-4-6-v1:0", want: 128000},
					{name: "vertex dated model maximum", upstreamModel: "claude-opus-4-6@20260205", want: 128000},
					{name: "wrapped account exact metadata wins", upstreamModel: "a/claude-opus-4-6-cu", metadata: map[string]UpstreamModelMetadata{"a/claude-opus-4-6-cu": {ID: "a/claude-opus-4-6-cu", MaxOutputTokens: 192000}}, want: 192000},
					{name: "unwrapped metadata cannot lower wrapped model", upstreamModel: "a/claude-opus-4-6-cu", metadata: map[string]UpstreamModelMetadata{"claude-opus-4-6": {ID: "claude-opus-4-6", MaxOutputTokens: 512}}, want: 128000},
					{name: "unknown suffix has no invented maximum", upstreamModel: "claude-opus-4-6-private"},
					{name: "unknown future model has no invented maximum", upstreamModel: "claude-opus-9-9"},
					{name: "explicit unknown model limit wins", upstreamModel: "compatible-private-model", limit: 262144, want: 262144},
					{name: "explicit larger than known capability wins", upstreamModel: "claude-opus-4-6-thinking", limit: 262144, want: 262144},
					{name: "explicit zero is not a default", upstreamModel: "claude-opus-4-6", explicit: true},
					{name: "explicit negative left for final validation", upstreamModel: "claude-opus-4-6", limit: -1, want: -1},
					{name: "explicit small never raised by intermediate floor", upstreamModel: "claude-opus-4-6", limit: 8, want: 8},
					{name: "explicit client limit wins", upstreamModel: "claude-opus-4-6", limit: 8192, metadata: map[string]UpstreamModelMetadata{"claude-opus-4-6": {ID: "claude-opus-4-6", MaxOutputTokens: 4096}}, want: 8192},
					{name: "explicit completion limit wins", upstreamModel: "claude-opus-4-6", limitField: "max_completion_tokens", limit: 16384, want: 16384},
				} {
					t.Run(fmt.Sprintf("%s/%s/stream=%t/%s", adapter, endpoint, stream, tc.name), func(t *testing.T) {
						request := map[string]any{"model": "public-alias", "stream": stream}
						if endpoint == "responses" {
							request["input"] = "hello"
							if tc.limit != 0 || tc.explicit {
								request["max_output_tokens"] = tc.limit
							}
						} else {
							request["messages"] = []map[string]string{{"role": "user", "content": "hello"}}
							if tc.limit != 0 || tc.explicit {
								field := tc.limitField
								if field == "" {
									field = "max_tokens"
								}
								request[field] = tc.limit
							}
						}
						body, err := json.Marshal(request)
						require.NoError(t, err)
						originalBody := string(body)
						c, _ := gin.CreateTestContext(httptest.NewRecorder())
						c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+endpoint, strings.NewReader(originalBody))
						upstream := &httpUpstreamRecorder{resp: nativeAnthropicStreamResponse()}
						account := newAnthropicAPIKeyAccountForTest()
						if adapter == "native" {
							account = nativeAnthropicTestAccount()
						}
						account.Credentials["model_mapping"] = map[string]any{"public-alias": tc.upstreamModel}
						if tc.metadata != nil {
							account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: tc.metadata})
						}
						if adapter == "gateway" {
							svc := &GatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}}
							if endpoint == "responses" {
								_, err = svc.ForwardAsResponses(context.Background(), c, account, body, nil)
							} else {
								_, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body, nil)
							}
						} else {
							svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
							if endpoint == "responses" {
								_, err = svc.forwardResponsesViaNativeAnthropic(context.Background(), c, account, body, "")
							} else {
								_, err = svc.forwardChatCompletionsViaNativeAnthropic(context.Background(), c, account, body, "")
							}
						}
						require.NoError(t, err)
						limit := gjson.GetBytes(upstream.lastBody, "max_tokens")
						require.Equal(t, tc.want != 0 || tc.explicit, limit.Exists(), "unknown capability must omit max_tokens rather than emit a synthetic limit or zero")
						require.Equal(t, int64(tc.want), limit.Int())
						require.Equal(t, tc.upstreamModel, gjson.GetBytes(upstream.lastBody, "model").String(), "capability lookup must preserve the actual upstream model")
						require.Equal(t, originalBody, string(body), "synthetic defaults must not change ingress estimation or continuation budgets")
					})
				}
			}
		}
	}
}
