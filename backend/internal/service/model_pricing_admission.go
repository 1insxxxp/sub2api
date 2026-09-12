package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

type pricingAdmissionRequestCtxKey struct{}

type pricingAdmissionRequest struct {
	groupID        *int64
	requestedModel string
}

// ensureModelPricingAvailable verifies that at least one effective billing-model
// candidate can be priced before an upstream request is attempted.
func (s *GatewayService) ensureModelPricingAvailable(ctx context.Context, groupID *int64, models ...string) error {
	if s == nil {
		return nil
	}
	return ensureModelPricingAvailableWithServices(ctx, s.resolver, s.billingService, groupID, pricingAdmissionGroupFromContext(ctx, groupID), models...)
}

func ensureModelPricingAvailableWithServices(ctx context.Context, resolver *ModelPricingResolver, billingService *BillingService, groupID *int64, group *Group, models ...string) error {
	if resolver == nil && billingService == nil {
		return nil
	}
	if strings.TrimSpace(strings.Join(models, "")) == "" {
		return fmt.Errorf("%w: empty model", ErrModelPricingUnavailable)
	}
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		key := strings.ToLower(model)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		if resolver != nil {
			resolved := resolver.Resolve(ctx, PricingInput{Model: model, GroupID: groupID, Group: group})
			if evaluateEffectiveResolvedPricing(resolved).usable {
				return nil
			}
			continue
		}
		if billingService != nil {
			if _, err := billingService.GetModelPricing(model); err == nil {
				return nil
			}
		}
	}

	return fmt.Errorf("%w for models: %s", ErrModelPricingUnavailable, strings.Join(models, ", "))
}

func (s *OpenAIGatewayService) validateSelectedAccountPricing(ctx context.Context, groupID *int64, requestedModel string, account *Account) error {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" || account == nil || s == nil || s.billingService == nil {
		return nil
	}
	mapping := ChannelMappingResult{MappedModel: requestedModel}
	if groupID != nil {
		mapping = s.ResolveChannelMapping(ctx, *groupID, requestedModel)
	}
	routingModel := strings.TrimSpace(mapping.MappedModel)
	if routingModel == "" {
		routingModel = requestedModel
	}
	concreteModel := account.GetMappedModel(routingModel)
	return ensureModelPricingAvailableWithServices(ctx, s.resolver, s.billingService, groupID, pricingAdmissionGroupFromContext(ctx, groupID), requestedModel, routingModel, concreteModel)
}

func (s *GatewayService) validateSelectedAccountPricing(ctx context.Context, groupID *int64, requestedModel string, account *Account) error {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" || account == nil || s == nil || s.billingService == nil {
		return nil
	}
	mapping := ChannelMappingResult{MappedModel: requestedModel}
	if groupID != nil {
		mapping = s.ResolveChannelMapping(ctx, *groupID, requestedModel)
	}
	routingModel := strings.TrimSpace(mapping.MappedModel)
	if routingModel == "" {
		routingModel = requestedModel
	}
	concreteModel := resolveAccountUpstreamModel(account, routingModel)
	return s.ensureModelPricingAvailable(ctx, groupID, requestedModel, routingModel, concreteModel)
}

func withPricingAdmissionRequest(ctx context.Context, groupID *int64, requestedModel string) context.Context {
	if ctx == nil || strings.TrimSpace(requestedModel) == "" {
		return ctx
	}
	return context.WithValue(ctx, pricingAdmissionRequestCtxKey{}, pricingAdmissionRequest{
		groupID:        groupID,
		requestedModel: requestedModel,
	})
}

func pricingAdmissionRequestFromContext(ctx context.Context) (pricingAdmissionRequest, bool) {
	if ctx == nil {
		return pricingAdmissionRequest{}, false
	}
	request, ok := ctx.Value(pricingAdmissionRequestCtxKey{}).(pricingAdmissionRequest)
	return request, ok && strings.TrimSpace(request.requestedModel) != ""
}

func pricingAdmissionGroupFromContext(ctx context.Context, groupID *int64) *Group {
	if ctx == nil || groupID == nil {
		return nil
	}
	group, ok := ctx.Value(ctxkey.Group).(*Group)
	if !ok || !IsGroupContextValid(group) || group.ID != *groupID {
		return nil
	}
	return group
}

//nolint:unused // retained as a focused unit-test predicate for effective channel pricing.
func resolvedChannelPricingUsable(resolved *ResolvedPricing) bool {
	if resolved == nil || resolved.Source != PricingSourceChannel {
		return false
	}
	evaluation := evaluateEffectiveResolvedPricing(resolved)
	return evaluation.usable && evaluation.source == PricingSourceChannel
}

type effectivePricingEvaluation struct {
	usable      bool
	source      string
	billingMode BillingMode
}

func evaluateEffectiveResolvedPricing(resolved *ResolvedPricing) effectivePricingEvaluation {
	if resolved == nil {
		return effectivePricingEvaluation{}
	}
	mode := resolved.Mode
	if mode == "" {
		mode = BillingModeToken
	}
	pricing := resolved.channelPricing
	switch mode {
	case BillingModePerRequest, BillingModeImage, BillingModeVideo:
		return effectivePricingEvaluation{
			usable:      resolvedRequestPricingConfigured(resolved),
			source:      resolved.Source,
			billingMode: mode,
		}
	case BillingModeToken, "":
		if channelTokenPricingConfigured(pricing) {
			return effectivePricingEvaluation{usable: true, source: resolved.Source, billingMode: mode}
		}
		return effectivePricingEvaluation{
			usable:      modelPricingHasUsableRate(resolved.BasePricing),
			source:      resolved.BaseSource,
			billingMode: mode,
		}
	default:
		return effectivePricingEvaluation{source: resolved.Source, billingMode: mode}
	}
}

func resolvedRequestPricingConfigured(resolved *ResolvedPricing) bool {
	if resolved == nil {
		return false
	}
	if resolved.channelPricing != nil && resolved.channelPricing.PerRequestPrice != nil {
		return true
	}
	if resolved.DefaultPerRequestPrice != 0 {
		return true
	}
	for i := range resolved.RequestTiers {
		if resolved.RequestTiers[i].PerRequestPrice != nil {
			return true
		}
	}
	return false
}

func channelTokenPricingConfigured(pricing *ChannelModelPricing) bool {
	if pricing == nil {
		return false
	}
	if pricing.InputPrice != nil || pricing.OutputPrice != nil ||
		pricing.CacheWritePrice != nil || pricing.CacheReadPrice != nil ||
		pricing.ImageInputPrice != nil || pricing.ImageOutputPrice != nil {
		return true
	}
	for _, interval := range pricing.Intervals {
		if interval.InputPrice != nil || interval.OutputPrice != nil ||
			interval.CacheWritePrice != nil || interval.CacheReadPrice != nil {
			return true
		}
	}
	return false
}

func modelPricingHasUsableRate(pricing *ModelPricing) bool {
	if pricing == nil {
		return false
	}
	return pricing.InputPricePerToken != 0 ||
		pricing.OutputPricePerToken != 0 ||
		pricing.CacheCreationPricePerToken != 0 ||
		pricing.CacheReadPricePerToken != 0 ||
		pricing.ImageInputPricePerToken != 0 ||
		pricing.ImageOutputPricePerToken != 0
}
