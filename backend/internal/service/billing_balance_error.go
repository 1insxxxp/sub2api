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
	reservation := "not calculated"
	if requestedReservation != nil {
		reservation = formatBalanceErrorUSD(*requestedReservation)
	}
	// Keep this prefix for protocol clients and Ops balance-error classification.
	err.Message = fmt.Sprintf("Insufficient account balance. Current balance: %s; estimated reservation for this request: %s.",
		formatBalanceErrorUSD(balance), reservation)
	return err
}

func newBalanceInFlightError(balance, inflight, requestedReservation float64) *pkgerrors.ApplicationError {
	err := pkgerrors.Clone(ErrBalanceInFlight)
	err.Message = fmt.Sprintf("Existing requests are processing or settling. Current balance: %s; reserved by other requests: %s; estimated reservation for this request: %s. Please retry shortly.",
		formatBalanceErrorUSD(balance), formatBalanceErrorUSD(inflight), formatBalanceErrorUSD(requestedReservation))
	return err
}

func formatBalanceErrorUSD(amount float64) string {
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return "unknown"
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
