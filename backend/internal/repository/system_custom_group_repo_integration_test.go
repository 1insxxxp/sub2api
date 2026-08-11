//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/systemcustomgroupmodel"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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
	var appErr *infraerrors.ApplicationError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, map[string]string{
		"public_model": "shared-model", "source_group_id": fmt.Sprintf("%d", sourceTwo.ID), "source_model": "source-two",
	}, appErr.Metadata)
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

func TestSystemCustomGroupRepositoryDeleteRejectsActiveSubscriptionBeforeItsStartTime(t *testing.T) {
	ctx := context.Background()
	repo := NewSystemCustomGroupRepository(integrationEntClient)
	container, _ := createDeletableSystemCustomGroupForIntegration(t, repo, "future-active-subscription")
	user := mustCreateUser(t, integrationEntClient, &service.User{})
	sub := mustCreateSubscription(t, integrationEntClient, &service.UserSubscription{
		UserID: user.ID, GroupID: container.ID, Status: service.SubscriptionStatusActive,
		StartsAt: time.Now().Add(time.Hour), ExpiresAt: time.Now().Add(2 * time.Hour),
	})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM user_subscriptions WHERE id = $1", sub.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
	})

	err := repo.Delete(ctx, container.ID)
	require.ErrorIs(t, err, service.ErrSystemCustomGroupInUse)
	requireSystemCustomContainerAndRouteExist(t, container.ID)
}

func TestSystemCustomGroupRepositoryDeleteLocksGroupBeforeReferencesWithoutBlockingUnrelatedWrites(t *testing.T) {
	ctx := context.Background()
	repo := NewSystemCustomGroupRepository(integrationEntClient)
	container, _ := createDeletableSystemCustomGroupForIntegration(t, repo, "lock-order")
	unrelatedGroup := createSystemCustomSourceGroupForIntegration(t, time.Now().UnixNano(), "unrelated-write")
	unrelatedUser := mustCreateUser(t, integrationEntClient, &service.User{})

	blocker, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	var lockedID int64
	require.NoError(t, blocker.QueryRowContext(ctx, "SELECT id FROM groups WHERE id = $1 FOR UPDATE", container.ID).Scan(&lockedID))

	deleteResult := make(chan error, 1)
	deleteCtx, cancelDelete := context.WithTimeout(ctx, 5*time.Second)
	defer cancelDelete()
	go func() { deleteResult <- repo.Delete(deleteCtx, container.ID) }()

	deadline := time.Now().Add(2 * time.Second)
	blockedOnGroup := false
	for time.Now().Before(deadline) {
		var waiting int
		require.NoError(t, integrationDB.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM pg_stat_activity
			WHERE datname = current_database()
			  AND wait_event_type = 'Lock'
			  AND query ILIKE '%system_custom_routing_enabled%'
			  AND query ILIKE '%FOR UPDATE%'
		`).Scan(&waiting))
		if waiting > 0 {
			blockedOnGroup = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.True(t, blockedOnGroup, "delete transaction did not reach the group row lock")

	writeCtx, cancelWrite := context.WithTimeout(ctx, time.Second)
	plan, err := integrationEntClient.SubscriptionPlan.Create().
		SetGroupID(unrelatedGroup.ID).
		SetName("unrelated plan during delete").
		SetPrice(1).
		SetForSale(false).
		Save(writeCtx)
	require.NoError(t, err, "unrelated subscription plan writes must not be blocked by delete")
	sub, err := integrationEntClient.UserSubscription.Create().
		SetUserID(unrelatedUser.ID).
		SetGroupID(unrelatedGroup.ID).
		SetStartsAt(time.Now()).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetStatus(service.SubscriptionStatusExpired).
		Save(writeCtx)
	cancelWrite()
	require.NoError(t, err, "unrelated subscription writes must not be blocked by delete")
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM subscription_plans WHERE id = $1", plan.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM user_subscriptions WHERE id = $1", sub.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", unrelatedUser.ID)
	})

	require.NoError(t, blocker.Rollback())
	select {
	case err := <-deleteResult:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("delete did not resume after releasing group row lock")
	}
}

func TestSystemCustomGroupRepositoryDeleteSerializesConcurrentPlanCreate(t *testing.T) {
	ctx := context.Background()
	repo := NewSystemCustomGroupRepository(integrationEntClient)
	container, _ := createDeletableSystemCustomGroupForIntegration(t, repo, "race-plan-create")
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM subscription_plans WHERE group_id = $1", container.ID)
	})

	blocker, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	var lockedID int64
	require.NoError(t, blocker.QueryRowContext(ctx, "SELECT id FROM groups WHERE id = $1 FOR UPDATE", container.ID).Scan(&lockedID))

	deleteResult := make(chan error, 1)
	deleteCtx, cancelDelete := context.WithTimeout(ctx, 5*time.Second)
	defer cancelDelete()
	go func() { deleteResult <- repo.Delete(deleteCtx, container.ID) }()
	requireSystemCustomDeleteWaitingOnGroupRow(t, ctx)

	type planResult struct {
		plan *dbent.SubscriptionPlan
		err  error
	}
	writerResult := make(chan planResult, 1)
	writerCtx, cancelWriter := context.WithTimeout(ctx, 5*time.Second)
	defer cancelWriter()
	configService := service.NewPaymentConfigService(integrationEntClient, nil, nil)
	go func() {
		plan, err := configService.CreatePlan(writerCtx, service.CreatePlanRequest{
			GroupID: container.ID, Name: "racing plan", Price: 10,
			ValidityDays: 30, ValidityUnit: "day", ForSale: true,
		})
		writerResult <- planResult{plan: plan, err: err}
	}()
	requireGroupReferenceWriterWaiting(t, ctx)

	require.NoError(t, blocker.Rollback())
	require.NoError(t, <-deleteResult)
	result := <-writerResult
	require.Nil(t, result.plan)
	require.ErrorIs(t, result.err, service.ErrGroupNotFound)
	requireDeletedGroupHasNoEffectiveReferences(t, ctx, container.ID)
}

func TestSystemCustomGroupRepositoryDeleteSerializesConcurrentActiveSubscriptionCreate(t *testing.T) {
	ctx := context.Background()
	repo := NewSystemCustomGroupRepository(integrationEntClient)
	container, _ := createDeletableSystemCustomGroupForIntegration(t, repo, "race-subscription-create")
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM user_subscriptions WHERE group_id = $1", container.ID)
	})
	user := mustCreateUser(t, integrationEntClient, &service.User{})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
	})

	blocker, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	var lockedID int64
	require.NoError(t, blocker.QueryRowContext(ctx, "SELECT id FROM groups WHERE id = $1 FOR UPDATE", container.ID).Scan(&lockedID))

	deleteResult := make(chan error, 1)
	deleteCtx, cancelDelete := context.WithTimeout(ctx, 5*time.Second)
	defer cancelDelete()
	go func() { deleteResult <- repo.Delete(deleteCtx, container.ID) }()
	requireSystemCustomDeleteWaitingOnGroupRow(t, ctx)

	writerResult := make(chan error, 1)
	writerCtx, cancelWriter := context.WithTimeout(ctx, 5*time.Second)
	defer cancelWriter()
	subRepo := NewUserSubscriptionRepository(integrationEntClient)
	now := time.Now()
	sub := &service.UserSubscription{
		UserID: user.ID, GroupID: container.ID, StartsAt: now,
		ExpiresAt: now.Add(time.Hour), Status: service.SubscriptionStatusActive,
	}
	go func() { writerResult <- subRepo.Create(writerCtx, sub) }()
	requireGroupReferenceWriterWaiting(t, ctx)

	require.NoError(t, blocker.Rollback())
	require.NoError(t, <-deleteResult)
	require.ErrorIs(t, <-writerResult, service.ErrGroupNotFound)
	require.Zero(t, sub.ID)
	requireDeletedGroupHasNoEffectiveReferences(t, ctx, container.ID)
}

func requireSystemCustomDeleteWaitingOnGroupRow(t *testing.T, ctx context.Context) {
	t.Helper()
	requirePostgresWaitingQuery(t, ctx, "%system_custom_routing_enabled%", "%FOR UPDATE%")
}

func requireGroupReferenceWriterWaiting(t *testing.T, ctx context.Context) {
	t.Helper()
	requirePostgresWaitingQuery(t, ctx, "%pg_advisory_xact_lock%")
}

func requirePostgresWaitingQuery(t *testing.T, ctx context.Context, patterns ...string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		query := `
			SELECT COUNT(*)
			FROM pg_stat_activity
			WHERE datname = current_database()
			  AND wait_event_type = 'Lock'
		`
		args := make([]any, 0, len(patterns))
		for i, pattern := range patterns {
			query += fmt.Sprintf(" AND query ILIKE $%d", i+1)
			args = append(args, pattern)
		}
		var waiting int
		require.NoError(t, integrationDB.QueryRowContext(ctx, query, args...).Scan(&waiting))
		if waiting > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for PostgreSQL lock query matching %v", patterns)
}

func requireDeletedGroupHasNoEffectiveReferences(t *testing.T, ctx context.Context, groupID int64) {
	t.Helper()
	var deleted, plans, subscriptions int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM groups WHERE id = $1 AND deleted_at IS NOT NULL", groupID).Scan(&deleted))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM subscription_plans WHERE group_id = $1 AND for_sale = TRUE", groupID).Scan(&plans))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_subscriptions WHERE group_id = $1 AND deleted_at IS NULL AND status = 'active' AND expires_at > NOW()", groupID).Scan(&subscriptions))
	require.Equal(t, 1, deleted)
	require.Zero(t, plans)
	require.Zero(t, subscriptions)
}

func TestSystemCustomGroupRepositoryDeleteUnreferencedGroupRetiresContainerAndPreservesHistoricalReferences(t *testing.T) {
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
	apiKey, err := integrationEntClient.APIKey.Create().
		SetUserID(user.ID).
		SetGroupID(container.ID).
		SetKey(fmt.Sprintf("sk-system-custom-delete-%d", time.Now().UnixNano())).
		SetName("historical system custom key").
		Save(ctx)
	require.NoError(t, err)
	account := mustCreateAccount(t, integrationEntClient, &service.Account{
		Name: fmt.Sprintf("system-custom-history-%d", time.Now().UnixNano()),
	})
	usage, err := integrationEntClient.UsageLog.Create().
		SetUserID(user.ID).
		SetAPIKeyID(apiKey.ID).
		SetAccountID(account.ID).
		SetGroupID(container.ID).
		SetRequestID(fmt.Sprintf("system-custom-history-%d", time.Now().UnixNano())).
		SetModel("historical-model").
		Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM usage_logs WHERE id = $1", usage.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM api_keys WHERE id = $1", apiKey.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", account.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
	})

	require.NoError(t, repo.Delete(ctx, container.ID))

	for _, query := range []struct {
		name string
		sql  string
		arg  int64
		want int
	}{
		{name: "soft-deleted group retained", sql: "SELECT COUNT(*) FROM groups WHERE id = $1 AND deleted_at IS NOT NULL", arg: container.ID, want: 1},
		{name: "routes removed", sql: "SELECT COUNT(*) FROM system_custom_group_models WHERE group_id = $1", arg: container.ID, want: 0},
		{name: "inactive plans cleaned", sql: "SELECT COUNT(*) FROM subscription_plans WHERE id = $1", arg: plan.ID, want: 0},
		{name: "expired subscriptions retained", sql: "SELECT COUNT(*) FROM user_subscriptions WHERE id = $1 AND group_id = $2", arg: sub.ID, want: 1},
		{name: "api key group retained", sql: "SELECT COUNT(*) FROM api_keys WHERE id = $1 AND group_id = $2", arg: apiKey.ID, want: 1},
		{name: "usage group retained", sql: "SELECT COUNT(*) FROM usage_logs WHERE id = $1 AND group_id = $2", arg: usage.ID, want: 1},
	} {
		t.Run(query.name, func(t *testing.T) {
			var count int
			args := []any{query.arg}
			if query.name == "expired subscriptions retained" || query.name == "api key group retained" || query.name == "usage group retained" {
				args = append(args, container.ID)
			}
			require.NoError(t, integrationDB.QueryRowContext(ctx, query.sql, args...).Scan(&count))
			require.Equal(t, query.want, count)
		})
	}
	reloadedKey, err := integrationEntClient.APIKey.Query().Where(apikey.IDEQ(apiKey.ID)).WithGroup().Only(ctx)
	require.NoError(t, err)
	require.NotNil(t, reloadedKey.GroupID)
	require.Equal(t, container.ID, *reloadedKey.GroupID)
	require.Nil(t, reloadedKey.Edges.Group, "soft-deleted group must reload as unavailable, not as a wallet key")
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
