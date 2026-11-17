//go:build unit

package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPatchGeminiNativeOutputUsageFillsMissingCandidates(t *testing.T) {
	raw := []byte(`{"candidates":[{"content":{"parts":[{"text":"hello from upstream"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":2592,"candidatesTokenCount":0,"thoughtsTokenCount":0,"totalTokenCount":2592}}`)

	patched := patchGeminiNativeOutputUsage(raw, "hello from upstream", 0)

	require.EqualValues(t, 5, gjson.GetBytes(patched, "usageMetadata.candidatesTokenCount").Int())
	require.EqualValues(t, 2597, gjson.GetBytes(patched, "usageMetadata.totalTokenCount").Int())
}

func TestPatchGeminiNativeOutputUsagePreservesMeasuredCandidates(t *testing.T) {
	raw := []byte(`{"candidates":[{"content":{"parts":[{"text":"hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":73,"totalTokenCount":83}}`)

	patched := patchGeminiNativeOutputUsage(raw, "hello", 999)

	require.Equal(t, string(raw), string(patched), "an upstream candidate count is authoritative")
}

func TestPatchGeminiNativeOutputUsageDoesNotInventOutputWithoutSemanticContent(t *testing.T) {
	raw := []byte(`{"candidates":[{"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":0,"totalTokenCount":10}}`)

	patched := patchGeminiNativeOutputUsage(raw, "", 0)

	require.Equal(t, string(raw), string(patched))
}

func TestGeminiNativeUsageTailPreservesProviderCounts(t *testing.T) {
	body := "data: " + `{"candidates":[{"content":{"parts":[{"text":"hello"}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":73,"thoughtsTokenCount":2,"totalTokenCount":85}}` + "\n\n" +
		"data: " + `{"candidates":[{"finishReason":"STOP"}],"usageMetadata":{"cachedContentTokenCount":0}}` + "\n\n"
	svc := newGeminiSignalService("text/event-stream", body)
	c, rec := newGeminiNativeTestContext(t)
	result, err := svc.ForwardNative(context.Background(), c, geminiSignalTestAccount(), "gemini-test", "streamGenerateContent", true, geminiSignalTestRequest())
	require.NoError(t, err)
	require.Equal(t, 75, result.Usage.OutputTokens)
	tail := nativeUsageLastFrame(t, rec.Body.String())
	require.EqualValues(t, 73, tail.Get("usageMetadata.candidatesTokenCount").Int())
	require.EqualValues(t, 2, tail.Get("usageMetadata.thoughtsTokenCount").Int())
	require.EqualValues(t, 85, tail.Get("usageMetadata.totalTokenCount").Int())
}

func TestGeminiNativeUsageEstimatesUniqueCandidatesOnly(t *testing.T) {
	for _, tc := range []struct {
		name, parts, metadata string
		want                  int64
	}{
		{"thought only", `{"text":"abcdefgh","thought":true}`, `{"promptTokenCount":10,"candidatesTokenCount":0,"thoughtsTokenCount":2,"totalTokenCount":12}`, 0},
		{"duplicate string tool", `{"functionCall":{"id":"a","name":"tool","args":"{}"}}`, `{"promptTokenCount":10,"thoughtsTokenCount":2,"totalTokenCount":12}`, 2},
		{"cumulative text", `{"text":"abcdefgh"}`, `{"promptTokenCount":10,"thoughtsTokenCount":2,"totalTokenCount":12}`, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame := "data: " + `{"candidates":[{"content":{"parts":[` + tc.parts + `]}}]}` + "\n\n"
			body := frame + frame + "data: " + `{"candidates":[{"finishReason":"STOP"}],"usageMetadata":` + tc.metadata + `}` + "\n\n"
			svc := newGeminiSignalService("text/event-stream", body)
			c, rec := newGeminiNativeTestContext(t)
			result, err := svc.ForwardNative(context.Background(), c, geminiSignalTestAccount(), "gemini-test", "streamGenerateContent", true, geminiSignalTestRequest())
			require.NoError(t, err)
			require.Equal(t, 2, result.Usage.OutputTokens, "wire repair must not become provider billing evidence")
			tail := nativeUsageLastFrame(t, rec.Body.String())
			require.Equal(t, tc.want, tail.Get("usageMetadata.candidatesTokenCount").Int())
			require.EqualValues(t, 2, tail.Get("usageMetadata.thoughtsTokenCount").Int())
			require.Equal(t, 12+tc.want, tail.Get("usageMetadata.totalTokenCount").Int())
		})
	}
}

func TestGeminiNativeJSONFallbackRepairsOutputUsage(t *testing.T) {
	for _, array := range []bool{false, true} {
		body := `{"candidates":[{"content":{"parts":[{"text":"abcdefgh"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"totalTokenCount":10}}`
		if array {
			body = "[" + body + "]"
		}
		svc := newGeminiSignalService("application/json", body)
		c, rec := newGeminiNativeTestContext(t)
		result, err := svc.ForwardNative(context.Background(), c, geminiSignalTestAccount(), "gemini-test", "streamGenerateContent", true, geminiSignalTestRequest())
		require.NoError(t, err)
		require.Zero(t, result.Usage.OutputTokens, "fallback projection is client-only")
		wire := gjson.Parse(rec.Body.String())
		if array {
			wire = wire.Get("0")
		}
		require.EqualValues(t, 2, wire.Get("usageMetadata.candidatesTokenCount").Int())
		require.EqualValues(t, 12, wire.Get("usageMetadata.totalTokenCount").Int())
	}
}

func TestGeminiCollectedResponseRetainsTerminalUsage(t *testing.T) {
	body := "data: " + `{"candidates":[{"content":{"parts":[{"text":"abcdefgh"}]}}]}` + "\n\n" +
		"data: " + `{"candidates":[{"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":73,"thoughtsTokenCount":2,"totalTokenCount":85}}` + "\n\n" +
		"data: " + `{"usageMetadata":{"cachedContentTokenCount":0}}` + "\n\n"
	collected, usage, _, err := collectGeminiSSEObserved(nil, strings.NewReader(body), false, nil)
	require.NoError(t, err)
	require.Equal(t, 75, usage.OutputTokens)
	raw, err := json.Marshal(collected)
	require.NoError(t, err)
	require.EqualValues(t, 73, gjson.GetBytes(raw, "usageMetadata.candidatesTokenCount").Int())
	require.EqualValues(t, 2, gjson.GetBytes(raw, "usageMetadata.thoughtsTokenCount").Int())
	require.Equal(t, "STOP", gjson.GetBytes(raw, "candidates.0.finishReason").String())
}

func nativeUsageLastFrame(t *testing.T, body string) gjson.Result {
	t.Helper()
	var last gjson.Result
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "data: ") {
			raw := strings.TrimPrefix(line, "data: ")
			if gjson.Valid(raw) {
				last = gjson.Parse(raw)
			}
		}
	}
	require.True(t, last.Exists(), body)
	return last
}

func TestGeminiCollectedResponseRepairsOnlyClientUsage(t *testing.T) {
	body := "data: " + `{"candidates":[{"content":{"parts":[{"text":"abcdefgh"}]}}]}` + "\n\n" +
		"data: " + `{"candidates":[{"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"thoughtsTokenCount":3,"totalTokenCount":13}}` + "\n\n"
	collected, usage, stats, err := collectGeminiSSEObserved(nil, strings.NewReader(body), false, nil)
	require.NoError(t, err)
	raw, err := json.Marshal(collected)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(raw, "usageMetadata.candidatesTokenCount").Exists(), "collected provider usage remains unmodified for billing")
	projected := stats.nativeUsage.project(raw)
	require.EqualValues(t, 2, gjson.GetBytes(projected, "usageMetadata.candidatesTokenCount").Int())
	require.EqualValues(t, 15, gjson.GetBytes(projected, "usageMetadata.totalTokenCount").Int())
	require.Equal(t, 3, usage.OutputTokens)
}
