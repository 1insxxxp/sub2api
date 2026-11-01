package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupMapperExposesGeminiInputCountingOnlyToAdmins(t *testing.T) {
	group := &service.Group{ID: 53, Platform: service.PlatformGemini, GeminiInputCountingEnabled: true}
	raw, err := json.Marshal(GroupFromServiceAdmin(group))
	require.NoError(t, err)
	var admin map[string]any
	require.NoError(t, json.Unmarshal(raw, &admin))
	require.Equal(t, true, admin["gemini_input_counting_enabled"])
	for _, dto := range []any{GroupFromService(group), GroupFromServiceShallow(group)} {
		raw, err = json.Marshal(dto)
		require.NoError(t, err)
		require.NotContains(t, string(raw), "gemini_input_counting_enabled")
	}
}
