package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestEmptyResponseClaimDTOUsesFrozenRefund(t *testing.T) {
	amount := 5.0
	claim := &service.EmptyResponseClaim{OriginalActualCost: 12, CompensationAmount: &amount, RuleVersion: 5}
	got := EmptyResponseClaimFromService(claim)
	require.Equal(t, amount, got.EstimatedRefund)
	admin := EmptyResponseClaimFromServiceAdmin(claim)
	require.Equal(t, amount, admin.EstimatedRefund)
	legacy := EmptyResponseClaimFromService(&service.EmptyResponseClaim{OriginalActualCost: 12, RuleVersion: 4})
	require.Equal(t, 12.0, legacy.EstimatedRefund)
}

func TestEmptyResponseQuoteDTOHasAmountWithoutTierPolicy(t *testing.T) {
	log := UsageLogFromService(&service.UsageLog{CompensationEstimatedRefund: 5})
	require.Equal(t, 5.0, log.CompensationEstimatedRefund)
	record := EmptyResponseRecordFromService(&service.EmptyResponseRecord{EstimatedRefund: 5})
	require.Equal(t, 5.0, record.EstimatedRefund)
	for _, value := range []any{log, record, EmptyResponseClaimFromService(&service.EmptyResponseClaim{})} {
		body, err := json.Marshal(value)
		require.NoError(t, err)
		require.NotContains(t, string(body), "compensation_rate")
		require.NotContains(t, string(body), "compensation_input_tokens")
	}
}
