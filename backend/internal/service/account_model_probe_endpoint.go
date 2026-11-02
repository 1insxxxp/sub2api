package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
)

// ModelProbeNativeEndpoint describes the account dialog's default test only.
// Unknown/adaptive protocols are deliberately unclaimed for explicit routes.
func ModelProbeNativeEndpoint(a *Account, model string) string {
	if a == nil {
		return ""
	}
	rawModel := model
	model = a.GetMappedModel(model)
	if a.RoutesProtocolByInbound() {
		return probeEndpointForProtocol(a.GetAPIProtocol())
	}
	if a.IsOpenAI() {
		return probeOpenAINativeEndpoint(a, model)
	}
	if a.IsGemini() {
		return CompositeRouteEndpointGemini
	}
	if a.Platform == PlatformGrok {
		if isGrokImageGenerationModel(model) {
			return CompositeRouteEndpointImages
		}
		if isGrokVideoGenerationModel(model) {
			return ""
		}
		return CompositeRouteEndpointResponses
	}
	if a.Platform == PlatformAntigravity {
		if a.Type == AccountTypeAPIKey {
			if strings.HasPrefix(rawModel, "gemini-") {
				return CompositeRouteEndpointGemini
			}
			return CompositeRouteEndpointMessages
		}
		if platform, ok := DetectModelPlatform(rawModel); ok {
			if platform == PlatformGemini {
				return CompositeRouteEndpointGemini
			}
			if platform == PlatformAnthropic {
				return CompositeRouteEndpointMessages
			}
		}
		return ""
	}
	if a.routesByModel() || a.IsTypeSafe() {
		return ""
	}
	return CompositeRouteEndpointMessages
}
func probeEndpointForProtocol(protocol string) string {
	switch protocol {
	case APIProtocolResponses:
		return CompositeRouteEndpointResponses
	case APIProtocolChatCompletions:
		return CompositeRouteEndpointChatCompletions
	case APIProtocolAnthropic:
		return CompositeRouteEndpointMessages
	default:
		return ""
	}
}
func probeOpenAINativeEndpoint(a *Account, model string) string {
	if isOpenAIImageModel(model) {
		return CompositeRouteEndpointImages
	}
	if a.Type == AccountTypeAPIKey && !openai_compat.ShouldUseResponsesAPI(a.Extra) {
		return CompositeRouteEndpointChatCompletions
	}
	return CompositeRouteEndpointResponses
}
