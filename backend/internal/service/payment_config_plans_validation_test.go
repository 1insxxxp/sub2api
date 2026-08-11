//go:build unit

package service

import (
	"context"
	"fmt"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestValidatePlanRequired_AllValid(t *testing.T) {
	err := validatePlanRequired("Pro", 1, 9.99, 30, "days", nil)
	require.NoError(t, err)
}

func TestValidatePlanRequired_EmptyName(t *testing.T) {
	err := validatePlanRequired("", 1, 9.99, 30, "days", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "plan name")
}

func TestValidatePlanRequired_WhitespaceName(t *testing.T) {
	err := validatePlanRequired("   ", 1, 9.99, 30, "days", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "plan name")
}

func TestValidatePlanRequired_ZeroGroupID(t *testing.T) {
	err := validatePlanRequired("Pro", 0, 9.99, 30, "days", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "group")
}

func TestValidatePlanRequired_NegativeGroupID(t *testing.T) {
	err := validatePlanRequired("Pro", -1, 9.99, 30, "days", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "group")
}

func TestValidatePlanRequired_ZeroPrice(t *testing.T) {
	err := validatePlanRequired("Pro", 1, 0, 30, "days", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "price")
}

func TestValidatePlanRequired_NegativePrice(t *testing.T) {
	err := validatePlanRequired("Pro", 1, -5, 30, "days", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "price")
}

func TestValidatePlanRequired_ZeroValidityDays(t *testing.T) {
	err := validatePlanRequired("Pro", 1, 9.99, 0, "days", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "validity days")
}

func TestValidatePlanRequired_NegativeValidityDays(t *testing.T) {
	err := validatePlanRequired("Pro", 1, 9.99, -7, "days", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "validity days")
}

func TestValidatePlanRequired_EmptyValidityUnit(t *testing.T) {
	err := validatePlanRequired("Pro", 1, 9.99, 30, "", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "validity unit")
}

func TestValidatePlanRequired_WhitespaceValidityUnit(t *testing.T) {
	err := validatePlanRequired("Pro", 1, 9.99, 30, "   ", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "validity unit")
}

func TestValidatePlanRequired_NameValidatedFirst(t *testing.T) {
	err := validatePlanRequired("", 0, 0, 0, "", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "plan name")
}

func TestValidatePlanRequired_TrimmedValidName(t *testing.T) {
	err := validatePlanRequired("  Pro  ", 1, 9.99, 30, "days", nil)
	require.NoError(t, err)
}

func TestValidatePlanRequired_NegativeOriginalPrice(t *testing.T) {
	neg := -10.0
	err := validatePlanRequired("Pro", 1, 9.99, 30, "days", &neg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "original price")
}

func TestValidatePlanRequired_ZeroOriginalPrice(t *testing.T) {
	zero := 0.0
	err := validatePlanRequired("Pro", 1, 9.99, 30, "days", &zero)
	require.NoError(t, err)
}

func TestValidatePlanRequired_ValidOriginalPrice(t *testing.T) {
	op := 19.99
	err := validatePlanRequired("Pro", 1, 9.99, 30, "days", &op)
	require.NoError(t, err)
}

// --- validatePlanPatch tests ---

func TestValidatePlanPatch_NegativeOriginalPrice(t *testing.T) {
	neg := -5.0
	err := validatePlanPatch(UpdatePlanRequest{OriginalPrice: &neg})
	require.Error(t, err)
	require.Contains(t, err.Error(), "original price")
}

func TestValidatePlanPatch_ZeroOriginalPrice(t *testing.T) {
	zero := 0.0
	err := validatePlanPatch(UpdatePlanRequest{OriginalPrice: &zero})
	require.NoError(t, err)
}

func TestValidatePlanPatch_ValidOriginalPrice(t *testing.T) {
	op := 29.99
	err := validatePlanPatch(UpdatePlanRequest{OriginalPrice: &op})
	require.NoError(t, err)
}

func TestValidatePlanPatch_NilOriginalPrice(t *testing.T) {
	err := validatePlanPatch(UpdatePlanRequest{OriginalPrice: nil})
	require.NoError(t, err)
}

// --- validatePlanPatch: other fields ---

func ptrStr(s string) *string     { return &s }
func ptrInt(i int) *int           { return &i }
func ptrInt64(i int64) *int64     { return &i }
func ptrFloat(f float64) *float64 { return &f }

func TestValidatePlanPatch_EmptyName(t *testing.T) {
	err := validatePlanPatch(UpdatePlanRequest{Name: ptrStr("")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "plan name")
}

func TestValidatePlanPatch_ValidName(t *testing.T) {
	err := validatePlanPatch(UpdatePlanRequest{Name: ptrStr("Basic")})
	require.NoError(t, err)
}

func TestValidatePlanPatch_ZeroGroupID(t *testing.T) {
	err := validatePlanPatch(UpdatePlanRequest{GroupID: ptrInt64(0)})
	require.Error(t, err)
	require.Contains(t, err.Error(), "group")
}

func TestValidatePlanPatch_NegativePrice(t *testing.T) {
	err := validatePlanPatch(UpdatePlanRequest{Price: ptrFloat(-1)})
	require.Error(t, err)
	require.Contains(t, err.Error(), "price")
}

func TestValidatePlanPatch_ZeroPrice(t *testing.T) {
	err := validatePlanPatch(UpdatePlanRequest{Price: ptrFloat(0)})
	require.Error(t, err)
	require.Contains(t, err.Error(), "price")
}

func TestValidatePlanPatch_ValidPrice(t *testing.T) {
	err := validatePlanPatch(UpdatePlanRequest{Price: ptrFloat(9.99)})
	require.NoError(t, err)
}

func TestValidatePlanPatch_ZeroValidityDays(t *testing.T) {
	err := validatePlanPatch(UpdatePlanRequest{ValidityDays: ptrInt(0)})
	require.Error(t, err)
	require.Contains(t, err.Error(), "validity days")
}

func TestValidatePlanPatch_EmptyValidityUnit(t *testing.T) {
	err := validatePlanPatch(UpdatePlanRequest{ValidityUnit: ptrStr("")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "validity unit")
}

func TestValidatePlanPatch_ValidValidityUnit(t *testing.T) {
	err := validatePlanPatch(UpdatePlanRequest{ValidityUnit: ptrStr("days")})
	require.NoError(t, err)
}

func TestValidatePlanPatch_AllNil(t *testing.T) {
	err := validatePlanPatch(UpdatePlanRequest{})
	require.NoError(t, err)
}

// --- normalizePlanCurrency tests ---
// Empty must stay empty (not coerced to the default payment currency),
// so existing plans keep rendering without any currency label.

func TestNormalizePlanCurrency_EmptyKeepsEmpty(t *testing.T) {
	currency, err := normalizePlanCurrency("")
	require.NoError(t, err)
	require.Equal(t, "", currency)
}

func TestNormalizePlanCurrency_WhitespaceKeepsEmpty(t *testing.T) {
	currency, err := normalizePlanCurrency("   ")
	require.NoError(t, err)
	require.Equal(t, "", currency)
}

func TestNormalizePlanCurrency_LowercaseNormalized(t *testing.T) {
	currency, err := normalizePlanCurrency("nzd")
	require.NoError(t, err)
	require.Equal(t, "NZD", currency)
}

func TestNormalizePlanCurrency_ValidUppercase(t *testing.T) {
	currency, err := normalizePlanCurrency("USD")
	require.NoError(t, err)
	require.Equal(t, "USD", currency)
}

func TestNormalizePlanCurrency_TooShort(t *testing.T) {
	_, err := normalizePlanCurrency("NZ")
	require.Error(t, err)
	require.Contains(t, err.Error(), "currency")
}

func TestNormalizePlanCurrency_TooLong(t *testing.T) {
	_, err := normalizePlanCurrency("NZDD")
	require.Error(t, err)
	require.Contains(t, err.Error(), "currency")
}

func TestNormalizePlanCurrency_NonLetter(t *testing.T) {
	_, err := normalizePlanCurrency("N2D")
	require.Error(t, err)
	require.Contains(t, err.Error(), "currency")
}

func TestPaymentConfigCreatePlanRejectsUnavailableSubscriptionGroup(t *testing.T) {
	tests := []struct {
		name      string
		configure func(context.Context, *dbent.Client, *dbent.Group)
		wantErr   error
	}{
		{
			name: "soft deleted",
			configure: func(ctx context.Context, client *dbent.Client, group *dbent.Group) {
				require.NoError(t, client.Group.DeleteOneID(group.ID).Exec(ctx))
			},
			wantErr: ErrGroupNotFound,
		},
		{
			name: "disabled",
			configure: func(ctx context.Context, client *dbent.Client, group *dbent.Group) {
				_, err := client.Group.UpdateOneID(group.ID).SetStatus(StatusDisabled).Save(ctx)
				require.NoError(t, err)
			},
			wantErr: ErrGroupDisabled,
		},
		{
			name: "not subscription type",
			configure: func(ctx context.Context, client *dbent.Client, group *dbent.Group) {
				_, err := client.Group.UpdateOneID(group.ID).SetSubscriptionType(SubscriptionTypeStandard).Save(ctx)
				require.NoError(t, err)
			},
			wantErr: ErrGroupNotSubscriptionType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			group := createPaymentPlanTestGroup(t, ctx, client, "create-"+tc.name)
			tc.configure(ctx, client, group)
			svc := &PaymentConfigService{entClient: client}

			plan, err := svc.CreatePlan(ctx, validCreatePlanRequest(group.ID))

			require.Nil(t, plan)
			require.ErrorIs(t, err, tc.wantErr)
			count, countErr := client.SubscriptionPlan.Query().Count(ctx)
			require.NoError(t, countErr)
			require.Zero(t, count)
		})
	}
}

func TestPaymentConfigUpdatePlanRejectsUnavailableTargetSubscriptionGroup(t *testing.T) {
	tests := []struct {
		name      string
		configure func(context.Context, *dbent.Client, *dbent.Group)
		wantErr   error
	}{
		{
			name: "soft deleted",
			configure: func(ctx context.Context, client *dbent.Client, group *dbent.Group) {
				require.NoError(t, client.Group.DeleteOneID(group.ID).Exec(ctx))
			},
			wantErr: ErrGroupNotFound,
		},
		{
			name: "disabled",
			configure: func(ctx context.Context, client *dbent.Client, group *dbent.Group) {
				_, err := client.Group.UpdateOneID(group.ID).SetStatus(StatusDisabled).Save(ctx)
				require.NoError(t, err)
			},
			wantErr: ErrGroupDisabled,
		},
		{
			name: "not subscription type",
			configure: func(ctx context.Context, client *dbent.Client, group *dbent.Group) {
				_, err := client.Group.UpdateOneID(group.ID).SetSubscriptionType(SubscriptionTypeStandard).Save(ctx)
				require.NoError(t, err)
			},
			wantErr: ErrGroupNotSubscriptionType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			oldGroup := createPaymentPlanTestGroup(t, ctx, client, "old-"+tc.name)
			targetGroup := createPaymentPlanTestGroup(t, ctx, client, "target-"+tc.name)
			plan, err := client.SubscriptionPlan.Create().
				SetGroupID(oldGroup.ID).
				SetName("existing").
				SetPrice(10).
				SetValidityDays(30).
				SetValidityUnit("day").
				SetForSale(false).
				Save(ctx)
			require.NoError(t, err)
			tc.configure(ctx, client, targetGroup)
			svc := &PaymentConfigService{entClient: client}
			forSale := true

			updated, err := svc.UpdatePlan(ctx, plan.ID, UpdatePlanRequest{GroupID: &targetGroup.ID, ForSale: &forSale})

			require.Nil(t, updated)
			require.ErrorIs(t, err, tc.wantErr)
			persisted, getErr := client.SubscriptionPlan.Get(ctx, plan.ID)
			require.NoError(t, getErr)
			require.Equal(t, oldGroup.ID, persisted.GroupID)
			require.False(t, persisted.ForSale)
		})
	}
}

func TestPaymentConfigPlanMutationsAllowActiveSubscriptionGroup(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	group := createPaymentPlanTestGroup(t, ctx, client, "normal")
	svc := &PaymentConfigService{entClient: client}

	plan, err := svc.CreatePlan(ctx, validCreatePlanRequest(group.ID))
	require.NoError(t, err)
	require.Equal(t, group.ID, plan.GroupID)
	name := "updated"
	updated, err := svc.UpdatePlan(ctx, plan.ID, UpdatePlanRequest{Name: &name})
	require.NoError(t, err)
	require.Equal(t, name, updated.Name)
}

func createPaymentPlanTestGroup(t *testing.T, ctx context.Context, client *dbent.Client, suffix string) *dbent.Group {
	t.Helper()
	group, err := client.Group.Create().
		SetName(fmt.Sprintf("payment-plan-%s", suffix)).
		SetStatus(StatusActive).
		SetSubscriptionType(SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)
	return group
}

func validCreatePlanRequest(groupID int64) CreatePlanRequest {
	return CreatePlanRequest{
		GroupID: groupID, Name: "monthly", Price: 10, ValidityDays: 30,
		ValidityUnit: "day", ForSale: true,
	}
}
