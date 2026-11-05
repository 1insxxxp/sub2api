package service

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

type categoryDefaultDeclarationPath struct {
	Platform string
	Model    string
	Endpoint string
	Route    *CompositeModelRoute
	Native   bool
}

func categoryDefaultDeclarationPaths(source CategoryDefaultSourceSnapshot, public, target string, aliased bool) []categoryDefaultDeclarationPath {
	if source.Group.Platform != PlatformComposite {
		return []categoryDefaultDeclarationPath{{Platform: source.Group.Platform, Model: target, Endpoint: CompositeRouteEndpointAny, Native: source.Group.Platform == PlatformGemini || source.Group.Platform == PlatformAntigravity}}
	}
	paths := []categoryDefaultDeclarationPath{}
	endpoints := []string{CompositeRouteEndpointAny, CompositeRouteEndpointMessages, CompositeRouteEndpointCountTokens, CompositeRouteEndpointResponses, CompositeRouteEndpointChatCompletions, CompositeRouteEndpointGemini}
	for _, endpoint := range endpoints {
		names := []string{public}
		// Native Gemini resolves its public URL before group aliases. Other
		// handlers and account probes may resolve the alias first.
		if aliased && endpoint != CompositeRouteEndpointGemini {
			names = append(names, target)
		}
		for _, name := range names {
			if route, matched := matchCompositeRoute(source.Routes, name, endpoint); matched {
				model := strings.TrimSpace(route.UpstreamModel)
				if model == "" {
					model = name
				}
				paths = append(paths, categoryDefaultDeclarationPath{Platform: route.TargetPlatform, Model: model, Endpoint: endpoint, Route: &route, Native: endpoint == CompositeRouteEndpointGemini && name == public})
				continue
			}
			platforms := map[string]bool{}
			for _, account := range source.Accounts {
				if explicitModelMappingClaims(account, name) {
					platforms[account.Platform] = true
				}
			}
			if len(platforms) == 0 {
				if platform, ok := DetectModelPlatform(name); ok {
					platforms[platform] = true
				}
			}
			ordered := make([]string, 0, len(platforms))
			for platform := range platforms {
				ordered = append(ordered, platform)
			}
			sort.Strings(ordered)
			for _, platform := range ordered {
				// An ambiguous account declaration cannot promise a native route.
				paths = append(paths, categoryDefaultDeclarationPath{Platform: platform, Model: name, Endpoint: endpoint, Native: endpoint == CompositeRouteEndpointGemini && !aliased && len(ordered) == 1})
			}
		}
	}
	return paths
}

func categoryDefaultSourceChannelCache(source CategoryDefaultSourceSnapshot) *channelCache {
	if source.Channel == nil || !source.Channel.IsActive() {
		return nil
	}
	// Reuse the actual platform, exact and wildcard precedence without copying
	// prices or relying on the gateway's independently refreshed runtime cache.
	channel := Channel{ID: source.Channel.ID, Status: source.Channel.Status, GroupIDs: []int64{source.Group.ID}, ModelMapping: source.Channel.ModelMapping}
	return populateChannelCache([]Channel{channel}, map[int64]string{source.Group.ID: source.Group.Platform})
}

func categoryDefaultChannelTarget(cache *channelCache, groupID int64, platform, model string, declarations map[string]bool) string {
	if cache == nil {
		return model
	}
	if mapped := lookupMappingAcrossPlatforms(cache, groupID, platform, strings.ToLower(model)); mapped != "" {
		declarations["channel:"+platform+":"+model+":"+mapped] = true
		return mapped
	}
	return model
}

func categoryDefaultPathAllowsAccount(group *Group, path categoryDefaultDeclarationPath, account *Account) bool {
	if group.Platform != PlatformComposite || account.Platform == path.Platform {
		return true
	}
	return account.Platform == PlatformAntigravity && (path.Platform == PlatformGemini || path.Platform == PlatformAnthropic)
}

func categoryDefaultCollectModelDeclarations(source CategoryDefaultSourceSnapshot, public, target string, aliased bool, declarations, platforms map[string]bool) bool {
	cache := categoryDefaultSourceChannelCache(source)
	native := false
	for _, path := range categoryDefaultDeclarationPaths(source, public, target, aliased) {
		pathDeclarations := make(map[string]bool)
		if path.Route != nil {
			route := path.Route
			data, _ := json.Marshal(struct {
				Public, Match, Platform, Upstream, Endpoint string
				Priority                                    int
			}{route.PublicModel, route.MatchType, route.TargetPlatform, route.UpstreamModel, route.Endpoint, route.Priority})
			pathDeclarations["route:"+string(data)] = true
		}
		mappedTarget := categoryDefaultChannelTarget(cache, source.Group.ID, path.Platform, path.Model, pathDeclarations)
		matchedAccount := false
		for i := range source.Accounts {
			account := &source.Accounts[i]
			if !categoryDefaultPathAllowsAccount(source.Group, path, account) {
				continue
			}
			mapping := account.GetModelMapping()
			mapped := mappedTarget
			matched := len(mapping) == 0 || (account.Platform == PlatformOpenAI && account.IsOpenAIPassthroughEnabled())
			if !matched {
				mapped, matched = account.ResolveMappedModel(mappedTarget)
			}
			if !matched {
				continue
			}
			matchedAccount = true
			pathDeclarations["account:"+path.Platform+":"+account.Platform+":"+mapped] = true
			if account.Platform == PlatformOpenAI {
				compact := ResolveOpenAIAccountUpstreamModelForRequest(account, mappedTarget, true)
				pathDeclarations["account-compact:"+compact] = true
			}
			platforms[account.Platform] = true
			native = native || (path.Native && (account.Platform == PlatformGemini || (account.Platform == PlatformAntigravity && isAntigravityGeminiModel(mapped))))
		}
		if !matchedAccount && path.Route != nil {
			pathDeclarations["native:"+path.Platform+":"+mappedTarget] = true
			native = native || (path.Native && (path.Platform == PlatformGemini || (path.Platform == PlatformAntigravity && isAntigravityGeminiModel(mappedTarget))))
		}
		if matchedAccount || path.Route != nil {
			platforms[path.Platform] = true
			for declaration := range pathDeclarations {
				declarations[declaration] = true
			}
			categoryDefaultCollectMessagesDispatchDeclarations(source, path, cache, declarations)
		}
	}
	return native
}

func categoryDefaultCollectMessagesDispatchDeclarations(source CategoryDefaultSourceSnapshot, path categoryDefaultDeclarationPath, cache *channelCache, declarations map[string]bool) {
	// Messages and its count endpoint have a separate group-level fallback.
	// Keep the normal protocol's declaration too, and record only the fallback
	// that this particular model can use. An unrelated family setting or a
	// fallback shadowed by an exact account mapping is not a model change.
	grok := source.Group.Platform == PlatformGrok && path.Platform == PlatformGrok
	if (!source.Group.AllowMessagesDispatch && !grok) || (path.Platform != PlatformOpenAI && !grok) ||
		(path.Endpoint != CompositeRouteEndpointAny && path.Endpoint != CompositeRouteEndpointMessages && path.Endpoint != CompositeRouteEndpointCountTokens) {
		return
	}
	fallback := source.Group.ResolveMessagesDispatchModel(path.Model)
	if grok && source.GrokMappingOptions != nil {
		fallback = ""
		if source.GrokMappingOptions.EnableCrossClientMap && claudeMessagesDispatchFamily(path.Model) != "" {
			fallback = xai.ModelMappingWithOptions(*source.GrokMappingOptions)["claude-*"]
		}
	}
	if cache != nil {
		if mapped := lookupMappingAcrossPlatforms(cache, source.Group.ID, path.Platform, strings.ToLower(path.Model)); mapped != "" {
			fallback = mapped
		}
	}
	matchedAccount := false
	for i := range source.Accounts {
		account := &source.Accounts[i]
		if account.Platform != path.Platform || !categoryDefaultPathAllowsAccount(source.Group, path, account) {
			continue
		}
		matchedAccount = true
		mapped := resolveOpenAIForwardModel(account, path.Model, fallback)
		declarations["messages:"+path.Platform+":"+mapped] = true
	}
	if !matchedAccount {
		declarations["messages:"+path.Platform+":"+resolveOpenAIForwardModel(nil, path.Model, fallback)] = true
	}
}
