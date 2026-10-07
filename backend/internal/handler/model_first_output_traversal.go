package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/googleapi"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Budget expiration is an application timeout, not a disconnected client.
// Keep it separate from the common context.Canceled -> 499 handling, including
// when account admission or upstream authentication consumed the last seconds.
func handleModelFirstOutputTraversalTimeout(c *gin.Context, protocol string) bool {
	if c == nil || c.Request == nil || !service.ModelFirstOutputTraversalBudgetExceeded(c.Request.Context()) {
		return false
	}
	errType := "server_error"
	if protocol == "messages" {
		errType = "api_error"
	}
	writeModelFirstOutputProtocolError(c, protocol, http.StatusGatewayTimeout, errType, "first_output_budget_exhausted", service.ModelFirstOutputTraversalTimeoutMessage)
	return true
}

// Keep heartbeat-only failures protocol-compatible after HTTP 200 was flushed.
func writeModelFirstOutputProtocolError(c *gin.Context, protocol string, status int, errType, code, message string) {
	committed := service.StopModelFirstOutputKeepaliveCommitted(c)
	errorBody := gin.H{"type": errType, "message": message}
	if code != "" {
		errorBody["code"] = code
	}
	var payload any
	switch protocol {
	case "gemini":
		googleStatus := googleapi.HTTPStatusToGoogleStatus(status)
		if status == http.StatusGatewayTimeout {
			googleStatus = "DEADLINE_EXCEEDED"
		}
		payload = gin.H{"error": gin.H{"code": status, "message": message, "status": googleStatus}}
	case "messages":
		payload = gin.H{"type": "error", "error": errorBody}
	default:
		payload = gin.H{"error": errorBody}
	}
	if committed || (c.Writer.Written() && strings.Contains(c.Writer.Header().Get("Content-Type"), "text/event-stream")) {
		service.MarkOpsStreamError(c, errType, message, status)
		body, _ := json.Marshal(payload)
		if protocol == "messages" {
			_, _ = fmt.Fprint(c.Writer, "event: error\n")
		}
		_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", body)
		if protocol == "chat" {
			_, _ = fmt.Fprint(c.Writer, "data: [DONE]\n\n")
		}
		c.Writer.Flush()
	} else {
		c.JSON(status, payload)
	}
}
