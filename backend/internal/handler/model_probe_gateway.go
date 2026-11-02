package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ModelsForProbe keeps discovery scoped to the gateway's public group catalog.
func (h *GatewayHandler) ModelsForProbe(ctx context.Context, g *service.Group) ([]string, error) {
	if h == nil || h.gatewayService == nil || g == nil {
		return nil, service.ErrUserSupportUnavailable
	}
	if g.IsSystemCustomRouteGroup() {
		if h.apiKeyService == nil {
			return nil, service.ErrUserSupportUnavailable
		}
		return h.apiKeyService.ListSystemCustomGroupModels(ctx, &service.APIKey{Group: g, GroupID: &g.ID}, "")
	}
	if g.Platform == service.PlatformOpenAI && g.CodexModelsManifestConfig.Enabled {
		return h.pinnedModelsForProbe(ctx, g)
	}
	models := h.localModelsForProbe(ctx, g)
	if g.ModelAllowlistEnabled() {
		models = g.EffectiveModelAllowlist().FilterForListing(models)
	}
	return models, nil
}

// Pinned discovery retains configured membership, cache and fallback policy.
func (h *GatewayHandler) pinnedModelsForProbe(ctx context.Context, g *service.Group) ([]string, error) {
	if h.openAIGatewayService == nil {
		return nil, service.ErrUserSupportUnavailable
	}
	manifest, _, err := h.openAIGatewayService.FetchPinnedOpenAIModelsList(ctx, g, h.maxAccountSwitches, "")
	if err != nil || manifest == nil {
		return nil, service.ErrUserSupportUnavailable
	}
	body, _ := appendOpenAIManifestModelAliases(manifest.Body, g)
	body, _ = filterOpenAIManifestModelAllowlist(body, g)
	var catalog struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &catalog) != nil {
		return nil, service.ErrUserSupportUnavailable
	}
	models := make([]string, 0, len(catalog.Data))
	for _, m := range catalog.Data {
		models = append(models, m.ID)
	}
	return models, nil
}
func (h *GatewayHandler) localModelsForProbe(ctx context.Context, g *service.Group) []string {
	var models []string
	switch g.Platform {
	case service.PlatformComposite:
		models = h.compositeAvailableModels(ctx, &g.ID, "", true)
		if len(models) == 0 {
			models = defaultModelIDsForPlatform(g.Platform)
		}
	case service.PlatformGemini:
		models = geminiPublicModelListing(h.geminiUpstreamAvailableModelIDsForGroup(ctx, g), defaultModelIDsForPlatform(g.Platform), g)
	default:
		models = h.gatewayService.GetAvailableModels(ctx, &g.ID, g.Platform)
		models = appendAvailableGroupModelAliases(models, g)
		if len(models) == 0 && (g.Platform != service.PlatformOpenAI || models == nil) {
			models = defaultModelIDsForPlatform(g.Platform)
		}
	}
	return models
}

type modelProbeRoute struct {
	ctx                                                       context.Context
	billingGroup, group                                       *service.Group
	publicModel, routeModel, forwardModel, platform, endpoint string
	imageIntent                                               bool
}

func (h *GatewayHandler) SelectForProbe(ctx context.Context, g *service.Group, model string) (*service.ModelProbeSelection, error) {
	if h == nil || h.gatewayService == nil || g == nil {
		return nil, service.ErrUserSupportUnavailable
	}
	route, err := h.prepareModelProbe(ctx, g, model)
	if err != nil {
		return nil, err
	}
	selected, err := h.selectModelProbeAccount(route)
	if err != nil || selected == nil || selected.Account == nil {
		releaseProbeSelection(selected)
		if err == nil {
			err = service.ErrNoAvailableAccounts
		}
		return nil, err
	}
	route.ctx = service.ContextWithSelectionProfitGate(route.ctx, selected)
	release, err := h.acquireModelProbeSlot(route.ctx, selected)
	if err != nil {
		return nil, err
	}
	release = wrapReleaseOnDone(route.ctx, release)
	result, err := h.finishModelProbeSelection(route, selected.Account, release)
	if err != nil && release != nil {
		release()
	}
	return result, err
}
func probeGroupContext(ctx context.Context, g *service.Group) (context.Context, error) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
	if !middleware.SetAPIKeyAndGroupContext(c, &service.APIKey{Group: g, GroupID: &g.ID}) {
		return ctx, service.ErrNoAvailableAccounts
	}
	return c.Request.Context(), nil
}
func (h *GatewayHandler) probeSource(ctx context.Context, g *service.Group, model string) (context.Context, *service.Group, string, error) {
	if !g.IsSystemCustomRouteGroup() {
		return ctx, g, model, nil
	}
	if h.apiKeyService == nil {
		return ctx, nil, "", service.ErrUserSupportUnavailable
	}
	resolution, err := h.apiKeyService.ResolveSystemCustomGroupModel(ctx, &service.APIKey{Group: g, GroupID: &g.ID}, model)
	if err != nil || resolution == nil || resolution.APIKey == nil {
		return ctx, nil, "", service.ErrNoAvailableAccounts
	}
	return service.WithSystemCustomGroupResolution(ctx, resolution.SystemCustomGroupResolution), resolution.APIKey.Group, resolution.SourceModel, nil
}
func (h *GatewayHandler) prepareModelProbe(ctx context.Context, billing *service.Group, public string) (*modelProbeRoute, error) {
	ctx, err := probeGroupContext(ctx, billing)
	if err != nil {
		return nil, err
	}
	ctx, g, model, err := h.probeSource(ctx, billing, public)
	if err != nil {
		return nil, err
	}
	ctx, platform, routeModel, endpoint, err := h.gatewayService.ResolveModelProbeRoute(ctx, g, model)
	if err != nil {
		return nil, err
	}
	ctx = service.WithGatewayPublicModel(ctx, public)
	mapping, restricted := h.gatewayService.ResolveChannelMappingAndRestrict(ctx, &g.ID, routeModel)
	if restricted {
		return nil, service.ErrNoAvailableAccounts
	}
	imageIntent := service.IsImageGenerationIntent("", mapping.MappedModel, nil)
	// Freeze pricing against the original billing group before installing a source.
	if !domain.UsesOpenAIGateway(platform) || !imageIntent {
		ctx, _ = service.WithGatewayTokenRequestPricing(ctx)
	}
	if g.ClaudeCodeOnly {
		ctx = service.SetClaudeCodeClient(ctx, true)
	}
	ctx, err = probeGroupContext(ctx, g)
	if err != nil {
		return nil, err
	}
	if probeImageDenied(billing, public, routeModel, mapping.MappedModel) {
		return nil, service.ErrModelProbeUnsupported
	}
	return &modelProbeRoute{ctx, billing, g, public, routeModel, mapping.MappedModel, platform, endpoint, imageIntent}, nil
}
func (h *GatewayHandler) selectModelProbeAccount(r *modelProbeRoute) (*service.AccountSelectionResult, error) {
	if !domain.UsesOpenAIGateway(r.platform) {
		return h.gatewayService.SelectAccountWithLoadAwareness(r.ctx, &r.group.ID, "", r.routeModel, nil, "", 0)
	}
	if h.openAIGatewayService == nil {
		return nil, service.ErrNoAvailableAccounts
	}
	r.ctx = service.WithOpenAIForwardModel(r.ctx, r.forwardModel, false)
	if service.IsGPTImageGenerationModel(r.forwardModel) || service.IsNovelAIImageGenerationModel(r.forwardModel) {
		v, _, err := h.openAIGatewayService.SelectAccountWithSchedulerForImages(r.ctx, &r.group.ID, "", r.forwardModel, nil, service.OpenAIImagesCapabilityNative)
		return v, err
	}
	capability := service.OpenAIEndpointCapability("")
	switch r.endpoint {
	case service.CompositeRouteEndpointResponses:
		capability = service.OpenAIEndpointCapabilityResponses
	case service.CompositeRouteEndpointChatCompletions:
		capability = service.OpenAIEndpointCapabilityChatCompletions
	}

	if r.platform == service.PlatformGrok && (strings.Contains(strings.ToLower(r.forwardModel), "imagine") || strings.Contains(strings.ToLower(r.forwardModel), "video")) {
		capability = service.OpenAIEndpointCapabilityGrokMediaGeneration
		r.ctx = service.WithOpenAIProfitControlSuppressed(r.ctx)
	}
	v, _, err := h.openAIGatewayService.SelectAccountWithSchedulerForCapability(r.ctx, &r.group.ID, "", "", r.forwardModel, nil, service.OpenAIUpstreamTransportHTTPSSE, capability, false, false, !r.imageIntent, r.platform)
	return v, err
}
func releaseProbeSelection(s *service.AccountSelectionResult) {
	if s != nil && s.ReleaseFunc != nil {
		s.ReleaseFunc()
	}
}
func (h *GatewayHandler) acquireModelProbeSlot(ctx context.Context, s *service.AccountSelectionResult) (func(), error) {
	if s.Acquired {
		return s.ReleaseFunc, nil
	}
	if s.WaitPlan == nil || h.concurrencyHelper == nil {
		return nil, service.ErrNoAvailableAccounts
	}
	p := s.WaitPlan
	canWait, err := h.concurrencyHelper.IncrementAccountWaitCount(ctx, s.Account.ID, p.MaxWaiting)
	if err != nil || !canWait {
		return nil, service.ErrNoAvailableAccounts
	}
	defer h.concurrencyHelper.DecrementAccountWaitCount(ctx, s.Account.ID)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
	streamStarted := false
	return h.concurrencyHelper.AcquireAccountSlotWithWaitTimeout(c, s.Account.ID, p.MaxConcurrency, p.Timeout, false, &streamStarted)
}
func (h *GatewayHandler) finishModelProbeSelection(r *modelProbeRoute, a *service.Account, release func()) (*service.ModelProbeSelection, error) {
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	account, vetoed, _ := h.gatewayService.GatewayProfitControlVetoLatest(r.ctx, a)
	if domain.UsesOpenAIGateway(r.platform) {
		account, vetoed, _ = h.openAIGatewayService.ProfitControlVetoLatest(r.ctx, a)
	}
	if vetoed || account == nil {
		return nil, service.ErrNoAvailableAccounts
	}
	if r.endpoint != service.CompositeRouteEndpointAny && service.ModelProbeNativeEndpoint(account, r.forwardModel) != r.endpoint {
		return nil, service.ErrModelProbeUnsupported
	}
	if probeImageDenied(r.billingGroup, r.publicModel, r.routeModel, r.forwardModel, account.GetMappedModel(r.forwardModel)) {
		return nil, service.ErrModelProbeUnsupported
	}
	return &service.ModelProbeSelection{AccountID: account.ID, Model: r.forwardModel, Release: release, Context: r.ctx}, nil
}
func probeImageDenied(g *service.Group, models ...string) bool {
	if service.GeminiImageGenerationDenied(g, nil, models...) {
		return true
	}
	if service.GroupAllowsImageGeneration(g) {
		return false
	}
	for _, model := range models {
		if service.IsExplicitImageGenerationIntent("", model, nil) {
			return true
		}
	}
	return false
}
