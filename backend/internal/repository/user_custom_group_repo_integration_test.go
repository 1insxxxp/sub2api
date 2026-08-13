//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/ent/usercustomgroup"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserCustomGroupRepositoryDeleteAndUnbindAPIKeys(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	owner, err := client.User.Create().
		SetEmail("custom-group-delete-owner-" + suffix + "@test.local").
		SetPasswordHash("test").
		Save(ctx)
	require.NoError(t, err)
	other, err := client.User.Create().
		SetEmail("custom-group-delete-other-" + suffix + "@test.local").
		SetPasswordHash("test").
		Save(ctx)
	require.NoError(t, err)

	group, err := client.UserCustomGroup.Create().
		SetUserID(owner.ID).
		SetName("delete-" + suffix).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	createKey := func(name, status string, customGroupID *int64) int64 {
		builder := client.APIKey.Create().
			SetUserID(owner.ID).
			SetKey("sk-" + name + "-" + suffix).
			SetName(name).
			SetStatus(status)
		if customGroupID != nil {
			builder.SetCustomGroupID(*customGroupID)
		}
		row, createErr := builder.Save(ctx)
		require.NoError(t, createErr)
		return row.ID
	}
	liveActiveID := createKey("live-active", service.StatusActive, &group.ID)
	liveDisabledID := createKey("live-disabled", service.StatusDisabled, &group.ID)
	deletedID := createKey("deleted", service.StatusActive, &group.ID)
	unrelatedID := createKey("unrelated", service.StatusActive, nil)
	_, err = client.APIKey.UpdateOneID(deletedID).SetDeletedAt(time.Now()).Save(mixins.SkipSoftDelete(ctx))
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM api_keys WHERE user_id = $1", owner.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM user_custom_groups WHERE user_id = $1", owner.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id IN ($1, $2)", owner.ID, other.ID)
	})

	repo := &userCustomGroupRepository{client: client, db: integrationDB}

	_, err = repo.DeleteAndUnbindAPIKeys(ctx, group.ID, other.ID)
	require.ErrorIs(t, err, service.ErrUserCustomGroupNotFound)
	stillBound, err := client.APIKey.Query().Where(apikey.IDEQ(liveActiveID), apikey.CustomGroupIDEQ(group.ID)).Exist(ctx)
	require.NoError(t, err)
	require.True(t, stillBound)

	unboundCount, err := repo.DeleteAndUnbindAPIKeys(ctx, group.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, 2, unboundCount)

	deletedGroup, err := client.UserCustomGroup.Query().
		Where(usercustomgroup.IDEQ(group.ID)).
		Only(mixins.SkipSoftDelete(ctx))
	require.NoError(t, err)
	require.Equal(t, service.StatusDisabled, deletedGroup.Status)
	require.NotNil(t, deletedGroup.DeletedAt)

	for _, id := range []int64{liveActiveID, liveDisabledID} {
		key, queryErr := client.APIKey.Get(ctx, id)
		require.NoError(t, queryErr)
		require.Nil(t, key.CustomGroupID)
	}
	deletedKey, err := client.APIKey.Query().Where(apikey.IDEQ(deletedID)).Only(mixins.SkipSoftDelete(ctx))
	require.NoError(t, err)
	require.NotNil(t, deletedKey.CustomGroupID)
	require.Equal(t, group.ID, *deletedKey.CustomGroupID)
	unrelatedKey, err := client.APIKey.Get(ctx, unrelatedID)
	require.NoError(t, err)
	require.Nil(t, unrelatedKey.CustomGroupID)
}
