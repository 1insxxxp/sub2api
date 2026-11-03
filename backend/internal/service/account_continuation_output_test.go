//go:build unit

package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func continuationTestOutput(t *testing.T, protocol AccountContinuationProtocol, stream bool) (*AccountContinuationOutput, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	return NewAccountContinuationOutput(ctx.Writer, protocol, stream), recorder
}

func TestAccountContinuationChatJoinsFragmentedStreamAndDelaysTerminal(t *testing.T) {
	out, recorder := continuationTestOutput(t, AccountContinuationChat, true)
	w := out.BeginRound()
	first := "data: {\"id\":\"one\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"part\"},\"finish_reason\":null}]}\n\n"
	for i := range first {
		_, err := w.Write([]byte(first[i : i+1]))
		require.NoError(t, err)
	}
	require.Contains(t, recorder.Body.String(), "part", "text must reach the client before the first round finishes")
	_, err := w.WriteString("data: {\"id\":\"one\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n")
	require.NoError(t, err)
	round, err := w.Finish()
	require.NoError(t, err)
	require.True(t, round.Truncated)
	require.Equal(t, "part", round.Text)
	require.NotContains(t, recorder.Body.String(), "length")
	w = out.BeginRound()
	_, err = w.WriteString("data: {\"id\":\"two\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" more\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	require.NoError(t, err)
	round, err = w.Finish()
	require.NoError(t, err)
	require.False(t, round.Truncated)
	require.NoError(t, out.Complete(AccountContinuationUsage{InputTokens: 30, OutputTokens: 7}))
	got := recorder.Body.String()
	require.Equal(t, 1, strings.Count(got, "[DONE]"))
	require.NotContains(t, got, `"id":"two"`)
	require.NotContains(t, got, "length")
	require.Contains(t, got, `"prompt_tokens":30`)
	require.Contains(t, got, `"completion_tokens":7`)
	require.Equal(t, 1, strings.Count(got, `"content":" more"`))
}

func TestAccountContinuationMessagesKeepsOneLifecycleAndDistinctBlocks(t *testing.T) {
	out, recorder := continuationTestOutput(t, AccountContinuationMessages, true)
	for i, reason := range []string{"max_tokens", "end_turn"} {
		w := out.BeginRound()
		stream := fmt.Sprintf("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"m%d\",\"role\":\"assistant\",\"content\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\nevent: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"text%d\"}}\n\nevent: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"%s\"},\"usage\":{\"output_tokens\":2}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", i, i, reason)
		_, err := w.WriteString(stream)
		require.NoError(t, err)
		round, err := w.Finish()
		require.NoError(t, err)
		require.Equal(t, i == 0, round.Truncated)
	}
	require.NoError(t, out.Complete(AccountContinuationUsage{InputTokens: 25, OutputTokens: 4}))
	got := recorder.Body.String()
	require.Equal(t, 1, strings.Count(got, "event: message_start"))
	require.Equal(t, 1, strings.Count(got, "event: message_stop"))
	require.NotContains(t, got, "max_tokens")
	require.Contains(t, got, `"index":1`)
	require.Contains(t, got, `"input_tokens":25`)
}

func TestAccountContinuationGeminiSplitsTextFromLimitTerminal(t *testing.T) {
	out, recorder := continuationTestOutput(t, AccountContinuationGemini, true)
	for _, value := range []string{
		`{"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"first"}]},"finishReason":"MAX_TOKENS"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":2}}`,
		`{"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"second"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":13,"candidatesTokenCount":3}}`,
	} {
		w := out.BeginRound()
		_, err := w.WriteString("data: " + value + "\n\n")
		require.NoError(t, err)
		round, err := w.Finish()
		require.NoError(t, err)
		require.NotEmpty(t, round.Text)
	}
	require.NoError(t, out.Complete(AccountContinuationUsage{InputTokens: 23, OutputTokens: 5}))
	got := recorder.Body.String()
	require.Equal(t, 1, strings.Count(got, `"text":"first"`))
	require.Equal(t, 1, strings.Count(got, `"text":"second"`))
	require.NotContains(t, got, "MAX_TOKENS")
	require.Equal(t, 1, strings.Count(got, "usageMetadata"))
	require.Contains(t, got, `"promptTokenCount":23`)
}

func TestAccountContinuationResponsesUnifiesIDsSequencesAndFinalOutput(t *testing.T) {
	out, recorder := continuationTestOutput(t, AccountContinuationResponses, true)
	for i, status := range []string{"incomplete", "completed"} {
		w := out.BeginRound()
		terminal := "response." + status
		stream := fmt.Sprintf("event: response.created\ndata: {\"type\":\"response.created\",\"sequence_number\":0,\"response\":{\"id\":\"r%d\",\"status\":\"in_progress\"}}\n\nevent: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"sequence_number\":1,\"output_index\":0,\"item\":{\"id\":\"item\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[]}}\n\nevent: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"sequence_number\":2,\"output_index\":0,\"item_id\":\"item\",\"delta\":\"text%d\"}\n\nevent: %s\ndata: {\"type\":\"%s\",\"sequence_number\":3,\"response\":{\"id\":\"r%d\",\"status\":\"%s\",\"incomplete_details\":{\"reason\":\"max_output_tokens\"},\"output\":[{\"id\":\"item\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"text%d\"}]}]}}\n\n", i, i, terminal, terminal, i, status, i)
		_, err := w.WriteString(stream)
		require.NoError(t, err)
		round, err := w.Finish()
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("r%d", i), round.ResponseID)
		require.Equal(t, i == 0, round.Truncated)
	}
	require.NoError(t, out.Complete(AccountContinuationUsage{InputTokens: 12, OutputTokens: 8}))
	got := recorder.Body.String()
	require.Equal(t, 1, strings.Count(got, "event: response.created"))
	require.NotContains(t, got, "event: response.incomplete")
	require.Equal(t, 1, strings.Count(got, "event: response.completed"))
	require.NotContains(t, got, `"id":"r1"`)
	lastSequence := int64(-1)
	for _, frame := range strings.Split(got, "\n\n") {
		for _, line := range strings.Split(frame, "\n") {
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			sequence := gjson.Get(payload, "sequence_number").Int()
			require.Greater(t, sequence, lastSequence)
			lastSequence = sequence
			if gjson.Get(payload, "type").String() == "response.completed" {
				require.Len(t, gjson.Get(payload, "response.output").Array(), 2)
				require.Equal(t, int64(12), gjson.Get(payload, "response.usage.input_tokens").Int())
			}
		}
	}
}

func TestAccountContinuationBufferedProtocolsMergeOnlyText(t *testing.T) {
	for _, tc := range []struct {
		protocol            AccountContinuationProtocol
		first, second, path string
	}{
		{AccountContinuationChat, `{"id":"one","choices":[{"message":{"role":"assistant","content":"a"},"finish_reason":"length"}]}`, `{"id":"two","choices":[{"message":{"role":"assistant","content":"b"},"finish_reason":"stop"}]}`, "choices.0.message.content"},
		{AccountContinuationMessages, `{"id":"one","content":[{"type":"text","text":"a"}],"stop_reason":"max_tokens"}`, `{"id":"two","content":[{"type":"text","text":"b"}],"stop_reason":"end_turn"}`, "content.0.text"},
		{AccountContinuationGemini, `{"candidates":[{"content":{"role":"model","parts":[{"text":"a"}]},"finishReason":"MAX_TOKENS"}]}`, `{"candidates":[{"content":{"role":"model","parts":[{"text":"b"}]},"finishReason":"STOP"}]}`, "candidates.0.content.parts.0.text"},
	} {
		t.Run(string(tc.protocol), func(t *testing.T) {
			out, recorder := continuationTestOutput(t, tc.protocol, false)
			for _, body := range []string{tc.first, tc.second} {
				w := out.BeginRound()
				w.WriteHeader(http.StatusOK)
				_, err := w.WriteString(body)
				require.NoError(t, err)
				round, err := w.Finish()
				require.NoError(t, err)
				require.False(t, round.Unsafe)
				require.Empty(t, recorder.Body.String())
			}
			require.NoError(t, out.Complete(AccountContinuationUsage{InputTokens: 20, OutputTokens: 2}))
			require.Equal(t, "ab", gjson.Get(recorder.Body.String(), tc.path).String())
		})
	}
}

func TestAccountContinuationDoesNotTreatToolsReasoningOrErrorsAsTruncation(t *testing.T) {
	for _, value := range []string{
		`{"choices":[{"delta":{"content":"text","reasoning_content":"private"},"finish_reason":"length"}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"id":"call"}]},"finish_reason":"length"}]}`,
		`{"error":{"message":"failed"}}`,
	} {
		out, _ := continuationTestOutput(t, AccountContinuationChat, true)
		w := out.BeginRound()
		_, err := w.WriteString("data: " + value + "\n\n")
		require.NoError(t, err)
		round, err := w.Finish()
		require.NoError(t, err)
		require.True(t, round.Unsafe)
	}
}

func TestAccountContinuationGeminiKeepsFinalStopWhenEveryChunkReportsUsage(t *testing.T) {
	out, recorder := continuationTestOutput(t, AccountContinuationGemini, true)
	w := out.BeginRound()
	for _, value := range []string{
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":1}}`,
		`{"candidates":[{"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":2}}`,
		`{"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":2}}`,
	} {
		_, err := w.WriteString("data: " + value + "\n\n")
		require.NoError(t, err)
	}
	_, err := w.Finish()
	require.NoError(t, err)
	require.NoError(t, out.Complete(AccountContinuationUsage{InputTokens: 10, OutputTokens: 2}))
	require.Contains(t, recorder.Body.String(), `"finishReason":"STOP"`)
	require.Equal(t, 1, strings.Count(recorder.Body.String(), "usageMetadata"))
}

func TestAccountContinuationMalformedFramesDoNotPanic(t *testing.T) {
	for _, tc := range []struct {
		protocol AccountContinuationProtocol
		payload  string
	}{
		{AccountContinuationChat, `{"choices":[null,{"delta":{},"finish_reason":"length"}]}`},
		{AccountContinuationMessages, `{"type":"message_start"}`},
		{AccountContinuationMessages, `{"type":"content_block_start","index":0}`},
		{AccountContinuationMessages, `{"type":"content_block_delta","index":0}`},
		{AccountContinuationGemini, `{"candidates":[null]}`},
		{AccountContinuationResponses, `{"type":"response.completed"}`},
	} {
		t.Run(string(tc.protocol)+tc.payload, func(t *testing.T) {
			out, _ := continuationTestOutput(t, tc.protocol, true)
			w := out.BeginRound()
			require.NotPanics(t, func() {
				_, err := w.WriteString("data: " + tc.payload + "\n\n")
				require.NoError(t, err)
				round, err := w.Finish()
				require.NoError(t, err)
				require.True(t, round.Unsafe)
				require.NoError(t, out.Complete(AccountContinuationUsage{}))
			})
		})
	}
}

func TestAccountContinuationTerminalRetentionIsBounded(t *testing.T) {
	out, _ := continuationTestOutput(t, AccountContinuationChat, true)
	w := out.BeginRound()
	for i := 0; i < 1000; i++ {
		_, err := w.WriteString(fmt.Sprintf("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":%d}}\n\n", i))
		require.NoError(t, err)
	}
	require.LessOrEqual(t, len(w.terminals), 3)
}

func TestAccountContinuationSecondHTTPErrorPreservesStreamError(t *testing.T) {
	for _, protocol := range []AccountContinuationProtocol{AccountContinuationChat, AccountContinuationMessages, AccountContinuationGemini, AccountContinuationResponses} {
		t.Run(string(protocol), func(t *testing.T) {
			out, recorder := continuationTestOutput(t, protocol, true)
			first := out.BeginRound()
			_, err := first.WriteString("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
			require.NoError(t, err)
			_, err = first.Finish()
			require.NoError(t, err)
			second := out.BeginRound()
			second.WriteHeader(503)
			_, err = second.WriteString(`{"error":{"message":"upstream unavailable","type":"server_error"}}`)
			require.NoError(t, err)
			_, err = second.Finish()
			require.NoError(t, err)
			require.NoError(t, out.Complete(AccountContinuationUsage{OutputTokens: 2}))
			require.Contains(t, recorder.Body.String(), `"error"`)
			require.NotContains(t, recorder.Body.String(), "[DONE]")
			require.NotContains(t, recorder.Body.String(), "response.completed")
		})
	}
}

func TestAccountContinuationPreservesHiddenReasoningUsage(t *testing.T) {
	out, rec := continuationTestOutput(t, AccountContinuationResponses, true)
	for i := 0; i < 2; i++ {
		w := out.BeginRound()
		_, err := w.WriteString(fmt.Sprintf("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_%d\",\"status\":\"completed\",\"output\":[],\"usage\":{\"output_tokens_details\":{\"reasoning_tokens\":7}}}}\n\n", i))
		require.NoError(t, err)
		_, err = w.Finish()
		require.NoError(t, err)
	}
	require.NoError(t, out.Complete(AccountContinuationUsage{InputTokens: 10, OutputTokens: 20}))
	require.Contains(t, rec.Body.String(), `"reasoning_tokens":14`)
}

func TestAccountContinuationEmptyBufferedFailureDoesNotCommit200(t *testing.T) {
	out, rec := continuationTestOutput(t, AccountContinuationChat, false)
	w := out.BeginRound()
	_, err := w.Finish()
	require.NoError(t, err)
	require.NoError(t, out.Complete(AccountContinuationUsage{}))
	require.Empty(t, rec.Body.String())
	require.False(t, out.base.Written())
}

func TestAccountContinuationCaptureLimitDoesNotAbortCurrentBufferedRound(t *testing.T) {
	out, rec := continuationTestOutput(t, AccountContinuationChat, false)
	first := out.BeginRound()
	_, err := first.WriteString(`{"choices":[{"message":{"content":"prefix"},"finish_reason":"length"}]}`)
	require.NoError(t, err)
	_, err = first.Finish()
	require.NoError(t, err)
	second := out.BeginRound()
	text := strings.Repeat("a", accountContinuationCaptureLimit+1)
	_, err = second.WriteString(`{"choices":[{"message":{"content":"` + text + `"},"finish_reason":"stop"}]}`)
	require.NoError(t, err)
	round, err := second.Finish()
	require.NoError(t, err)
	require.True(t, round.Unsafe, "oversize disables further replay")
	require.NoError(t, out.Complete(AccountContinuationUsage{}))
	require.Equal(t, "prefix"+text, gjson.Get(rec.Body.String(), "choices.0.message.content").String())
}

func TestAccountContinuationChatUsageEveryChunkKeepsAllText(t *testing.T) {
	out, recorder := continuationTestOutput(t, AccountContinuationChat, true)
	w := out.BeginRound()
	for _, text := range []string{"alpha", "beta"} {
		_, err := w.WriteString(fmt.Sprintf("data: {\"id\":\"one\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"%s\"},\"finish_reason\":null}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":2}}\n\n", text))
		require.NoError(t, err)
		require.Contains(t, recorder.Body.String(), text, "usage-bearing text must stream immediately")
	}
	_, err := w.WriteString("data: {\"id\":\"one\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	require.NoError(t, err)
	_, err = w.Finish()
	require.NoError(t, err)
	require.NoError(t, out.Complete(AccountContinuationUsage{InputTokens: 10, OutputTokens: 4}))
	require.Equal(t, 1, strings.Count(recorder.Body.String(), `"content":"alpha"`))
	require.Equal(t, 1, strings.Count(recorder.Body.String(), `"content":"beta"`))
}
