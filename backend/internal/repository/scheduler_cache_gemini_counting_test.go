package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSchedulerMetadataGeminiInputCounting_ValidatedProjection(t *testing.T) {
	for _, models := range []any{[]any{"[量]gemini-4-argon"}, []string{"[量]gemini-4-argon"}} {
		metadata := buildSchedulerMetadataAccount(service.Account{ID: 6021, Extra: map[string]any{"gemini_input_counting": map[string]any{"mode": "vertex_like_discount_v1", "models": models, "api_key": "do-not-cache", "body": "do-not-cache"}}})
		require.Equal(t, map[string]any{"mode": "vertex_like_discount_v1", "models": []string{"[量]gemini-4-argon"}}, metadata.Extra["gemini_input_counting"])
	}
	for _, value := range []any{"invalid", map[string]any{}, map[string]any{"mode": "future", "models": []string{"m"}}, map[string]any{"mode": "vertex_like_discount_v1", "models": []any{42}}} {
		metadata := buildSchedulerMetadataAccount(service.Account{Extra: map[string]any{"gemini_input_counting": value}})
		require.NotContains(t, metadata.Extra, "gemini_input_counting")
	}
}
