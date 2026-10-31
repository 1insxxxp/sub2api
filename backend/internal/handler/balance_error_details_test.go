package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	pkgerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func detailedInflightErrorForTest() *pkgerrors.ApplicationError {
	err := pkgerrors.Clone(service.ErrBalanceInFlight)
	err.Message = "Existing requests are processing or settling. Current balance: US$0.50; reserved by other requests: US$0.40; estimated reservation for this request: US$0.25. Please retry shortly."
	return err
}

func TestBalanceErrorDetails_KeepWireStatusCodeAndRetry(t *testing.T) {
	amount := 0.125
	for _, tc := range []struct {
		name   string
		err    *pkgerrors.ApplicationError
		status int
		code   string
		retry  int
	}{
		{"unpriced wallet", service.NewInsufficientBalanceError(-0.005, nil), http.StatusForbidden, "billing_error", 0},
		{"known reservation", service.NewInsufficientBalanceError(0.02, &amount), http.StatusForbidden, "billing_error", 0},
		{"inflight reservation", detailedInflightErrorForTest(), http.StatusTooManyRequests, "balance_in_flight", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, code, message, retry := billingErrorDetails(fmt.Errorf("billing admission: %w", tc.err))
			require.Equal(t, tc.status, status)
			require.Equal(t, tc.code, code)
			require.Equal(t, tc.retry, retry)
			require.Equal(t, tc.err.Message, message)
		})
	}
}

func TestBalanceErrorDetails_ProtocolWritersKeepAmountMessage(t *testing.T) {
	amount := 0.125
	err := service.NewInsufficientBalanceError(0.02, &amount)
	status, code, message, _ := billingErrorDetails(err)
	for _, protocol := range []string{"chat", "messages", "gemini"} {
		for _, streaming := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", protocol, streaming), func(t *testing.T) {
				c, recorder := securityAuditErrorTestContext(t)
				if streaming {
					c.Header("Content-Type", "text/event-stream")
					c.Writer.WriteHeader(http.StatusOK)
					c.Writer.Flush()
				}
				writeModelFirstOutputProtocolError(c, protocol, status, code, "", message)
				body := recorder.Body.Bytes()
				if streaming {
					require.Equal(t, http.StatusOK, recorder.Code)
					found := false
					for _, line := range strings.Split(string(body), "\n") {
						if strings.HasPrefix(line, "data: {") {
							body = []byte(strings.TrimPrefix(line, "data: "))
							found = true
							break
						}
					}
					require.True(t, found, "stream must contain a complete JSON error event")
				} else {
					require.Equal(t, http.StatusForbidden, recorder.Code)
				}
				var payload map[string]any
				require.NoError(t, json.Unmarshal(body, &payload))
				errorObject := requireObject(t, payload["error"])
				require.Equal(t, message, errorObject["message"])
				if protocol == "gemini" {
					require.Equal(t, float64(http.StatusForbidden), errorObject["code"])
					require.Equal(t, "PERMISSION_DENIED", errorObject["status"])
				} else {
					require.Equal(t, "billing_error", errorObject["type"])
				}
			})
		}
	}
}

func TestBalanceErrorDetails_OpsClassificationAndIgnoreRemainCompatible(t *testing.T) {
	amount := 0.125
	wallet := service.NewInsufficientBalanceError(0.02, &amount)
	inflight := detailedInflightErrorForTest()
	c, _ := securityAuditErrorTestContext(t)
	phase, limited, owner, source := classifyOpsErrorLog(c, "billing_error", wallet.Message, "", http.StatusForbidden)
	require.Equal(t, "request", phase)
	require.True(t, limited)
	require.Equal(t, "client", owner)
	require.Equal(t, "client_request", source)

	// Adding amounts must not silently turn the existing 429 into a wallet
	// error, changing its monitoring attribution or the balance-only filter.
	oldPhase, oldLimited, oldOwner, oldSource := classifyOpsErrorLog(c, "balance_in_flight", service.ErrBalanceInFlight.Message, "", http.StatusTooManyRequests)
	phase, limited, owner, source = classifyOpsErrorLog(c, "balance_in_flight", inflight.Message, "", http.StatusTooManyRequests)
	require.Equal(t, oldPhase, phase)
	require.Equal(t, oldLimited, limited)
	require.Equal(t, oldOwner, owner)
	require.Equal(t, oldSource, source)

	for _, ignore := range []bool{false, true} {
		settings := &opsAdvancedSettingsRepoStub{advanced: fmt.Sprintf(`{"ignore_insufficient_balance_errors":%t}`, ignore)}
		ops := service.NewOpsService(nil, settings, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		require.Equal(t, ignore, ops.OpsAdvancedSettingsSnapshot().IgnoreInsufficientBalanceErrors)
		for _, asBody := range []bool{false, true} {
			message, body := wallet.Message, ""
			if asBody {
				message = ""
				encoded, err := json.Marshal(map[string]any{"error": map[string]string{"message": wallet.Message}})
				require.NoError(t, err)
				body = string(encoded)
			}
			require.Equal(t, ignore, shouldSkipOpsErrorLog(context.Background(), ops, message, body, "/v1/messages"))
		}
		require.False(t, shouldSkipOpsErrorLog(context.Background(), ops, inflight.Message, "", "/v1/messages"))
		require.False(t, shouldSkipOpsErrorLog(context.Background(), ops, service.ErrBillingServiceUnavailable.Message, "", "/v1/messages"))
	}
}
