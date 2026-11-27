package handler

import (
	"errors"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// prepareBillingErrorResponse is used immediately before returning a terminal
// billing error. Only typed wallet errors may carry this user's verified amounts
// through public error filtering. Upstream messages cannot establish this trust.
func prepareBillingErrorResponse(c *gin.Context, err error) (status int, code, message string) {
	status, code, message, retryAfter := billingErrorDetails(err)
	service.StopModelFirstOutputKeepaliveCommitted(c)
	service.StopOpenAICompactSSEKeepaliveCommitted(c)
	if retryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(retryAfter))
	}
	publicCode := ""
	switch {
	case errors.Is(err, service.ErrInsufficientBalance):
		publicCode = "insufficient_balance"
	case errors.Is(err, service.ErrBalanceInFlight):
		publicCode = "balance_in_flight"
	}
	if publicCode != "" {
		if value, ok := c.Get(publicGatewayWriterKey); ok {
			if writer, ok := value.(*publicGatewayWriter); ok {
				writer.mu.Lock()
				writer.localBillingError = &service.PublicGatewayError{Status: status, Code: publicCode, Message: message}
				// Discard any incomplete frame from a previous failover attempt.
				writer.pending = ""
				writer.mu.Unlock()
			}
		}
	}
	return status, code, message
}
