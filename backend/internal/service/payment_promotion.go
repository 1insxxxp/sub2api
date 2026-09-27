package service

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// BalanceRechargePromotion describes the one site-wide balance recharge
// activity. StartAt is inclusive and EndAt is exclusive. A malformed or
// missing setting is treated as the zero (disabled) promotion by the parser.
type BalanceRechargePromotion struct {
	Enabled          bool                                `json:"enabled"`
	Name             string                              `json:"name,omitempty"`
	StartAt          string                              `json:"start_at,omitempty"`
	EndAt            string                              `json:"end_at,omitempty"`
	Multiplier       float64                             `json:"multiplier"`
	BlacklistUserIDs []int64                             `json:"blacklist_user_ids,omitempty"`
	PriceTiers       []BalanceRechargePromotionPriceTier `json:"price_tiers,omitempty"`
}

type BalanceRechargePromotionPriceTier struct {
	CreditedAmount float64 `json:"credited_amount"`
	Price          float64 `json:"price"`
}

func defaultBalanceRechargePromotion() *BalanceRechargePromotion {
	return &BalanceRechargePromotion{}
}

// parseBalanceRechargePromotion safely parses the JSON value stored in the
// payment settings KV. Invalid values must not disable ordinary recharge
// behavior, so they degrade to a disabled promotion.
func parseBalanceRechargePromotion(raw string) *BalanceRechargePromotion {
	if strings.TrimSpace(raw) == "" {
		return defaultBalanceRechargePromotion()
	}

	var promotion BalanceRechargePromotion
	if err := json.Unmarshal([]byte(raw), &promotion); err != nil {
		return defaultBalanceRechargePromotion()
	}
	if err := validateBalanceRechargePromotion(promotion); err != nil {
		return defaultBalanceRechargePromotion()
	}
	return &promotion
}

// validateBalanceRechargePromotion validates the user-managed promotion
// value. A disabled promotion may use the zero-value window and multiplier so
// old installations and a freshly cleared setting remain valid.
func validateBalanceRechargePromotion(promotion BalanceRechargePromotion) error {
	if math.IsNaN(promotion.Multiplier) || math.IsInf(promotion.Multiplier, 0) || promotion.Multiplier < 0 {
		return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_PROMOTION", "promotion multiplier must be a non-negative finite number")
	}
	if len(promotion.PriceTiers) > 50 {
		return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_PROMOTION", "at most 50 promotion price tiers are allowed")
	}
	for i, tier := range promotion.PriceTiers {
		for _, value := range []float64{tier.CreditedAmount, tier.Price} {
			if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 || math.Abs(value-math.Round(value*100)/100) > 0.0000001 {
				return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_PROMOTION", "promotion prices and credits require positive amounts with at most two decimal places")
			}
		}
		for _, previous := range promotion.PriceTiers[:i] {
			if math.Abs(tier.CreditedAmount-previous.CreditedAmount) < 0.0000001 || math.Abs(tier.Price-previous.Price) < 0.0000001 {
				return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_PROMOTION", "promotion prices and credited amounts must be unique")
			}
		}
	}
	if promotion.Enabled {
		if promotion.Multiplier <= 0 && len(promotion.PriceTiers) == 0 {
			return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_PROMOTION", "promotion requires a positive multiplier or price tiers")
		}
		if strings.TrimSpace(promotion.StartAt) == "" || strings.TrimSpace(promotion.EndAt) == "" {
			return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_PROMOTION", "promotion start_at and end_at are required")
		}
	}

	var start, end time.Time
	var err error
	if strings.TrimSpace(promotion.StartAt) != "" {
		start, err = time.Parse(time.RFC3339, strings.TrimSpace(promotion.StartAt))
		if err != nil {
			return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_PROMOTION", "promotion start_at must be RFC3339")
		}
	}
	if strings.TrimSpace(promotion.EndAt) != "" {
		end, err = time.Parse(time.RFC3339, strings.TrimSpace(promotion.EndAt))
		if err != nil {
			return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_PROMOTION", "promotion end_at must be RFC3339")
		}
	}
	if (promotion.StartAt == "") != (promotion.EndAt == "") {
		return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_PROMOTION", "promotion start_at and end_at must be provided together")
	}
	if !start.IsZero() && !end.IsZero() && !start.Before(end) {
		return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_PROMOTION", "promotion start_at must be before end_at")
	}

	seen := make(map[int64]struct{}, len(promotion.BlacklistUserIDs))
	for _, userID := range promotion.BlacklistUserIDs {
		if userID <= 0 {
			return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_PROMOTION", "promotion blacklist user IDs must be positive")
		}
		if _, ok := seen[userID]; ok {
			return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_PROMOTION", "promotion blacklist user IDs must be unique")
		}
		seen[userID] = struct{}{}
	}
	return nil
}

func validateBalanceRechargePromotionAgainstTiers(promotion *BalanceRechargePromotion, tiers []BalanceRechargeTier) error {
	if promotion == nil || !promotion.Enabled || len(promotion.PriceTiers) == 0 {
		return nil
	}
	if err := validateBalanceRechargePromotion(*promotion); err != nil {
		return err
	}
	if err := validateBalanceRechargeTiers(tiers); err != nil {
		return err
	}
	for _, priceTier := range promotion.PriceTiers {
		matches := 0
		for _, tier := range tiers {
			credit := calculateCreditedBalance(tier.Amount, tier.Multiplier)
			if math.Abs(credit-priceTier.CreditedAmount) < 0.0000001 {
				matches++
			} else if math.Abs(tier.Amount-priceTier.Price) < 0.0000001 {
				return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_PROMOTION", "promotion price conflicts with another recharge tier")
			}
		}
		if matches != 1 {
			return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_PROMOTION", "promotion credit must match exactly one configured recharge tier")
		}
	}
	return nil
}

func findBalanceRechargePromotionPriceTier(amount float64, tiers []BalanceRechargeTier, promotion *BalanceRechargePromotion, userID int64, now time.Time) *BalanceRechargePromotionPriceTier {
	if !promotion.AppliesTo(userID, now) || validateBalanceRechargePromotionAgainstTiers(promotion, tiers) != nil {
		return nil
	}
	for i := range promotion.PriceTiers {
		if math.Abs(amount-promotion.PriceTiers[i].Price) < 0.0000001 {
			return &promotion.PriceTiers[i]
		}
	}
	return nil
}

// IsActiveAt reports whether now is within the promotion's RFC3339 window.
// The comparison deliberately uses [start_at, end_at) semantics.
func (promotion *BalanceRechargePromotion) IsActiveAt(now time.Time) bool {
	if promotion == nil || !promotion.Enabled || validateBalanceRechargePromotion(*promotion) != nil {
		return false
	}
	start, err := time.Parse(time.RFC3339, strings.TrimSpace(promotion.StartAt))
	if err != nil {
		return false
	}
	end, err := time.Parse(time.RFC3339, strings.TrimSpace(promotion.EndAt))
	if err != nil {
		return false
	}
	return !now.Before(start) && now.Before(end)
}

// IsBlacklisted reports whether the promotion excludes userID.
func (promotion *BalanceRechargePromotion) IsBlacklisted(userID int64) bool {
	if promotion == nil || userID <= 0 {
		return false
	}
	for _, blacklistedID := range promotion.BlacklistUserIDs {
		if blacklistedID == userID {
			return true
		}
	}
	return false
}

// AppliesTo reports whether userID can receive the promotion at now.
func (promotion *BalanceRechargePromotion) AppliesTo(userID int64, now time.Time) bool {
	return userID > 0 && promotion.IsActiveAt(now) && !promotion.IsBlacklisted(userID)
}

// resolveBalanceRechargeMultiplier returns the activity multiplier for an
// eligible user, replacing the normal tier/global multiplier. All other users
// use the existing tier/global selection unchanged.
func resolveBalanceRechargeMultiplier(amount float64, tiers []BalanceRechargeTier, fallback float64, promotion *BalanceRechargePromotion, userID int64, now time.Time) float64 {
	normal := selectBalanceRechargeMultiplier(amount, tiers, fallback)
	if tier := findBalanceRechargePromotionPriceTier(amount, tiers, promotion, userID, now); tier != nil {
		return tier.CreditedAmount / amount
	}
	if promotion != nil && promotion.Multiplier > 0 && promotion.AppliesTo(userID, now) && validateBalanceRechargePromotionAgainstTiers(promotion, tiers) == nil {
		return normalizeBalanceRechargeMultiplier(promotion.Multiplier)
	}
	return normal
}

// ResolveBalanceRechargeMultiplier returns the effective multiplier for a user.
func ResolveBalanceRechargeMultiplier(amount float64, tiers []BalanceRechargeTier, fallback float64, promotion *BalanceRechargePromotion, userID int64, now time.Time) float64 {
	return resolveBalanceRechargeMultiplier(amount, tiers, fallback, promotion, userID, now)
}
