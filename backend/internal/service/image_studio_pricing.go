package service

import (
	"context"
	"fmt"
	"strings"
)

// ImageStudioCostEstimator is optional so alternate executors retain their
// existing estimates. It must not create an API key or perform a charge.
type ImageStudioCostEstimator interface {
	EstimateImageStudioCost(context.Context, int64, *Group, string, string, int) (float64, error)
}

type imageStudioUsageCostEstimator interface {
	EstimateImageStudioUsageCost(context.Context, *OpenAIRecordUsageInput) (float64, error)
}

func (e *ImageStudioGatewayExecutor) EstimateImageStudioCost(ctx context.Context, userID int64, group *Group, model, size string, count int) (float64, error) {
	if estimator, ok := e.gateway.(ImageStudioCostEstimator); ok {
		return estimator.EstimateImageStudioCost(ctx, userID, group, model, size, count)
	}
	return estimateImageStudioCost(group, size) * float64(count), nil
}

// EstimateImageStudioCost uses the selected, already-authorized group directly.
// GET options must never call the API-key provider, which may create a key.
// Before account selection the upstream account mapping is unknown; requested
// and channel-mapped models use the same billing precedence as RecordUsage.
func (s *OpenAIGatewayService) EstimateImageStudioCost(ctx context.Context, userID int64, group *Group, model, size string, count int) (float64, error) {
	if group == nil {
		return estimateImageStudioCost(nil, size) * float64(count), nil
	}
	key := &APIKey{UserID: userID, User: &User{ID: userID}, GroupID: &group.ID, Group: group}
	input := &OpenAIRecordUsageInput{
		APIKey: key, User: key.User,
		Result: &OpenAIForwardResult{Model: model, ImageSize: size, ImageCount: count},
	}
	if imageStudioShouldUseGeminiNative(group, model) {
		// Native Gemini execution does not apply the OpenAI channel mapping.
		input.Result.UpstreamModel = imageStudioGeminiModelForUpstream(model)
		input.ChannelUsageFields = ChannelUsageFields{OriginalModel: model, ChannelMappedModel: input.Result.UpstreamModel, BillingModelSource: BillingModelSourceRequested}
	} else {
		mapping, _ := s.ResolveChannelMappingAndRestrict(ctx, key.GroupID, model)
		input.Result.UpstreamModel = firstNonEmptyTrimmed(mapping.MappedModel, model)
		input.ChannelUsageFields = mapping.ToUsageFields(model, mapping.MappedModel)
	}
	models := s.openAIRecordUsageBillingModels(ctx, input)
	if resolved := s.resolveOpenAIChannelPricing(ctx, firstUsageBillingModel(models), key); resolved != nil && resolved.Mode == BillingModeToken {
		// Token-billed image generation cannot be quoted exactly before its token
		// usage exists. Preserve the legacy estimate; completion uses real usage.
		multiplier := s.ResolveUserGroupRateMultiplier(ctx, userID, group.ID, group.RateMultiplier)
		copyGroup := *group
		copyGroup.RateMultiplier = 1
		copyGroup.ImageRateIndependent = false
		return estimateImageStudioCost(&copyGroup, size) * resolveImageRateMultiplier(key, multiplier) * float64(count), nil
	}
	return s.EstimateImageStudioUsageCost(ctx, input)
}

// EstimateImageStudioUsageCost shares the final model, token/image mode and
// multiplier calculation with RecordUsage, without applying billing effects.
func (s *OpenAIGatewayService) EstimateImageStudioUsageCost(ctx context.Context, input *OpenAIRecordUsageInput) (float64, error) {
	if input == nil || input.Result == nil || input.APIKey == nil || input.User == nil || s == nil || s.billingService == nil {
		return 0, fmt.Errorf("image studio pricing context is missing")
	}
	result, key := input.Result, input.APIKey
	if result.ImageCount <= 0 {
		return 0, nil
	}
	baseMultiplier := 1.0
	if s.cfg != nil {
		baseMultiplier = s.cfg.Default.RateMultiplier
	}
	if key.GroupID != nil && key.Group != nil {
		baseMultiplier = s.ResolveUserGroupRateMultiplier(ctx, input.User.ID, *key.GroupID, key.Group.RateMultiplier)
	}
	pricingAt := openAIUsagePricingAt(input)
	multiplier, imageMultiplier := computePeakAwareMultipliers(key, baseMultiplier, pricingAt)
	tokens := UsageTokens{
		InputTokens:         max(result.Usage.InputTokens-result.Usage.CacheReadInputTokens-result.Usage.CacheCreationInputTokens, 0),
		OutputTokens:        recordedOpenAIOutputTokens(result),
		CacheCreationTokens: result.Usage.CacheCreationInputTokens, CacheReadTokens: result.Usage.CacheReadInputTokens,
		ImageInputTokens:     max(result.Usage.ImageInputTokens-result.Usage.ImageCacheReadTokens, 0),
		ImageCacheReadTokens: result.Usage.ImageCacheReadTokens,
		ImageOutputTokens:    result.Usage.ImageOutputTokens,
	}
	serviceTier := ""
	if result.ServiceTier != nil {
		serviceTier = strings.TrimSpace(*result.ServiceTier)
	}
	cost, err := s.calculateOpenAIRecordUsageCost(ctx, result, key, s.openAIRecordUsageBillingModels(ctx, input),
		multiplier, imageMultiplier, resolveVideoRateMultiplier(key, baseMultiplier), baseMultiplier, tokens, serviceTier,
		openAILongContextBillingGate(input.Account), openAISubscriptionLongContextExempt(input.Account, result), pricingAt)
	if err != nil {
		if isUsagePricingUnavailableError(err) {
			// Ordinary RecordUsage preserves a zero-cost audit record in this case.
			return 0, nil
		}
		return 0, err
	}
	if err := ValidateUsageBillingActualCost(cost.ActualCost); err != nil {
		return 0, err
	}
	return QuantizeUsageBillingAmount(cost.ActualCost), nil
}

func (e *ImageStudioGatewayExecutor) imageStudioExecutionCost(ctx context.Context, input *OpenAIRecordUsageInput) (float64, error) {
	if estimator, ok := e.gateway.(imageStudioUsageCostEstimator); ok {
		return estimator.EstimateImageStudioUsageCost(ctx, input)
	}
	return estimateImageStudioGatewayCost(input.APIKey, input.Result), nil
}

func (s *ImageStudioService) imageStudioModelPrices(ctx context.Context, userID int64, group *Group, model string) ([]ImageStudioQualityOption, []ImageStudioPricePreviewItem, error) {
	estimator, ok := s.executor.(ImageStudioCostEstimator)
	if !ok {
		return nil, nil, nil
	}
	qualities := SupportedImageStudioQualities()
	for i := range qualities {
		cost, err := estimator.EstimateImageStudioCost(ctx, userID, group, model, qualities[i].BillingTier, 1)
		if err != nil {
			return nil, nil, err
		}
		qualities[i].EstimatedCost = cost
	}
	// Each ratio uses the selected quality's billing tier. Reuse the three
	// quotes rather than performing a resolver lookup for every ratio.
	prices := imageStudioPricePreviewItems(group, qualities)
	for i := range prices {
		for _, quality := range qualities {
			if prices[i].BillingTier == quality.BillingTier {
				prices[i].EstimatedCost = quality.EstimatedCost
				break
			}
		}
	}
	return qualities, prices, nil
}
