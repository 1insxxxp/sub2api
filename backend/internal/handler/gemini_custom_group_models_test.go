package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiSystemCustomGroupModelsExposesGeminiAliasesOnly(t *testing.T) {
	handler, key := newSystemCustomModelsHandler(t)
	recorder, c := systemCustomModelsRequestContext(t, key, "/v1beta/models")

	handler.GeminiV1BetaListModels(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []string{"models/gemini-monthly"}, gjsonStrings(recorder.Body.String(), "models.#.name"))
	require.NotContains(t, recorder.Body.String(), "gemini-2.5-flash")
	require.NotContains(t, recorder.Body.String(), "claude-monthly")
}

func TestCustomGroupGeminiModelsExposesAliasesOnly(t *testing.T) {
	response := customGroupGeminiModels([]string{"gemini-balanced", "gemini-fast"})
	require.Len(t, response.Models, 2)
	require.Equal(t, "models/gemini-balanced", response.Models[0].Name)
	require.Equal(t, "models/gemini-fast", response.Models[1].Name)
}
