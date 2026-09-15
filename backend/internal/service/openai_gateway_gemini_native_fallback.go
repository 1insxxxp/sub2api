package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func shouldFallbackOpenAIAccountToGeminiNative(account *Account, upstreamModel string, statusCode int, respBody []byte, responsesShape bool) bool {
	if responsesShape || account == nil || account.Type != AccountTypeAPIKey || account.Platform != PlatformOpenAI {
		return false
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(upstreamModel)), "gemini-") {
		return false
	}
	if statusCode != http.StatusBadRequest {
		return false
	}
	return strings.Contains(strings.ToLower(extractUpstreamErrorMessage(respBody)), "contents is required")
}

func (s *OpenAIGatewayService) forwardOpenAIAccountGeminiNativeFallback(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	upstreamModel string,
	stream bool,
) (*OpenAIForwardResult, error) {
	if s.geminiCompatForwarder == nil {
		return nil, errors.New("gemini native fallback is not configured")
	}

	logger.L().Info("openai chat_completions: falling back to gemini native",
		zap.Int64("account_id", account.ID),
		zap.String("upstream_model", upstreamModel),
		zap.Bool("stream", stream),
	)

	result, err := s.geminiCompatForwarder.ForwardAsChatCompletions(ctx, c, account, body)
	if err != nil {
		return nil, err
	}

	action := "generateContent"
	if stream {
		action = "streamGenerateContent"
	}
	return openAIForwardResultFromGeminiCompat(result, fmt.Sprintf("/v1beta/models/%s:%s", upstreamModel, action)), nil
}

func openAIForwardResultFromGeminiCompat(result *ForwardResult, upstreamEndpoint string) *OpenAIForwardResult {
	if result == nil {
		return nil
	}
	usage := claudeUsageToOpenAIUsage(&result.Usage)
	usage.ImageOutputTokens = result.Usage.ImageOutputTokens
	return &OpenAIForwardResult{
		RequestID:                     result.RequestID,
		UpstreamHeaders:               result.UpstreamHeaders,
		Usage:                         usage,
		Model:                         result.Model,
		DeliveredOutputTokens:         result.DeliveredOutputTokens,
		BillingModel:                  result.UpstreamModel,
		UpstreamModel:                 result.UpstreamModel,
		UpstreamResponseModel:         result.UpstreamResponseModel,
		UpstreamResponseModelConflict: result.UpstreamResponseModelConflict,
		UpstreamEndpoint:              upstreamEndpoint,
		ServiceTier:                   result.ServiceTier,
		ReasoningEffort:               result.ReasoningEffort,
		RequestedReasoningEffort:      result.RequestedReasoningEffort,
		Stream:                        result.Stream,
		Duration:                      result.Duration,
		FirstTokenMs:                  result.FirstTokenMs,
		ClientDisconnect:              result.ClientDisconnect,
		ImageCount:                    result.ImageCount,
		ImageSize:                     result.ImageSize,
		ImageInputSize:                result.ImageInputSize,
		ImageOutputSize:               result.ImageOutputSize,
		ImageOutputSizes:              append([]string(nil), result.ImageOutputSizes...),
		ImageSizeSource:               result.ImageSizeSource,
		ImageSizeBreakdown:            result.ImageSizeBreakdown,
		Outcome:                       result.Outcome,
	}
}
