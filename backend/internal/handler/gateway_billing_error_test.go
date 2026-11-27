//go:build unit

package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	pkgerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayBillingError_TrustedWalletErrorOverridesStaleUpstream(t *testing.T) {
	amount := 0.5
	for _, walletErr := range []*pkgerrors.ApplicationError{
		service.NewInsufficientBalanceError(0.01, &amount),
		detailedInflightErrorForTest(),
	} {
		t.Run(walletErr.Reason, func(t *testing.T) {
			engine := gin.New()
			engine.Use(PublicGatewayErrorMiddleware(true), PublicGatewayErrorsAfterIngress())
			engine.POST("/v1/chat/completions", func(c *gin.Context) {
				service.SetOpsUpstreamError(c, 503, "private-upstream.example secret-model", "")
				c.Header("X-Upstream-Account", "private-account")
				c.Header("Location", "https://private-upstream.example")
				status, code, message := prepareBillingErrorResponse(c, fmt.Errorf("admission: %w", walletErr.WithCause(errors.New("private database failure"))))
				(&GatewayHandler{}).chatCompletionsErrorResponse(c, status, code, message)
			})
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
			status, _, _, _ := billingErrorDetails(walletErr)
			require.Equal(t, status, recorder.Code)
			require.Contains(t, recorder.Body.String(), walletErr.Message)
			require.NotContains(t, recorder.Body.String(), "private")
			require.Empty(t, recorder.Result().Header.Get("Location"))
			require.Empty(t, recorder.Result().Header.Get("X-Upstream-Account"))
			if status == http.StatusTooManyRequests {
				require.Equal(t, "2", recorder.Result().Header.Get("Retry-After"))
			}
		})
	}
}

func TestGatewayBillingError_UntrustedMessagesRemainFiltered(t *testing.T) {
	for _, localOtherError := range []bool{false, true} {
		t.Run(fmt.Sprintf("other-local-error=%t", localOtherError), func(t *testing.T) {
			engine := gin.New()
			engine.Use(PublicGatewayErrorMiddleware(true), PublicGatewayErrorsAfterIngress())
			engine.POST("/v1/chat/completions", func(c *gin.Context) {
				if localOtherError {
					status, code, message := prepareBillingErrorResponse(c, errors.New("private-upstream.example balance_in_flight 余额不足"))
					(&GatewayHandler{}).chatCompletionsErrorResponse(c, status, code, message)
					return
				}
				c.JSON(429, gin.H{"error": gin.H{"code": "balance_in_flight", "type": "billing_error", "message": "private-upstream.example 余额不足，当前余额 US$100.00"}})
			})
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
			require.Equal(t, http.StatusBadGateway, recorder.Code)
			require.NotContains(t, recorder.Body.String(), "private")
			require.NotContains(t, recorder.Body.String(), "US$100.00")
			require.NotContains(t, recorder.Body.String(), "balance_in_flight")
		})
	}
}

func TestGatewayBillingError_UntrustedStreamRemainsFiltered(t *testing.T) {
	engine := gin.New()
	engine.Use(PublicGatewayErrorMiddleware(true), PublicGatewayErrorsAfterIngress())
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		_, _ = c.Writer.WriteString(`data: {"error":{"code":"balance_in_flight","message":"private-upstream.example 当前余额 US$100.00"}}` + "\n\n")
		c.Writer.Flush()
	})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "private")
	require.NotContains(t, recorder.Body.String(), "US$100.00")
	require.NotContains(t, recorder.Body.String(), "balance_in_flight")
	require.Contains(t, recorder.Body.String(), "request_failed")
}
