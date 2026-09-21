package service

import (
	"math"
	"sort"
)

const (
	DefaultBalancePreauthorizationSampleCount          = 100
	DefaultBalancePreauthorizationMeanSafetyFactor     = 2.0
	DefaultBalancePreauthorizationP95Quantile          = 0.95
	DefaultBalancePreauthorizationMaxOverdraftFraction = 0.5
)

func normalizeBalancePreauthorizationPolicy(policy BalancePreauthorizationPolicy) BalancePreauthorizationPolicy {
	if policy.SampleCount <= 0 {
		policy.SampleCount = DefaultBalancePreauthorizationSampleCount
	}
	if policy.MeanSafetyFactor <= 0 || math.IsNaN(policy.MeanSafetyFactor) || math.IsInf(policy.MeanSafetyFactor, 0) {
		policy.MeanSafetyFactor = DefaultBalancePreauthorizationMeanSafetyFactor
	}
	if policy.P95Quantile <= 0 || policy.P95Quantile > 1 || math.IsNaN(policy.P95Quantile) || math.IsInf(policy.P95Quantile, 0) {
		policy.P95Quantile = DefaultBalancePreauthorizationP95Quantile
	}
	if policy.MaxOverdraftFraction < 0 || policy.MaxOverdraftFraction > 1 || math.IsNaN(policy.MaxOverdraftFraction) || math.IsInf(policy.MaxOverdraftFraction, 0) {
		policy.MaxOverdraftFraction = DefaultBalancePreauthorizationMaxOverdraftFraction
	}
	return policy
}

// BalancePreauthorizationPolicy controls the conservative balance gate used
// before a balance-billed request is forwarded upstream.
type BalancePreauthorizationPolicy struct {
	SampleCount          int
	MeanSafetyFactor     float64
	P95Quantile          float64
	MaxOverdraftFraction float64
	MinimumReserve       float64
}

// BalancePreauthorizationEstimate is the auditable result of one estimate.
type BalancePreauthorizationEstimate struct {
	Authorization float64
	Mean          float64
	P95           float64
	SampleCount   int
}

// EstimateBalancePreauthorization computes the request authorization from
// recent positive actual costs. P95 is a floor: the estimate cannot be lower
// than either mean*safety or the configured percentile.
func EstimateBalancePreauthorization(samples []float64, policy BalancePreauthorizationPolicy) BalancePreauthorizationEstimate {
	policy = normalizeBalancePreauthorizationPolicy(policy)
	limit := policy.SampleCount
	if limit <= 0 || limit > len(samples) {
		limit = len(samples)
	}
	filtered := make([]float64, 0, limit)
	for _, sample := range samples {
		if len(filtered) >= limit {
			break
		}
		if math.IsNaN(sample) || math.IsInf(sample, 0) || sample <= 0 {
			continue
		}
		filtered = append(filtered, sample)
	}
	if len(filtered) == 0 {
		return BalancePreauthorizationEstimate{
			Authorization: nonNegativeFinite(policy.MinimumReserve),
		}
	}

	sort.Float64s(filtered)
	var total float64
	for _, sample := range filtered {
		total += sample
	}
	mean := total / float64(len(filtered))
	quantile := policy.P95Quantile
	if quantile <= 0 || quantile > 1 || math.IsNaN(quantile) || math.IsInf(quantile, 0) {
		quantile = 0.95
	}
	// Nearest-rank percentile: p95 of 100 samples is the 95th value.
	rank := int(math.Ceil(quantile * float64(len(filtered))))
	if rank < 1 {
		rank = 1
	}
	if rank > len(filtered) {
		rank = len(filtered)
	}
	p95 := filtered[rank-1]
	safety := policy.MeanSafetyFactor
	if safety <= 0 || math.IsNaN(safety) || math.IsInf(safety, 0) {
		safety = 1
	}
	authorization := math.Max(mean*safety, p95)
	authorization = math.Max(authorization, nonNegativeFinite(policy.MinimumReserve))
	return BalancePreauthorizationEstimate{
		Authorization: authorization,
		Mean:          mean,
		P95:           p95,
		SampleCount:   len(filtered),
	}
}

// BalancePreauthorizationFloor returns the balance required before forwarding
// a request. The configured fraction is the maximum one-request debt allowance
// relative to the authorization.
func BalancePreauthorizationFloor(authorization, maxOverdraftFraction float64) float64 {
	if math.IsNaN(authorization) || math.IsInf(authorization, 0) || authorization <= 0 {
		return 0
	}
	fraction := maxOverdraftFraction
	if math.IsNaN(fraction) || math.IsInf(fraction, 0) {
		fraction = 0
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	return authorization * (1 - fraction)
}

func nonNegativeFinite(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return 0
	}
	return value
}
