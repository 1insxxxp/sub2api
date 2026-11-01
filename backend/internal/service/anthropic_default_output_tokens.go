package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
)

// defaultAnthropicOutputTokens supplies Anthropic's required max_tokens only
// when the client omitted its output limit. Resolve against the selected
// account and final upstream model, never a public alias or another account.
// This is a generation default, not an inflight estimate or continuation cap.
func defaultAnthropicOutputTokens(account *Account, upstreamModel string) int {
	upstreamModel = strings.TrimSpace(upstreamModel)
	if metadata, ok := account.GetUpstreamModelMetadata(upstreamModel); ok &&
		metadata.MaxOutputTokens > 0 && metadata.MaxOutputTokens <= int64(^uint(0)>>1) {
		return int(metadata.MaxOutputTokens)
	}

	// Official model capabilities checked 2026-11-01:
	// https://platform.claude.com/docs/en/models/overview
	// 4.5 models retain 64K; 4.6 and the later listed models support 128K.
	// Only known IDs qualify: a private compatible model must not inherit an
	// arbitrary 128K default that its upstream can reject with HTTP 400.
	model := claude.NormalizeModelID(upstreamModel)
	switch model {
	case "claude-fable-5-1", "claude-fable-5",
		"claude-opus-5-5", "claude-opus-5", "claude-opus-4-8", "claude-opus-4-7", "claude-opus-4-6",
		"claude-sonnet-5-5", "claude-sonnet-5", "claude-sonnet-4-6", "claude-haiku-5-5":
		return 128000
	case "claude-opus-4-5-20251101", "claude-sonnet-4-5-20250929", "claude-haiku-4-5-20251001",
		"claude-opus-4-5@20251101", "claude-sonnet-4-5@20250929", "claude-haiku-4-5@20251001":
		return 64000
	default:
		// Preserve legacy compatibility for unknown Anthropic-compatible IDs
		// without synced capabilities. Native OpenAI/Gemini never use this
		// fallback, and caller-supplied limits always take precedence.
		return 8192
	}
}
