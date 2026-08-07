//go:build unit

package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageLogCompensationProjectionIncludesRefundAndNetCost(t *testing.T) {
	t.Parallel()
	require.Contains(t, usageLogSelectColumns, "compensated_cost")
	require.Equal(t, "GREATEST(actual_cost - compensated_cost, 0)", usageLogNetActualCostExpr)
}
