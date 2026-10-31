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
				}{
					{name: "known model maximum", upstreamModel: "claude-opus-4-6", want: 128000},
					{name: "older model maximum", upstreamModel: "claude-sonnet-4-5", want: 64000},
					{name: "selected account metadata wins", upstreamModel: "claude-opus-4-6", metadata: map[string]UpstreamModelMetadata{"claude-opus-4-6": {ID: "claude-opus-4-6", MaxOutputTokens: 96000}}, want: 96000},
					{name: "public alias metadata does not win", upstreamModel: "claude-opus-4-6", metadata: map[string]UpstreamModelMetadata{"public-alias": {ID: "public-alias", MaxOutputTokens: 512}}, want: 128000},
					{name: "unknown compatible model metadata", upstreamModel: "compatible-private-model", metadata: map[string]UpstreamModelMetadata{"compatible-private-model": {ID: "compatible-private-model", MaxOutputTokens: 48000}}, want: 48000},
					{name: "unknown compatible model legacy fallback", upstreamModel: "compatible-private-model", want: 8192},
					{name: "explicit client limit wins", upstreamModel: "claude-opus-4-6", limit: 8192, metadata: map[string]UpstreamModelMetadata{"claude-opus-4-6": {ID: "claude-opus-4-6", MaxOutputTokens: 4096}}, want: 8192},
					{name: "explicit completion limit wins", upstreamModel: "claude-opus-4-6", limitField: "max_completion_tokens", limit: 16384, want: 16384},
				} {
					t.Run(fmt.Sprintf("%s/%s/stream=%t/%s", adapter, endpoint, stream, tc.name), func(t *testing.T) {
						request := map[string]any{"model": "public-alias", "stream": stream}
						if endpoint == "responses" {
							request["input"] = "hello"
							if tc.limit > 0 {
								request["max_output_tokens"] = tc.limit
							}
						} else {
							request["messages"] = []map[string]string{{"role": "user", "content": "hello"}}
							if tc.limit > 0 {
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
						require.Equal(t, int64(tc.want), gjson.GetBytes(upstream.lastBody, "max_tokens").Int())
						require.Equal(t, originalBody, string(body), "synthetic defaults must not change ingress estimation or continuation budgets")
					})
				}
			}
		}
	}
}
