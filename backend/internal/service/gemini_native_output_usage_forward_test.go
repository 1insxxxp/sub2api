//go:build unit

package service

import (
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
