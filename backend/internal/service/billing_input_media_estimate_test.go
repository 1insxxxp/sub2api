//go:build unit

package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInflightInputEstimateMediaPayloadDoesNotInflateReservation(t *testing.T) {
	// Same model-visible text and output budget; only transport payload changes.
	for name, template := range map[string]string{
		"chat image":              `{"messages":[{"role":"user","content":[{"type":"text","text":"describe image"},{"type":"image_url","image_url":{"url":"data:image/png;base64,PAYLOAD"}}]}]}`,
		"chat audio":              `{"messages":[{"role":"user","content":[{"type":"text","text":"describe audio"},{"type":"input_audio","input_audio":{"format":"wav","data":"PAYLOAD"}}]}]}`,
		"messages image":          `{"messages":[{"role":"user","content":[{"type":"text","text":"describe image"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"PAYLOAD"}}]}]}`,
		"messages document":       `{"messages":[{"role":"user","content":[{"type":"text","text":"describe document"},{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"PAYLOAD"}}]}]}`,
		"responses file":          `{"input":[{"role":"user","content":[{"type":"input_text","text":"describe file"},{"type":"input_file","file_data":"data:application/pdf;base64,PAYLOAD"}]}]}`,
		"gemini inline":           `{"contents":[{"parts":[{"text":"describe audio"},{"inlineData":{"mimeType":"audio/wav","data":"PAYLOAD"}}]}]}`,
		"gemini snake case":       `{"contents":[{"parts":[{"text":"describe audio"},{"inline_data":{"mime_type":"audio/wav","data":"PAYLOAD"}}]}]}`,
		"gemini cached and media": `{"cachedContent":"cachedContents/test","contents":[{"parts":[{"text":"describe image"},{"fileData":{"mimeType":"image/png","fileUri":"https://example.invalid/PAYLOAD"}}]}]}`,
		"encrypted reasoning":     `{"input":[{"type":"reasoning","encrypted_content":"PAYLOAD","summary":[{"type":"summary_text","text":"keep this summary"}]},{"role":"user","content":"continue"}]}`,
		"redacted thinking":       `{"messages":[{"role":"assistant","content":[{"type":"redacted_thinking","data":"PAYLOAD"}]},{"role":"user","content":"continue"}]}`,
		"responses tool media":    `{"input":[{"type":"function_call_output","call_id":"test","output":[{"type":"input_text","text":"describe image"},{"type":"input_image","image_url":"data:image/png;base64,PAYLOAD"}]}]}`,
		"document mixed content":  `{"messages":[{"role":"user","content":[{"type":"document","source":{"type":"content","content":[{"type":"text","text":"describe image"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"PAYLOAD"}}]}}]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			svc := newInflightEstimateGateway(t, nil)
			user := &User{ID: 1, Balance: 0.01}
			key := &APIKey{User: user, Group: &Group{RateMultiplier: 1}}
			estimate := func(size int) float64 {
				body := []byte(strings.ReplaceAll(template, "PAYLOAD", strings.Repeat("A", size)))
				count, ok := estimateInflightInputTokens("claude-sonnet-4-5", body)
				require.True(t, ok, "media must not trigger raw request-byte fallback")
				require.Greater(t, count, 0, "model-visible text must still be counted")
				amount, priced := svc.EstimateInflightReservation(context.Background(), key, InflightEstimateRequest{
					Model: "claude-sonnet-4-5", Body: body, BodyBytes: len(body), MaxTokens: 32,
				})
				require.True(t, priced)
				return amount
			}
			small, large := estimate(100), estimate(100_000)
			require.Equal(t, small, large)
			require.Less(t, large, user.Balance, "media transport size must not cause a false insufficient-balance error")
			cache := newMemInflightCache(user.Balance)
			billing := newInflightSvc(t, cache, 60)
			billing.userRepo = &inflightRecheckUserRepo{user: user}
			reservation, err := billing.ReserveInflight(context.Background(), user, nil, nil, large)
			require.NoError(t, err)
			require.NotNil(t, reservation)
			reservation.HandlerDone()
			require.Zero(t, cache.count())
			t.Logf("media_bytes=100/100000 reservation=%.8f/%.8f", small, large)
		})
	}
}

func TestInflightInputEstimateRetainsDocumentTextAndStructuredToolData(t *testing.T) {
	text := strings.Repeat("Important model-visible text. ", 1000)
	for name, input := range map[string]any{
		"text document":           map[string]any{"type": "document", "source": map[string]any{"type": "text", "media_type": "text/plain", "data": text}},
		"document content string": map[string]any{"type": "document", "source": map[string]any{"type": "content", "content": text}},
		"document content blocks": map[string]any{"type": "document", "source": map[string]any{"type": "content", "content": []any{map[string]any{"type": "text", "text": text}}}},
		"document title":          map[string]any{"type": "document", "title": text, "source": map[string]any{"type": "text", "data": "short text"}},
		"document context":        map[string]any{"type": "document", "context": text, "source": map[string]any{"type": "text", "data": "short text"}},
		"tool input":              map[string]any{"type": "tool_use", "input": map[string]any{"data": text, "url": text}},
		"tool output":             map[string]any{"type": "function_call_output", "output": map[string]any{"data": text}},
		"tool output array":       map[string]any{"type": "function_call_output", "output": []any{map[string]any{"data": text}}},
	} {
		t.Run(name, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{"messages": []any{map[string]any{"role": "user", "content": []any{input}}}})
			require.NoError(t, err)
			count, ok := estimateInflightInputTokens("claude-sonnet-4-5", body)
			require.True(t, ok)
			codec, err := openAIInputTokensCodecForModel("claude-sonnet-4-5")
			require.NoError(t, err)
			minimum, err := codec.Count(text)
			require.NoError(t, err)
			require.GreaterOrEqual(t, count, minimum)
		})
	}
}

func TestInflightInputEstimateOpaqueOnlyRetainsOutputReservation(t *testing.T) {
	for _, body := range []string{
		`{"cached_content":"cachedContents/test"}`,
		`{"input":[],"metadata":"client-only"}`,
	} {
		count, ok := estimateInflightInputTokens("claude-sonnet-4-5", []byte(body))
		require.True(t, ok)
		require.Zero(t, count)
		svc := newInflightEstimateGateway(t, nil)
		amount, priced := svc.EstimateInflightReservation(context.Background(), &APIKey{User: &User{ID: 1}, Group: &Group{RateMultiplier: 1}}, InflightEstimateRequest{Model: "claude-sonnet-4-5", Body: []byte(body), BodyBytes: len(body), MaxTokens: 32})
		require.True(t, priced)
		require.Greater(t, amount, 0.0)
	}
}
