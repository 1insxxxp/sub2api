package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEstimateInflightInputTokensIgnoresMultimediaPayloadBytes(t *testing.T) {
	// A 17 MB inline image must not become 4.25 M text tokens.
	body := `{"model":"gcp/gemini-3.1-pro-preview","messages":[{"role":"user","content":[{"type":"text","text":"请分析这张图片"},{"type":"image_url","image_url":{"url":"data:image/png;base64,` + strings.Repeat("A", 17*1024*1024) + `"}}]}]}`

	tokens, ok := estimateInflightInputTokens("gcp/gemini-3.1-pro-preview", []byte(body))

	require.True(t, ok)
	require.Less(t, tokens, 100)

	geminiBody := `{"contents":[{"parts":[{"text":"请分析"},{"inlineData":{"mimeType":"image/png","data":"` + strings.Repeat("A", 17*1024*1024) + `"}}]}]}`
	tokens, ok = estimateInflightInputTokens("gemini-3.1-pro-preview", []byte(geminiBody))
	require.True(t, ok)
	require.Less(t, tokens, 100)
}
