package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
)

// defaultAnthropicOutputTokens supplies a known Anthropic max_tokens default only
// when the client omitted its output limit. Resolve against the selected
// account and final upstream model, never a public alias or another account.
// This is a generation default, not an inflight estimate or continuation cap.
// Unknown capabilities stay omitted rather than inventing an output ceiling.
func defaultAnthropicOutputTokens(account *Account, upstreamModel string) int {
	upstreamModel = strings.TrimSpace(upstreamModel)
	if metadata, ok := account.GetUpstreamModelMetadata(upstreamModel); ok &&
		metadata.MaxOutputTokens > 0 && metadata.MaxOutputTokens <= int64(^uint(0)>>1) {
		return int(metadata.MaxOutputTokens)
	}

	limit, _ := claude.ModelMaxOutputTokens(upstreamModel)
	return limit
}
