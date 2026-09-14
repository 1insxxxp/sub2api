package service

import (
	"encoding/json"
	"math"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const defaultBalanceRechargeMultiplier = 1.0

// defaultBalanceRechargeAmounts keeps legacy installations restricted to the
// same preset amounts shown by the user payment page when no custom tiers have
// been saved yet.
var defaultBalanceRechargeAmounts = []float64{10, 20, 50, 100, 200, 500, 1000, 2000, 5000}

func isPresetBalanceRechargeAmount(amount float64, tiers []BalanceRechargeTier) bool {
	if len(tiers) > 0 {
		for _, tier := range tiers {
			if math.Abs(amount-tier.Amount) < 0.0000001 {
				return true
			}
		}
		return false
	}
	for _, preset := range defaultBalanceRechargeAmounts {
		if math.Abs(amount-preset) < 0.0000001 {
			return true
		}
	}
	return false
}

type BalanceRechargeTier struct {
	Amount     float64 `json:"amount"`
	Multiplier float64 `json:"multiplier"`
}

func selectBalanceRechargeMultiplier(amount float64, tiers []BalanceRechargeTier, fallback float64) float64 {
	if validateBalanceRechargeTiers(tiers) != nil {
		return normalizeBalanceRechargeMultiplier(fallback)
	}
	for _, tier := range tiers {
		if math.Abs(amount-tier.Amount) < 0.0000001 {
			return normalizeBalanceRechargeMultiplier(tier.Multiplier)
		}
	}
	return normalizeBalanceRechargeMultiplier(fallback)
}

func validateBalanceRechargeTiers(tiers []BalanceRechargeTier) error {
	if len(tiers) > 50 {
		return infraerrors.BadRequest("INVALID_RECHARGE_TIERS", "at most 50 recharge tiers are allowed")
	}
	for index, tier := range tiers {
		if math.IsNaN(tier.Amount) || math.IsInf(tier.Amount, 0) || math.IsNaN(tier.Multiplier) || math.IsInf(tier.Multiplier, 0) || tier.Amount <= 0 || tier.Multiplier <= 0 {
			return infraerrors.BadRequest("INVALID_RECHARGE_TIERS", "recharge presets require a positive amount and multiplier")
		}
		if index > 0 && tier.Amount <= tiers[index-1].Amount {
			return infraerrors.BadRequest("INVALID_RECHARGE_TIERS", "recharge presets must be ordered with unique amounts")
		}
	}
	return nil
}

func parseBalanceRechargeTiers(raw string) []BalanceRechargeTier {
	if raw == "" {
		return nil
	}
	var items []struct {
		Amount     float64 `json:"amount"`
		MinAmount  float64 `json:"min_amount"`
		Multiplier float64 `json:"multiplier"`
	}
	if json.Unmarshal([]byte(raw), &items) != nil {
		return nil
	}
	tiers := make([]BalanceRechargeTier, 0, len(items))
	for _, item := range items {
		amount := item.Amount
		if amount == 0 {
			amount = item.MinAmount
		}
		tiers = append(tiers, BalanceRechargeTier{Amount: amount, Multiplier: item.Multiplier})
	}
	if validateBalanceRechargeTiers(tiers) != nil {
		return nil
	}
	return tiers
}

func normalizeBalanceRechargeMultiplier(multiplier float64) float64 {
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier <= 0 {
		return defaultBalanceRechargeMultiplier
	}
	return multiplier
}

// normalizeSubscriptionUSDToCNYRate 将非法值归一为 0（换算关闭）。
// 与余额倍率不同，0 是合法状态：表示订阅保持 price 直付的存量行为。
func normalizeSubscriptionUSDToCNYRate(rate float64) float64 {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
		return 0
	}
	return rate
}

func calculateCreditedBalance(paymentAmount, multiplier float64) float64 {
	return decimal.NewFromFloat(paymentAmount).
		Mul(decimal.NewFromFloat(normalizeBalanceRechargeMultiplier(multiplier))).
		Round(2).
		InexactFloat64()
}

func calculateGatewayRefundAmount(orderAmount, payAmount, refundAmount float64, currency string) float64 {
	if orderAmount <= 0 || payAmount <= 0 || refundAmount <= 0 {
		return 0
	}
	fractionDigits := int32(payment.CurrencyMaxFractionDigits(currency))
	if math.Abs(refundAmount-orderAmount) <= paymentAmountToleranceForCurrency(currency) {
		return decimal.NewFromFloat(payAmount).Round(fractionDigits).InexactFloat64()
	}
	return decimal.NewFromFloat(payAmount).
		Mul(decimal.NewFromFloat(refundAmount)).
		Div(decimal.NewFromFloat(orderAmount)).
		Round(fractionDigits).
		InexactFloat64()
}
