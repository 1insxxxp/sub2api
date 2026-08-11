//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestCreateSubscriptionOrderInTxRejectsPlanThatBecameUnavailable(t *testing.T) {
	ctx := context.Background()
	client, user, _, snapshot := createPaymentOrderPlanLockFixture(t, ctx)
	_, err := client.SubscriptionPlan.UpdateOneID(snapshot.ID).SetForSale(false).Save(ctx)
	require.NoError(t, err)

	order, err := createSubscriptionOrderFromPlanSnapshot(ctx, client, user, snapshot)

	require.Nil(t, order)
	requirePaymentPlanNotAvailable(t, err)
	requirePaymentOrderCount(t, ctx, client, 0)
}

func TestCreateSubscriptionOrderInTxRejectsPlanWhoseGroupBecameUnavailable(t *testing.T) {
	ctx := context.Background()
	client, user, group, snapshot := createPaymentOrderPlanLockFixture(t, ctx)
	_, err := client.Group.UpdateOneID(group.ID).SetStatus(StatusDisabled).Save(ctx)
	require.NoError(t, err)

	order, err := createSubscriptionOrderFromPlanSnapshot(ctx, client, user, snapshot)

	require.Nil(t, order)
	requirePaymentPlanNotAvailable(t, err)
	requirePaymentOrderCount(t, ctx, client, 0)
}

func TestCreateSubscriptionOrderInTxRejectsChangedPlanSnapshot(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(context.Context, *testing.T, *dbent.Client, *dbent.SubscriptionPlan)
	}{
		{name: "price", mutate: func(ctx context.Context, t *testing.T, client *dbent.Client, plan *dbent.SubscriptionPlan) {
			_, err := client.SubscriptionPlan.UpdateOneID(plan.ID).SetPrice(plan.Price + 5).Save(ctx)
			require.NoError(t, err)
		}},
		{name: "validity days", mutate: func(ctx context.Context, t *testing.T, client *dbent.Client, plan *dbent.SubscriptionPlan) {
			_, err := client.SubscriptionPlan.UpdateOneID(plan.ID).SetValidityDays(plan.ValidityDays + 1).Save(ctx)
			require.NoError(t, err)
		}},
		{name: "validity unit", mutate: func(ctx context.Context, t *testing.T, client *dbent.Client, plan *dbent.SubscriptionPlan) {
			_, err := client.SubscriptionPlan.UpdateOneID(plan.ID).SetValidityUnit("day").Save(ctx)
			require.NoError(t, err)
		}},
		{name: "group", mutate: func(ctx context.Context, t *testing.T, client *dbent.Client, plan *dbent.SubscriptionPlan) {
			target := createPaymentPlanTestGroup(t, ctx, client, "order-lock-target")
			_, err := client.SubscriptionPlan.UpdateOneID(plan.ID).SetGroupID(target.ID).Save(ctx)
			require.NoError(t, err)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client, user, _, snapshot := createPaymentOrderPlanLockFixture(t, ctx)
			tc.mutate(ctx, t, client, snapshot)

			order, err := createSubscriptionOrderFromPlanSnapshot(ctx, client, user, snapshot)

			require.Nil(t, order)
			requirePaymentPlanNotAvailable(t, err)
			requirePaymentOrderCount(t, ctx, client, 0)
		})
	}
}

func TestCreateSubscriptionOrderInTxUsesLockedAvailablePlan(t *testing.T) {
	ctx := context.Background()
	client, user, _, snapshot := createPaymentOrderPlanLockFixture(t, ctx)

	order, err := createSubscriptionOrderFromPlanSnapshot(ctx, client, user, snapshot)

	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, snapshot.ID, paymentOrderValueOrZero(order.PlanID))
	require.Equal(t, snapshot.GroupID, paymentOrderValueOrZero(order.SubscriptionGroupID))
	requirePaymentOrderCount(t, ctx, client, 1)
}

func createPaymentOrderPlanLockFixture(t *testing.T, ctx context.Context) (*dbent.Client, *User, *dbent.Group, *dbent.SubscriptionPlan) {
	t.Helper()
	client := newPaymentConfigServiceTestClient(t)
	userRow, err := client.User.Create().
		SetEmail("plan-lock@example.com").
		SetPasswordHash("hash").
		SetUsername("plan-lock-user").
		Save(ctx)
	require.NoError(t, err)
	group := createPaymentPlanTestGroup(t, ctx, client, "order-lock")
	plan, err := client.SubscriptionPlan.Create().
		SetGroupID(group.ID).
		SetName("locked plan").
		SetPrice(12.5).
		SetValidityDays(1).
		SetValidityUnit("month").
		SetForSale(true).
		Save(ctx)
	require.NoError(t, err)
	return client, &User{ID: userRow.ID, Email: userRow.Email, Username: userRow.Username}, group, plan
}

func createSubscriptionOrderFromPlanSnapshot(ctx context.Context, client *dbent.Client, user *User, snapshot *dbent.SubscriptionPlan) (*dbent.PaymentOrder, error) {
	return (&PaymentService{entClient: client}).createOrderInTx(
		ctx,
		CreateOrderRequest{
			UserID: user.ID, PaymentType: payment.TypeAlipay, OrderType: payment.OrderTypeSubscription,
			PlanID: snapshot.ID, ClientIP: "127.0.0.1", SrcHost: "test.local",
		},
		user,
		snapshot,
		&PaymentConfig{MaxPendingOrders: 5, OrderTimeoutMin: 30},
		snapshot.Price,
		snapshot.Price,
		0,
		snapshot.Price,
		nil,
	)
}

func requirePaymentPlanNotAvailable(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var appErr *infraerrors.ApplicationError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "PLAN_NOT_AVAILABLE", appErr.Reason)
}

func requirePaymentOrderCount(t *testing.T, ctx context.Context, client *dbent.Client, want int) {
	t.Helper()
	count, err := client.PaymentOrder.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, want, count)
}

func paymentOrderValueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
