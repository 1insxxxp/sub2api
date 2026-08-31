package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// SystemCustomOpenAIRuntimeEligibilityProbe exposes only the transient account
// and account-model blocks owned by the OpenAI-compatible scheduler.
type SystemCustomOpenAIRuntimeEligibilityProbe interface {
	IsSystemCustomAccountRuntimeEligible(account *Account, requestedModel string) bool
}

func (s *GatewayService) SetSystemCustomOpenAIRuntimeEligibilityProbe(probe SystemCustomOpenAIRuntimeEligibilityProbe) {
	if s != nil {
		s.systemCustomOpenAIRuntimeProbe = probe
	}
}

type SystemCustomGroupRuntimeCandidate struct {
	SourceGroup     Group
	PublicModel     string
	SourceModel     string
	AllowedAccounts SystemCustomGroupAccountAllowlist
}

type SystemCustomGroupRuntimeCatalog struct {
	availableModels map[string]string
	candidates      map[string][]SystemCustomGroupRuntimeCandidate
	advertised      map[string]struct{}
}

// NewSystemCustomGroupRuntimeCatalog builds an immutable-by-convention catalog
// from an already validated candidate snapshot. It is primarily useful at
// narrow service boundaries and in contract tests; production discovery uses
// BuildSystemCustomGroupModelCatalog.
func NewSystemCustomGroupRuntimeCatalog(candidates []SystemCustomGroupRuntimeCandidate, advertisedModels []string) *SystemCustomGroupRuntimeCatalog {
	catalog := &SystemCustomGroupRuntimeCatalog{
		availableModels: make(map[string]string),
		candidates:      make(map[string][]SystemCustomGroupRuntimeCandidate),
		advertised:      make(map[string]struct{}),
	}
	for _, model := range advertisedModels {
		key := strings.ToLower(strings.TrimSpace(model))
		if key != "" {
			catalog.advertised[key] = struct{}{}
		}
	}
	for _, candidate := range candidates {
		candidate.AllowedAccounts = candidate.AllowedAccounts.clone()
		model := strings.TrimSpace(candidate.PublicModel)
		if model == "" {
			continue
		}
		key := strings.ToLower(model)
		catalog.advertised[key] = struct{}{}
		catalog.candidates[key] = append(catalog.candidates[key], candidate)
		if _, exists := catalog.availableModels[key]; !exists {
			catalog.availableModels[key] = model
		}
	}
	return catalog
}

func (c *SystemCustomGroupRuntimeCatalog) Models() []string {
	if c == nil || len(c.availableModels) == 0 {
		return []string{}
	}
	models := make([]string, 0, len(c.availableModels))
	for _, model := range c.availableModels {
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool {
		left, right := strings.ToLower(models[i]), strings.ToLower(models[j])
		if left == right {
			return models[i] < models[j]
		}
		return left < right
	})
	return models
}

func (c *SystemCustomGroupRuntimeCatalog) Resolve(model string) ([]SystemCustomGroupRuntimeCandidate, bool) {
	if c == nil {
		return nil, false
	}
	key := strings.ToLower(strings.TrimSpace(model))
	if key == "" {
		return nil, false
	}
	_, advertised := c.advertised[key]
	return append([]SystemCustomGroupRuntimeCandidate(nil), c.candidates[key]...), advertised
}

// BuildSystemCustomGroupModelCatalog derives the public model catalog from one
// live source-reference snapshot and one schedulable-account batch. It does not
// use the retained static route rows and deliberately does not cache the result.
func (s *GatewayService) BuildSystemCustomGroupModelCatalog(
	ctx context.Context,
	sources []SystemCustomGroupSource,
	platform string,
) (*SystemCustomGroupRuntimeCatalog, error) {
	return s.buildSystemCustomGroupModelCatalog(ctx, sources, platform, "")
}

// ResolveSystemCustomGroupModelCatalog evaluates only one requested public
// model. It shares the same source/account snapshot and admission rules as the
// full /models catalog without materializing or sorting unrelated models.
func (s *GatewayService) ResolveSystemCustomGroupModelCatalog(
	ctx context.Context,
	sources []SystemCustomGroupSource,
	platform string,
	model string,
) ([]SystemCustomGroupRuntimeCandidate, bool, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, false, nil
	}
	catalog, err := s.buildSystemCustomGroupModelCatalog(ctx, sources, platform, model)
	if err != nil {
		return nil, false, err
	}
	candidates, advertised := catalog.Resolve(model)
	return candidates, advertised, nil
}

func (s *GatewayService) buildSystemCustomGroupModelCatalog(
	ctx context.Context,
	sources []SystemCustomGroupSource,
	platform string,
	requestedModel string,
) (*SystemCustomGroupRuntimeCatalog, error) {
	catalog := NewSystemCustomGroupRuntimeCatalog(nil, nil)
	if s == nil || s.billingService == nil {
		return nil, fmt.Errorf("system custom group model catalog pricing is not configured")
	}
	if len(sources) == 0 {
		return catalog, nil
	}
	if s.accountRepo == nil {
		return nil, fmt.Errorf("system custom group model catalog is not configured")
	}
	batchRepo, ok := s.accountRepo.(systemCustomGroupSchedulableAccountRepository)
	if !ok {
		return nil, fmt.Errorf("account repository does not support system custom group snapshots")
	}

	ordered := append([]SystemCustomGroupSource(nil), sources...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Priority != ordered[j].Priority {
			return ordered[i].Priority < ordered[j].Priority
		}
		if ordered[i].ID != ordered[j].ID {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].SourceGroupID < ordered[j].SourceGroupID
	})
	platform = strings.TrimSpace(platform)
	groupIDs := make([]int64, 0, len(ordered))
	seenGroups := make(map[int64]struct{}, len(ordered))
	for _, source := range ordered {
		group := source.SourceGroup
		if group == nil || group.ID != source.SourceGroupID ||
			(platform != "" && group.Platform != platform) {
			continue
		}
		if _, exists := seenGroups[group.ID]; exists {
			continue
		}
		seenGroups[group.ID] = struct{}{}
		if isDirectSystemCustomSource(group) && IsGroupContextValid(group) {
			groupIDs = append(groupIDs, group.ID)
		}
	}

	var accounts []SystemCustomGroupSchedulableAccount
	if len(groupIDs) > 0 {
		var err error
		accounts, err = batchRepo.ListSchedulableByGroupIDs(ctx, groupIDs)
		if err != nil {
			return nil, fmt.Errorf("list system custom group schedulable accounts: %w", err)
		}
	}
	candidateSnapshot := accounts
	if s.schedulerSnapshot != nil {
		var err error
		candidateSnapshot, err = s.schedulerSnapshot.IntersectSystemCustomGroupAccounts(ctx, ordered, accounts)
		if err != nil {
			return nil, fmt.Errorf("intersect system custom scheduler snapshot: %w", err)
		}
	}
	accountsByGroup := make(map[int64][]Account, len(groupIDs))
	for i := range accounts {
		entry := accounts[i]
		accountsByGroup[entry.GroupID] = append(accountsByGroup[entry.GroupID], entry.Account)
	}
	uniqueAccountSnapshot := make([]Account, 0, len(candidateSnapshot))
	seenAccountIDs := make(map[int64]struct{}, len(candidateSnapshot))
	for i := range candidateSnapshot {
		entry := candidateSnapshot[i]
		if _, exists := seenAccountIDs[entry.Account.ID]; exists {
			continue
		}
		seenAccountIDs[entry.Account.ID] = struct{}{}
		accountCopy := entry.Account
		if accountCopy.Extra != nil {
			accountCopy.Extra = cloneSystemCustomAccountMap(accountCopy.Extra)
		}
		uniqueAccountSnapshot = append(uniqueAccountSnapshot, accountCopy)
	}
	eligibleAccounts := s.filterAccountsBySchedulingThreshold(ctx, uniqueAccountSnapshot)
	grokAccounts := make([]Account, 0, len(eligibleAccounts))
	for i := range eligibleAccounts {
		if eligibleAccounts[i].Platform == PlatformGrok {
			grokAccounts = append(grokAccounts, eligibleAccounts[i])
		}
	}
	grokAccounts = s.filterGrokFreeQuotaAccountsForGateway(ctx, grokAccounts)
	grokEligibleIDs := make(map[int64]struct{}, len(grokAccounts))
	for i := range grokAccounts {
		grokEligibleIDs[grokAccounts[i].ID] = struct{}{}
	}
	eligibleByID := make(map[int64]Account, len(eligibleAccounts))
	accountSnapshot := make([]Account, 0, len(eligibleAccounts))
	for i := range eligibleAccounts {
		account := eligibleAccounts[i]
		if account.Platform == PlatformGrok {
			if _, eligible := grokEligibleIDs[account.ID]; !eligible {
				continue
			}
		}
		eligibleByID[account.ID] = account
		accountSnapshot = append(accountSnapshot, account)
	}
	candidateAccountsByGroup := make(map[int64][]Account, len(groupIDs))
	for i := range candidateSnapshot {
		entry := candidateSnapshot[i]
		if account, eligible := eligibleByID[entry.Account.ID]; eligible {
			candidateAccountsByGroup[entry.GroupID] = append(candidateAccountsByGroup[entry.GroupID], account)
		}
	}
	ctx = s.withWindowCostPrefetch(ctx, accountSnapshot)
	ctx = s.withRPMPrefetch(ctx, accountSnapshot)

	seenGroups = make(map[int64]struct{}, len(ordered))
	for _, source := range ordered {
		group := source.SourceGroup
		if group == nil || group.ID != source.SourceGroupID ||
			(platform != "" && group.Platform != platform) {
			continue
		}
		if _, exists := seenGroups[group.ID]; exists {
			continue
		}
		seenGroups[group.ID] = struct{}{}

		allGroupAccounts := systemCustomCatalogAccountsForPlatform(accountsByGroup[group.ID], group.Platform)
		var advertisedModels []string
		if requestedModel == "" {
			advertisedModels = systemCustomAdvertisedModels(group, allGroupAccounts)
		} else if advertisedModel, advertised := systemCustomAdvertisedModel(group, allGroupAccounts, requestedModel); advertised {
			advertisedModels = []string{advertisedModel}
		}
		validSource := isDirectSystemCustomSource(group) && IsGroupContextValid(group)
		candidateAccounts := systemCustomAccountsForPlatform(candidateAccountsByGroup[group.ID], group.Platform)
		sourceCtx := s.withGroupContext(ctx, group)
		groupID := group.ID
		sourceCtx = s.withGatewayProfitControlGate(sourceCtx, &groupID)
		checkUpstreamRestriction := validSource && s.needsUpstreamChannelRestrictionCheck(sourceCtx, &groupID)
		for _, model := range advertisedModels {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			key := strings.ToLower(model)
			catalog.advertised[key] = struct{}{}
			if !validSource || s.checkChannelPricingRestriction(sourceCtx, &groupID, model) {
				continue
			}
			allowedAccounts := s.systemCustomModelPricedAccounts(sourceCtx, group, candidateAccounts, model, checkUpstreamRestriction)
			if allowedAccounts.Empty() {
				continue
			}
			candidate := SystemCustomGroupRuntimeCandidate{
				SourceGroup:     *group,
				PublicModel:     model,
				SourceModel:     model,
				AllowedAccounts: allowedAccounts,
			}
			catalog.candidates[key] = append(catalog.candidates[key], candidate)
			if _, exists := catalog.availableModels[key]; !exists {
				catalog.availableModels[key] = model
			}
		}
	}
	return catalog, nil
}

func systemCustomAdvertisedModel(group *Group, accounts []Account, requestedModel string) (string, bool) {
	requestedModel = strings.TrimSpace(requestedModel)
	if group == nil || requestedModel == "" {
		return "", false
	}
	if len(accounts) == 0 {
		models := DefaultModelIDsForPlatform(group.Platform)
		if group.CustomModelsListEnabled() {
			models = group.ModelsListConfig.Models
		}
		return equalFoldSystemCustomModel(models, requestedModel)
	}

	hasAnyMapping := false
	useDefaults := false
	mappedMatch := ""
	customAllowedByMapping := false
	selectedCustomModel := ""
	if group.CustomModelsListEnabled() {
		var selected bool
		selectedCustomModel, selected = equalFoldSystemCustomModel(group.ModelsListConfig.Models, requestedModel)
		if !selected {
			return "", false
		}
	}
	for i := range accounts {
		account := &accounts[i]
		if group.Platform == PlatformOpenAI && account.IsOpenAIPassthroughEnabled() {
			useDefaults = true
			break
		}
		mapping := account.GetModelMapping()
		if len(mapping) == 0 {
			continue
		}
		hasAnyMapping = true
		for model := range mapping {
			model = strings.TrimSpace(model)
			if model == "" || shouldHideUnavailableProviderModel(account, model) {
				continue
			}
			if mappedMatch == "" && strings.EqualFold(model, requestedModel) {
				mappedMatch = model
			}
			if group.CustomModelsListEnabled() && CustomModelsListAllowsModel([]string{model}, selectedCustomModel) {
				customAllowedByMapping = true
			}
		}
	}

	if group.CustomModelsListEnabled() {
		if useDefaults || !hasAnyMapping {
			_, allowed := equalFoldSystemCustomModel(DefaultModelIDsForPlatform(group.Platform), selectedCustomModel)
			return selectedCustomModel, allowed
		}
		if group.Platform == PlatformAnthropic {
			if _, allowed := equalFoldSystemCustomModel(DefaultModelIDsForPlatform(group.Platform), selectedCustomModel); allowed {
				return selectedCustomModel, true
			}
		}
		return selectedCustomModel, customAllowedByMapping
	}

	if useDefaults || !hasAnyMapping {
		return equalFoldSystemCustomModel(DefaultModelIDsForPlatform(group.Platform), requestedModel)
	}
	return mappedMatch, mappedMatch != ""
}

func equalFoldSystemCustomModel(models []string, requestedModel string) (string, bool) {
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model != "" && strings.EqualFold(model, requestedModel) {
			return model, true
		}
	}
	return "", false
}

func cloneSystemCustomAccountMap(source map[string]any) map[string]any {
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		if nested, ok := value.(map[string]any); ok {
			cloned[key] = cloneSystemCustomAccountMap(nested)
			continue
		}
		cloned[key] = value
	}
	return cloned
}

func systemCustomAdvertisedModels(group *Group, accounts []Account) []string {
	if len(accounts) == 0 {
		if group.CustomModelsListEnabled() {
			return normalizedSystemCustomDeclaredModels(group.ModelsListConfig.Models)
		}
		return DefaultModelIDsForPlatform(group.Platform)
	}
	modelSet := make(map[string]string)
	hasAnyMapping := false
	useDefaults := false
	for i := range accounts {
		account := &accounts[i]
		if group.Platform == PlatformOpenAI && account.IsOpenAIPassthroughEnabled() {
			useDefaults = true
			break
		}
		mapping := account.GetModelMapping()
		if len(mapping) == 0 {
			continue
		}
		hasAnyMapping = true
		for model := range mapping {
			model = strings.TrimSpace(model)
			if model == "" || shouldHideUnavailableProviderModel(account, model) {
				continue
			}
			key := strings.ToLower(model)
			if existing, ok := modelSet[key]; !ok || model < existing {
				modelSet[key] = model
			}
		}
	}

	available := make([]string, 0, len(modelSet))
	if hasAnyMapping && !useDefaults {
		for _, model := range modelSet {
			available = append(available, model)
		}
		sort.Slice(available, func(i, j int) bool {
			left, right := strings.ToLower(available[i]), strings.ToLower(available[j])
			if left == right {
				return available[i] < available[j]
			}
			return left < right
		})
	}
	if group.CustomModelsListEnabled() {
		return ResolveCustomModelsList(group.Platform, available, group.ModelsListConfig.Models)
	}
	if len(available) == 0 {
		return DefaultModelIDsForPlatform(group.Platform)
	}
	return available
}

func normalizedSystemCustomDeclaredModels(models []string) []string {
	byKey := make(map[string]string, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		key := strings.ToLower(model)
		if _, exists := byKey[key]; !exists {
			byKey[key] = model
		}
	}
	result := make([]string, 0, len(byKey))
	for _, model := range byKey {
		result = append(result, model)
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := strings.ToLower(result[i]), strings.ToLower(result[j])
		if left == right {
			return result[i] < result[j]
		}
		return left < right
	})
	return result
}

func (s *GatewayService) systemCustomModelPricedAccounts(
	ctx context.Context,
	group *Group,
	accounts []Account,
	model string,
	checkUpstreamRestriction bool,
) SystemCustomGroupAccountAllowlist {
	accountIDs := make([]int64, 0, len(accounts))
	for i := range accounts {
		account := &accounts[i]
		if !s.isSystemCustomSnapshotAccountEligible(ctx, group, account, model, checkUpstreamRestriction) {
			continue
		}
		groupID := group.ID
		if err := s.validateSelectedAccountPricing(ctx, &groupID, model, account); err != nil {
			continue
		}
		accountIDs = append(accountIDs, account.ID)
	}
	return NewSystemCustomGroupAccountAllowlist(accountIDs)
}

func (s *GatewayService) isSystemCustomSnapshotAccountEligible(
	ctx context.Context,
	group *Group,
	account *Account,
	model string,
	checkUpstreamRestriction bool,
) bool {
	if group == nil || account == nil {
		return false
	}
	if systemCustomUsesOpenAIGateway(group.Platform) && s.systemCustomOpenAIRuntimeProbe != nil &&
		!s.systemCustomOpenAIRuntimeProbe.IsSystemCustomAccountRuntimeEligible(account, model) {
		return false
	}
	useMixed := group.Platform == PlatformAnthropic || group.Platform == PlatformGemini
	if !s.isAccountAllowedForPlatform(account, group.Platform, useMixed) ||
		!s.isAccountSchedulableForSelection(account) ||
		!s.isGatewayAccountProfitEligible(ctx, account) ||
		(group.RequireOAuthOnly && account.Type == AccountTypeAPIKey) ||
		(group.RequirePrivacySet && !account.IsPrivacySet()) ||
		shouldHideUnavailableProviderModel(account, model) ||
		!s.isModelSupportedByAccountWithContext(ctx, account, model) ||
		!s.isAccountSchedulableForModelSelection(ctx, account, model) ||
		!s.isAccountSchedulableForQuota(account) ||
		!s.isAccountSchedulableForWindowCost(ctx, account, false) ||
		!s.isAccountSchedulableForRPM(ctx, account, false) {
		return false
	}
	return !checkUpstreamRestriction || !s.isUpstreamModelRestrictedByChannel(ctx, group.ID, account, model)
}

func systemCustomUsesOpenAIGateway(platform string) bool {
	switch platform {
	case PlatformOpenAI, PlatformGrok, PlatformKimi, PlatformZhipu, PlatformDeepseek:
		return true
	default:
		return false
	}
}

func systemCustomAccountsForPlatform(accounts []Account, platform string) []Account {
	filtered := make([]Account, 0, len(accounts))
	useMixed := platform == PlatformAnthropic || platform == PlatformGemini
	for i := range accounts {
		if accounts[i].Platform == platform ||
			(useMixed && accounts[i].Platform == PlatformAntigravity && accounts[i].IsMixedSchedulingEnabled()) {
			filtered = append(filtered, accounts[i])
		}
	}
	return filtered
}

func systemCustomCatalogAccountsForPlatform(accounts []Account, platform string) []Account {
	filtered := make([]Account, 0, len(accounts))
	useMixed := platform == PlatformAnthropic || platform == PlatformGemini
	for i := range accounts {
		if accounts[i].Platform == platform || (useMixed && accounts[i].Platform == PlatformAntigravity) {
			filtered = append(filtered, accounts[i])
		}
	}
	return filtered
}
