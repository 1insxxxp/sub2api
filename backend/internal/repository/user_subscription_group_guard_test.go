//go:build unit

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func TestUserSubscriptionCreateRejectsUnavailableGroupForActiveReference(t *testing.T) {
	tests := []struct {
		name      string
		configure func(context.Context, *dbent.Client, *dbent.Group) error
		wantErr   error
	}{
		{
			name: "soft deleted",
			configure: func(ctx context.Context, client *dbent.Client, group *dbent.Group) error {
				return client.Group.DeleteOneID(group.ID).Exec(ctx)
			},
			wantErr: service.ErrGroupNotFound,
		},
		{
			name: "disabled",
			configure: func(ctx context.Context, client *dbent.Client, group *dbent.Group) error {
				_, err := client.Group.UpdateOneID(group.ID).SetStatus(service.StatusDisabled).Save(ctx)
				return err
			},
			wantErr: service.ErrGroupDisabled,
		},
		{
			name: "not subscription type",
			configure: func(ctx context.Context, client *dbent.Client, group *dbent.Group) error {
				_, err := client.Group.UpdateOneID(group.ID).SetSubscriptionType(service.SubscriptionTypeStandard).Save(ctx)
				return err
			},
			wantErr: service.ErrGroupNotSubscriptionType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := newUserSubscriptionGuardTestClient(t)
			user := createUserSubscriptionGuardUser(t, ctx, client, tc.name)
			group := createUserSubscriptionGuardGroup(t, ctx, client, tc.name)
			require.NoError(t, tc.configure(ctx, client, group))
			repo := NewUserSubscriptionRepository(client)
			now := time.Now()
			sub := &service.UserSubscription{
				UserID: user.ID, GroupID: group.ID, StartsAt: now,
				ExpiresAt: now.Add(time.Hour), Status: service.SubscriptionStatusActive,
			}

			err := repo.Create(ctx, sub)

			require.ErrorIs(t, err, tc.wantErr)
			count, countErr := client.UserSubscription.Query().Count(ctx)
			require.NoError(t, countErr)
			require.Zero(t, count)
		})
	}
}

func TestUserSubscriptionActiveMutationPathsRevalidateGroup(t *testing.T) {
	t.Run("Update rejects soft-deleted target group", func(t *testing.T) {
		ctx := context.Background()
		client := newUserSubscriptionGuardTestClient(t)
		user := createUserSubscriptionGuardUser(t, ctx, client, "update")
		group := createUserSubscriptionGuardGroup(t, ctx, client, "update")
		existing := createUserSubscriptionGuardRecord(t, ctx, client, user.ID, group.ID, service.SubscriptionStatusExpired, time.Now().Add(-time.Hour))
		require.NoError(t, client.Group.DeleteOneID(group.ID).Exec(ctx))
		repo := NewUserSubscriptionRepository(client)
		input := userSubscriptionEntityToService(existing)
		input.Status = service.SubscriptionStatusActive
		input.ExpiresAt = time.Now().Add(time.Hour)

		err := repo.Update(ctx, input)

		require.ErrorIs(t, err, service.ErrGroupNotFound)
		persisted, getErr := client.UserSubscription.Get(ctx, existing.ID)
		require.NoError(t, getErr)
		require.Equal(t, service.SubscriptionStatusExpired, persisted.Status)
	})

	t.Run("Restore rejects disabled group", func(t *testing.T) {
		ctx := context.Background()
		client := newUserSubscriptionGuardTestClient(t)
		user := createUserSubscriptionGuardUser(t, ctx, client, "restore")
		group := createUserSubscriptionGuardGroup(t, ctx, client, "restore")
		existing := createUserSubscriptionGuardRecord(t, ctx, client, user.ID, group.ID, service.SubscriptionStatusActive, time.Now().Add(time.Hour))
		require.NoError(t, client.UserSubscription.DeleteOneID(existing.ID).Exec(ctx))
		_, err := client.Group.UpdateOneID(group.ID).SetStatus(service.StatusDisabled).Save(ctx)
		require.NoError(t, err)
		repo := NewUserSubscriptionRepository(client)

		restored, err := repo.Restore(ctx, existing.ID, service.SubscriptionStatusActive)

		require.Nil(t, restored)
		require.ErrorIs(t, err, service.ErrGroupDisabled)
		deleted, getErr := client.UserSubscription.Query().Where().Only(mixins.SkipSoftDelete(ctx))
		require.NoError(t, getErr)
		require.NotNil(t, deleted.DeletedAt)
	})

	t.Run("ExtendExpiry rejects non-subscription group when it activates reference", func(t *testing.T) {
		ctx := context.Background()
		client := newUserSubscriptionGuardTestClient(t)
		user := createUserSubscriptionGuardUser(t, ctx, client, "extend")
		group := createUserSubscriptionGuardGroup(t, ctx, client, "extend")
		_, err := client.Group.UpdateOneID(group.ID).SetSubscriptionType(service.SubscriptionTypeStandard).Save(ctx)
		require.NoError(t, err)
		existing := createUserSubscriptionGuardRecord(t, ctx, client, user.ID, group.ID, service.SubscriptionStatusActive, time.Now().Add(-time.Hour))
		repo := NewUserSubscriptionRepository(client)

		err = repo.ExtendExpiry(ctx, existing.ID, time.Now().Add(time.Hour))

		require.ErrorIs(t, err, service.ErrGroupNotSubscriptionType)
		persisted, getErr := client.UserSubscription.Get(ctx, existing.ID)
		require.NoError(t, getErr)
		require.True(t, persisted.ExpiresAt.Before(time.Now()))
	})

	t.Run("UpdateStatus rejects disabled group when it activates reference", func(t *testing.T) {
		ctx := context.Background()
		client := newUserSubscriptionGuardTestClient(t)
		user := createUserSubscriptionGuardUser(t, ctx, client, "status")
		group := createUserSubscriptionGuardGroup(t, ctx, client, "status")
		_, err := client.Group.UpdateOneID(group.ID).SetStatus(service.StatusDisabled).Save(ctx)
		require.NoError(t, err)
		existing := createUserSubscriptionGuardRecord(t, ctx, client, user.ID, group.ID, service.SubscriptionStatusExpired, time.Now().Add(time.Hour))
		repo := NewUserSubscriptionRepository(client)

		err = repo.UpdateStatus(ctx, existing.ID, service.SubscriptionStatusActive)

		require.ErrorIs(t, err, service.ErrGroupDisabled)
		persisted, getErr := client.UserSubscription.Get(ctx, existing.ID)
		require.NoError(t, getErr)
		require.Equal(t, service.SubscriptionStatusExpired, persisted.Status)
	})
}

func TestUserSubscriptionHistoricalWriteDoesNotRequireAvailableGroup(t *testing.T) {
	ctx := context.Background()
	client := newUserSubscriptionGuardTestClient(t)
	user := createUserSubscriptionGuardUser(t, ctx, client, "historical")
	group := createUserSubscriptionGuardGroup(t, ctx, client, "historical")
	require.NoError(t, client.Group.DeleteOneID(group.ID).Exec(ctx))
	repo := NewUserSubscriptionRepository(client)
	now := time.Now()
	sub := &service.UserSubscription{
		UserID: user.ID, GroupID: group.ID, StartsAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Hour), Status: service.SubscriptionStatusExpired,
	}

	require.NoError(t, repo.Create(ctx, sub))
	require.NotZero(t, sub.ID)
}

func TestUserSubscriptionActiveCreateAllowsAvailableSubscriptionGroup(t *testing.T) {
	ctx := context.Background()
	client := newUserSubscriptionGuardTestClient(t)
	user := createUserSubscriptionGuardUser(t, ctx, client, "normal")
	group := createUserSubscriptionGuardGroup(t, ctx, client, "normal")
	repo := NewUserSubscriptionRepository(client)
	now := time.Now()
	sub := &service.UserSubscription{
		UserID: user.ID, GroupID: group.ID, StartsAt: now,
		ExpiresAt: now.Add(time.Hour), Status: service.SubscriptionStatusActive,
	}

	require.NoError(t, repo.Create(ctx, sub))
	require.NotZero(t, sub.ID)
}

func newUserSubscriptionGuardTestClient(t *testing.T) *dbent.Client {
	t.Helper()
	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	db, err := sql.Open("sqlite", dbName)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, func() error { _, err := db.Exec("PRAGMA foreign_keys = ON"); return err }())
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func createUserSubscriptionGuardUser(t *testing.T, ctx context.Context, client *dbent.Client, suffix string) *dbent.User {
	t.Helper()
	user, err := client.User.Create().
		SetEmail(fmt.Sprintf("sub-guard-%s@example.com", strings.ReplaceAll(suffix, " ", "-"))).
		SetPasswordHash("hash").
		SetStatus(service.StatusActive).
		SetRole(service.RoleUser).
		Save(ctx)
	require.NoError(t, err)
	return user
}

func createUserSubscriptionGuardGroup(t *testing.T, ctx context.Context, client *dbent.Client, suffix string) *dbent.Group {
	t.Helper()
	group, err := client.Group.Create().
		SetName("sub-guard-" + strings.ReplaceAll(suffix, " ", "-")).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)
	return group
}

func createUserSubscriptionGuardRecord(
	t *testing.T,
	ctx context.Context,
	client *dbent.Client,
	userID, groupID int64,
	status string,
	expiresAt time.Time,
) *dbent.UserSubscription {
	t.Helper()
	sub, err := client.UserSubscription.Create().
		SetUserID(userID).
		SetGroupID(groupID).
		SetStartsAt(time.Now().Add(-time.Hour)).
		SetExpiresAt(expiresAt).
		SetStatus(status).
		Save(ctx)
	require.NoError(t, err)
	return sub
}
