package service

import (
	"context"
	"math"
	"testing"
)

func TestSelectBalanceRechargeMultiplier(t *testing.T) {
	tiers := []BalanceRechargeTier{
		{Amount: 10, Multiplier: 2}, {Amount: 18, Multiplier: 3}, {Amount: 80, Multiplier: 4},
	}
	for _, tt := range []struct {
		amount, want float64
	}{
		{10, 2}, {18, 3}, {80, 4},
	} {
		if got := selectBalanceRechargeMultiplier(tt.amount, tiers, 1); got != tt.want {
			t.Fatalf("amount %.2f: got %.2f, want %.2f", tt.amount, got, tt.want)
		}
	}
}

func TestRechargeTiersConfigRoundTrip(t *testing.T) {
	repo := &paymentConfigSettingRepoStub{values: map[string]string{}}
	svc := &PaymentConfigService{settingRepo: repo}
	tiers := []BalanceRechargeTier{{Amount: 10, Multiplier: 2}, {Amount: 18, Multiplier: 3}}
	if err := svc.UpdatePaymentConfig(context.Background(), UpdatePaymentConfigRequest{BalanceRechargeTiers: &tiers}); err != nil {
		t.Fatal(err)
	}
	cfg := svc.parsePaymentConfig(repo.values)
	if len(cfg.BalanceRechargeTiers) != 2 || selectBalanceRechargeMultiplier(18, cfg.BalanceRechargeTiers, 1) != 3 {
		t.Fatalf("tiers not preserved: %+v", cfg.BalanceRechargeTiers)
	}
	invalid := []BalanceRechargeTier{{Amount: 10, Multiplier: 0}}
	if err := svc.UpdatePaymentConfig(context.Background(), UpdatePaymentConfigRequest{BalanceRechargeTiers: &invalid}); err == nil {
		t.Fatal("invalid rules accepted")
	}
	if len(svc.parsePaymentConfig(repo.values).BalanceRechargeTiers) != 2 {
		t.Fatal("invalid update overwrote saved rules")
	}
	empty := []BalanceRechargeTier{}
	if err := svc.UpdatePaymentConfig(context.Background(), UpdatePaymentConfigRequest{BalanceRechargeTiers: &empty}); err != nil {
		t.Fatal(err)
	}
	if len(svc.parsePaymentConfig(repo.values).BalanceRechargeTiers) != 0 {
		t.Fatal("rules were not cleared")
	}
}

func TestSelectBalanceRechargeMultiplierFallsBack(t *testing.T) {
	if got := selectBalanceRechargeMultiplier(20, nil, 5); got != 5 {
		t.Fatalf("got %.2f, want 5", got)
	}
}

func TestIsPresetBalanceRechargeAmountUsesLegacyDefaultsWhenTiersAreEmpty(t *testing.T) {
	if !isPresetBalanceRechargeAmount(100, nil) {
		t.Fatal("expected legacy preset amount to be accepted")
	}
	if isPresetBalanceRechargeAmount(101, nil) {
		t.Fatal("arbitrary amount must be rejected without configured tiers")
	}
	if !isPresetBalanceRechargeAmount(18, []BalanceRechargeTier{{Amount: 18, Multiplier: 3}}) {
		t.Fatal("configured preset amount must be accepted")
	}
	if isPresetBalanceRechargeAmount(100, []BalanceRechargeTier{{Amount: 18, Multiplier: 3}}) {
		t.Fatal("amount outside configured tiers must be rejected")
	}
}

func TestValidateBalanceRechargeTiers(t *testing.T) {
	for _, tiers := range [][]BalanceRechargeTier{
		{{Amount: 10, Multiplier: 0}},
		{{Amount: 10, Multiplier: 2}, {Amount: 10, Multiplier: 3}},
		{{Amount: -1, Multiplier: 2}},
		{{Amount: 10, Multiplier: math.Inf(1)}},
		{{Amount: 18, Multiplier: 2}, {Amount: 18, Multiplier: 3}},
		{{Amount: 18, Multiplier: 2}, {Amount: 10, Multiplier: 3}},
	} {
		if err := validateBalanceRechargeTiers(tiers); err == nil {
			t.Fatalf("accepted invalid tiers: %+v", tiers)
		}
	}
	if err := validateBalanceRechargeTiers(nil); err != nil {
		t.Fatal(err)
	}
	if err := validateBalanceRechargeTiers([]BalanceRechargeTier{{Amount: 10, Multiplier: 2}, {Amount: 18, Multiplier: 3}}); err != nil {
		t.Fatal(err)
	}
}
