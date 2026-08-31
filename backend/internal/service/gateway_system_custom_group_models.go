package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type SystemCustomGroupRuntimeCandidate struct {
	SourceGroup Group
	PublicModel string
	SourceModel string
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
	accountsByGroup := make(map[int64][]Account, len(groupIDs))
	uniqueAccountSnapshot := make([]Account, 0, len(accounts))
	seenAccountIDs := make(map[int64]struct{}, len(accounts))
	for i := range accounts {
		entry := accounts[i]
		accountsByGroup[entry.GroupID] = append(accountsByGroup[entry.GroupID], entry.Account)
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
	for i := range accounts {
		entry := accounts[i]
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
		advertisedModels := systemCustomAdvertisedModels(group, allGroupAccounts)
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
			if !validSource || s.checkChannelPricingRestriction(sourceCtx, &groupID, model) ||
				!s.systemCustomModelHasPricedAccount(sourceCtx, group, candidateAccounts, model, checkUpstreamRestriction) {
				continue
			}
			candidate := SystemCustomGroupRuntimeCandidate{
				SourceGroup: *group,
				PublicModel: model,
				SourceModel: model,
			}
			catalog.candidates[key] = append(catalog.candidates[key], candidate)
			if _, exists := catalog.availableModels[key]; !exists {
				catalog.availableModels[key] = model
			}
		}
	}
	return catalog, nil
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

func (s *GatewayService) systemCustomModelHasPricedAccount(
	ctx context.Context,
	group *Group,
	accounts []Account,
	model string,
	checkUpstreamRestriction bool,
) bool {
	for i := range accounts {
		account := &accounts[i]
		if !s.isSystemCustomSnapshotAccountEligible(ctx, group, account, model, checkUpstreamRestriction) {
			continue
		}
		groupID := group.ID
		if err := s.validateSelectedAccountPricing(ctx, &groupID, model, account); err != nil {
			continue
		}
		return true
	}
	return false
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

// ListSystemCustomGroupModelAvailability evaluates a complete system-custom
// route snapshot against one batched schedulable-account snapshot. Model
// support is delegated to the gateway's real scheduler predicates so wildcard,
// provider normalization, casing, Bedrock, and passthrough behavior cannot
// drift from request routing.
func (s *GatewayService) ListSystemCustomGroupModelAvailability(
	ctx context.Context,
	sources []SystemCustomGroupModelListSource,
) (SystemCustomGroupModelAvailability, error) {
	availability := make(SystemCustomGroupModelAvailability, len(sources))
	if len(sources) == 0 {
		return availability, nil
	}
	if s == nil || s.accountRepo == nil {
		return nil, fmt.Errorf("system custom group model catalog is not configured")
	}
	batchRepo, ok := s.accountRepo.(systemCustomGroupSchedulableAccountRepository)
	if !ok {
		return nil, fmt.Errorf("account repository does not support system custom group snapshots")
	}

	groupIDs := make([]int64, 0, len(sources))
	seenGroupIDs := make(map[int64]struct{}, len(sources))
	for i := range sources {
		groupID := sources[i].Group.ID
		if groupID <= 0 {
			continue
		}
		if _, exists := seenGroupIDs[groupID]; exists {
			continue
		}
		seenGroupIDs[groupID] = struct{}{}
		groupIDs = append(groupIDs, groupID)
	}
	accounts, err := batchRepo.ListSchedulableByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("list system custom group schedulable accounts: %w", err)
	}
	accountsByGroup := make(map[int64][]Account, len(groupIDs))
	for i := range accounts {
		entry := accounts[i]
		accountsByGroup[entry.GroupID] = append(accountsByGroup[entry.GroupID], entry.Account)
	}

	for i := range sources {
		source := &sources[i]
		group := &source.Group
		if !isDirectSystemCustomSource(group) || !IsGroupContextValid(group) {
			continue
		}
		groupAccounts := systemCustomAccountsForPlatform(accountsByGroup[group.ID], group.Platform)
		if len(groupAccounts) == 0 {
			continue
		}
		available := make(map[string]bool, len(source.Models))
		availability[group.ID] = available
		for _, sourceModel := range source.Models {
			sourceModel = strings.TrimSpace(sourceModel)
			if sourceModel == "" || available[sourceModel] {
				continue
			}
			// A system route is already an explicit administrator-owned model
			// enumeration. The source custom list remains an additional display
			// whitelist when enabled, but account mapping catalogs must not replace
			// the actual scheduler candidate set.
			if group.CustomModelsListEnabled() &&
				!CustomModelsListAllowsModel(group.ModelsListConfig.Models, sourceModel) {
				continue
			}
			for j := range groupAccounts {
				account := &groupAccounts[j]
				if shouldHideUnavailableProviderModel(account, sourceModel) ||
					!account.IsSchedulableForModelWithContext(ctx, sourceModel) ||
					!s.isModelSupportedByAccountWithContext(ctx, account, sourceModel) {
					continue
				}
				available[sourceModel] = true
				break
			}
		}
	}
	return availability, nil
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
