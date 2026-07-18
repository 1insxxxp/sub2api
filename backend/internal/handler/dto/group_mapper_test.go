package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupFromServiceAdmin_IncludesDefaultReasoningEffort(t *testing.T) {
	t.Parallel()

	group := &service.Group{
		ID:                     42,
		Name:                   "ccmax",
		DefaultReasoningEffort: "high",
	}

	got := GroupFromServiceAdmin(group)

	require.NotNil(t, got)
	require.Equal(t, "high", got.DefaultReasoningEffort)
}
