package service

import (
	"testing"
	"time"
)

func TestParseBalanceRechargePromotionDefaults(t *testing.T) {
	for _, raw := range []string{"", "not-json", `{"enabled":true,"multiplier":0}`} {
		got := parseBalanceRechargePromotion(raw)
		if got == nil {
			t.Fatalf("parseBalanceRechargePromotion(%q) returned nil", raw)
		}
		if got.Enabled || got.Name != "" || got.Multiplier != 0 || len(got.BlacklistUserIDs) != 0 {
			t.Fatalf("parseBalanceRechargePromotion(%q) = %+v, want disabled defaults", raw, got)
		}
	}

	raw := `{"enabled":true,"name":"Double balance","start_at":"2026-10-01T00:00:00Z","end_at":"2026-10-08T00:00:00Z","multiplier":2.5,"blacklist_user_ids":[3,8]}`
	got := parseBalanceRechargePromotion(raw)
	if got == nil || !got.Enabled || got.Name != "Double balance" || got.Multiplier != 2.5 {
		t.Fatalf("valid promotion did not parse: %+v", got)
	}
	if got.StartAt != "2026-10-01T00:00:00Z" || got.EndAt != "2026-10-08T00:00:00Z" {
		t.Fatalf("promotion window not preserved: %+v", got)
	}
	if len(got.BlacklistUserIDs) != 2 || got.BlacklistUserIDs[0] != 3 || got.BlacklistUserIDs[1] != 8 {
		t.Fatalf("promotion blacklist not preserved: %+v", got.BlacklistUserIDs)
	}
}

func TestValidateBalanceRechargePromotion(t *testing.T) {
	valid := BalanceRechargePromotion{
		Enabled:          true,
		StartAt:          "2026-10-01T00:00:00Z",
		EndAt:            "2026-10-08T00:00:00Z",
		Multiplier:       2,
		BlacklistUserIDs: []int64{3, 8},
	}
	if err := validateBalanceRechargePromotion(valid); err != nil {
		t.Fatalf("valid promotion rejected: %v", err)
	}

	tests := []struct {
		name string
		edit func(*BalanceRechargePromotion)
	}{
		{"non-positive multiplier", func(p *BalanceRechargePromotion) { p.Multiplier = 0 }},
		{"invalid start timestamp", func(p *BalanceRechargePromotion) { p.StartAt = "2026-10-01" }},
		{"invalid end timestamp", func(p *BalanceRechargePromotion) { p.EndAt = "not-a-time" }},
		{"equal timestamps", func(p *BalanceRechargePromotion) { p.EndAt = p.StartAt }},
		{"end before start", func(p *BalanceRechargePromotion) { p.EndAt = "2026-09-30T00:00:00Z" }},
		{"zero blacklist id", func(p *BalanceRechargePromotion) { p.BlacklistUserIDs = []int64{0} }},
		{"negative blacklist id", func(p *BalanceRechargePromotion) { p.BlacklistUserIDs = []int64{-1} }},
		{"duplicate blacklist id", func(p *BalanceRechargePromotion) { p.BlacklistUserIDs = []int64{3, 3} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := valid
			candidate.BlacklistUserIDs = append([]int64(nil), valid.BlacklistUserIDs...)
			tt.edit(&candidate)
			if err := validateBalanceRechargePromotion(candidate); err == nil {
				t.Fatalf("invalid promotion accepted: %+v", candidate)
			}
		})
	}
}

func TestBalanceRechargePromotionActiveWindowIsStartInclusiveEndExclusive(t *testing.T) {
	promotion := &BalanceRechargePromotion{
		Enabled:    true,
		StartAt:    "2026-10-01T00:00:00Z",
		EndAt:      "2026-10-08T00:00:00Z",
		Multiplier: 2,
	}
	for _, tt := range []struct {
		name string
		now  string
		want bool
	}{
		{"before start", "2026-09-30T23:59:59Z", false},
		{"at start", "2026-10-01T00:00:00Z", true},
		{"inside window", "2026-10-04T12:00:00Z", true},
		{"at end", "2026-10-08T00:00:00Z", false},
		{"after end", "2026-10-08T00:00:01Z", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, tt.now)
			if err != nil {
				t.Fatal(err)
			}
			if got := promotion.IsActiveAt(now); got != tt.want {
				t.Fatalf("IsActiveAt(%s) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}

func TestBalanceRechargePromotionBlacklist(t *testing.T) {
	promotion := &BalanceRechargePromotion{BlacklistUserIDs: []int64{3, 8}}
	for _, tt := range []struct {
		userID int64
		want   bool
	}{
		{3, true}, {8, true}, {9, false}, {0, false}, {-1, false},
	} {
		if got := promotion.IsBlacklisted(tt.userID); got != tt.want {
			t.Fatalf("IsBlacklisted(%d) = %v, want %v", tt.userID, got, tt.want)
		}
	}
}

func TestResolveBalanceRechargeMultiplierUsesPromotionOrFallback(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	active := &BalanceRechargePromotion{
		Enabled:          true,
		StartAt:          "2026-10-01T00:00:00Z",
		EndAt:            "2026-10-08T00:00:00Z",
		Multiplier:       2.5,
		BlacklistUserIDs: []int64{8},
	}
	tiers := []BalanceRechargeTier{{Amount: 10, Multiplier: 1.5}}
	if got := resolveBalanceRechargeMultiplier(10, tiers, 1, active, 3, now); got != 2.5 {
		t.Fatalf("eligible user got multiplier %.2f, want 2.5", got)
	}
	if got := resolveBalanceRechargeMultiplier(10, tiers, 1, active, 8, now); got != 1.5 {
		t.Fatalf("blacklisted user got multiplier %.2f, want tier multiplier 1.5", got)
	}
	if got := resolveBalanceRechargeMultiplier(20, tiers, 1, active, 3, now.Add(24*time.Hour)); got != 1 {
		t.Fatalf("inactive promotion got multiplier %.2f, want global fallback 1", got)
	}
	if got := resolveBalanceRechargeMultiplier(10, tiers, 1, nil, 3, now); got != 1.5 {
		t.Fatalf("missing promotion got multiplier %.2f, want tier multiplier 1.5", got)
	}
}
