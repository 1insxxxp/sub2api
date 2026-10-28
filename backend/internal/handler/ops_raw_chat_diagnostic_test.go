//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpsRawChatCancellationIsNeutralWithoutCanceledContext(t *testing.T) {
	for _, status := range []int{http.StatusOK, statusClientClosedRequest} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			setupOpsErrorLogTestQueue(t, 2)
			ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			router := gin.New()
			router.Use(OpsErrorLoggerMiddleware(ops))
			router.POST("/v1/chat/completions", func(c *gin.Context) {
				c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{Platform: "openai", AccountID: 7, Stage: "inference", Attempt: 1, Kind: "client_cancel", ClientCanceled: true, UsageState: "unreported", Message: "Client connection closed"}})
				c.Writer.WriteHeader(status)
				if status == http.StatusOK {
					c.Header("Content-Type", "text/event-stream")
					_, _ = c.Writer.WriteString("data: {\"error\":{\"type\":\"upstream_error\",\"message\":\"stream ended\"}}\n\n")
				}
			})
			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
			require.EqualValues(t, 1, OpsErrorLogQueueLength())
			entry := (<-opsErrorLogQueue).entry
			require.Equal(t, http.StatusOK, entry.StatusCode, "confirmed local disconnect is outside upstream SLA")
			require.Equal(t, "client_cancel", entry.ErrorType)
			require.Equal(t, "client", entry.ErrorOwner)
			var events []map[string]any
			require.NoError(t, json.Unmarshal([]byte(*entry.UpstreamErrorsJSON), &events))
			require.EqualValues(t, status, events[0]["downstream_status_code"])
		})
	}
}

func TestOpsRawChatServerFailureDoesNotBlameProvider(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 2)
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{Platform: "openai", AccountID: 7, Stage: "inference", Scope: "gateway", Attempt: 1, Kind: "request_error", UpstreamStatusCode: http.StatusOK, Message: "Gateway interrupted response delivery"}})
	service.MarkOpsStreamErrorValue(c, service.OpsStreamError{ErrType: "api_error", IntendedStatus: http.StatusBadGateway, CountTowardsSLA: true, Message: "Gateway interrupted response delivery"})
	logOpsStreamError(c, ops, http.StatusOK)
	require.EqualValues(t, 1, OpsErrorLogQueueLength())
	entry := (<-opsErrorLogQueue).entry
	require.Equal(t, "internal", entry.ErrorPhase)
	require.Equal(t, "platform", entry.ErrorOwner)
	require.Equal(t, "gateway", entry.ErrorSource)
	require.Equal(t, http.StatusBadGateway, entry.StatusCode)
}

func TestOpsRawChatRecoveredGatewayFailureKeepsPlatformAttribution(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "retry_success"
		if canceled {
			name = "then_client_cancel"
		}
		t.Run(name, func(t *testing.T) {
			setupOpsErrorLogTestQueue(t, 2)
			ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			router := gin.New()
			router.Use(OpsErrorLoggerMiddleware(ops))
			router.POST("/v1/chat/completions", func(c *gin.Context) {
				events := []*service.OpsUpstreamErrorEvent{
					{Platform: "openai", AccountID: 5, Stage: "inference", Attempt: 1, Kind: "failover", UpstreamStatusCode: http.StatusServiceUnavailable, Message: "Earlier genuine provider failure"},
					{Platform: "openai", AccountID: 7, Stage: "inference", Scope: "gateway", Attempt: 2, Kind: "request_error", UpstreamStatusCode: http.StatusOK, Message: "Gateway interrupted response delivery"},
				}
				if canceled {
					events = append(events, &service.OpsUpstreamErrorEvent{Platform: "openai", AccountID: 9, Stage: "inference", Attempt: 3, Kind: "client_cancel", ClientCanceled: true, Message: "Client connection closed"})
				}
				c.Set(service.OpsUpstreamErrorsKey, events)
				if canceled {
					c.Status(statusClientClosedRequest)
				} else {
					c.JSON(http.StatusOK, gin.H{"ok": true})
				}
			})
			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
			require.EqualValues(t, 1, OpsErrorLogQueueLength())
			entry := (<-opsErrorLogQueue).entry
			require.Equal(t, http.StatusOK, entry.StatusCode)
			require.Equal(t, "internal", entry.ErrorPhase)
			require.Equal(t, "platform", entry.ErrorOwner)
			require.Equal(t, "gateway", entry.ErrorSource)
			require.Equal(t, "api_error", entry.ErrorType)
			require.NotContains(t, entry.ErrorMessage, "Recovered upstream error")
			require.NotNil(t, entry.AccountID)
			require.EqualValues(t, 7, *entry.AccountID)
			var events []map[string]any
			require.NoError(t, json.Unmarshal([]byte(*entry.UpstreamErrorsJSON), &events))
			wantEvents := 2
			if canceled {
				wantEvents++
			}
			require.Len(t, events, wantEvents, "keep earlier provider failures and subsequent cancel in the attempt history")
			require.EqualValues(t, http.StatusServiceUnavailable, events[0]["upstream_status_code"])
			require.Equal(t, "Earlier genuine provider failure", events[0]["message"])
			require.Equal(t, "gateway", events[1]["scope"])
			if canceled {
				require.Equal(t, true, events[2]["client_canceled"])
			}
		})
	}
}
