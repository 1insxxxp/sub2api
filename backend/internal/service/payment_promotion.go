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
	Enabled          bool    `json:"enabled"`
	Name             string  `json:"name,omitempty"`
	StartAt          string  `json:"start_at,omitempty"`
	EndAt            string  `json:"end_at,omitempty"`
	Multiplier       float64 `json:"multiplier"`
	BlacklistUserIDs []int64 `json:"blacklist_user_ids,omitempty"`
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
	if promotion.Enabled {
		if math.IsNaN(promotion.Multiplier) || math.IsInf(promotion.Multiplier, 0) || promotion.Multiplier <= 0 {
			return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_PROMOTION", "promotion multiplier must be greater than 0")
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
	if promotion != nil && promotion.AppliesTo(userID, now) {
		return normalizeBalanceRechargeMultiplier(promotion.Multiplier)
	}
	return normal
}
