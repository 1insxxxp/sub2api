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

func TestOpsCanceledRequestPreservesResponseStatusWithoutChangingSLACode(t *testing.T) {
	for _, status := range []int{http.StatusOK, statusClientClosedRequest} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			setupOpsErrorLogTestQueue(t, 2)
			gin.SetMode(gin.TestMode)
			ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			c.Writer.WriteHeader(status)
			c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{
				{Platform: "gemini", AccountID: 8, UpstreamStatusCode: 503, Kind: "failover", Stage: "inference", Attempt: 1},
				geminiCancelDiagnostic(t),
			})
			logOpsRecoveredUpstream(c, ops, http.StatusOK)
			require.EqualValues(t, 1, OpsErrorLogQueueLength())
			entry := (<-opsErrorLogQueue).entry
			require.Equal(t, http.StatusOK, entry.StatusCode, "cancellation remains neutral for SLA")
			require.Equal(t, 503, *entry.UpstreamStatusCode, "previous provider failure is retained")
			var events []map[string]any
			require.NoError(t, json.Unmarshal([]byte(*entry.UpstreamErrorsJSON), &events))
			require.NotContains(t, events[0], "downstream_status_code")
			require.EqualValues(t, status, events[1]["downstream_status_code"], "capture the response writer status, never the neutral SLA override")
		})
	}
}
