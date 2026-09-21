//go:build unit

package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEstimateBalancePreauthorization_UsesConfiguredMeanAndP95Floor(t *testing.T) {
	policy := BalancePreauthorizationPolicy{
		SampleCount:          100,
		MeanSafetyFactor:     2,
		P95Quantile:          0.95,
		MaxOverdraftFraction: 0.5,
		MinimumReserve:       0.01,
	}

	t.Run("mean driven", func(t *testing.T) {
		estimate := EstimateBalancePreauthorization([]float64{1, 1, 1, 1, 1}, policy)
		require.InDelta(t, 2, estimate.Authorization, 1e-9)
		require.InDelta(t, 1, estimate.Mean, 1e-9)
		require.InDelta(t, 1, estimate.P95, 1e-9)
		require.Equal(t, 5, estimate.SampleCount)
	})

	t.Run("p95 floor", func(t *testing.T) {
		estimate := EstimateBalancePreauthorization([]float64{1, 2, 3, 4, 10}, policy)
		require.InDelta(t, 10, estimate.Authorization, 1e-9)
		require.InDelta(t, 4, estimate.Mean, 1e-9)
		require.InDelta(t, 10, estimate.P95, 1e-9)
	})
}

func TestEstimateBalancePreauthorization_FiltersInvalidSamplesAndUsesMinimum(t *testing.T) {
	policy := BalancePreauthorizationPolicy{
		SampleCount:          100,
		MeanSafetyFactor:     2,
		P95Quantile:          0.95,
		MaxOverdraftFraction: 0.5,
		MinimumReserve:       0.25,
	}

	estimate := EstimateBalancePreauthorization([]float64{0, -1, math.NaN(), math.Inf(1)}, policy)
	require.Equal(t, 0, estimate.SampleCount)
	require.Zero(t, estimate.Mean)
	require.Zero(t, estimate.P95)
	require.InDelta(t, 0.25, estimate.Authorization, 1e-9)
}

func TestBalancePreauthorizationFloor_AllowsOnlyConfiguredOverdraft(t *testing.T) {
	require.InDelta(t, 5, BalancePreauthorizationFloor(10, 0.5), 1e-9)
	require.InDelta(t, 10, BalancePreauthorizationFloor(10, 0), 1e-9)
	require.Zero(t, BalancePreauthorizationFloor(10, 1))
}
