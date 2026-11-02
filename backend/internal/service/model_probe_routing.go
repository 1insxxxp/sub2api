package service

import "context"

// ResolveModelProbeRoute preserves the same authoritative composite route used
// by a native endpoint. A model with conflicting endpoint routes cannot be
// represented by a single default account test and is explicitly unsupported.
// The endpoint scan stays together so conflict detection precedes any account
// selection; it intentionally exceeds the usual function-length guideline.
func (s *GatewayService) ResolveModelProbeRoute(ctx context.Context, g *Group, model string) (context.Context, string, string, string, error) {
	routeModel, _ := g.ResolveModelAlias(model)
	platform := g.Platform
	endpoint := CompositeRouteEndpointAny
	if platform != PlatformComposite {
		return ctx, platform, routeModel, endpoint, nil
	}
	endpoints := []string{CompositeRouteEndpointAny, CompositeRouteEndpointMessages, CompositeRouteEndpointResponses, CompositeRouteEndpointChatCompletions, CompositeRouteEndpointImages, CompositeRouteEndpointGemini, CompositeRouteEndpointEmbeddings, CompositeRouteEndpointCountTokens}
	var chosen *CompositeRouteDecision
	for _, ep := range endpoints {
		d, err := s.compositeResolver.Resolve(ctx, g.ID, routeModel, ep)
		if err != nil {
			return ctx, "", "", "", err
		}
		if !d.Matched || d.Source != CompositeRouteSourceExplicit {
			continue
		}
		if chosen != nil && (chosen.TargetPlatform != d.TargetPlatform || chosen.UpstreamModel != d.UpstreamModel) {
			return ctx, "", "", "", ErrModelProbeUnsupported
		}
		if chosen == nil {
			clone := d
			chosen = &clone
		}
	}
	if chosen == nil {
		d, _, err := s.resolveCompositeRouteDecision(ctx, g, routeModel, CompositeRouteEndpointAny)
		if err != nil {
			return ctx, "", "", "", err
		}
		if !d.Matched {
			return ctx, "", "", "", ErrNoAvailableAccounts
		}
		chosen = &d
	}
	if chosen.Endpoint == CompositeRouteEndpointEmbeddings || chosen.Endpoint == CompositeRouteEndpointCountTokens {
		return ctx, "", "", "", ErrModelProbeUnsupported
	}
	return WithCompositeRouteDecision(ctx, *chosen), chosen.TargetPlatform, chosen.UpstreamModel, chosen.Endpoint, nil
}
