package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSafeRechargePriceTiers(t *testing.T) {
	var p service.BalanceRechargePromotion
	require.NoError(t, json.Unmarshal([]byte(`{"enabled":true,"start_at":"2026-10-01T00:00:00Z","end_at":"2026-11-01T00:00:00Z","price_tiers":[{"credited_amount":50,"price":8}],"blacklist_user_ids":[8]}`), &p))
	now := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	info := safeBalanceRechargePromotionInfo(&p, 7, now)
	require.NotNil(t, info)
	raw, err := json.Marshal(info)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"price_tiers":[{"credited_amount":50,"price":8}]`)
	require.NotContains(t, string(raw), "blacklist")
	require.Nil(t, safeBalanceRechargePromotionInfo(&p, 8, now))
	require.Nil(t, safeBalanceRechargePromotionInfo(&p, 7, now.AddDate(0, 1, 0)))
}

func TestSafeBalanceRechargePromotionInfoOmitsBlacklistedUsers(t *testing.T) {
	promotion := &service.BalanceRechargePromotion{
		Enabled:          true,
		Name:             "Autumn bonus",
		StartAt:          "2026-10-01T00:00:00Z",
		EndAt:            "2026-11-01T00:00:00Z",
		PriceTiers:       []service.BalanceRechargePromotionPriceTier{{CreditedAmount: 15, Price: 10}},
		BlacklistUserIDs: []int64{8},
	}
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	info := safeBalanceRechargePromotionInfo(promotion, 7, now)
	if info == nil || !info.Active || info.Name != promotion.Name || info.StartAt != promotion.StartAt || info.EndAt != promotion.EndAt || len(info.PriceTiers) != 1 {
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
