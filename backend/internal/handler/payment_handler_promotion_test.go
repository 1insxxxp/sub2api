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
	if info == nil || !info.Active || info.Name != promotion.Name || info.StartAt != promotion.StartAt || info.EndAt != promotion.EndAt || info.Multiplier != promotion.Multiplier {
		t.Fatalf("safe promotion info = %+v", info)
	}
	upcoming := safeBalanceRechargePromotionInfo(promotion, 7, time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC))
	if upcoming == nil || upcoming.Active || upcoming.StartAt != promotion.StartAt {
		t.Fatalf("upcoming promotion info = %+v", upcoming)
	}
	if blocked := safeBalanceRechargePromotionInfo(promotion, 8, now); blocked != nil {
		t.Fatalf("blacklisted user received promotion info: %+v", blocked)
	}
}
