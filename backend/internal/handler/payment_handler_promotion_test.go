package handler

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestSafeBalanceRechargePromotionInfoOmitsBlacklistedUsers(t *testing.T) {
	promotion := &service.BalanceRechargePromotion{
		Enabled:          true,
		Name:             "Autumn bonus",
		StartAt:          "2026-10-01T00:00:00Z",
		EndAt:            "2026-11-01T00:00:00Z",
		Multiplier:       2.5,
		BlacklistUserIDs: []int64{8},
	}
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	info := safeBalanceRechargePromotionInfo(promotion, 7, now)
	if info == nil || !info.Active || info.Name != promotion.Name || info.EndAt != promotion.EndAt || info.Multiplier != promotion.Multiplier {
		t.Fatalf("safe promotion info = %+v", info)
	}
	if blocked := safeBalanceRechargePromotionInfo(promotion, 8, now); blocked != nil {
		t.Fatalf("blacklisted user received promotion info: %+v", blocked)
	}
}
