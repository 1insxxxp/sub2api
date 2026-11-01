package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotGeminiInputCountingRoundtrip(t *testing.T) {
	groupID := int64(58)
	group := &Group{ID: groupID, Name: "discount", Platform: PlatformGemini, Status: StatusActive, Hydrated: true, GeminiInputCountingEnabled: true}
	key := &APIKey{ID: 92, UserID: 42, GroupID: &groupID, Key: "test-group-discount", Status: StatusActive,
		User: &User{ID: 42, Status: StatusActive}, Group: group}
	svc := &APIKeyService{}
	raw, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), key)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(raw, &cached))
	materialized, used, err := svc.applyAuthCacheEntry(key.Key, &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.NotNil(t, materialized.Group)
	require.True(t, materialized.Group.GeminiInputCountingEnabled, "auth cache must retain group discount policy")
}

func TestAPIKeyAuthSnapshotRejectsV28WithoutGeminiInputCounting(t *testing.T) {
	svc := &APIKeyService{}
	key, used, err := svc.applyAuthCacheEntry("test-v28-input-discount", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{Version: 28},
	})
	require.NoError(t, err)
	require.False(t, used)
	require.Nil(t, key)
}
