package service

import "context"

// prepareInflightEstimateContext loads the policy once for both precharge and
// forwarding. The handler propagates the returned context even without a hold.
func (d inflightEstimateDeps) prepareInflightEstimateContext(ctx context.Context, key *APIKey, req InflightEstimateRequest) context.Context {
	if key == nil || req.Kind != InflightEstimateToken || d.settingService == nil ||
		!d.usesOpenAIServiceTiers(ctx, key) || openAIFastPolicySettingsFromContext(ctx) != nil {
		return ctx
	}
	settings, err := d.settingService.GetOpenAIFastPolicySettings(ctx)
	if err != nil {
		// Freeze the existing pass-through fallback for this request as well;
		// retries must not change the policy after the amount was reserved.
		settings = DefaultOpenAIFastPolicySettings()
	}
	return withOpenAIFastPolicyContext(ctx, settings)
}

func (s *GatewayService) PrepareInflightEstimateContext(ctx context.Context, key *APIKey, req InflightEstimateRequest) context.Context {
	return s.inflightEstimateDeps().prepareInflightEstimateContext(ctx, key, req)
}

func (s *OpenAIGatewayService) PrepareInflightEstimateContext(ctx context.Context, key *APIKey, req InflightEstimateRequest) context.Context {
	return s.inflightEstimateDeps().prepareInflightEstimateContext(ctx, key, req)
}
