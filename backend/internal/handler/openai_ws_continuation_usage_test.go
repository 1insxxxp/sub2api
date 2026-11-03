//go:build unit

package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIWSInterruptedContinuationCanSettleWithoutResponseID(t *testing.T) {
	delivered := 3
	result := &service.OpenAIForwardResult{
		Usage:                 service.OpenAIUsage{InputTokens: 12, OutputTokens: 5},
		DeliveredOutputTokens: &delivered,
		ContinuationResults:   []*service.OpenAIForwardResult{{}},
	}
	require.True(t, openAIWSInterruptedTurnHasBillableOutput(result), "missing provider response ID must not discard real delivered text and round evidence")
	delivered = 0
	require.False(t, openAIWSInterruptedTurnHasBillableOutput(result), "failed requests with no delivered text must not enter partial settlement")
	delivered = 3
	result.ContinuationResults = nil
	require.False(t, openAIWSInterruptedTurnHasBillableOutput(result), "keep unrelated websocket failure behavior unchanged")
}
