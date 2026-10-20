package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const anthropicIncompleteStreamMessage = "stream usage incomplete: missing terminal event"

// Track actual downstream bytes, not just recognized output: metadata,
// signatures, unknown events and partial passthrough lines also prevent replay.
// Each attempt owns its baseline, so earlier attempts' pings do not count as
// this attempt's protocol content. Flush alone changes Gin's size from -1 to 0.
type anthropicStreamReplayState struct {
	writtenBefore  int
	heartbeatBytes int
}

func newAnthropicStreamReplayState(c *gin.Context) *anthropicStreamReplayState {
	return &anthropicStreamReplayState{writtenBefore: max(c.Writer.Size(), 0)}
}

func (s *anthropicStreamReplayState) incompleteEOF(ctx context.Context, c *gin.Context, resp *http.Response, account *Account, disconnected bool, outcome ResponseOutcome) *UpstreamFailoverError {
	if disconnected || outcome.HasEffectiveOutput() || ctx.Err() != nil || c.Request.Context().Err() != nil {
		return nil
	}
	if max(c.Writer.Size(), 0) != s.writtenBefore+s.heartbeatBytes {
		return nil
	}
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		ProxyID: opsUpstreamProxyID(account), ProxyName: opsUpstreamProxyName(account),
		Platform: account.Platform, AccountID: account.ID, AccountName: account.Name,
		UpstreamStatusCode: http.StatusBadGateway, UpstreamRequestID: resp.Header.Get("x-request-id"),
		Kind: "failover_stream_incomplete", Message: anthropicIncompleteStreamMessage,
	})
	return &UpstreamFailoverError{
		StatusCode:      http.StatusBadGateway,
		ResponseBody:    []byte(`{"type":"error","error":{"type":"upstream_disconnected","message":"Upstream stream ended before a terminal event"}}`),
		ResponseHeaders: resp.Header.Clone(), SafeToFailoverAfterWrite: true,
		NextAccountAction: NextAccountRetry, Reason: GatewayFailureReason("stream_incomplete"),
	}
}

// Only complete blocks from the normal SSE parser may use this exemption.
// Passthrough writes lines immediately, so even a partial ping stays unsafe.
// Unknown fields and malformed JSON are deliberately treated as content.
func anthropicSSEBlockIsHeartbeat(block string) bool {
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "", strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "event:"):
			if strings.TrimSpace(strings.TrimPrefix(line, "event:")) != "ping" {
				return false
			}
		case strings.HasPrefix(line, "data:"):
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if gjson.Get(data, "type").String() != "ping" {
				return false
			}
			var ping map[string]any
			if json.Unmarshal([]byte(data), &ping) != nil || len(ping) != 1 || ping["type"] != "ping" {
				return false
			}
		default:
			return false
		}
	}
	return true
}
