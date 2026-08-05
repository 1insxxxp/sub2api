package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomGroupGeminiModelsExposesAliasesOnly(t *testing.T) {
	response := customGroupGeminiModels([]string{"gemini-balanced", "gemini-fast"})
	require.Len(t, response.Models, 2)
	require.Equal(t, "models/gemini-balanced", response.Models[0].Name)
	require.Equal(t, "models/gemini-fast", response.Models[1].Name)
}
