package handler

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateBalanceTransferCodeRequestPropagatesThresholdExempt(t *testing.T) {
	var request GenerateBalanceTransferRedeemCodeRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"amount": 12.5,
		"count": 2,
		"threshold_exempt": true
	}`), &request))

	input := balanceTransferCodeInputFromRequest(request, request.Count)

	require.True(t, input.ThresholdExempt)
}
