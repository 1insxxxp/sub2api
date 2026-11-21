package service

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
)

// InputTokenProtection is the immutable evidence captured from the client's
// original request. Pending means that input cannot be verified locally.
type InputTokenProtection struct {
	Limit   int  `json:"limit"`
	Pending bool `json:"pending,omitempty"`
}

type inputTokenProtectionKey struct{}

const (
	inputTokenProtectionTolerance         = 5000
	inputTokenProtectionStrictBelowTokens = 20000
)

// WithInputTokenProtection freezes a value copy so retries and detached usage
// workers cannot observe changes to the caller's evidence.
func WithInputTokenProtection(ctx context.Context, p *InputTokenProtection) context.Context {
	if p == nil {
		return context.WithValue(ctx, inputTokenProtectionKey{}, (*InputTokenProtection)(nil))
	}
	snapshot := *p
	return context.WithValue(ctx, inputTokenProtectionKey{}, &snapshot)
}

func InputTokenProtectionFromContext(ctx context.Context) *InputTokenProtection {
	if ctx == nil {
		return nil
	}
	p, _ := ctx.Value(inputTokenProtectionKey{}).(*InputTokenProtection)
	return p
}

// Cap keeps reported usage within the tolerance. When the provider exceeds
// the estimate by more, retain writes first, then ordinary input, then reads.
func (p *InputTokenProtection) Cap(read, input, write5m, write1h int) (int, int, int, int) {
	if !p.hasEstimate() || !p.exceedsTolerance(read, input, write5m, write1h) {
		return read, input, write5m, write1h
	}
	remaining := p.Limit
	take := func(n int) int {
		n = min(remaining, max(n, 0))
		remaining -= n
		return n
	}
	short, long := take(write5m), take(write1h)
	i := take(input)
	return take(read), i, short, long
}

func (p *InputTokenProtection) hasEstimate() bool {
	return p != nil && !p.Pending && p.Limit >= 0
}

func (p *InputTokenProtection) exceedsTolerance(counts ...int) bool {
	remaining, tolerance := p.Limit, inputTokenProtectionTolerance
	if remaining < inputTokenProtectionStrictBelowTokens {
		tolerance = 0
	}
	for _, count := range counts {
		count = max(count, 0)
		within := min(remaining, count)
		remaining -= within
		excess := count - within
		if excess > tolerance {
			return true
		}
		tolerance -= excess
	}
	return false
}

// InputTokenProtectionBillsByTokens checks the wire tariff before an account
// model is available. Settlement always receives the original evidence.
func (s *OpenAIGatewayService) InputTokenProtectionBillsByTokens(ctx context.Context, key *APIKey, model string) bool {
	return s.InputTokenProtectionAppliesToTariff(ctx, key, model, false)
}

func (s *OpenAIGatewayService) InputTokenProtectionAppliesToTariff(ctx context.Context, key *APIKey, model string, imageEndpoint bool) bool {
	return s.InputTokenProtectionWireBillsByTokens(ctx, key, model, "", imageEndpoint)
}

// InputTokenProtectionWireBillsByTokens selects the tariff when customer usage is
// written. upstreamModel is the model actually dispatched for this attempt.
func (s *OpenAIGatewayService) InputTokenProtectionWireBillsByTokens(ctx context.Context, key *APIKey, requestedModel, upstreamModel string, imageEndpoint bool) bool {
	return s.InputTokenProtectionProjectsUsage(ctx, key, requestedModel, InputTokenProtectionAttempt{UpstreamModel: upstreamModel}, imageEndpoint)
}

// InputTokenProtectionAttempt contains only observations available before the
// customer usage object is written. The public response may already hide the
// upstream declaration by restoring the client's model alias.
type InputTokenProtectionAttempt struct {
	UpstreamModel    string
	ResponseModel    string
	ResponseConflict bool
	Passthrough      bool
}

func InputTokenProtectionHTTPAttempt(c *gin.Context) InputTokenProtectionAttempt {
	observer := upstreamResponseModelObserverFromContext(c)
	return InputTokenProtectionAttempt{
		UpstreamModel:    c.GetString(OpsUpstreamModelKey),
		ResponseModel:    observer.Model(),
		ResponseConflict: observer.Conflict(),
		Passthrough:      c.GetBool("openai_passthrough"),
	}
}

func (s *OpenAIGatewayService) InputTokenProtectionProjectsUsage(ctx context.Context, key *APIKey, requestedModel string, attempt InputTokenProtectionAttempt, imageEndpoint bool) bool {
	if s == nil || key == nil || key.Group == nil || imageCountBillingApplies(key, imageEndpoint) {
		return false
	}
	if s.resolver == nil {
		return !imageEndpoint
	}
	source := requestedModel
	if resolved, ok := ResolvedUpstreamModelFromContext(ctx); ok {
		source = resolved
	}
	routeModel := source
	if !imageEndpoint {
		routeModel, _ = key.Group.ResolveModelAlias(source)
	}
	mapping := s.ResolveChannelMapping(ctx, key.Group.ID, routeModel)
	upstreamModel := attempt.UpstreamModel
	if upstreamModel == "" {
		upstreamModel = mapping.MappedModel
	}
	fields := mapping.ToUsageFields(requestedModel, upstreamModel)
	var billingModel string
	if _, sharedGateway := gatewayTokenRequestPricingAtFromContext(ctx); sharedGateway {
		pricing := GatewayService{resolver: s.resolver, billingService: s.billingService}
		result := ForwardResult{Model: gatewayPublicModel(ctx, mapping.MappedModel), UpstreamModel: upstreamModel}
		billingModel = pricing.recordUsageBillingModel(ctx, key, &result, fields)
	} else {
		result := OpenAIForwardResult{Model: mapping.MappedModel, UpstreamModel: upstreamModel}
		// Dedicated images and Responses passthrough leave BillingModel empty.
		// Text adapters normally set it to the account-mapped model.
		if !imageEndpoint && !attempt.Passthrough {
			result.BillingModel = upstreamModel
		}
		models := s.openAIRecordUsageBillingModels(ctx, &OpenAIRecordUsageInput{
			APIKey:             key,
			Result:             &result,
			ChannelUsageFields: fields,
		})
		billingModel = firstUsageBillingModel(models)
	}
	price := s.resolveOpenAIChannelPricing(ctx, billingModel, key)
	if price == nil {
		return !imageEndpoint
	}
	if price.Mode == BillingModeToken || price.Mode == "" {
		return true
	}
	if imageEndpoint || mapping.BillingModelSource != BillingModelSourceResponse || attempt.ResponseConflict {
		return false
	}
	// An explicit flat baseline can adopt only another explicitly configured
	// price. Once the provider has declared a flat model, a later disagreement
	// forces settlement back to the flat baseline. Until the model is known,
	// protect early counters if any applicable tariff could be metered.
	if attempt.ResponseModel != "" {
		if strings.EqualFold(attempt.ResponseModel, billingModel) {
			return false
		}
		other := s.resolveOpenAIChannelPricing(ctx, attempt.ResponseModel, key)
		return other != nil && (other.Mode == BillingModeToken || other.Mode == "")
	}
	return s.inputProtectionHasTokenTariff(ctx, key.Group)
}

func (s *OpenAIGatewayService) inputProtectionHasTokenTariff(ctx context.Context, group *Group) bool {
	hasTokenPrice := func(prices []ChannelModelPricing, platform string) bool {
		for i := range prices {
			p := &prices[i]
			if (p.BillingMode == BillingModeToken || p.BillingMode == "") && (p.Platform == "" || isPlatformPricingMatch(platform, p.Platform)) {
				return true
			}
		}
		return false
	}
	if hasTokenPrice(group.ModelPricing, group.Platform) {
		return true
	}
	if s.resolver.channelService == nil {
		return false
	}
	lookup, err := s.resolver.channelService.lookupGroupChannel(ctx, group.ID)
	return err != nil || (lookup != nil && hasTokenPrice(lookup.channel.ModelPricing, lookup.platform))
}
