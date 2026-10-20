//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDownstreamOutputTokenCollectorFreezeCancelsUpstreamOnce(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	var calls atomic.Int32
	collector.bindUpstreamCancel(func() {
		calls.Add(1)
	})

	collector.Freeze()
	collector.Freeze()

	require.Equal(t, int32(1), calls.Load())
}

func TestDownstreamOutputTokenCollectorLateCancelBindingAfterFreeze(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.Freeze()
	var calls atomic.Int32

	collector.bindUpstreamCancel(func() {
		calls.Add(1)
	})

	require.Equal(t, int32(1), calls.Load())
}

func TestDownstreamOutputTokenCollectorFreezesAfterClientCancellation(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"before\"}}\n\n"))

	collector.Freeze()
	collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"after\"}}\n\n"))

	if got := collector.VisibleText(); got != "before" {
		t.Fatalf("visible text after freeze = %q, want %q", got, "before")
	}
}

func TestAttachedDownstreamCollectorFreezesWhenRequestContextIsCanceled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	requestContext, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil).WithContext(requestContext)

	collector, restore := AttachDownstreamOutputTokenCollector(c, "claude-opus-4-6")
	defer restore()
	_, err := fmt.Fprint(c.Writer, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"before\"}}\n\n")
	require.NoError(t, err)

	cancel()
	_, err = fmt.Fprint(c.Writer, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"after\"}}\n\n")
	require.NoError(t, err)
	require.True(t, collector.Frozen())
	require.Equal(t, "before", collector.VisibleText())
}

func TestAttachedDownstreamCollectorFreezeCancelsForwardContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)

	collector, restore := AttachDownstreamOutputTokenCollector(c, "claude-opus-4-6")
	defer restore()
	forwardContext := c.Request.Context()

	collector.Freeze()

	select {
	case <-forwardContext.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("forward context was not canceled after downstream freeze")
	}
}

func TestDownstreamCollectorUsesSSEEventNameWhenPayloadOmitsType(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gpt-5.6-terra")
	collector.ObserveWritten([]byte("event: response.output_text.delta\ndata: {\"delta\":\"visible\"}\n\n"))
	require.Equal(t, "visible", collector.VisibleText())
}

func TestDownstreamCollectorParsesCRLFSSEFramesAcrossWrites(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.ObserveWritten([]byte("event: content_block_delta\r\ndata: {\"delta\":{\"type\":\"text_delta\",\"text\":\"visible\"}}\r"))
	collector.ObserveWritten([]byte("\n\r\n"))
	require.Equal(t, "visible", collector.VisibleText())
}

func TestDownstreamCollectorCountsOpenAIResponsesFunctionArguments(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gpt-5.6-terra")
	collector.ObserveWritten([]byte("event: response.function_call_arguments.delta\ndata: {\"type\":\"response.function_call_arguments.delta\",\"delta\":\"{\\\"query\\\":\\\"status\\\"}\"}\n\n"))

	require.Equal(t, `{"query":"status"}`, collector.VisibleText())
	require.Greater(t, collector.TokenCount(), 0)
}

func TestDownstreamCollectorCountsOpenAIResponsesVisibleReasoningAndCustomToolInput(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gpt-5.6-terra")
	collector.ObserveWritten([]byte("data: {\"type\":\"response.reasoning_summary_text.delta\",\"delta\":\"checking facts\"}\n\n"))
	collector.ObserveWritten([]byte("data: {\"type\":\"response.custom_tool_call_input.delta\",\"delta\":\"patch body\"}\n\n"))

	require.Equal(t, "checking factspatch body", collector.VisibleText())
	require.Greater(t, collector.TokenCount(), 0)
}

func TestDownstreamCollectorCountsOpenAIChatToolArguments(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gpt-5.6-terra")
	collector.ObserveWritten([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"function\":{\"arguments\":\"{\\\"path\\\":\\\"README.md\\\"}\"}}]}}]}\n\n"))

	require.Equal(t, `{"path":"README.md"}`, collector.VisibleText())
	require.Greater(t, collector.TokenCount(), 0)
}

func TestDownstreamCollectorCountsOpenAIChatVisibleReasoning(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gpt-5.6-terra")
	collector.ObserveWritten([]byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"checking facts\"}}]}\n\n"))

	require.Equal(t, "checking facts", collector.VisibleText())
	require.Greater(t, collector.TokenCount(), 0)
}

func TestDownstreamCollectorCountsAnthropicToolInput(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"query\\\":\\\"status\\\"}\"}}\n\n"))

	require.Equal(t, `{"query":"status"}`, collector.VisibleText())
	require.Greater(t, collector.TokenCount(), 0)
}

func TestDownstreamCollectorCountsVisibleReasoningButNotSignature(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"checking facts\"}}\n\n"))
	collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"signature_delta\",\"signature\":\"opaque-signature\"}}\n\n"))

	require.Equal(t, "checking facts", collector.VisibleText())
	require.Greater(t, collector.TokenCount(), 0)
}

func TestDownstreamCollectorCountsOnlyNonStreamingJSONOutput(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "anthropic",
			payload: `{"type":"message","id":"not-output","model":"not-output","content":[{"type":"text","text":"answer"},{"type":"thinking","thinking":"checking","signature":"opaque"},{"type":"tool_use","id":"opaque","name":"lookup","input":{"q":"status"}},{"type":"redacted_thinking","data":"opaque"}],"usage":{"input_tokens":0,"output_tokens":0}}`,
			want:    `answerchecking{"q":"status"}`,
		},
		{
			name:    "chat completions",
			payload: `{"object":"chat.completion","choices":[{"message":{"role":"assistant","content":"answer","reasoning_content":"checking","tool_calls":[{"id":"opaque","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"status\"}"}}]}}],"usage":{"prompt_tokens":0,"completion_tokens":0}}`,
			want:    `answerchecking{"q":"status"}`,
		},
		{
			name:    "chat text parts",
			payload: `{"choices":[{"message":{"content":[{"type":"text","text":"answer"},{"type":"image_url","image_url":{"url":"opaque"}}],"function_call":{"name":"lookup","arguments":"{\"q\":\"status\"}"}}}]}`,
			want:    `answer{"q":"status"}`,
		},
		{
			name:    "responses",
			payload: `{"object":"response","status":"completed","error":null,"output":[{"type":"message","content":[{"type":"output_text","text":"answer"}]},{"type":"reasoning","summary":[{"type":"summary_text","text":"checking"}],"encrypted_content":"opaque"},{"type":"function_call","name":"lookup","arguments":"{\"q\":\"status\"}"},{"type":"custom_tool_call","name":"patch","input":"patch body"}]}`,
			want:    `answerchecking{"q":"status"}patch body`,
		},
		{
			name:    "gemini",
			payload: `{"candidates":[{"content":{"parts":[{"text":"answer","thoughtSignature":"opaque"}]}}],"usageMetadata":{"promptTokenCount":0,"candidatesTokenCount":0}}`,
			want:    "answer",
		},
		{name: "error", payload: `{"type":"error","error":{"message":"not output"},"content":[{"type":"text","text":"not output"}]}`},
		{name: "failed response", payload: `{"object":"response","status":"failed","output":[{"type":"message","content":[{"type":"output_text","text":"not output"}]}]}`},
		{name: "protocol metadata", payload: `{"id":"opaque","model":"opaque","usage":{"output_tokens":999},"signature":"opaque"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
			collector.ObserveWritten([]byte(tc.payload))
			require.Equal(t, tc.want, collector.VisibleText())
			if tc.want == "" {
				require.Zero(t, collector.TokenCount())
			} else {
				require.Positive(t, collector.TokenCount())
			}
		})
	}
}

func TestAttachedDownstreamCollectorCountsJSONOnlyAfterCompleteSuccessfulWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, status := range []int{http.StatusOK, http.StatusBadGateway} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			collector, restore := AttachDownstreamOutputTokenCollector(c, "claude-opus-4-6")
			defer restore()
			c.Status(status)
			_, err := c.Writer.WriteString("{\n\n\"type\":\"message\",\"content\":[{\"type\":\"text\",\"text\":\"answer\"}]")
			require.NoError(t, err)
			require.Zero(t, collector.TokenCount())
			_, err = c.Writer.Write([]byte("}"))
			require.NoError(t, err)
			if status == http.StatusOK {
				require.Equal(t, "answer", collector.VisibleText())
				first := collector.TokenCount()
				require.Positive(t, first)
				require.Equal(t, first, collector.TokenCount(), "reading the snapshot must not count JSON twice")
			} else {
				require.Zero(t, collector.TokenCount(), "an HTTP failure is not model output")
			}
		})
	}
}

func TestNonStreamingAnthropicAdaptersMissingUsageBillDeliveredJSONOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "responses", "native", "passthrough"} {
		t.Run(protocol, func(t *testing.T) {
			body := []byte(`{"model":"claude-sonnet-4-5","stream":false,"max_tokens":128,"messages":[{"role":"user","content":"Explain HTTP streaming."}]}`)
			path := "/v1/messages"
			if protocol == "chat" {
				path = "/v1/chat/completions"
			} else if protocol == "responses" {
				path = "/v1/responses"
				body = []byte(`{"model":"claude-sonnet-4-5","stream":false,"max_output_tokens":128,"input":"Explain HTTP streaming."}`)
			}
			upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"id":"zero-usage","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"HTTP streams deliver the response incrementally."}],"stop_reason":"end_turn","usage":{"input_tokens":0,"output_tokens":0}}`)),
			}}
			if protocol == "chat" || protocol == "responses" {
				// These adapters always request an upstream SSE response, even
				// when their client requested a buffered JSON response.
				upstream.resp.Header.Set("Content-Type", "text/event-stream")
				upstream.resp.Body = io.NopCloser(strings.NewReader(
					"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"zero-usage\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-5\",\"content\":[],\"usage\":{\"input_tokens\":0,\"output_tokens\":0}}}\n\n" +
						"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
						"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"HTTP streams deliver the response incrementally.\"}}\n\n" +
						"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
						"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":0}}\n\n" +
						"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
			}
			svc := &GatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, tlsFPProfileService: &TLSFingerprintProfileService{}}
			account := newAnthropicAPIKeyAccountForTest()
			account.Extra["anthropic_passthrough"] = protocol == "passthrough"
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
			collector, restore := AttachDownstreamOutputTokenCollector(c, "claude-sonnet-4-5")
			defer restore()
			var result *ForwardResult
			var err error
			switch protocol {
			case "chat":
				result, err = svc.ForwardAsChatCompletions(c.Request.Context(), c, account, body, nil)
			case "responses":
				result, err = svc.ForwardAsResponses(c.Request.Context(), c, account, body, nil)
			default:
				parsed, parseErr := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
				require.NoError(t, parseErr)
				result, err = svc.Forward(c.Request.Context(), c, account, parsed)
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			ApplyDeliveredOutputTokens(result, collector)
			require.Positive(t, recordedOutputTokens(result))
			require.Zero(t, result.Usage.OutputTokens, "retain the raw provider measurement")
		})
	}
}

func TestCompletedStreamRecordsProviderOutputTokens(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"visible\"}}\n\n"))
	result := &ForwardResult{Usage: ClaudeUsage{OutputTokens: 321}}

	ApplyDeliveredOutputTokens(result, collector)

	require.Equal(t, 321, result.Usage.OutputTokens)
	require.NotNil(t, result.DeliveredOutputTokens)
	require.Equal(t, collector.TokenCount(), *result.DeliveredOutputTokens)
	require.Equal(t, 321, recordedOutputTokens(result))
	require.Equal(t, 321, customerBillableOutputTokens(result.ClientDisconnect, result.DeliveredOutputTokens, result.Usage.OutputTokens))
}

func TestCompletedStreamWithoutProviderOutputUsesDeliveredTokens(t *testing.T) {
	for _, disconnected := range []bool{false, true} {
		t.Run(fmt.Sprintf("client_disconnect_%t", disconnected), func(t *testing.T) {
			collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
			collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"The answer reached the client.\"}}\n\n"))
			delivered := collector.TokenCount()
			require.Greater(t, delivered, 0)
			if disconnected {
				collector.Freeze()
			}

			t.Run("anthropic", func(t *testing.T) {
				result := &ForwardResult{Outcome: &ResponseOutcome{StreamCompleted: true}}
				ApplyDeliveredOutputTokens(result, collector)
				require.Equal(t, delivered, recordedOutputTokens(result))
				require.Equal(t, delivered, customerBillingForwardResult(result).Usage.OutputTokens)
				require.Zero(t, result.Usage.OutputTokens, "keep the upstream usage separate from the billing estimate")
			})

			t.Run("openai", func(t *testing.T) {
				result := &OpenAIForwardResult{Outcome: &ResponseOutcome{StreamCompleted: true}}
				ApplyDeliveredOpenAIOutputTokens(result, collector)
				require.Equal(t, delivered, recordedOpenAIOutputTokens(result))
				require.Zero(t, result.Usage.OutputTokens)
			})
		})
	}
}

func TestCompletedStreamWithoutDeliveredOutputPreservesZero(t *testing.T) {
	for _, delivered := range []*int{nil, new(int)} {
		result := &ForwardResult{
			DeliveredOutputTokens: delivered,
			Outcome:               &ResponseOutcome{StreamCompleted: true},
		}
		require.Zero(t, recordedOutputTokens(result))

		openAIResult := &OpenAIForwardResult{
			DeliveredOutputTokens: delivered,
			Outcome:               &ResponseOutcome{StreamCompleted: true},
		}
		require.Zero(t, recordedOpenAIOutputTokens(openAIResult))
	}
}

func TestMissingProviderOutputUsesDeliveredTokensWithoutDisconnect(t *testing.T) {
	delivered := 7
	result := &ForwardResult{DeliveredOutputTokens: &delivered}
	openAIResult := &OpenAIForwardResult{DeliveredOutputTokens: &delivered}

	require.Equal(t, delivered, recordedOutputTokens(result))
	require.Equal(t, delivered, recordedOpenAIOutputTokens(openAIResult))
	require.Equal(t, delivered, customerBillableOutputTokens(false, &delivered, 0))
}

func TestApplyDeliveredOutputTokensMarksFrozenCollectorAsClientDisconnect(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"visible\"}}\n\n"))
	collector.Freeze()
	result := &ForwardResult{Usage: ClaudeUsage{OutputTokens: 321}}

	ApplyDeliveredOutputTokens(result, collector)

	require.True(t, result.ClientDisconnect)
	require.NotNil(t, result.DeliveredOutputTokens)
}

func TestApplyDeliveredOpenAIOutputTokensMarksFrozenCollectorAsClientDisconnect(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gpt-5.1")
	collector.ObserveWritten([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"visible\"}\n\n"))
	collector.Freeze()
	result := &OpenAIForwardResult{Usage: OpenAIUsage{OutputTokens: 321}}

	ApplyDeliveredOpenAIOutputTokens(result, collector)

	require.True(t, result.ClientDisconnect)
	require.NotNil(t, result.DeliveredOutputTokens)
}

func TestUsageLogSeparatesDeliveredAndProviderOutputTokens(t *testing.T) {
	delivered := 7
	result := &ForwardResult{
		RequestID:             "delivered-output-test",
		Model:                 "claude-opus-4-6",
		Stream:                true,
		ClientDisconnect:      true,
		DeliveredOutputTokens: &delivered,
		Usage:                 ClaudeUsage{InputTokens: 10, OutputTokens: 321},
	}
	groupID := int64(1)
	usageLog := (&GatewayService{}).buildRecordUsageLog(
		context.Background(),
		&recordUsageCoreInput{},
		result,
		&APIKey{ID: 1, GroupID: &groupID, Group: &Group{ID: groupID}},
		&User{ID: 2},
		&Account{ID: 3},
		nil,
		result.Model,
		1,
		1,
		1,
		BillingTypeBalance,
		false,
		&CostBreakdown{},
		&recordUsageOpts{},
	)

	require.Equal(t, 7, usageLog.OutputTokens)
	require.Equal(t, 321, usageLog.UpstreamOutputTokens)
}

func TestCompletedUsageLogUsesProviderOutputTokens(t *testing.T) {
	delivered := 7
	result := &ForwardResult{
		RequestID:             "completed-output-test",
		Model:                 "claude-opus-4-6",
		Stream:                true,
		DeliveredOutputTokens: &delivered,
		Usage:                 ClaudeUsage{InputTokens: 10, OutputTokens: 321},
	}
	groupID := int64(1)
	usageLog := (&GatewayService{}).buildRecordUsageLog(
		context.Background(),
		&recordUsageCoreInput{},
		result,
		&APIKey{ID: 1, GroupID: &groupID, Group: &Group{ID: groupID}},
		&User{ID: 2},
		&Account{ID: 3},
		nil,
		result.Model,
		1,
		1,
		1,
		BillingTypeBalance,
		false,
		&CostBreakdown{},
		&recordUsageOpts{},
	)

	require.Equal(t, 321, usageLog.OutputTokens)
	require.Equal(t, 321, usageLog.UpstreamOutputTokens)
}

func TestCompletedOpenAIStreamRecordsProviderOutputTokens(t *testing.T) {
	delivered := 7
	result := &OpenAIForwardResult{
		DeliveredOutputTokens: &delivered,
		Usage:                 OpenAIUsage{OutputTokens: 321},
	}

	require.Equal(t, 321, recordedOpenAIOutputTokens(result))
	require.Equal(t, 321, customerBillableOutputTokens(result.ClientDisconnect, result.DeliveredOutputTokens, result.Usage.OutputTokens))
}

func TestCustomerBillableOutputTokensUsesDeliveredOnlyAfterDisconnect(t *testing.T) {
	delivered := 1935

	require.Equal(t, 1935, customerBillableOutputTokens(true, &delivered, 13811))
	require.Equal(t, 13811, customerBillableOutputTokens(false, &delivered, 13811))
	require.Equal(t, 13811, customerBillableOutputTokens(true, nil, 13811))
}

func TestCompletedUpstreamUsageWinsOverDeliveredTokensAfterDisconnect(t *testing.T) {
	delivered := 1935
	result := &OpenAIForwardResult{
		ClientDisconnect:      true,
		DeliveredOutputTokens: &delivered,
		Usage:                 OpenAIUsage{OutputTokens: 13811},
		Outcome:               &ResponseOutcome{StreamCompleted: true},
	}

	require.Equal(t, 13811, recordedOpenAIOutputTokens(result))

	claudeResult := &ForwardResult{
		ClientDisconnect:      true,
		DeliveredOutputTokens: &delivered,
		Usage:                 ClaudeUsage{OutputTokens: 13811},
		Outcome:               &ResponseOutcome{StreamCompleted: true},
	}
	require.Equal(t, 13811, recordedOutputTokens(claudeResult))
}

func TestIncompleteUpstreamUsageFallsBackToDeliveredTokensWithoutDisconnect(t *testing.T) {
	delivered := 1935
	result := &OpenAIForwardResult{
		DeliveredOutputTokens: &delivered,
		Usage:                 OpenAIUsage{OutputTokens: 13811},
		Outcome:               &ResponseOutcome{StreamCompleted: false},
	}

	require.Equal(t, 1935, recordedOpenAIOutputTokens(result))
}
