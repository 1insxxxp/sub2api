package service

// projectCategoryDefaultCatalog derives the effective catalogue from one
// repository snapshot. Saved source IDs, rather than the legacy published
// manifest, are the authority boundary. It never mutates the saved config.
func projectCategoryDefaultCatalog(snapshot *CategoryDefaultCatalogSnapshot) (*categoryDefaultRuntime, error) {
	if snapshot == nil || snapshot.Config == nil || !validCategoryDefaultCategory(snapshot.Config.Category) {
		return nil, ErrCategoryDefaultSourceUnavailable
	}
	config := *snapshot.Config
	config.SourceGroupIDs = append([]int64{}, config.SourceGroupIDs...)
	config.Models = []CategoryDefaultPublishedModel{}
	config.InvalidatedModels = nil
	config.Conflicts = nil
	config.UnavailableSources = nil
	runtime := &categoryDefaultRuntime{
		config: &config, current: make(map[string]CategoryDefaultPublishedModel),
		conflicts: make(map[string]bool), invalidated: make(map[string]bool),
		groups: make(map[int64]*Group), sources: make(map[int64]CategoryDefaultSourceSnapshot),
	}
	byID := make(map[int64]CategoryDefaultSourceSnapshot, len(snapshot.Sources))
	for _, source := range snapshot.Sources {
		if source.Group != nil {
			byID[source.Group.ID] = source
		}
	}
	valid := make([]CategoryDefaultSourceSnapshot, 0, len(config.SourceGroupIDs))
	for _, id := range config.SourceGroupIDs {
		source, exists := byID[id]
		if !exists || !categoryDefaultSourceEligible(config.Category, source.Group) {
			config.UnavailableSources = append(config.UnavailableSources, CategoryDefaultUnavailableSource{SourceGroupID: id, Reason: "unavailable"})
			continue
		}
		declarations, err := categoryDefaultSourceModels(source)
		if err != nil {
			config.UnavailableSources = append(config.UnavailableSources, CategoryDefaultUnavailableSource{SourceGroupID: id, Reason: "unbounded"})
			continue
		}
		for model := range categoryDefaultGrokProcessMismatches(source, declarations) {
			runtime.invalidated[model] = true
		}
		valid = append(valid, source)
		runtime.groups[id] = source.Group
		runtime.sources[id] = source
	}
	models, conflicts, err := buildCategoryDefaultCatalog(config.Category, valid)
	if err != nil {
		return nil, err
	}
	config.Conflicts = conflicts
	for _, model := range models {
		key := categoryDefaultModelKey(model.PublicModel)
		runtime.current[key] = model
		if !runtime.invalidated[key] {
			config.Models = append(config.Models, model)
		}
	}
	for _, conflict := range conflicts {
		runtime.conflicts[categoryDefaultModelKey(conflict.PublicModel)] = true
	}
	config.ModelsCount = len(config.Models)
	return runtime, nil
}

func categoryDefaultSelectedSources(sources []CategoryDefaultSourceSnapshot, ids []int64) []CategoryDefaultSourceSnapshot {
	wanted := make(map[int64]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	selected := make([]CategoryDefaultSourceSnapshot, 0, len(ids))
	for _, source := range sources {
		if source.Group != nil && wanted[source.Group.ID] {
			selected = append(selected, source)
		}
	}
	return selected
}
