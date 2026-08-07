package service

import (
	"context"
	"time"
)

type EmptyResponseClaimMetricDimension struct {
	ID                int64   `json:"id"`
	Name              string  `json:"name"`
	ChargedRequests   int64   `json:"charged_requests"`
	Claims            int64   `json:"claims"`
	RefundAmount      float64 `json:"refund_amount"`
	EmptyResponseRate float64 `json:"empty_response_rate"`
}

type EmptyResponseClaimMetrics struct {
	TotalChargedRequests int64                               `json:"total_charged_requests"`
	TotalClaims          int64                               `json:"total_claims"`
	CompensatedClaims    int64                               `json:"compensated_claims"`
	ManualReviewClaims   int64                               `json:"manual_review_claims"`
	RejectedClaims       int64                               `json:"rejected_claims"`
	TotalRefundAmount    float64                             `json:"total_refund_amount"`
	EmptyResponseRate    float64                             `json:"empty_response_rate"`
	ByGroup              []EmptyResponseClaimMetricDimension `json:"by_group"`
	ByAccount            []EmptyResponseClaimMetricDimension `json:"by_account"`
	ByModel              []EmptyResponseClaimMetricDimension `json:"by_model"`
}

type EmptyResponseClaimMetricsRepository interface {
	GetEmptyResponseClaimMetrics(ctx context.Context, start, end time.Time) (*EmptyResponseClaimMetrics, error)
}

type EmptyResponseClaimMetricsService struct {
	repo EmptyResponseClaimMetricsRepository
}

func NewEmptyResponseClaimMetricsService(repo EmptyResponseClaimMetricsRepository) *EmptyResponseClaimMetricsService {
	return &EmptyResponseClaimMetricsService{repo: repo}
}

func (s *EmptyResponseClaimMetricsService) Get(ctx context.Context, start, end time.Time) (*EmptyResponseClaimMetrics, error) {
	if s == nil || s.repo == nil || start.IsZero() || end.IsZero() || !end.After(start) {
		return nil, ErrEmptyResponseClaimInvalidInput
	}
	metrics, err := s.repo.GetEmptyResponseClaimMetrics(ctx, start, end)
	if err != nil {
		return nil, err
	}
	if metrics == nil {
		metrics = &EmptyResponseClaimMetrics{}
	}
	if metrics.TotalChargedRequests > 0 {
		metrics.EmptyResponseRate = float64(metrics.TotalClaims) / float64(metrics.TotalChargedRequests)
	}
	calculateEmptyResponseClaimDimensionRates(metrics.ByGroup)
	calculateEmptyResponseClaimDimensionRates(metrics.ByAccount)
	calculateEmptyResponseClaimDimensionRates(metrics.ByModel)
	return metrics, nil
}

func calculateEmptyResponseClaimDimensionRates(items []EmptyResponseClaimMetricDimension) {
	for i := range items {
		if items[i].ChargedRequests > 0 {
			items[i].EmptyResponseRate = float64(items[i].Claims) / float64(items[i].ChargedRequests)
		}
	}
}
