package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestGroupModelAliasesAuthSnapshotRoundTrip(t *testing.T) {
	svc := &APIKeyService{}
	aliases := []domain.ModelAlias{{Alias: "claude-long", UpstreamModel: "claude-opus-4-6", AntiTruncation: domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 2, MaxDurationSeconds: 180}}}
	key := &APIKey{ID: 1, UserID: 2, User: &User{ID: 2}, Group: &Group{ID: 3, ModelAliases: aliases}}
	snapshot := svc.snapshotFromAPIKey(context.Background(), key)
	raw, err := json.Marshal(snapshot)
	require.NoError(t, err)
	var cached APIKeyAuthSnapshot
	require.NoError(t, json.Unmarshal(raw, &cached))
	restored := svc.snapshotToAPIKey("test-key", &cached)
	require.NotNil(t, restored.Group)
	require.Equal(t, aliases, restored.Group.ModelAliases)
	model, ok := restored.Group.ResolveModelAlias("CLAUDE-LONG")
	require.True(t, ok)
	require.Equal(t, "claude-opus-4-6", model)
	_, isAlias := restored.Group.ResolveModelAlias("claude-opus-4-6")
	require.False(t, isAlias)
}
