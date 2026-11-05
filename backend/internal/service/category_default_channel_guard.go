package service

import "context"

// SetChannelService uses the same cache as gateway routing and billing. It is
// attached by the provider after the API key service has been constructed.
func (s *CategoryDefaultGroupService) SetChannelService(channels *ChannelService) {
	s.channels = channels
}

func (s *CategoryDefaultGroupService) validateSourceChannel(ctx context.Context, source CategoryDefaultSourceSnapshot) error {
	if s.channels == nil {
		return nil
	}
	cache, err := s.channels.loadCache(ctx)
	if err != nil {
		return ErrCategoryDefaultSourceUnavailable.WithCause(err)
	}
	if categoryDefaultChannelMatches(cache, source) {
		return nil
	}
	// A missed cross-instance notification must not let a fresh catalogue route
	// through an old mapping for the entire cache TTL. Refresh locally once;
	// another concurrent declaration change still fails closed for this request.
	s.channels.clearCache()
	cache, err = s.channels.loadCache(ctx)
	if err != nil {
		return ErrCategoryDefaultSourceUnavailable.WithCause(err)
	}
	if !categoryDefaultChannelMatches(cache, source) {
		return ErrCategoryDefaultPendingSync
	}
	return nil
}

func categoryDefaultChannelMatches(cache *channelCache, source CategoryDefaultSourceSnapshot) bool {
	if cache == nil || source.Group == nil {
		return false
	}
	actual, declared := cache.channelByGroupID[source.Group.ID], source.Channel
	if actual == nil || declared == nil {
		return actual == nil && declared == nil
	}
	return actual.ID == declared.ID && actual.IsActive() &&
		cache.groupPlatform[source.Group.ID] == source.Group.Platform &&
		publicGroupSyncMappingsEqual(actual.ModelMapping, declared.ModelMapping)
}
