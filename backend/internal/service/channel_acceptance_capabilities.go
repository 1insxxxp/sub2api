package service

import (
	"slices"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/typesafe"
)

func acceptanceAccountReason(a *Account) string {
	switch {
	case a == nil:
		return "账号不存在"
	case a.IsShadow():
		return "影子账号使用母账号凭据，无法独立固定验收来源"
	case a.IsOpenAIAgentIdentity():
		return "代理身份账号需要注册或续期任务，不能在只读验收中执行"
	case a.Platform == PlatformComposite:
		return "组合分组本身不是上游账号，请选择绑定的具体账号"
	case len(acceptancePlatformProtocols(a.Platform, a.Type, "")) == 0:
		return "该平台与凭据类型没有可固定账号的文本转发接口"
	}
	return ""
}

func acceptanceModelModeReason(platform, model string) string {
	if !acceptanceConcreteID(model) {
		return "缺少可直接请求的具体模型名"
	}
	if platform == PlatformTypeSafe && model != typesafe.JevLatestModel {
		return "System One 仅接受 jev-latest，不支持该模型或别名"
	}
	if platform == PlatformOpenCodeGo && IsOpenCodeUnsupportedModel(model) {
		return "此 OpenCode 模型仅支持 Google SDK 或 System One 专用接口"
	}
	id := strings.ToLower(lastOpenAIModelSegment(model))
	switch {
	case strings.Contains(id, "image"), strings.HasPrefix(id, "imagen"), strings.HasPrefix(id, "dall-e"), strings.HasPrefix(id, "nai-diffusion"), strings.HasPrefix(id, "cogview"), strings.HasPrefix(id, "grok-imagine") && !strings.Contains(id, "video"):
		return "图片生成或编辑模型需要图片专用验收，不发送文本样本"
	case strings.Contains(id, "video"), strings.HasPrefix(id, "veo"), strings.HasPrefix(id, "sora"), strings.HasPrefix(id, "hailuo"):
		return "视频模型需要视频专用验收，不发送文本样本"
	case strings.Contains(id, "embedding"), strings.Contains(id, "rerank"), strings.Contains(id, "moderation"):
		return "向量、重排或审核模型不提供对话生成接口"
	case strings.Contains(id, "-live"), strings.HasPrefix(id, "live-"), strings.HasPrefix(id, "lyria"), strings.HasPrefix(id, "music-"), strings.HasPrefix(id, "speech-"), !modelProbeSupportsDefaultMode(id):
		return "该模型使用语音、实时或其他专用接口，不适用文本验收"
	}
	return ""
}

func acceptanceConcreteID(model string) bool {
	return model != "" && len(model) <= 200 && strings.TrimSpace(model) == model && !strings.ContainsAny(model, "\r\n\x00*?")
}

func acceptanceProtocols(platform, accountType, model string) []string {
	if acceptanceModelModeReason(platform, model) != "" {
		return nil
	}
	return acceptancePlatformProtocols(platform, accountType, model)
}

func acceptancePlatformProtocols(platform, accountType, model string) []string {
	switch platform {
	case PlatformTypeSafe:
		if accountType == AccountTypeAPIKey {
			return []string{"systemone"}
		}
	case PlatformAnthropic:
		switch accountType {
		case AccountTypeAPIKey, AccountTypeOAuth, AccountTypeSetupToken, AccountTypeServiceAccount:
			return []string{"messages", "chat", "responses"}
		case AccountTypeBedrock:
			return []string{"messages"}
		}
	case PlatformGemini:
		if accountType == AccountTypeAPIKey || accountType == AccountTypeOAuth || accountType == AccountTypeServiceAccount {
			return []string{"gemini", "chat", "messages"}
		}
	case PlatformAntigravity:
		if accountType == AccountTypeUpstream {
			return []string{"messages"}
		}
		if accountType == AccountTypeAPIKey || accountType == AccountTypeOAuth {
			if isAntigravityGeminiModel(model) {
				return []string{"gemini", "chat", "messages", "responses"}
			}
			return []string{"messages", "chat", "responses"}
		}
	case PlatformOpenAI:
		if accountType == AccountTypeAPIKey || accountType == AccountTypeOAuth || accountType == AccountTypeSetupToken {
			return []string{"responses", "chat", "messages"}
		}
	case PlatformGrok:
		if accountType == AccountTypeAPIKey || accountType == AccountTypeOAuth {
			return []string{"responses", "chat", "messages"}
		}
	default:
		if IsMultiProtocolAPIKeyProvider(platform) && accountType == AccountTypeAPIKey {
			return []string{"chat", "responses", "messages"}
		}
	}
	return nil
}

func acceptanceDispatchedModel(a *Account, requested string) string {
	switch {
	case a.IsTypeSafe() || a.Platform == PlatformAntigravity && a.Type == AccountTypeUpstream:
		return requested
	case a.IsBedrock():
		model, _ := ResolveBedrockModelID(a, requested)
		return model
	case a.Platform == PlatformAnthropic && a.Type == AccountTypeServiceAccount:
		if model, ok := a.ResolveMappedModel(requested); ok {
			return model
		}
		return normalizeVertexAnthropicModelID(claude.NormalizeModelID(requested))
	case a.Platform == PlatformAnthropic && (a.Type == AccountTypeOAuth || a.Type == AccountTypeSetupToken):
		return claude.NormalizeModelID(requested)
	case a.Platform == PlatformGemini && a.Type == AccountTypeOAuth:
		return requested
	case a.Platform == PlatformAntigravity:
		return mapAntigravityModel(a, requested)
	case domain.UsesOpenAIGateway(a.Platform) && a.Platform != PlatformGrok:
		return resolveOpenAIAccountUpstreamModelForRequest(a, requested, false)
	default:
		return a.GetMappedModel(requested)
	}
}

func acceptanceModelPlan(a *Account, requested string, group *Group) AcceptanceModel {
	model := AcceptanceModel{Model: requested, UpstreamModel: acceptanceDispatchedModel(a, requested), Protocols: []string{}}
	if reason := acceptanceModelModeReason(a.Platform, model.UpstreamModel); reason != "" {
		model.UnavailableReason = reason
		return model
	}
	protocols := acceptanceProtocols(a.Platform, a.Type, model.UpstreamModel)
	if a.Platform == PlatformAntigravity && a.Type == AccountTypeAPIKey && strings.TrimSpace(a.GetCredential("base_url")) == "" {
		protocols = slices.DeleteFunc(protocols, func(p string) bool { return p != "gemini" })
		if len(protocols) == 0 {
			model.UnavailableReason = "此渠道未配置转发地址，不能向默认端点发送中转密钥"
			return model
		}
	}
	if a.Platform == PlatformOpenAI && group != nil && !group.AllowMessagesDispatch {
		protocols = slices.DeleteFunc(protocols, func(p string) bool { return p == "messages" })
	}
	if domain.UsesOpenAIGateway(a.Platform) {
		primary := ModelProbeNativeEndpoint(a, requested)
		if a.routesByModel() {
			primary = probeEndpointForProtocol(a.resolveModelRoutedProtocol(model.UpstreamModel))
		}
		if a.IsCline() {
			primary = "chat"
		}
		if primary == CompositeRouteEndpointChatCompletions {
			primary = "chat"
		}
		if index := slices.Index(protocols, primary); index > 0 {
			copy(protocols[1:index+1], protocols[:index])
			protocols[0] = primary
		}
	}
	model.Protocols = protocols
	if len(protocols) == 0 {
		model.UnavailableReason = "该模型没有可用的文本验收协议"
	}
	return model
}

func acceptanceDefaultModelIDs(a *Account) []string {
	if a.IsBedrock() {
		ids := make([]string, 0, len(domain.DefaultBedrockModelMapping))
		for id := range domain.DefaultBedrockModelMapping {
			ids = append(ids, id)
		}
		return ids
	}
	switch a.Platform {
	case PlatformDeepseek:
		return deepseekServableModels
	case PlatformKimi:
		if a.GetCredential("account_mode") == AccountModeCoding {
			return []string{"kimi-for-coding", "k3", "k3-256k"}
		}
		return []string{"kimi-k3", "kimi-k2.6", "kimi-latest", "moonshot-v1-8k", "moonshot-v1-32k", "moonshot-v1-128k"}
	case PlatformZhipu:
		return []string{"glm-5.3", "glm-5.3-flash", "glm-5.2", "glm-5.1", "glm-4.7", "cogview-3", "cogvideo"}
	case PlatformMiniMax:
		return []string{"MiniMax-M3", "MiniMax-M2.7", "MiniMax-M2.7-highspeed", "MiniMax-M2.5", "MiniMax-M2.5-highspeed"}
	case PlatformCline, PlatformCommandCode:
		return []string{a.providerDefaultTestModel()}
	default:
		return DefaultModelIDsForPlatform(a.Platform)
	}
}

func acceptanceModelIDs(a *Account, groupModels []string) []string {
	if a.IsTypeSafe() {
		if isModelSupportedByAccount(a, typesafe.JevLatestModel) {
			return []string{typesafe.JevLatestModel}
		}
		return nil
	}
	mapping := a.GetModelMapping()
	candidates := make(map[string]bool)
	defaults := len(mapping) == 0 || (a.IsOpenAIPassthroughEnabled() && !a.IsOpenAIPassthroughModelFilterEnabled())
	for id, upstream := range mapping {
		if strings.HasSuffix(id, "*") {
			defaults = true
			candidates[upstream] = true
		} else {
			candidates[id] = true
		}
	}
	if defaults {
		for _, id := range acceptanceDefaultModelIDs(a) {
			candidates[id] = true
		}
	}
	for _, id := range groupModels {
		if mappingSupportsRequestedModel(mapping, id) {
			candidates[id] = true
		}
	}
	ids := make([]string, 0, len(candidates))
	for id := range candidates {
		if acceptanceConcreteID(id) && isModelSupportedByAccount(a, id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
