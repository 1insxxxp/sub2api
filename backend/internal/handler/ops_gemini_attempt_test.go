package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func geminiCancelDiagnostic(t *testing.T) *service.OpsUpstreamErrorEvent {
	t.Helper()
	var event service.OpsUpstreamErrorEvent
	require.NoError(t, json.Unmarshal([]byte(`{"platform":"gemini","account_id":7,"upstream_status_code":200,"kind":"client_cancel","stage":"inference","reason":"client_canceled","message":"Client canceled upstream inference","attempt":1,"client_canceled":true,"usage_state":"unreported"}`), &event))
	return &event
}

func TestOpsGeminiAttempt_ClientCancelIsNeutralDiagnostic(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 2)
	gin.SetMode(gin.TestMode)
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	serveClientClosedRequest(t, ops, func(c *gin.Context) {
		c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{geminiCancelDiagnostic(t)})
	})
	require.Equal(t, int64(1), OpsErrorLogQueueLength())
	entry := (<-opsErrorLogQueue).entry
	require.Equal(t, http.StatusOK, entry.StatusCode, "cancel diagnostics must stay outside failed-request counters")
	require.Equal(t, "client", entry.ErrorOwner, "cancel must not count as provider health failure")
	require.Equal(t, "client_cancel", entry.ErrorType)
	require.Equal(t, "P3", entry.Severity, "diagnostics retain the existing Ops priority vocabulary")
	require.NotEqual(t, "upstream", entry.ErrorPhase)
}

func TestOpsGeminiAttempt_CancelPreservesEarlierRealFailure(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 2)
	gin.SetMode(gin.TestMode)
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	serveClientClosedRequest(t, ops, func(c *gin.Context) {
		c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{
			{Platform: "gemini", AccountID: 8, UpstreamStatusCode: 504, Kind: "first_output_timeout", Message: "First output timed out"},
			geminiCancelDiagnostic(t),
		})
	})
	require.Equal(t, int64(1), OpsErrorLogQueueLength(), "one bounded summary per request")
	entry := (<-opsErrorLogQueue).entry
	require.Equal(t, http.StatusOK, entry.StatusCode)
	require.Equal(t, "provider", entry.ErrorOwner, "earlier genuine timeout remains health evidence")
	require.NotNil(t, entry.UpstreamStatusCode)
	require.Equal(t, 504, *entry.UpstreamStatusCode)
	require.NotNil(t, entry.AccountID)
	require.Equal(t, int64(8), *entry.AccountID)
	var events []map[string]any
	require.NotNil(t, entry.UpstreamErrorsJSON)
	require.NoError(t, json.Unmarshal([]byte(*entry.UpstreamErrorsJSON), &events))
	require.Len(t, events, 2)
	require.Equal(t, true, events[1]["client_canceled"])
}

func TestOpsGeminiAttempt_CancelAfterSameAttemptErrorIsNeutralButRetainsProviderEvidence(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 2)
	gin.SetMode(gin.TestMode)
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	serveClientClosedRequest(t, ops, func(c *gin.Context) {
		c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{
			AccountID: 7, UpstreamStatusCode: 504, Kind: "request_error", Message: "upstream timeout", Attempt: 1, ClientCanceled: true,
		}})
	})
	require.EqualValues(t, 1, OpsErrorLogQueueLength())
	entry := (<-opsErrorLogQueue).entry
	require.Equal(t, http.StatusOK, entry.StatusCode)
	require.Equal(t, "provider", entry.ErrorOwner)
	require.NotNil(t, entry.UpstreamStatusCode)
	require.Equal(t, 504, *entry.UpstreamStatusCode)
}
