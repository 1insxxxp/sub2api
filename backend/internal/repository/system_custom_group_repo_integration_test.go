//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/systemcustomgroupmodel"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSystemCustomGroupRepositoryCreateRollsBackGroupRoutesAndOutboxWhenAnyRouteFails(t *testing.T) {
	ctx := context.Background()
	repo := NewSystemCustomGroupRepository(integrationEntClient)
	suffix := time.Now().UnixNano()
	sourceOne := createSystemCustomSourceGroupForIntegration(t, suffix, "one")
	sourceTwo := createSystemCustomSourceGroupForIntegration(t, suffix, "two")
	container := newSystemCustomContainerForIntegration(fmt.Sprintf("system-custom-create-rollback-%d", suffix))

	err := repo.Create(ctx, container, []service.SystemCustomGroupModelInput{
		{PublicModel: "Shared-Model", SourceGroupID: sourceOne.ID, SourceModel: "source-one", Enabled: true},
		{PublicModel: "shared-model", SourceGroupID: sourceTwo.ID, SourceModel: "source-two", Enabled: true},
	})

	require.Error(t, err)
	require.ErrorIs(t, err, service.ErrSystemCustomGroupDuplicatePublicModel)
	var groupCount, routeCount, outboxCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM groups WHERE name = $1", container.Name).Scan(&groupCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM system_custom_group_models WHERE group_id = $1", container.ID).Scan(&routeCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE group_id = $1", container.ID).Scan(&outboxCount))
	require.Zero(t, groupCount)
	require.Zero(t, routeCount)
	require.Zero(t, outboxCount)
}

func TestSystemCustomGroupRepositoryUpdateRollsBackGroupRouteSnapshotAndOutboxWhenAnyRouteFails(t *testing.T) {
	ctx := context.Background()
	repo := NewSystemCustomGroupRepository(integrationEntClient)
	suffix := time.Now().UnixNano()
	sourceOne := createSystemCustomSourceGroupForIntegration(t, suffix, "update-one")
	sourceTwo := createSystemCustomSourceGroupForIntegration(t, suffix, "update-two")
	container := newSystemCustomContainerForIntegration(fmt.Sprintf("system-custom-update-rollback-%d", suffix))
	require.NoError(t, repo.Create(ctx, container, []service.SystemCustomGroupModelInput{
		{PublicModel: "original-alias", SourceGroupID: sourceOne.ID, SourceModel: "source-one", Enabled: true},
	}))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM scheduler_outbox WHERE group_id = $1", container.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM system_custom_group_models WHERE group_id = $1", container.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", container.ID)
	})
	require.NoError(t, integrationDB.QueryRowContext(ctx, "DELETE FROM scheduler_outbox WHERE group_id = $1 RETURNING 1", container.ID).Scan(new(int)))

	originalName := container.Name
	container.Name = originalName + "-changed"
	err := repo.Update(ctx, container, []service.SystemCustomGroupModelInput{
		{PublicModel: "duplicate", SourceGroupID: sourceOne.ID, SourceModel: "new-one", Enabled: true},
		{PublicModel: "DUPLICATE", SourceGroupID: sourceTwo.ID, SourceModel: "new-two", Enabled: true},
	})

	require.Error(t, err)
	require.ErrorIs(t, err, service.ErrSystemCustomGroupDuplicatePublicModel)
	storedGroup, groupErr := integrationEntClient.Group.Query().Where(group.IDEQ(container.ID)).Only(ctx)
	require.NoError(t, groupErr)
	require.Equal(t, originalName, storedGroup.Name)
	routes, routeErr := integrationEntClient.SystemCustomGroupModel.Query().Where(systemcustomgroupmodel.GroupIDEQ(container.ID)).All(ctx)
	require.NoError(t, routeErr)
	require.Len(t, routes, 1)
	require.Equal(t, "original-alias", routes[0].PublicModel)
	var outboxCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE group_id = $1", container.ID).Scan(&outboxCount))
	require.Zero(t, outboxCount)
}

func TestSystemCustomGroupRepositoryResolveModelIsExactCaseInsensitiveAndEnabledOnly(t *testing.T) {
	ctx := context.Background()
	repo := NewSystemCustomGroupRepository(integrationEntClient)
	suffix := time.Now().UnixNano()
	source := createSystemCustomSourceGroupForIntegration(t, suffix, "resolve")
	container := newSystemCustomContainerForIntegration(fmt.Sprintf("system-custom-resolve-%d", suffix))
	require.NoError(t, repo.Create(ctx, container, []service.SystemCustomGroupModelInput{
		{PublicModel: "Claude-Premium", SourceGroupID: source.ID, SourceModel: "claude-upstream", Enabled: true},
		{PublicModel: "Claude-Disabled", SourceGroupID: source.ID, SourceModel: "claude-disabled", Enabled: false},
	}))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM scheduler_outbox WHERE group_id = $1", container.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM system_custom_group_models WHERE group_id = $1", container.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", container.ID)
	})

	resolved, err := repo.ResolveModel(ctx, container.ID, "claude-premium")
	require.NoError(t, err)
	require.Equal(t, "Claude-Premium", resolved.PublicModel)
	require.Equal(t, source.ID, resolved.SourceGroupID)
	require.NotNil(t, resolved.SourceGroup)

	_, err = repo.ResolveModel(ctx, container.ID, "Claude")
	require.ErrorIs(t, err, service.ErrSystemCustomGroupRouteNotFound, "partial aliases must not match")
	_, err = repo.ResolveModel(ctx, container.ID, "claude-disabled")
	require.ErrorIs(t, err, service.ErrSystemCustomGroupRouteNotFound)
}

func TestSystemCustomGroupRepositoryDeleteRejectsActivePlan(t *testing.T) {
	ctx := context.Background()
	repo := NewSystemCustomGroupRepository(integrationEntClient)
	container, _ := createDeletableSystemCustomGroupForIntegration(t, repo, "active-plan")
	plan, err := integrationEntClient.SubscriptionPlan.Create().
		SetGroupID(container.ID).
		SetName("active plan").
		SetPrice(10).
		SetForSale(true).
		Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM subscription_plans WHERE id = $1", plan.ID)
	})

	err = repo.Delete(ctx, container.ID)
	require.ErrorIs(t, err, service.ErrSystemCustomGroupInUse)
	requireSystemCustomContainerAndRouteExist(t, container.ID)
}

func TestSystemCustomGroupRepositoryDeleteRejectsOrdinaryGroup(t *testing.T) {
	ctx := context.Background()
	repo := NewSystemCustomGroupRepository(integrationEntClient)
	ordinary := createSystemCustomSourceGroupForIntegration(t, time.Now().UnixNano(), "ordinary-delete-target")

	err := repo.Delete(ctx, ordinary.ID)
	require.ErrorIs(t, err, service.ErrSystemCustomGroupNotFound)

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM groups WHERE id = $1", ordinary.ID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestSystemCustomGroupRepositoryDeleteRejectsCurrentActiveSubscription(t *testing.T) {
	ctx := context.Background()
	repo := NewSystemCustomGroupRepository(integrationEntClient)
	container, _ := createDeletableSystemCustomGroupForIntegration(t, repo, "active-subscription")
	user := mustCreateUser(t, integrationEntClient, &service.User{})
	sub := mustCreateSubscription(t, integrationEntClient, &service.UserSubscription{
		UserID: user.ID, GroupID: container.ID, Status: service.SubscriptionStatusActive,
		StartsAt: time.Now().Add(-time.Hour), ExpiresAt: time.Now().Add(time.Hour),
	})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM user_subscriptions WHERE id = $1", sub.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
	})

	err := repo.Delete(ctx, container.ID)
	require.ErrorIs(t, err, service.ErrSystemCustomGroupInUse)
	requireSystemCustomContainerAndRouteExist(t, container.ID)
}

func TestSystemCustomGroupRepositoryDeleteUnreferencedGroupAtomicallyCleansRoutesPlansAndExpiredSubscriptions(t *testing.T) {
	ctx := context.Background()
	repo := NewSystemCustomGroupRepository(integrationEntClient)
	container, _ := createDeletableSystemCustomGroupForIntegration(t, repo, "unreferenced")
	plan, err := integrationEntClient.SubscriptionPlan.Create().
		SetGroupID(container.ID).
		SetName("inactive plan").
		SetPrice(10).
		SetForSale(false).
		Save(ctx)
	require.NoError(t, err)
	user := mustCreateUser(t, integrationEntClient, &service.User{})
	sub := mustCreateSubscription(t, integrationEntClient, &service.UserSubscription{
		UserID: user.ID, GroupID: container.ID, Status: service.SubscriptionStatusExpired,
		StartsAt: time.Now().Add(-48 * time.Hour), ExpiresAt: time.Now().Add(-24 * time.Hour),
	})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
	})

	require.NoError(t, repo.Delete(ctx, container.ID))

	for _, query := range []struct {
		name string
		sql  string
		arg  int64
	}{
		{name: "group", sql: "SELECT COUNT(*) FROM groups WHERE id = $1", arg: container.ID},
		{name: "routes", sql: "SELECT COUNT(*) FROM system_custom_group_models WHERE group_id = $1", arg: container.ID},
		{name: "inactive plans", sql: "SELECT COUNT(*) FROM subscription_plans WHERE id = $1", arg: plan.ID},
		{name: "expired subscriptions", sql: "SELECT COUNT(*) FROM user_subscriptions WHERE id = $1", arg: sub.ID},
	} {
		t.Run(query.name, func(t *testing.T) {
			var count int
			require.NoError(t, integrationDB.QueryRowContext(ctx, query.sql, query.arg).Scan(&count))
			require.Zero(t, count)
		})
	}
	var outboxCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE group_id = $1", container.ID).Scan(&outboxCount))
	require.Equal(t, 1, outboxCount)
}

func createDeletableSystemCustomGroupForIntegration(t *testing.T, repo service.SystemCustomGroupRepository, label string) (*service.Group, *dbent.Group) {
	t.Helper()
	suffix := time.Now().UnixNano()
	source := createSystemCustomSourceGroupForIntegration(t, suffix, "delete-"+label)
	container := newSystemCustomContainerForIntegration(fmt.Sprintf("system-custom-delete-%s-%d", label, suffix))
	require.NoError(t, repo.Create(context.Background(), container, []service.SystemCustomGroupModelInput{
		{PublicModel: "alias", SourceGroupID: source.ID, SourceModel: "source-model", Enabled: true},
	}))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM scheduler_outbox WHERE group_id = $1", container.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM system_custom_group_models WHERE group_id = $1", container.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", container.ID)
	})
	return container, source
}

func requireSystemCustomContainerAndRouteExist(t *testing.T, groupID int64) {
	t.Helper()
	var groupCount, routeCount int
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM groups WHERE id = $1", groupID).Scan(&groupCount))
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM system_custom_group_models WHERE group_id = $1", groupID).Scan(&routeCount))
	require.Equal(t, 1, groupCount)
	require.Equal(t, 1, routeCount)
}

func createSystemCustomSourceGroupForIntegration(t *testing.T, suffix int64, label string) *dbent.Group {
	t.Helper()
	row, err := integrationEntClient.Group.Create().
		SetName(fmt.Sprintf("system-custom-source-%s-%d", label, suffix)).
		SetPlatform(service.PlatformAnthropic).
		SetStatus(service.StatusActive).
		Save(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", row.ID)
	})
	return row
}

func newSystemCustomContainerForIntegration(name string) *service.Group {
	return &service.Group{
		Name: name, Platform: service.PlatformComposite, RateMultiplier: 1,
		IsExclusive: true, Status: service.StatusActive,
		SubscriptionType:           service.SubscriptionTypeSubscription,
		SystemCustomRoutingEnabled: true, DefaultValidityDays: 30,
	}
}
