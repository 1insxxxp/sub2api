package admin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupRequestsDecodeGeminiInputCounting(t *testing.T) {
	var create CreateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{"name":"group-discount","gemini_input_counting_enabled":true}`), &create))
	require.True(t, create.GeminiInputCountingEnabled)
	var update UpdateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{"gemini_input_counting_enabled":false}`), &update))
	require.NotNil(t, update.GeminiInputCountingEnabled)
	require.False(t, *update.GeminiInputCountingEnabled)
	var omitted UpdateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{}`), &omitted))
	require.Nil(t, omitted.GeminiInputCountingEnabled)
	sanitizeCreateGroupRequestForSimpleMode(&create)
	require.False(t, create.GeminiInputCountingEnabled)
	sanitizeUpdateGroupRequestForSimpleMode(&update)
	require.Nil(t, update.GeminiInputCountingEnabled)
}
