//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGroupGeminiInputCountingPersistenceAndAuthProjection(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	groups := newGroupRepositoryWithSQL(client, tx)
	keys := newAPIKeyRepositoryWithSQL(client, tx)
	group := &service.Group{
		Name: "group-discount-" + uuid.NewString(), Platform: service.PlatformGemini,
		RateMultiplier: 1, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard,
		GeminiInputCountingEnabled: true,
	}
	require.NoError(t, groups.Create(ctx, group))
	loaded, err := groups.GetByIDLite(ctx, group.ID)
	require.NoError(t, err)
	require.True(t, loaded.GeminiInputCountingEnabled)

	user := mustCreateUser(t, client, &service.User{Email: "group-discount-" + uuid.NewString() + "@example.com", Concurrency: 1})
	key := &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "test-group-discount-" + uuid.NewString(), Name: "group-discount", Status: service.StatusActive}
	require.NoError(t, keys.Create(ctx, key))
	fromAuth, err := keys.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.NotNil(t, fromAuth.Group)
	require.True(t, fromAuth.Group.Hydrated)
	require.True(t, fromAuth.Group.GeminiInputCountingEnabled, "the explicit auth Select must carry the group policy")

	loaded.GeminiInputCountingEnabled = false
	require.NoError(t, groups.Update(ctx, loaded))
	fromAuth, err = keys.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.False(t, fromAuth.Group.GeminiInputCountingEnabled)

	defaultGroup, err := client.Group.Create().SetName("default-group-discount-" + uuid.NewString()).Save(ctx)
	require.NoError(t, err)
	require.False(t, defaultGroup.GeminiInputCountingEnabled, "migration/schema must default every group to disabled")
}
