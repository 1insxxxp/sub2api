package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// forwardAnthropicViaRawChatCompletions serves /v1/messages clients through
// an OpenAI-compatible upstream that only supports /v1/chat/completions.
//
// Conversion chain (direct, no Responses intermediary):
//
//	Request:  Anthropic Messages → Chat Completions (AnthropicToChatCompletionsRequest)
//	Response: CC chunk/response → Anthropic events/response (direct bridge)
//
// This is the /v1/messages counterpart of forwardResponsesViaRawChatCompletions
// (which serves /v1/responses clients). Unlike the Responses path, the direct
// bridge skips the Responses API intermediate representation entirely — every
// streaming token runs through a single state machine instead of two.
func (s *OpenAIGatewayService) forwardAnthropicViaRawChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	defaultMappedModel string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()

	// 1. Parse Anthropic request
	var anthropicReq apicompat.AnthropicRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return nil, fmt.Errorf("parse anthropic request: %w", err)
	}
	originalModel := anthropicReq.Model
	if strings.TrimSpace(originalModel) == "" {
		writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return nil, fmt.Errorf("missing model in request")
	}
	applyOpenAICompatModelNormalization(&anthropicReq)
	clientStream := anthropicReq.Stream

	// 2. Anthropic → Chat Completions (direct, no Responses intermediary)
	chatReq, err := apicompat.AnthropicToChatCompletionsRequest(&anthropicReq)
	if err != nil {
		writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, fmt.Errorf("convert anthropic to chat completions: %w", err)
	}

	billingModel := resolveOpenAIForwardModel(account, anthropicReq.Model, defaultMappedModel)
	upstreamModel := normalizeOpenAIModelForUpstream(account, billingModel)
	if err := validateGPT61SolCompatRequest(body, upstreamModel); err != nil {
		writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, err
	}
	if openai.IsGPT61SolModelSpelling(upstreamModel) && len(anthropicReq.Tools) > 0 {
		err := fmt.Errorf("gpt-6.1-sol requires Responses for tool calls; this account only supports Chat Completions")
		writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, err
	}
	chatReq.Model = upstreamModel
	chatReq.ReasoningEffort = openAICompatAnthropicReasoningEffort(&anthropicReq, upstreamModel, chatReq.ReasoningEffort)
	chatReq.Stream = clientStream
	if clientStream {
		chatReq.StreamOptions = &apicompat.ChatStreamOptions{IncludeUsage: true}
	}

	serviceTier := extractOpenAIServiceTierFromBody(body)
	var antiTruncationPolicy domain.AntiTruncationPolicy
	requestCtx := ctx
	var cancelRequest context.CancelFunc
	if clientStream {
		if key := getAPIKeyFromContext(c); key != nil {
			policyCtx := WithGroupAliasRawChatAntiTruncationPolicy(ctx, key.Group, originalModel)
			antiTruncationPolicy, _ = RawChatAntiTruncationPolicyFromContext(policyCtx)
		}
		if antiTruncationPolicy.Enabled {
			seconds := antiTruncationPolicy.MaxDurationSeconds
			if seconds <= 0 {
				seconds = 180
			}
			requestCtx, cancelRequest = context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
			defer cancelRequest()
		}
	}

	chatBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal chat completions request: %w", err)
	}
	if injectedBody, applied, injectErr := ApplyAccountModelSystemPrompt(chatBody, account, upstreamModel, ModelSystemPromptOpenAIChat); injectErr != nil {
		return nil, fmt.Errorf("inject account model system prompt: %w", injectErr)
	} else if applied {
		chatBody = injectedBody
	}
	if normalizedBody, normalized := NormalizeGLMOpenAIReasoningEffort(chatBody, upstreamModel); normalized {
		chatBody = normalizedBody
	}
	if account.Platform == PlatformOpenAI {
		policyBody, changed, policyErr := ApplyOpenAIReasoningEffortPolicyFromContext(ctx, chatBody)
		if policyErr != nil {
			if IsReasoningEffortPolicyDenied(policyErr) {
				MarkOpsClientBusinessLimited(c, OpsClientBusinessLimitedReasonLocalPolicyDenied)
				writeAnthropicError(c, http.StatusForbidden, "forbidden_error", policyErr.Error())
			}
			return nil, policyErr
		}
		if changed {
			chatBody = policyBody
		}
	}
	// Provider normalization and policy caps can both change the converted effort.
	// Use the final outbound value for usage logs and billing.
	reasoningEffort := extractOpenAIReasoningEffortFromBody(chatBody, upstreamModel, billingModel, originalModel)
	reasoningEffort = ApplyThinkingEnabledFallback(reasoningEffort, chatBody, upstreamModel)
	// Unlike forwardResponsesViaRawChatCompletions, applyOpenAIFastPolicyToBody
	// is intentionally skipped: Anthropic Messages bodies carry no service_tier,
	// so the converted Chat Completions body never contains one and the policy
	// would always be a no-op on this path.

	logger.L().Debug("openai messages: forwarding via raw chat completions",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("billing_model", billingModel),
		zap.String("upstream_model", upstreamModel),
		zap.Bool("stream", clientStream),
	)

	// 3. Build and send upstream request via the shared CC pipeline
	apiKey, targetURL, err := s.resolveCCFallbackTarget(account)
	if err != nil {
		return nil, err
	}
	resp, err := s.sendCCUpstreamRequest(requestCtx, c, account, targetURL, chatBody, clientStream, apiKey, account.GetOpenAIUserAgent(), "")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// 4. Handle error responses
	if resp.StatusCode >= 400 {
		respBody, upstreamMsg := s.readOpenAIUpstreamError(resp)
		if foErr := s.failoverOpenAIUpstreamHTTPError(ctx, c, account, resp, respBody, upstreamMsg, upstreamModel); foErr != nil {
			return nil, foErr
		}
		// Non-failover error: return Anthropic-formatted error to client via the
		// shared compat handler (passthrough rules, ops recording, cyber_policy).
		return s.handleAnthropicErrorResponse(resp, c, account, billingModel)
	}

	// 5. Convert response
	if clientStream {
		var continueRequest func(string, int) (*http.Response, error)
		if antiTruncationPolicy.Enabled && rawChatContinuationAllowed(chatBody) {
			continueRequest = func(assistantText string, remaining int) (*http.Response, error) {
				continuedBody, buildErr := buildRawChatContinuationBody(chatBody, assistantText, remaining)
				if buildErr != nil {
					return nil, fmt.Errorf("build continuation request: %w", buildErr)
				}
				return s.sendCCUpstreamRequest(requestCtx, c, account, targetURL, continuedBody, true, apiKey, account.GetOpenAIUserAgent(), "")
			}
		}
		return s.streamChatCompletionsAsAnthropic(c, resp, originalModel, billingModel, upstreamModel, reasoningEffort, serviceTier, startTime, antiTruncationPolicy, continueRequest)
	}
	return s.bufferChatCompletionsAsAnthropic(c, resp, originalModel, billingModel, upstreamModel, reasoningEffort, serviceTier, startTime)
}

func (s *OpenAIGatewayService) bufferChatCompletionsAsAnthropic(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	billingModel string,
	upstreamModel string,
	reasoningEffort *string,
	serviceTier *string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	ccResp, usage, err := s.readCCUpstreamJSONResponse(c, resp, writeAnthropicError)
	if err != nil {
		return nil, err
	}
	anthropicResp := apicompat.ChatCompletionsResponseToAnthropic(ccResp, originalModel)

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.JSON(http.StatusOK, anthropicResp)

	return &OpenAIForwardResult{
		RequestID:                   requestID,
		UpstreamHeaders:             resp.Header,
		Usage:                       usage,
		Model:                       originalModel,
		BillingModel:                billingModel,
		UpstreamModel:               upstreamModel,
		ReasoningEffort:             reasoningEffort,
		UpstreamResponseServiceTier: observedUpstreamResponseServiceTier(c),
		ServiceTier:                 resolvedOpenAIUpstreamServiceTier(c, serviceTier),
		Stream:                      false,
		Duration:                    time.Since(startTime),
	}, nil
}

func (s *OpenAIGatewayService) streamChatCompletionsAsAnthropic(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	billingModel string,
	upstreamModel string,
	reasoningEffort *string,
	serviceTier *string,
	startTime time.Time,
	antitruncationPolicy domain.AntiTruncationPolicy,
	continueRequest func(string, int) (*http.Response, error),
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	initialHeaders := resp.Header
	writeStreamHeaders := s.newStreamHeaderWriter(c, initialHeaders)

	anthropicState := apicompat.NewChatCompletionsToAnthropicStreamState(originalModel)
	clientDisconnected := false

	// 与 responses 兄弟不同：客户端断开后仍继续做事件转换（喂 anthropicState），
	// 仅跳过写出，保证 finalize 阶段的 usage 汇总不受断开影响。
	emitChunk := func(chunk *apicompat.ChatCompletionsChunk) {
		// CC chunk → Anthropic events (direct, single state machine)
		anthropicEvents := apicompat.ChatCompletionsChunkToAnthropicEvents(chunk, anthropicState)
		if clientDisconnected {
			return
		}
		for _, aEvt := range anthropicEvents {
			sse, err := apicompat.ResponsesAnthropicEventToSSE(aEvt)
			if err != nil {
				continue
			}
			writeStreamHeaders()
			if _, err := fmt.Fprint(c.Writer, sse); err != nil {
				clientDisconnected = true
				break
			}
		}
		if !clientDisconnected && len(anthropicEvents) > 0 {
			c.Writer.Flush()
		}
	}

	var usage OpenAIUsage
	var firstTokenMs *int
	var scanErr error
	continuationRounds, outputBudget := 0, 0
	var fullAssistantText strings.Builder
	maxRounds := antitruncationPolicy.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 1
	}
	maxDuration := time.Duration(antitruncationPolicy.MaxDurationSeconds) * time.Second
	if maxDuration <= 0 {
		maxDuration = 180 * time.Second
	}
	maxOutputTokens := antitruncationPolicy.MaxOutputTokens
	resultWithUsage := func() *OpenAIForwardResult {
		return &OpenAIForwardResult{RequestID: requestID, UpstreamHeaders: initialHeaders, Usage: usage,
			Model: originalModel, BillingModel: billingModel, UpstreamModel: upstreamModel,
			ReasoningEffort: reasoningEffort, UpstreamResponseServiceTier: observedUpstreamResponseServiceTier(c),
			ServiceTier: resolvedOpenAIUpstreamServiceTier(c, serviceTier), Stream: true,
			Duration: time.Since(startTime), FirstTokenMs: firstTokenMs, ClientDisconnect: clientDisconnected,
			AntiTruncationRounds: continuationRounds}
	}
	for {
		var roundUsage OpenAIUsage
		var roundAssistant strings.Builder
		finishReason, toolCallSeen, semanticOutput, sawDone := "", false, false, false
		continuationSafe := continueRequest != nil
		terminalChunks := make([]*apicompat.ChatCompletionsChunk, 0, 3)
		scanner := s.newUpstreamSSEScanner(resp.Body)
		for scanner.Scan() {
			payload, ok := extractOpenAISSEDataLine(scanner.Text())
			if !ok {
				continue
			}
			payload = strings.TrimSpace(payload)
			if payload == "[DONE]" {
				sawDone = true
				break
			}
			if observer := upstreamResponseModelObserverFromContext(c); observer != nil {
				observer.ObserveOpenAI([]byte(payload), "")
			}
			if current := extractCCStreamUsage(payload); current != nil {
				roundUsage = *current
			}
			var chunk apicompat.ChatCompletionsChunk
			if json.Unmarshal([]byte(payload), &chunk) != nil {
				continue
			}
			if len(chunk.Choices) > 1 {
				continuationSafe = false
			}
			terminal := false
			for _, choice := range chunk.Choices {
				if choice.Index != 0 {
					continuationSafe = false
				}
				if choice.FinishReason != nil && *choice.FinishReason != "" {
					finishReason, terminal = *choice.FinishReason, true
				}
				if choice.Delta.Content != nil && *choice.Delta.Content != "" {
					roundAssistant.WriteString(*choice.Delta.Content)
					semanticOutput = true
				}
				if choice.Delta.Refusal != nil && *choice.Delta.Refusal != "" {
					semanticOutput = true
				}
				if choice.Delta.ReasoningContent != nil || choice.Delta.Reasoning != nil {
					continuationSafe = false
				}
				if len(choice.Delta.ToolCalls) > 0 {
					toolCallSeen = true
				}
			}
			if firstTokenMs == nil && !isOpenAIChatUsageOnlyStreamChunk(payload) && chatChunkStartsResponsesOutput(&chunk) {
				ms := int(time.Since(startTime).Milliseconds())
				firstTokenMs = &ms
			}
			if terminal || isOpenAIChatUsageOnlyStreamChunk(payload) {
				terminalChunks = append(terminalChunks, &chunk)
			} else {
				emitChunk(&chunk)
			}
		}
		scanErr = scanner.Err()
		_ = resp.Body.Close()
		addOpenAIUsage(&usage, roundUsage)
		roundTokens := roundUsage.OutputTokens
		if roundTokens <= 0 && roundAssistant.Len() > 0 {
			roundTokens = (len([]rune(roundAssistant.String())) + 3) / 4
		}
		outputBudget += roundTokens
		shouldContinue := continueRequest != nil && continuationSafe && !clientDisconnected && c.Request.Context().Err() == nil && !toolCallSeen && semanticOutput &&
			!errors.Is(scanErr, context.Canceled) && !errors.Is(scanErr, context.DeadlineExceeded) &&
			(finishReason == "length" || (scanErr == nil && !sawDone)) && continuationRounds < maxRounds &&
			time.Since(startTime) < maxDuration && (maxOutputTokens <= 0 || outputBudget < maxOutputTokens)
		if shouldContinue {
			fullAssistantText.WriteString(roundAssistant.String())
			remaining := 0
			if maxOutputTokens > 0 {
				remaining = maxOutputTokens - outputBudget
			}
			if maxOutputTokens <= 0 || remaining > 0 {
				next, err := continueRequest(fullAssistantText.String(), remaining)
				if err != nil {
					return resultWithUsage(), err
				}
				if next == nil || next.StatusCode >= http.StatusBadRequest {
					if next != nil {
						_, message := s.readOpenAIUpstreamError(next)
						_ = next.Body.Close()
						return resultWithUsage(), fmt.Errorf("continuation upstream request failed: %s", message)
					}
					return resultWithUsage(), fmt.Errorf("continuation upstream request returned no response")
				}
				resp = next
				if id := strings.TrimSpace(resp.Header.Get("x-request-id")); id != "" {
					requestID = id
				}
				continuationRounds++
				continue
			}
		}
		for _, chunk := range terminalChunks {
			emitChunk(chunk)
		}
		break
	}
	if scanErr != nil && !errors.Is(scanErr, context.Canceled) && !errors.Is(scanErr, context.DeadlineExceeded) {
		return resultWithUsage(), fmt.Errorf("stream usage incomplete: %w", scanErr)
	}

	// Finalize: close open blocks + emit message_delta/message_stop.
	// Anthropic exposes one stream-level usage value, so report the sum across
	// continuation requests rather than only the last upstream round.
	anthropicState.InputTokens = usage.InputTokens
	anthropicState.OutputTokens = usage.OutputTokens
	anthropicState.CacheReadInputTokens = usage.CacheReadInputTokens
	anthropicState.CacheCreationInputTokens = usage.CacheCreationInputTokens
	finalEvents := apicompat.FinalizeChatCompletionsAnthropicStream(anthropicState)
	if !clientDisconnected {
		for _, aEvt := range finalEvents {
			sse, err := apicompat.ResponsesAnthropicEventToSSE(aEvt)
			if err != nil {
				continue
			}
			writeStreamHeaders()
			if _, err := fmt.Fprint(c.Writer, sse); err != nil {
				clientDisconnected = true
				break
			}
		}
		c.Writer.Flush()
	}

	return resultWithUsage(), nil
}
