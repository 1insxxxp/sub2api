package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func priceTierPromotion(t *testing.T, rows string) *BalanceRechargePromotion {
	t.Helper()
	var p BalanceRechargePromotion
	require.NoError(t, json.Unmarshal([]byte(`{"enabled":true,"start_at":"2026-10-01T00:00:00Z","end_at":"2026-11-01T00:00:00Z","blacklist_user_ids":[8],"price_tiers":`+rows+`}`), &p))
	return &p
}

func TestRechargePriceTiersParseAndResolve(t *testing.T) {
	p := priceTierPromotion(t, `[{"credited_amount":50,"price":8}]`)
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	require.JSONEq(t, `[{"credited_amount":50,"price":8}]`, func() string {
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &fields))
		return string(fields["price_tiers"])
	}())
	require.True(t, parseBalanceRechargePromotion(string(raw)).Enabled)
	cfg := &PaymentConfig{BalanceRechargeMultiplier: 1, BalanceRechargeTiers: []BalanceRechargeTier{{Amount: 10, Multiplier: 5}, {Amount: 18, Multiplier: 5}}, BalanceRechargePromotion: p}
	now := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	require.Equal(t, 50.0, resolvePaymentOrderBalanceAmount(8, cfg, 7, now))
	require.Equal(t, 50.0, resolvePaymentOrderBalanceAmount(10, cfg, 7, now))
	require.Equal(t, 90.0, resolvePaymentOrderBalanceAmount(18, cfg, 7, now))
	for _, user := range []int64{0, 8} {
		require.Equal(t, 8.0, resolvePaymentOrderBalanceAmount(8, cfg, user, now))
	}
	for _, date := range []time.Time{now.AddDate(0, -1, 0), now.AddDate(0, 1, 0)} {
		require.Equal(t, 8.0, resolvePaymentOrderBalanceAmount(8, cfg, 7, date))
	}
	p.Multiplier = 6
	require.Equal(t, 50.0, resolvePaymentOrderBalanceAmount(8, cfg, 7, now), "price tier wins over legacy multiplier")
	require.Equal(t, 108.0, resolvePaymentOrderBalanceAmount(18, cfg, 7, now), "legacy multiplier still applies to other amounts")
}

func TestRechargePriceTiersConfigValidation(t *testing.T) {
	base := []BalanceRechargeTier{{Amount: 10, Multiplier: 5}, {Amount: 18, Multiplier: 5}}
	for _, tc := range []struct {
		name, rows string
		valid      bool
	}{
		{"discount", `[{"credited_amount":50,"price":8}]`, true},
		{"same price", `[{"credited_amount":50,"price":10}]`, true},
		{"zero price", `[{"credited_amount":50,"price":0}]`, false},
		{"negative credit", `[{"credited_amount":-50,"price":8}]`, false},
		{"fractional cent", `[{"credited_amount":50,"price":8.001}]`, false},
		{"unknown credit", `[{"credited_amount":60,"price":8}]`, false},
		{"duplicate credit", `[{"credited_amount":50,"price":8},{"credited_amount":50,"price":9}]`, false},
		{"duplicate price", `[{"credited_amount":50,"price":8},{"credited_amount":90,"price":8}]`, false},
		{"base price collision", `[{"credited_amount":50,"price":18}]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &paymentConfigSettingRepoStub{values: map[string]string{}}
			svc := &PaymentConfigService{settingRepo: repo}
			p := priceTierPromotion(t, tc.rows)
			err := svc.UpdatePaymentConfig(context.Background(), UpdatePaymentConfigRequest{BalanceRechargeTiers: &base, BalanceRechargePromotion: p})
			if !tc.valid {
				require.Error(t, err)
				require.Empty(t, repo.updates)
				return
			}
			require.NoError(t, err)
			cfg := svc.parsePaymentConfig(repo.values)
			require.True(t, cfg.BalanceRechargePromotion.Enabled)
			require.NoError(t, svc.UpdatePaymentConfig(context.Background(), UpdatePaymentConfigRequest{BalanceRechargePromotion: p}))
			changed := []BalanceRechargeTier{{Amount: 10, Multiplier: 6}}
			require.Error(t, svc.UpdatePaymentConfig(context.Background(), UpdatePaymentConfigRequest{BalanceRechargeTiers: &changed}))
			// A stale persisted reference must never grant the old credit amount.
			repo.values[SettingBalanceRechargeTiers] = `[{"amount":10,"multiplier":6}]`
			require.False(t, svc.parsePaymentConfig(repo.values).BalanceRechargePromotion.Enabled)
		})
	}
}

func TestRechargePriceTiersOrderEligibilityAndLimits(t *testing.T) {
	p := priceTierPromotion(t, `[{"credited_amount":50,"price":8}]`)
	p.StartAt = time.Now().Add(-time.Hour).Format(time.RFC3339)
	p.EndAt = time.Now().Add(time.Hour).Format(time.RFC3339)
	cfg := &PaymentConfig{BalanceRechargeTiers: []BalanceRechargeTier{{Amount: 10, Multiplier: 5}}, BalanceRechargePromotion: p}
	svc := &PaymentService{}
	req := CreateOrderRequest{UserID: 7, Amount: 8, OrderType: payment.OrderTypeBalance}
	_, err := svc.validateOrderInput(context.Background(), req, cfg, time.Now())
	require.NoError(t, err)
	_, pay, err := calculateCreateOrderPayAmountForOrderType(req.Amount, 0, "CNY", payment.OrderTypeBalance, 0)
	require.NoError(t, err)
	require.Equal(t, 8.0, pay)
	require.Equal(t, 50.0, resolvePaymentOrderBalanceAmount(req.Amount, cfg, req.UserID, time.Now()))
	require.Equal(t, 8.0, calculateGatewayRefundAmount(50, pay, 50, "CNY"))
	require.Equal(t, 4.0, calculateGatewayRefundAmount(50, pay, 25, "CNY"))
	for _, id := range []int64{0, 8} {
		req.UserID = id
		_, err = svc.validateOrderInput(context.Background(), req, cfg, time.Now())
		require.Error(t, err)
	}
	req.UserID = 7
	cfg.MinAmount = 9
	_, err = svc.validateOrderInput(context.Background(), req, cfg, time.Now())
	require.Error(t, err)
	cfg.MinAmount = 0
	p.Enabled = false
	_, err = svc.validateOrderInput(context.Background(), req, cfg, time.Now())
	require.Error(t, err)
	req.Amount = 10
	_, err = svc.validateOrderInput(context.Background(), req, cfg, time.Now())
	require.NoError(t, err)
}
