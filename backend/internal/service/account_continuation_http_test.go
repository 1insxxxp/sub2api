//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAccountContinuationOpenAIChatHTTP(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run("stream="+boolString(stream), func(t *testing.T) {
			var responses []*http.Response
			for i, text := range []string{"alpha", "beta"} {
				reason := "length"
				if i == 1 {
					reason = "stop"
				}
				payload := fmt.Sprintf(`{"id":"round-%d","object":"chat.completion","model":"long-text","choices":[{"index":0,"message":{"role":"assistant","content":%q},"finish_reason":%q}],"usage":{"prompt_tokens":100,"completion_tokens":5,"total_tokens":105}}`, i, text, reason)
				ct := "application/json"
				if stream {
					payload = strings.ReplaceAll(payload, `"message":`, `"delta":`)
					payload = "data: " + payload + "\n\ndata: [DONE]\n\n"
					ct = "text/event-stream"
				}
				responses = append(responses, &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {ct}, "X-Request-Id": {fmt.Sprintf("req-%d", i)}}, Body: io.NopCloser(strings.NewReader(payload))})
			}
			upstream := &httpUpstreamRecorder{responses: responses}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			account := rawChatCompletionsTestAccount()
			account.Extra = accountContinuationTestAccount(true).Extra
			account.Extra["openai_responses_supported"] = false
			body := []byte(`{"model":"long-text","stream":` + boolString(stream) + `,"messages":[{"role":"user","content":"write"}]}`)
			c, rec := newGeminiOutputLimitContext(body)
			result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
			require.NoError(t, err)
			require.Len(t, upstream.bodies, 2)
			require.Equal(t, 200, result.Usage.InputTokens)
			require.Equal(t, 10, result.Usage.OutputTokens)
			require.Len(t, result.ContinuationResults, 2)
			require.Contains(t, rec.Body.String(), "alpha")
			require.Contains(t, rec.Body.String(), "beta")
			require.False(t, gjson.GetBytes(upstream.bodies[0], "max_tokens").Exists())
		})
	}
}

func TestAccountContinuationOpenAIResponsesHTTPIdentity(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run("stream="+boolString(stream), func(t *testing.T) {
			var responses []*http.Response
			for i, text := range []string{"alpha", "beta"} {
				status, kind, details := "incomplete", "response.incomplete", `,"incomplete_details":{"reason":"max_output_tokens"}`
				if i == 1 {
					status, kind, details = "completed", "response.completed", ""
				}
				response := fmt.Sprintf(`{"id":"resp-%d","object":"response","model":"gpt-5.4","status":%q%s,"output":[{"id":"msg-%d","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":%q}]}],"usage":{"input_tokens":100,"output_tokens":5,"total_tokens":105}}`, i, status, details, i, text)
				ct, payload := "application/json", response
				if stream {
					ct = "text/event-stream"
					payload = fmt.Sprintf("data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp-%d\",\"status\":\"in_progress\"}}\n\ndata: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg-%d\",\"output_index\":0,\"content_index\":0,\"delta\":%q}\n\ndata: {\"type\":%q,\"response\":%s}\n\n", i, i, text, kind, response)
				}
				responses = append(responses, &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {ct}, "X-Request-Id": {fmt.Sprintf("req-%d", i)}}, Body: io.NopCloser(strings.NewReader(payload))})
			}
			upstream := &httpUpstreamRecorder{responses: responses}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			account := rawChatCompletionsTestAccount()
			account.Extra = accountContinuationTestAccount(true).Extra
			account.Extra["openai_responses_supported"] = true
			body := []byte(`{"model":"long-text","stream":` + boolString(stream) + `,"input":"write"}`)
			c, rec := newGeminiOutputLimitContext(body)
			SetOpenAIHTTPResponseOwner(c, 81, 91)
			result, err := svc.Forward(context.Background(), c, account, body)
			require.NoError(t, err)
			require.Len(t, upstream.bodies, 2)
			require.Equal(t, 200, result.Usage.InputTokens)
			require.Equal(t, 10, result.Usage.OutputTokens)
			require.Equal(t, "resp-0", result.ContinuationPublicResponseID)
			require.Contains(t, rec.Body.String(), "alpha")
			require.Contains(t, rec.Body.String(), "beta")
			resolved, err := svc.ResolveOpenAIContinuationAlias(context.Background(), 0, 81, "resp-0")
			require.NoError(t, err)
			require.Equal(t, "resp-1", resolved)
		})
	}
}

func TestAccountContinuationOpenAIMessagesHTTP(t *testing.T) {
	for _, upstreamProtocol := range []string{APIProtocolChatCompletions, APIProtocolResponses, APIProtocolAnthropic} {
		for _, stream := range []bool{false, true} {
			t.Run(upstreamProtocol+"/stream="+boolString(stream), func(t *testing.T) {
				var responses []*http.Response
				for i, text := range []string{"alpha", "beta"} {
					last := i == 1
					ct, payload := "application/json", ""
					switch upstreamProtocol {
					case APIProtocolChatCompletions:
						reason := "length"
						if last {
							reason = "stop"
						}
						payload = fmt.Sprintf(`{"id":"round-%d","model":"long-text","choices":[{"index":0,"message":{"role":"assistant","content":%q},"finish_reason":%q}],"usage":{"prompt_tokens":100,"completion_tokens":5}}`, i, text, reason)
						if stream {
							ct = "text/event-stream"
							payload = "data: " + strings.ReplaceAll(payload, `"message":`, `"delta":`) + "\n\ndata: [DONE]\n\n"
						}
					case APIProtocolResponses:
						status, kind, details := "incomplete", "response.incomplete", `,"incomplete_details":{"reason":"max_output_tokens"}`
						if last {
							status, kind, details = "completed", "response.completed", ""
						}
						response := fmt.Sprintf(`{"id":"resp-%d","object":"response","model":"gpt-5.4","status":%q%s,"output":[{"id":"msg-%d","type":"message","role":"assistant","content":[{"type":"output_text","text":%q}]}],"usage":{"input_tokens":100,"output_tokens":5}}`, i, status, details, i, text)
						ct = "text/event-stream"
						payload = fmt.Sprintf("data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp-%d\",\"status\":\"in_progress\"}}\n\ndata: {\"type\":\"response.output_text.delta\",\"output_index\":0,\"content_index\":0,\"delta\":%q}\n\ndata: {\"type\":%q,\"response\":%s}\n\n", i, text, kind, response)
					case APIProtocolAnthropic:
						reason := "max_tokens"
						if last {
							reason = "end_turn"
						}
						payload = fmt.Sprintf(`{"id":"msg-%d","type":"message","role":"assistant","model":"long-text","content":[{"type":"text","text":%q}],"stop_reason":%q,"usage":{"input_tokens":100,"output_tokens":5}}`, i, text, reason)
						if stream {
							ct, payload = "text/event-stream", anthropicTextStream(text, reason, 100, 5)
						}
					}
					responses = append(responses, &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {ct}}, Body: io.NopCloser(strings.NewReader(payload))})
				}
				account := rawChatCompletionsTestAccount()
				switch upstreamProtocol {
				case APIProtocolChatCompletions:
					account = forceChatMessagesFallbackAccount()
				case APIProtocolAnthropic:
					account = nativeAnthropicTestAccount()
				}
				if account.Extra == nil {
					account.Extra = make(map[string]any)
				}
				account.Extra[AccountExtraAntiTruncation] = accountContinuationTestAccount(true).Extra[AccountExtraAntiTruncation]
				upstream := &httpUpstreamRecorder{responses: responses}
				svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
				body := []byte(`{"model":"long-text","max_tokens":100,"stream":` + boolString(stream) + `,"messages":[{"role":"user","content":"write"}]}`)
				c, rec := newGeminiOutputLimitContext(body)
				result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
				require.NoError(t, err)
				require.Len(t, upstream.bodies, 2)
				require.Equal(t, 200, result.Usage.InputTokens)
				require.Equal(t, 10, result.Usage.OutputTokens)
				require.Contains(t, rec.Body.String(), "alpha")
				require.Contains(t, rec.Body.String(), "beta")
			})
		}
	}
}

func TestAccountContinuationAnthropicHTTP(t *testing.T) {
	for _, mode := range []string{"messages", "chat", "responses"} {
		chat := mode == "chat"
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%v", mode, stream), func(t *testing.T) {
				var responses []*http.Response
				for i, text := range []string{"alpha", "beta"} {
					reason := "max_tokens"
					if i == 1 {
						reason = "end_turn"
					}
					ct, payload := "text/event-stream", anthropicTextStream(text, reason, 100, 5)
					if !stream && mode == "messages" {
						ct = "application/json"
						payload = fmt.Sprintf(`{"id":"msg-%d","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[{"type":"text","text":%q}],"stop_reason":%q,"usage":{"input_tokens":100,"output_tokens":5}}`, i, text, reason)
					}
					responses = append(responses, &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {ct}}, Body: io.NopCloser(strings.NewReader(payload))})
				}
				upstream := &httpUpstreamRecorder{responses: responses}
				svc := &GatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}, rateLimitService: &RateLimitService{}, deferredService: &DeferredService{}}
				account := newAnthropicAPIKeyAccountForTest()
				account.Extra["anti_truncation"] = accountContinuationTestAccount(true).Extra["anti_truncation"]
				account.Credentials["model_mapping"] = map[string]any{"long-text": "claude-sonnet-4-6"}
				body := []byte(`{"model":"long-text","max_tokens":100,"stream":` + boolString(stream) + `,"messages":[{"role":"user","content":"write"}]}`)
				if mode == "responses" {
					body = []byte(`{"model":"long-text","max_output_tokens":100,"stream":` + boolString(stream) + `,"input":"write"}`)
				}
				c, rec := newGeminiOutputLimitContext(body)
				var result *ForwardResult
				var err error
				if chat {
					result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body, nil)
				} else if mode == "responses" {
					result, err = svc.ForwardAsResponses(context.Background(), c, account, body, nil)
				} else {
					parsed, parseErr := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
					require.NoError(t, parseErr)
					result, err = svc.Forward(context.Background(), c, account, parsed)
				}
				require.NoError(t, err)
				require.Len(t, upstream.bodies, 2)
				require.Equal(t, 200, result.Usage.InputTokens)
				require.Equal(t, 10, result.Usage.OutputTokens)
				require.Equal(t, int64(95), gjson.GetBytes(upstream.bodies[1], "max_tokens").Int())
				require.Contains(t, rec.Body.String(), "alpha")
				require.Contains(t, rec.Body.String(), "beta")
			})
		}
	}
}

func TestAccountContinuationAntigravityCompatHTTP(t *testing.T) {
	for _, protocol := range []AccountContinuationProtocol{AccountContinuationChat, AccountContinuationResponses, AccountContinuationMessages, AccountContinuationGemini} {
		for _, stream := range []bool{false, true} {
			t.Run(string(protocol)+"/stream="+boolString(stream), func(t *testing.T) {
				var responses []*http.Response
				for i, text := range []string{"alpha", "beta"} {
					reason := "MAX_TOKENS"
					if i == 1 {
						reason = "STOP"
					}
					payload := fmt.Sprintf("data: {\"response\":{\"responseId\":\"round-%d\",\"candidates\":[{\"content\":{\"parts\":[{\"text\":%q}]},\"finishReason\":%q}],\"usageMetadata\":{\"promptTokenCount\":100,\"candidatesTokenCount\":5}}}\n\n", i, text, reason)
					responses = append(responses, &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(payload))})
				}
				upstream := &queuedHTTPUpstreamStub{responses: responses}
				svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)
				account := newAntigravityCompatAccount(AccountTypeOAuth)
				account.Extra = accountContinuationTestAccount(true).Extra
				account.Extra["anti_truncation"].(map[string]any)["rules"].([]any)[0].(map[string]any)["model"] = "gemini-3.1-pro-high"
				body := []byte(`{"model":"gemini-3.1-pro-high","stream":` + boolString(stream) + `,"messages":[{"role":"user","content":"write"}]}`)
				if protocol == AccountContinuationResponses {
					body = []byte(`{"model":"gemini-3.1-pro-high","stream":` + boolString(stream) + `,"input":"write"}`)
				}
				if protocol == AccountContinuationGemini {
					body = []byte(`{"contents":[{"role":"user","parts":[{"text":"write"}]}]}`)
				}
				c, rec := newGeminiOutputLimitContext(body)
				var result *ForwardResult
				var err error
				switch protocol {
				case AccountContinuationChat:
					result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body, nil)
				case AccountContinuationResponses:
					result, err = svc.ForwardAsResponses(context.Background(), c, account, body, nil)
				case AccountContinuationMessages:
					result, err = svc.Forward(context.Background(), c, account, body, false)
				case AccountContinuationGemini:
					action := "generateContent"
					if stream {
						action = "streamGenerateContent"
					}
					result, err = svc.ForwardGemini(context.Background(), c, account, "gemini-3.1-pro-high", action, stream, body, false)
				}
				require.NoError(t, err)
				require.Equal(t, 200, result.Usage.InputTokens)
				require.Equal(t, 10, result.Usage.OutputTokens)
				require.Len(t, result.ContinuationResults, 2)
				require.Contains(t, rec.Body.String(), "alpha")
				require.Contains(t, rec.Body.String(), "beta")
			})
		}
	}
}

// Use the real converters, HTTP forwarder and response parsers. Synthetic
// callbacks alone cannot establish that account rules reach the outbound path.
func TestAccountContinuationGeminiHTTPProtocols(t *testing.T) {
	for _, protocol := range []AccountContinuationProtocol{AccountContinuationChat, AccountContinuationMessages, AccountContinuationGemini} {
		for _, stream := range []bool{false, true} {
			t.Run(string(protocol)+"/stream="+boolString(stream), func(t *testing.T) {
				body := []byte(`{"model":"paygo-vet-4a","stream":` + boolString(stream) + `,"messages":[{"role":"user","content":"write"}]}`)
				if protocol == AccountContinuationMessages {
					body = []byte(`{"model":"paygo-vet-4a","stream":` + boolString(stream) + `,"max_tokens":100,"messages":[{"role":"user","content":"write"}]}`)
				} else if protocol == AccountContinuationGemini {
					body = []byte(`{"contents":[{"role":"user","parts":[{"text":"write"}]}]}`)
				}
				upstream := &continuationBudgetUpstream{responses: []*http.Response{
					geminiOutputLimitResponse(t, stream, "alpha", 5, "MAX_TOKENS"),
					geminiOutputLimitResponse(t, stream, "beta", 7, "STOP"),
				}}
				svc := &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: upstream}
				account := geminiOutputLimitAccount()
				account.Extra = accountContinuationTestAccount(true).Extra
				account.Extra["anti_truncation"].(map[string]any)["rules"].([]any)[0].(map[string]any)["model"] = "paygo-vet-4a"
				c, recorder := newGeminiOutputLimitContext(body)
				var result *ForwardResult
				var err error
				switch protocol {
				case AccountContinuationChat:
					result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body)
				case AccountContinuationMessages:
					result, err = svc.Forward(context.Background(), c, account, body)
				case AccountContinuationGemini:
					action := "generateContent"
					if stream {
						action = "streamGenerateContent"
					}
					result, err = svc.ForwardNative(context.Background(), c, account, "paygo-vet-4a", action, stream, body)
				}
				require.NoError(t, err)
				require.Len(t, upstream.bodies, 2)
				require.Equal(t, 400, result.Usage.InputTokens)
				require.Equal(t, 12, result.Usage.OutputTokens)
				require.Len(t, result.ContinuationResults, 2)
				require.Contains(t, string(upstream.bodies[1]), "alpha")
				require.Contains(t, recorder.Body.String(), "alpha")
				require.Contains(t, recorder.Body.String(), "beta")
				if protocol != AccountContinuationMessages {
					require.False(t, gjson.GetBytes(upstream.bodies[0], "generationConfig.maxOutputTokens").Exists())
				}
				if stream && protocol == AccountContinuationChat {
					require.Equal(t, 1, strings.Count(recorder.Body.String(), "[DONE]"))
				}
			})
		}
	}
}
