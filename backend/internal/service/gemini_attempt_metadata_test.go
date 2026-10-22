package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiAttemptMetadata_ObservedZeroSurvivesQueueSanitizer(t *testing.T) {
	var event OpsUpstreamErrorEvent
	require.NoError(t, json.Unmarshal([]byte(`{"message":"stream interrupted","attempt":2,"model":"gemini-test","usage_state":"observed","input_tokens":0,"output_tokens":0}`), &event))
	entry := &OpsInsertErrorLogInput{UpstreamErrors: []*OpsUpstreamErrorEvent{&event}}
	require.NoError(t, SanitizeOpsUpstreamErrorsForQueue(entry))
	var events []map[string]any
	require.NotNil(t, entry.UpstreamErrorsJSON)
	require.NoError(t, json.Unmarshal([]byte(*entry.UpstreamErrorsJSON), &events))
	require.Len(t, events, 1)
	require.Equal(t, "observed", events[0]["usage_state"], "explicit zero differs from unreported usage")
	require.Equal(t, float64(2), events[0]["attempt"])
}
