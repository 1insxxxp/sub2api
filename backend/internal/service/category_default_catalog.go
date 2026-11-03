package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

func EffectiveKeyDisplayCategory(group *Group) string {
	if group == nil {
		return "other"
	}
	if group.KeyDisplayCategory != "" && domain.ValidateGroupKeyDisplayCategory(group.KeyDisplayCategory) == nil {
		return group.KeyDisplayCategory
	}
	if group.Tag == "image" {
		return "image"
	}
	switch group.Platform {
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformGrok:
		return group.Platform
	case PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax:
		return "domestic"
	default:
		return "other"
	}
}

func validCategoryDefaultCategory(category string) bool {
	for _, supported := range CategoryDefaultCategories {
		if category == supported {
			return true
		}
	}
	return false
}

func categoryDefaultSourceEligible(category string, group *Group) bool {
	return group != nil && group.ID > 0 && group.Status == StatusActive &&
		group.SubscriptionType == SubscriptionTypeStandard && !group.IsCategoryDefault &&
		!group.SystemCustomRoutingEnabled && EffectiveKeyDisplayCategory(group) == category &&
		(domain.IsConcretePlatform(group.Platform) || group.Platform == PlatformComposite)
}

func categoryDefaultModelKey(model string) string { return strings.ToLower(strings.TrimSpace(model)) }

// CategoryDefaultSourceDeclarations exposes the pure local declaration
// projection to repository change hooks. Pricing and temporary capacity never
// participate in these fingerprints.
func CategoryDefaultSourceDeclarations(source CategoryDefaultSourceSnapshot) ([]CategoryDefaultPublishedModel, error) {
	if source.Group == nil {
		return nil, ErrCategoryDefaultInvalidSource
	}
	return categoryDefaultSourceModels(source)
}

// This catalog describes configured public names, not current capacity. A
// cooldown must never turn a duplicate name into a temporarily unique route.
func buildCategoryDefaultCatalog(category string, sources []CategoryDefaultSourceSnapshot) ([]CategoryDefaultPublishedModel, []CategoryDefaultModelConflict, error) {
	if !validCategoryDefaultCategory(category) {
		return nil, nil, ErrCategoryDefaultInvalidCategory
	}
	owners := make(map[string][]CategoryDefaultPublishedModel)
	names := make(map[int64]string)
	seenSources := make(map[int64]bool)
	for _, source := range sources {
		if !categoryDefaultSourceEligible(category, source.Group) || seenSources[source.Group.ID] {
			return nil, nil, ErrCategoryDefaultInvalidSource
		}
		seenSources[source.Group.ID] = true
		names[source.Group.ID] = source.Group.Name
		models, err := categoryDefaultSourceModels(source)
		if err != nil {
			return nil, nil, err
		}
		for _, model := range models {
			key := categoryDefaultModelKey(model.PublicModel)
			owners[key] = append(owners[key], model)
		}
	}
	models := make([]CategoryDefaultPublishedModel, 0, len(owners))
	conflicts := make([]CategoryDefaultModelConflict, 0)
	keys := make([]string, 0, len(owners))
	for key := range owners {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entries := owners[key]
		if len(entries) == 1 {
			models = append(models, entries[0])
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].SourceGroupID < entries[j].SourceGroupID })
		conflict := CategoryDefaultModelConflict{PublicModel: entries[0].PublicModel, SourceGroupIDs: []int64{}, SourceGroupNames: []string{}}
		for _, entry := range entries {
			conflict.SourceGroupIDs = append(conflict.SourceGroupIDs, entry.SourceGroupID)
			conflict.SourceGroupNames = append(conflict.SourceGroupNames, names[entry.SourceGroupID])
		}
		conflicts = append(conflicts, conflict)
	}
	return models, conflicts, nil
}

func categoryDefaultSourceModels(source CategoryDefaultSourceSnapshot) ([]CategoryDefaultPublishedModel, error) {
	source = categoryDefaultFreezeGrokDeclarations(source)
	group := source.Group
	declared := make(map[string]string)
	add := func(model string) {
		model = strings.TrimSpace(model)
		if model != "" && !strings.ContainsAny(model, "*?") {
			key := categoryDefaultModelKey(model)
			if previous, exists := declared[key]; !exists || model < previous {
				declared[key] = model
			}
		}
	}
	hasWildcard := false
	for i := range source.Accounts {
		account := &source.Accounts[i]
		mapping := account.GetModelMapping()
		if len(mapping) == 0 || (account.Platform == PlatformOpenAI && account.IsOpenAIPassthroughEnabled()) {
			for _, model := range DefaultModelIDsForPlatform(account.Platform) {
				add(model)
			}
			continue
		}
		for public := range mapping {
			if strings.ContainsAny(public, "*?") {
				hasWildcard = true
				continue
			}
			add(public)
		}
	}
	for _, route := range source.Routes {
		if route.Enabled && route.MatchType == CompositeRouteMatchExact {
			add(route.PublicModel)
		}
		if route.Enabled && route.MatchType == CompositeRouteMatchPrefix {
			hasWildcard = true
		}
	}
	for _, alias := range group.ModelAliases {
		add(alias.Alias)
	}
	allowlist := group.EffectiveModelAllowlist()
	if allowlist.Enabled {
		for _, model := range allowlist.Models {
			add(model)
		}
	}
	if len(declared) == 0 && hasWildcard {
		return nil, ErrCategoryDefaultUnboundedSource
	}
	if len(declared) == 0 && len(source.Accounts) == 0 && group.Platform != PlatformComposite {
		for _, model := range DefaultModelIDsForPlatform(group.Platform) {
			add(model)
		}
	}
	if len(declared) == 0 {
		return nil, ErrCategoryDefaultUnboundedSource
	}
	keys := make([]string, 0, len(declared))
	for key := range declared {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	models := make([]CategoryDefaultPublishedModel, 0, len(keys))
	for _, key := range keys {
		public := declared[key]
		if !allowlist.Allows(public) {
			continue
		}
		target, aliased := group.ResolveModelAlias(public)
		declarations := make(map[string]bool)
		platformSet := make(map[string]bool)
		geminiNative := false
		if aliased {
			for _, alias := range group.ModelAliases {
				if strings.EqualFold(strings.TrimSpace(alias.Alias), public) {
					data, _ := json.Marshal(alias)
					declarations["group-alias:"+string(data)] = true
					break
				}
			}
		}
		geminiNative = categoryDefaultCollectModelDeclarations(source, public, target, aliased, declarations, platformSet)
		if len(declarations) == 0 {
			return nil, ErrCategoryDefaultUnboundedSource.WithMetadata(map[string]string{"source_group_id": fmt.Sprint(group.ID)})
		}
		parts := make([]string, 0, len(declarations))
		for declaration := range declarations {
			parts = append(parts, declaration)
		}
		sort.Strings(parts)
		platforms := make([]string, 0, len(platformSet))
		for platform := range platformSet {
			platforms = append(platforms, platform)
		}
		sort.Strings(platforms)
		data, _ := json.Marshal(struct {
			Version         int
			GroupID         int64
			Platform, Model string
			Declarations    []string
		}{1, group.ID, group.Platform, key, parts})
		sum := sha256.Sum256(data)
		models = append(models, CategoryDefaultPublishedModel{PublicModel: public, SourceGroupID: group.ID, SourceModel: public, Fingerprint: hex.EncodeToString(sum[:]), Platforms: platforms, GeminiNative: geminiNative})
	}
	if len(models) == 0 {
		return nil, ErrCategoryDefaultUnboundedSource
	}
	return models, nil
}
