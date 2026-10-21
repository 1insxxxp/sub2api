package service

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Missing models cannot recover on the same account. Keep the response
// uncommitted so the handler can try another account within its normal budget.
func (s *GeminiMessagesCompatService) handleGeminiModelNotFound(ctx context.Context, c *gin.Context, account *Account, resp *http.Response, body []byte, requestedModel string) *UpstreamFailoverError {
	if resp.StatusCode != http.StatusNotFound || geminiCachedContentBoundFromContext(ctx) {
		return nil
	}
	body = unwrapIfNeeded(account.Type == AccountTypeOAuth, body)
	if isGeminiCachedContentNotFound(resp.StatusCode, body) || !isUpstreamModelNotFoundError(resp.StatusCode, body) {
		return nil
	}
	if s.rateLimitService != nil {
		s.rateLimitService.HandleUpstreamModelNotFound(ctx, account, requestedModel, resp.StatusCode, body)
	}
	requestID := resp.Header.Get("x-request-id")
	if requestID == "" {
		requestID = resp.Header.Get("x-goog-request-id")
	}
	message := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(body))
	detail := s.upstreamErrorDetail(body)
	setOpsUpstreamError(c, resp.StatusCode, message, detail)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		ProxyID:            opsUpstreamProxyID(account),
		ProxyName:          opsUpstreamProxyName(account),
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: resp.StatusCode,
		UpstreamRequestID:  requestID,
		Kind:               "failover",
		Message:            message,
		Detail:             detail,
	})
	return &UpstreamFailoverError{
		StatusCode:        resp.StatusCode,
		ResponseBody:      body,
		ResponseHeaders:   resp.Header.Clone(),
		NextAccountAction: NextAccountRetry,
	}
}
