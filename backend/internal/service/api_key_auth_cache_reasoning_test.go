//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthCache_RefreshesV24ReasoningDefault(t *testing.T) {
	groupID := int64(84)
	key := &APIKey{
		ID: 1, UserID: 2, GroupID: &groupID, Key: "sk-reasoning-cache-test", Status: StatusActive,
		User:  &User{ID: 2, Status: StatusActive},
		Group: &Group{ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true, DefaultReasoningEffort: "low"},
	}
	repoCalls := 0
	repo := &authRepoStub{getByKeyForAuth: func(context.Context, string) (*APIKey, error) {
		repoCalls++
		return key, nil
	}}
	cache := &authCacheStub{}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, &config.Config{APIKeyAuth: config.APIKeyAuthCacheConfig{L2TTLSeconds: 60}})
	stale := svc.snapshotFromAPIKey(context.Background(), key)
	stale.Version = 24
	stale.Group.DefaultReasoningEffort = ""
	cache.getAuthCache = func(context.Context, string) (*APIKeyAuthCacheEntry, error) {
		return &APIKeyAuthCacheEntry{Snapshot: stale}, nil
	}
	got, err := svc.GetByKey(context.Background(), key.Key)
	require.NoError(t, err)
	require.Equal(t, 1, repoCalls)
	require.Equal(t, "low", got.Group.DefaultReasoningEffort)
	require.Len(t, cache.setAuthKeys, 1)
	entry := &APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), got)}
	materialized, used, err := svc.applyAuthCacheEntry(key.Key, entry)
	require.NoError(t, err)
	require.True(t, used)
	require.Equal(t, "low", materialized.Group.DefaultReasoningEffort)
}
