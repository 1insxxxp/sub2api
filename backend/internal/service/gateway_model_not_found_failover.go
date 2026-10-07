package service

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// A missing model cannot recover by retrying the same account. Leave the client
// response untouched so the handler can select another eligible account.
func (s *GatewayService) handleUpstreamModelNotFound(ctx context.Context, c *gin.Context, account *Account, resp *http.Response, requestedModel string, event OpsUpstreamErrorEvent) *UpstreamFailoverError {
	if resp == nil || resp.StatusCode != http.StatusNotFound || resp.Body == nil {
		return nil
	}
	body, err := s.readUpstreamErrorBody(resp)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil || !isUpstreamModelNotFoundError(resp.StatusCode, body) {
		return nil
	}

	// Reuse the model-scoped cooldown; other models on this account stay available.
	if s.rateLimitService != nil {
		s.rateLimitService.HandleUpstreamModelNotFound(ctx, account, requestedModel, resp.StatusCode, body)
	}
	event.ProxyID, event.ProxyName = opsUpstreamProxyAttribution(account)
	event.Platform = account.Platform
	event.AccountID = account.ID
	event.AccountName = account.Name
	event.UpstreamStatusCode = resp.StatusCode
	event.UpstreamRequestID = resp.Header.Get("x-request-id")
	event.Kind = "failover"
	event.Message = sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(body))
	setOpsUpstreamError(c, resp.StatusCode, event.Message, "")
	appendOpsUpstreamError(c, event)
	return &UpstreamFailoverError{
		StatusCode:        resp.StatusCode,
		ResponseBody:      body,
		ResponseHeaders:   resp.Header.Clone(),
		NextAccountAction: NextAccountRetry,
	}
}
