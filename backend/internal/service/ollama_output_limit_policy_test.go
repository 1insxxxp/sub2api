//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOllamaOutputLimitsUnchangedWithoutExplicitCap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "responses", "responses fallback", "messages", "gateway messages", "passthrough messages"} {
		for _, tc := range []struct {
			name     string
			limit    int
			explicit bool
		}{
			{name: "missing"},
			{name: "explicit zero", explicit: true},
			{name: "explicit negative", limit: -1, explicit: true},
			{name: "explicit small", limit: 8, explicit: true},
			{name: "explicit large", limit: 256000, explicit: true},
		} {
			t.Run(protocol+"/"+tc.name, func(t *testing.T) {
				account := ollamaUpstreamTestAccount(PlatformDeepseek, 901)
				delete(account.Extra, OllamaCloudMaxTokensCapExtraKey)
				account.Credentials["base_url"] = "https://ollama.com"
				payload := map[string]any{"model": "deepseek-v4-flash", "stream": false}
				field := "max_tokens"
				if protocol == "responses" || protocol == "responses fallback" {
					payload["input"] = "hello"
					field = "max_output_tokens"
				} else {
					payload["messages"] = []map[string]string{{"role": "user", "content": "hello"}}
				}
				if tc.explicit {
					payload[field] = tc.limit
				}
				body, err := json.Marshal(payload)
				require.NoError(t, err)
				upstream := &httpUpstreamRecorder{err: errors.New("stop after capturing the outbound request")}
				svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
				var wire []byte
				switch protocol {
				case "chat":
					account.Credentials["api_protocol"] = APIProtocolChatCompletions
					_, err = svc.forwardAsRawChatCompletions(context.Background(), adaptiveProtocolTestContext("/v1/chat/completions", body), account, body, "")
					var failover *UpstreamFailoverError
					require.ErrorAs(t, err, &failover)
					require.Equal(t, http.StatusBadGateway, failover.StatusCode)
					require.NotEmpty(t, upstream.lastBody)
					wire = upstream.lastBody
				case "responses", "responses fallback":
					account.Credentials["api_protocol"] = APIProtocolResponses
					if protocol == "responses fallback" {
						account.Credentials["api_protocol"] = APIProtocolChatCompletions
						field = "max_completion_tokens"
					}
					_, err = svc.Forward(context.Background(), adaptiveProtocolTestContext("/v1/responses", body), account, body)
					var failover *UpstreamFailoverError
					require.ErrorAs(t, err, &failover)
					require.Equal(t, http.StatusBadGateway, failover.StatusCode)
					require.NotEmpty(t, upstream.lastBody)
					wire = upstream.lastBody
				case "messages":
					account.Credentials["api_protocol"] = APIProtocolAnthropic
					target, targetErr := svc.nativeAnthropicTargetURL(account)
					require.NoError(t, targetErr)
					_, wire, err = svc.buildNativeAnthropicUpstreamRequest(context.Background(), newMessagesClampTestContext(t), account, body, "test-key", target)
					require.NoError(t, err)
				case "gateway messages":
					account.Platform = PlatformAnthropic
					gateway := &GatewayService{cfg: messagesClampTestConfig()}
					_, wire, err = gateway.buildUpstreamRequest(context.Background(), newMessagesClampTestContext(t), account, body, "test-key", "api_key", "deepseek-v4-flash", false, false)
					require.NoError(t, err)
				case "passthrough messages":
					account.Platform = PlatformAnthropic
					gateway := &GatewayService{cfg: messagesClampTestConfig()}
					_, wire, err = gateway.buildUpstreamRequestAnthropicAPIKeyPassthrough(context.Background(), newMessagesClampTestContext(t), account, body, "test-key")
					require.NoError(t, err)
				}
				outLimit := gjson.GetBytes(wire, field)
				require.Equal(t, tc.explicit, outLimit.Exists(), "preserve the presence of an explicit client limit")
				require.Equal(t, int64(tc.limit), outLimit.Int(), "the relay must not silently reduce a client limit")
			})
		}
	}
}

func TestOllamaInvalidCapDoesNotCreateOutputLimit(t *testing.T) {
	for _, cap := range []any{nil, "65535", true, json.Number("invalid"), 1.5, math.NaN(), math.Inf(1)} {
		account := ollamaMaxTokensCapTestAccount(902, cap)
		body := []byte(`{"model":"deepseek-v4-flash","max_tokens":256000}`)
		require.Equal(t, body, clampOllamaCloudUpstreamMaxTokens(account, body))
	}
}
