package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotGiftBalancesRoundTrip(t *testing.T) {
	svc := &APIKeyService{}
	apiKey := &APIKey{
		ID:     82,
		UserID: 40,
		Key:    "sk-gift-balance-roundtrip",
		Status: StatusActive,
		User: &User{
			ID:                40,
			Status:            StatusActive,
			Balance:           20,
			GiftBalance:       7.5,
			FrozenGiftBalance: 1.25,
		},
	}

	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.NotNil(t, snapshot)
	require.Equal(t, apiKeyAuthSnapshotVersion, snapshot.Version)

	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: snapshot})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))

	materialized, used, err := svc.applyAuthCacheEntry(apiKey.Key, &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.NotNil(t, materialized.User)
	require.Equal(t, 7.5, materialized.User.GiftBalance)
	require.Equal(t, 1.25, materialized.User.FrozenGiftBalance)
}
