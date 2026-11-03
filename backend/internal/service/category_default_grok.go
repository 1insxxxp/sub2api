package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

// CategoryDefaultGrokMappingOptionsFromSettings uses the same defaults as the
// gateway settings parser, without publishing new process-wide runtime state.
func CategoryDefaultGrokMappingOptionsFromSettings(settings map[string]string) xai.ModelMappingOptions {
	model := strings.TrimSpace(settings[SettingKeyGrokDefaultTextModel])
	if model == "" {
		model = xai.DefaultTextModel
	}
	return xai.ModelMappingOptions{DefaultText: model, EnableCrossClientMap: !isFalseSettingValue(settings[SettingKeyGrokCrossClientModelMapEnabled])}
}

func categoryDefaultFreezeGrokDeclarations(source CategoryDefaultSourceSnapshot) CategoryDefaultSourceSnapshot {
	if source.GrokMappingOptions == nil {
		return source
	}
	accounts := append([]Account(nil), source.Accounts...)
	for i := range accounts {
		account := &accounts[i]
		if account.Platform != PlatformGrok {
			continue
		}
		raw, _ := account.Credentials["model_mapping"].(map[string]any)
		hasConfiguredMapping := false
		for _, value := range raw {
			if _, ok := value.(string); ok {
				hasConfiguredMapping = true
				break
			}
		}
		if hasConfiguredMapping {
			continue
		}
		// An empty Grok mapping means the operator's default catalog. Materialize
		// that non-secret declaration on a copy so historical parsing never reads
		// a future process default, and never alters real account credentials.
		credentials := make(map[string]any, len(account.Credentials)+1)
		for key, value := range account.Credentials {
			credentials[key] = value
		}
		mapping := make(map[string]any)
		for public, target := range xai.ModelMappingWithOptions(*source.GrokMappingOptions) {
			mapping[public] = target
		}
		credentials["model_mapping"] = mapping
		account.Credentials = credentials
	}
	source.Accounts = accounts
	return source
}

func categoryDefaultGrokProcessMismatches(source CategoryDefaultSourceSnapshot, current []CategoryDefaultPublishedModel) map[string]bool {
	invalid := make(map[string]bool)
	if source.GrokMappingOptions == nil {
		return invalid
	}
	dependent := source.Group.Platform == PlatformGrok
	for _, account := range source.Accounts {
		dependent = dependent || account.Platform == PlatformGrok
	}
	if !dependent {
		return invalid
	}
	// A replica may not yet have refreshed settings. Its scheduler still uses
	// process defaults, so withhold just the models that would map differently.
	processSource := source
	processSource.GrokMappingOptions = nil
	processModels, err := categoryDefaultSourceModels(processSource)
	processByName := make(map[string]CategoryDefaultPublishedModel, len(processModels))
	for _, model := range processModels {
		processByName[categoryDefaultModelKey(model.PublicModel)] = model
	}
	for _, model := range current {
		key := categoryDefaultModelKey(model.PublicModel)
		actual, exists := processByName[key]
		if err != nil || !exists || actual.Fingerprint != model.Fingerprint || actual.SourceModel != model.SourceModel {
			invalid[key] = true
		}
	}
	return invalid
}
