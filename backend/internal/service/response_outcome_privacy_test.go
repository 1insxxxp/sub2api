//go:build unit

package service_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestResponseOutcomePrivacyExcludesResponseContentFromEveryClaimDTO(t *testing.T) {
	fixtures := []string{
		"PRIVATE_RESPONSE_TEXT_7f91",
		"PRIVATE_REASONING_TEXT_31aa",
		"PRIVATE_TOOL_ARGUMENT_892b",
		"https://secret.example.test/private-result.png?token=2e4c",
		"cHJpdmF0ZS1pbWFnZS1wYXlsb2FkLTkxZDc=",
	}
	collector := service.NewResponseOutcomeCollector(200, 200)
	payload := `{"choices":[{"message":{"content":"` + fixtures[0] + `","reasoning_content":"` + fixtures[1] + `","tool_calls":[{"function":{"arguments":"` + fixtures[2] + `"}}]}}],"data":[{"url":"` + fixtures[3] + `","b64_json":"` + fixtures[4] + `"}]}`
	require.NoError(t, collector.ObserveJSONPayload(service.ResponseOutcomeProtocolOpenAI, []byte(payload)))
	collector.MarkCompleted(fixtures[3])
	outcome := collector.Snapshot()

	claim := &service.EmptyResponseClaim{
		ID:                 1,
		UsageLogID:         2,
		Status:             service.EmptyResponseClaimManualReview,
		ReasonCode:         service.EmptyResponseReasonMissingEvidence,
		OriginalActualCost: 1.25,
		Evidence:           outcome,
		UserReason:         "The assistant returned no visible answer.",
		AdminNote:          "Structured evidence reviewed.",
	}

	values := []any{
		outcome,
		dto.EmptyResponseClaimFromService(claim),
		dto.EmptyResponseClaimFromServiceAdmin(claim),
	}
	for _, value := range values {
		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		serialized := string(encoded)
		for _, fixture := range fixtures {
			require.False(t, strings.Contains(serialized, fixture), "serialized privacy-safe DTO leaked %q: %s", fixture, serialized)
		}
	}
}
