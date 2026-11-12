package service

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	pkgerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// NewInsufficientBalanceError adds this user's verified wallet details without
// changing the shared sentinel. A nil reservation means pricing has not run yet.
func NewInsufficientBalanceError(balance float64, requestedReservation *float64) *pkgerrors.ApplicationError {
	err := pkgerrors.Clone(ErrInsufficientBalance)
	if requestedReservation == nil {
		err.Message = fmt.Sprintf("余额不足，当前余额 %s，请充值后重试。", formatBalanceErrorUSD(balance))
		return err
	}
	err.Message = fmt.Sprintf("余额不足，当前余额 %s，本次预扣额度 %s，请充值后重试。",
		formatBalanceErrorUSD(balance), formatBalanceErrorUSD(*requestedReservation))
	return err
}

func newBalanceInFlightError(balance, inflight, requestedReservation float64) *pkgerrors.ApplicationError {
	err := pkgerrors.Clone(ErrBalanceInFlight)
	err.Message = fmt.Sprintf("可用额度不足，当前余额 %s，其他请求已预扣 %s，本次预扣额度 %s。请等待其他请求结算后重试，或充值后重试。",
		formatBalanceErrorUSD(balance), formatBalanceErrorUSD(inflight), formatBalanceErrorUSD(requestedReservation))
	return err
}

func formatBalanceErrorUSD(amount float64) string {
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return "未知"
	}
	if amount != 0 && math.Abs(amount) < 1e-8 {
		// An estimate may have more precision than the wallet's eight decimals.
		// Never make a small positive reservation look free.
		return "US$" + strconv.FormatFloat(amount, 'g', -1, 64)
	}
	if amount == 0 {
		amount = 0 // Normalize negative zero.
	}
	value := strings.TrimRight(strconv.FormatFloat(amount, 'f', 8, 64), "0")
	decimals := len(value) - strings.IndexByte(value, '.') - 1
	if decimals < 2 {
		value += strings.Repeat("0", 2-decimals)
	}
	return "US$" + value
}
