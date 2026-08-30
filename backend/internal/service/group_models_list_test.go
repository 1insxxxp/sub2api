package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomModelsListAllowsModel_ClaudeAliases(t *testing.T) {
	available := []string{"claude-opus-4-6", "claude-sonnet-4-5-20250929"}

	require.True(t, CustomModelsListAllowsModel(available, "claude-opus-4-6-thinking"))
	require.True(t, CustomModelsListAllowsModel(available, "claude-sonnet-4-5"))
}
