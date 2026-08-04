//go:build unit

package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyListQueriesPreloadCustomGroup(t *testing.T) {
	source, err := os.ReadFile("api_key_repo.go")
	require.NoError(t, err)

	text := string(source)
	listStart := strings.Index(text, "func (r *apiKeyRepository) ListByUserID")
	listAllStart := strings.Index(text, "func (r *apiKeyRepository) ListAllByUserID")
	attachStart := strings.Index(text, "func (r *apiKeyRepository) attachLastUsedIPs")
	require.NotEqual(t, -1, listStart)
	require.Greater(t, listAllStart, listStart)
	require.Greater(t, attachStart, listAllStart)

	require.Equal(t, 1, strings.Count(text[listStart:listAllStart], "WithCustomGroup()"))
	require.Equal(t, 1, strings.Count(text[listAllStart:attachStart], "WithCustomGroup()"))
}
