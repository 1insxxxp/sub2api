package service

import (
	"testing"
	"time"
)

func TestBalanceRechargeOrderAmountUsesEligiblePromotionOnly(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	cfg := &PaymentConfig{
		BalanceRechargeMultiplier: 1.25,
		BalanceRechargeTiers:      []BalanceRechargeTier{{Amount: 100, Multiplier: 1.5}},
		BalanceRechargePromotion: &BalanceRechargePromotion{
			Enabled:          true,
			StartAt:          "2026-10-01T00:00:00Z",
			EndAt:            "2026-11-01T00:00:00Z",
			Multiplier:       2.25,
			BlacklistUserIDs: []int64{8},
		},
	}
	if got := resolvePaymentOrderBalanceAmount(100, cfg, 7, now); got != 225 {
		t.Fatalf("eligible order amount = %v, want 225", got)
	}
	if got := resolvePaymentOrderBalanceAmount(100, cfg, 8, now); got != 150 {
		t.Fatalf("blacklisted order amount = %v, want tier result 150", got)
	}
	if got := resolvePaymentOrderBalanceAmount(100, cfg, 7, now.Add(30*24*time.Hour)); got != 150 {
		t.Fatalf("expired order amount = %v, want tier result 150", got)
	}
}
