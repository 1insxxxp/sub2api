package service

import (
	"context"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// SystemCustomGroupAccountAllowlist is an immutable-by-construction sorted set
// of account IDs admitted by the dynamic catalog for one source/model pair.
// The backing slice is never exposed directly.
type SystemCustomGroupAccountAllowlist struct {
	ids []int64
}

func NewSystemCustomGroupAccountAllowlist(accountIDs []int64) SystemCustomGroupAccountAllowlist {
	seen := make(map[int64]struct{}, len(accountIDs))
	ids := make([]int64, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		if accountID <= 0 {
			continue
		}
		if _, exists := seen[accountID]; exists {
			continue
		}
		seen[accountID] = struct{}{}
		ids = append(ids, accountID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return SystemCustomGroupAccountAllowlist{ids: ids}
}

func (a SystemCustomGroupAccountAllowlist) IDs() []int64 {
	return append([]int64(nil), a.ids...)
}

func (a SystemCustomGroupAccountAllowlist) Allows(accountID int64) bool {
	index := sort.Search(len(a.ids), func(i int) bool { return a.ids[i] >= accountID })
	return index < len(a.ids) && a.ids[index] == accountID
}

func (a SystemCustomGroupAccountAllowlist) Empty() bool {
	return len(a.ids) == 0
}

func (a SystemCustomGroupAccountAllowlist) clone() SystemCustomGroupAccountAllowlist {
	return SystemCustomGroupAccountAllowlist{ids: append([]int64(nil), a.ids...)}
}

// SystemCustomGroupResolution keeps the monthly subscription container used
// for billing separate from the direct source group used by request routing.
type SystemCustomGroupResolution struct {
	BillingGroupID  int64
	SourceGroupID   int64
	PublicModel     string
	SourceModel     string
	SourcePlatform  string
	AllowedAccounts SystemCustomGroupAccountAllowlist
}

// WithSystemCustomGroupResolution stores a complete system custom route and
// also updates the shared public/upstream model context consumed by existing
// logging and pricing code.
func WithSystemCustomGroupResolution(ctx context.Context, resolution SystemCustomGroupResolution) context.Context {
	if ctx == nil || resolution.BillingGroupID <= 0 || resolution.SourceGroupID <= 0 {
		return ctx
	}
	resolution.PublicModel = strings.TrimSpace(resolution.PublicModel)
	resolution.SourceModel = strings.TrimSpace(resolution.SourceModel)
	resolution.SourcePlatform = strings.TrimSpace(resolution.SourcePlatform)
	resolution.AllowedAccounts = resolution.AllowedAccounts.clone()
	if resolution.PublicModel == "" || resolution.SourceModel == "" || resolution.SourcePlatform == "" {
		return ctx
	}
	ctx = context.WithValue(ctx, ctxkey.SystemCustomGroupResolution, resolution)
	return WithCustomGroupModelResolution(ctx, resolution.PublicModel, resolution.SourceModel)
}

// SystemCustomGroupResolutionFromContext returns the container/source route
// selected for the current request.
func SystemCustomGroupResolutionFromContext(ctx context.Context) (SystemCustomGroupResolution, bool) {
	if ctx == nil {
		return SystemCustomGroupResolution{}, false
	}
	resolution, ok := ctx.Value(ctxkey.SystemCustomGroupResolution).(SystemCustomGroupResolution)
	if !ok || resolution.BillingGroupID <= 0 || resolution.SourceGroupID <= 0 ||
		strings.TrimSpace(resolution.PublicModel) == "" || strings.TrimSpace(resolution.SourceModel) == "" ||
		strings.TrimSpace(resolution.SourcePlatform) == "" {
		return SystemCustomGroupResolution{}, false
	}
	resolution.AllowedAccounts = resolution.AllowedAccounts.clone()
	return resolution, true
}

func systemCustomAccountAllowed(ctx context.Context, accountID int64) bool {
	resolution, systemCustom := SystemCustomGroupResolutionFromContext(ctx)
	if !systemCustom {
		return true
	}
	return resolution.AllowedAccounts.Allows(accountID)
}

func filterSystemCustomAllowedAccounts(ctx context.Context, accounts []Account) []Account {
	resolution, systemCustom := SystemCustomGroupResolutionFromContext(ctx)
	if !systemCustom {
		return accounts
	}
	filtered := make([]Account, 0, len(accounts))
	for i := range accounts {
		if resolution.AllowedAccounts.Allows(accounts[i].ID) {
			filtered = append(filtered, accounts[i])
		}
	}
	return filtered
}

// WithResolvedTargetPlatform stores the concrete provider chosen for a request
// made through a composite group.
func WithResolvedTargetPlatform(ctx context.Context, platform string) context.Context {
	platform = strings.TrimSpace(platform)
	if ctx == nil || platform == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxkey.ResolvedTargetPlatform, platform)
}

// ResolvedTargetPlatformFromContext returns the concrete provider chosen for
// the current request, if one was resolved.
func ResolvedTargetPlatformFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	platform, ok := ctx.Value(ctxkey.ResolvedTargetPlatform).(string)
	platform = strings.TrimSpace(platform)
	if !ok || platform == "" {
		return "", false
	}
	return platform, true
}

func WithCompositeRouteDecision(ctx context.Context, decision CompositeRouteDecision) context.Context {
	if ctx == nil || !decision.Matched {
		return ctx
	}
	ctx = WithResolvedTargetPlatform(ctx, decision.TargetPlatform)
	if model := strings.TrimSpace(decision.UpstreamModel); model != "" {
		ctx = context.WithValue(ctx, ctxkey.ResolvedUpstreamModel, model)
	}
	if model := strings.TrimSpace(decision.PublicModel); model != "" {
		ctx = context.WithValue(ctx, ctxkey.RequestedPublicModel, model)
	}
	if source := strings.TrimSpace(decision.Source); source != "" {
		ctx = context.WithValue(ctx, ctxkey.CompositeRouteSource, source)
	}
	return ctx
}

func WithCustomGroupModelResolution(ctx context.Context, publicModel, sourceModel string) context.Context {
	if ctx == nil {
		return ctx
	}
	if model := strings.TrimSpace(publicModel); model != "" {
		ctx = context.WithValue(ctx, ctxkey.RequestedPublicModel, model)
	}
	if model := strings.TrimSpace(sourceModel); model != "" {
		ctx = context.WithValue(ctx, ctxkey.ResolvedUpstreamModel, model)
	}
	return ctx
}

func CustomGroupModelResolutionFromContext(ctx context.Context) (string, string, bool) {
	publicModel, publicOK := RequestedPublicModelFromContext(ctx)
	sourceModel, sourceOK := ResolvedUpstreamModelFromContext(ctx)
	return publicModel, sourceModel, publicOK && sourceOK
}

func ResolvedUpstreamModelFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	model, ok := ctx.Value(ctxkey.ResolvedUpstreamModel).(string)
	model = strings.TrimSpace(model)
	if !ok || model == "" {
		return "", false
	}
	return model, true
}

func RequestedPublicModelFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	model, ok := ctx.Value(ctxkey.RequestedPublicModel).(string)
	model = strings.TrimSpace(model)
	if !ok || model == "" {
		return "", false
	}
	return model, true
}

func requestedModelForUsage(ctx context.Context, fallback string) string {
	if publicModel, ok := RequestedPublicModelFromContext(ctx); ok {
		return publicModel
	}
	return fallback
}

// billingModelFromOriginalUsageField converts ChannelUsageFields.OriginalModel
// from its display identity to the configured source identity for system custom
// traffic. Actual result/upstream fields must not use this helper.
func billingModelFromOriginalUsageField(ctx context.Context, model string) string {
	model = strings.TrimSpace(model)
	resolution, ok := SystemCustomGroupResolutionFromContext(ctx)
	if !ok {
		return model
	}
	return resolution.SourceModel
}

// billingModelFromMappedUsageField removes the no-op public alias produced by
// ToUsageFields when no downstream channel mapping occurred. A distinct mapped
// model is an actual pricing candidate and remains unchanged.
func billingModelFromMappedUsageField(ctx context.Context, model string) string {
	model = strings.TrimSpace(model)
	resolution, ok := SystemCustomGroupResolutionFromContext(ctx)
	if ok && strings.EqualFold(model, resolution.PublicModel) {
		return resolution.SourceModel
	}
	return model
}

func CompositeRouteSourceFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	source, ok := ctx.Value(ctxkey.CompositeRouteSource).(string)
	source = strings.TrimSpace(source)
	if !ok || source == "" {
		return "", false
	}
	return source, true
}

// DetectModelPlatform maps common public model IDs to the concrete provider
// platform used by sub2api. It intentionally returns false for ambiguous model
// names so composite groups fail closed instead of guessing.
func DetectModelPlatform(model string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if normalized == "" {
		return "", false
	}

	normalized = strings.TrimPrefix(normalized, "models/")
	if slash := strings.IndexByte(normalized, '/'); slash > 0 {
		provider := strings.TrimSpace(normalized[:slash])
		rest := strings.TrimSpace(normalized[slash+1:])
		switch provider {
		case "anthropic", "claude":
			return PlatformAnthropic, true
		case "openai", "chatgpt":
			return PlatformOpenAI, true
		case "google", "google-ai-studio", "gemini":
			return PlatformGemini, true
		case "xai", "x-ai", "grok":
			return PlatformGrok, true
		case "kimi", "moonshot":
			return PlatformKimi, true
		case "zhipu", "glm", "bigmodel":
			return PlatformZhipu, true
		case "deepseek":
			return PlatformDeepseek, true
		}
		if rest != "" {
			normalized = strings.TrimPrefix(rest, "models/")
		}
	}

	switch {
	case strings.HasPrefix(normalized, "anthropic.claude-"),
		strings.HasPrefix(normalized, "claude-"):
		return PlatformAnthropic, true
	case strings.HasPrefix(normalized, "gpt-"),
		strings.HasPrefix(normalized, "chatgpt-"),
		strings.HasPrefix(normalized, "codex-"),
		strings.HasPrefix(normalized, "text-embedding-"),
		strings.HasPrefix(normalized, "text-moderation-"),
		strings.HasPrefix(normalized, "omni-moderation-"),
		strings.HasPrefix(normalized, "dall-e-"),
		strings.HasPrefix(normalized, "gpt-image-"),
		strings.HasPrefix(normalized, "tts-"),
		strings.HasPrefix(normalized, "whisper-"),
		hasOpenAISeriesPrefix(normalized):
		return PlatformOpenAI, true
	case strings.HasPrefix(normalized, "gemini-"),
		strings.HasPrefix(normalized, "learnlm-"):
		return PlatformGemini, true
	case normalized == "grok" || strings.HasPrefix(normalized, "grok-"):
		return PlatformGrok, true
	case normalized == "k3",
		normalized == "k3-256k",
		strings.HasPrefix(normalized, "kimi-"),
		strings.HasPrefix(normalized, "moonshot-"):
		return PlatformKimi, true
	case strings.HasPrefix(normalized, "glm-"):
		return PlatformZhipu, true
	case strings.HasPrefix(normalized, "deepseek-"):
		return PlatformDeepseek, true
	default:
		return "", false
	}
}

func hasOpenAISeriesPrefix(model string) bool {
	for _, prefix := range []string{"o1", "o3", "o4", "o5"} {
		if model == prefix || strings.HasPrefix(model, prefix+"-") {
			return true
		}
	}
	return false
}

func (s *GatewayService) resolveCompositeRouteDecision(ctx context.Context, group *Group, requestedModel, endpoint string) (CompositeRouteDecision, bool, error) {
	if group == nil || group.Platform != PlatformComposite {
		return CompositeRouteDecision{}, false, nil
	}
	if platform, ok := ResolvedTargetPlatformFromContext(ctx); ok {
		upstreamModel := requestedModel
		if resolvedModel, modelOK := ResolvedUpstreamModelFromContext(ctx); modelOK {
			upstreamModel = resolvedModel
		}
		source := CompositeRouteSourceDetector
		if resolvedSource, sourceOK := CompositeRouteSourceFromContext(ctx); sourceOK {
			source = resolvedSource
		}
		return CompositeRouteDecision{
			Matched:        true,
			Source:         source,
			GroupID:        group.ID,
			PublicModel:    requestedModel,
			TargetPlatform: platform,
			UpstreamModel:  upstreamModel,
			Endpoint:       normalizeCompositeRouteEndpoint(endpoint),
		}, true, nil
	}
	decision, err := s.compositeResolver.Resolve(ctx, group.ID, requestedModel, endpoint)
	if err != nil {
		return decision, false, err
	}
	return decision, decision.Matched, nil
}

func isConcreteRequestPlatform(platform string) bool {
	switch platform {
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformGrok,
		PlatformKimi, PlatformZhipu, PlatformDeepseek:
		return true
	default:
		return false
	}
}
