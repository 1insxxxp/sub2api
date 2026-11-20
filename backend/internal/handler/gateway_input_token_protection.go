package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// The resolver and channel catalog are shared by both HTTP gateway adapters.
func (h *OpenAIGatewayHandler) InputTokenProtectionBillsByTokens(ctx context.Context, key *service.APIKey, model string, attempt service.InputTokenProtectionAttempt, imageEndpoint bool) bool {
	return h != nil && h.gatewayService != nil && h.gatewayService.InputTokenProtectionProjectsUsage(ctx, key, model, attempt, imageEndpoint)
}
