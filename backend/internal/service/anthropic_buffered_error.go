package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

var anthropicEmbeddedStatusPattern = regexp.MustCompile(`(?i)\bstatus[_ ]?code\s*[:=]?\s*([45][0-9]{2})\b`)

// anthropicBufferedErrorStatus recognizes explicit error envelopes only. A
// message containing actual output remains deliverable even if a relay adds
// diagnostic error metadata, so retrying cannot discard or duplicate output.
func anthropicBufferedErrorStatus(body []byte) int {
	if !gjson.ValidBytes(body) {
		return 0
	}
	root := gjson.ParseBytes(body)
	if !root.IsObject() || hasAnthropicBufferedContent(root) {
		return 0
	}
	errNode := root.Get("error")
	hasError := errNode.IsObject() && len(errNode.Map()) > 0 ||
		errNode.Type == gjson.String && strings.TrimSpace(errNode.String()) != ""
	if root.Get("type").String() != "error" && !hasError {
		return 0
	}
	for _, path := range []string{"error.status_code", "error.status", "status_code", "status", "error.code", "code", "error.type"} {
		if status := anthropicEmbeddedHTTPStatus(root.Get(path)); status != 0 {
			return status
		}
	}
	errorType := strings.ToLower(strings.TrimSpace(errNode.Get("type").String()))
	status := anthropicErrorTypeStatus(errorType)
	// Typed validation/authentication errors may quote arbitrary client text.
	// Only generic relay errors may infer a status from their message.
	if status != http.StatusBadGateway && errorType != "api_error" {
		return status
	}
	if match := anthropicEmbeddedStatusPattern.FindStringSubmatch(extractUpstreamErrorMessage(body)); len(match) == 2 {
		status, _ := strconv.Atoi(match[1])
		return status
	}
	return status
}

func anthropicEmbeddedHTTPStatus(value gjson.Result) int {
	if value.Type != gjson.Number && value.Type != gjson.String {
		return 0
	}
	status, err := strconv.Atoi(strings.TrimSpace(value.String()))
	if err != nil || status < 400 || status > 599 {
		return 0
	}
	return status
}

// Standard Anthropic types have stable HTTP meanings. Unknown relay errors
// remain provider failures instead of being reported as successful empty usage.
func anthropicErrorTypeStatus(errorType string) int {
	switch strings.ToLower(strings.TrimSpace(errorType)) {
	case "invalid_request_error":
		return http.StatusBadRequest
	case "authentication_error":
		return http.StatusUnauthorized
	case "permission_error":
		return http.StatusForbidden
	case "not_found_error":
		return http.StatusNotFound
	case "request_too_large":
		return http.StatusRequestEntityTooLarge
	case "rate_limit_error":
		return http.StatusTooManyRequests
	case "overloaded_error":
		return 529
	case "api_error":
		return http.StatusInternalServerError
	default:
		return http.StatusBadGateway
	}
}

func hasAnthropicBufferedContent(root gjson.Result) bool {
	for _, block := range root.Get("content").Array() {
		switch block.Get("type").String() {
		case "text":
			if strings.TrimSpace(block.Get("text").String()) != "" {
				return true
			}
		case "thinking":
			if block.Get("thinking").String() != "" || block.Get("signature").String() != "" {
				return true
			}
		case "redacted_thinking", "tool_use", "server_tool_use", "image", "document", "audio", "video":
			return true
		default:
			// Preserve new provider content types rather than speculatively retry
			// a response which might already contain a usable result.
			if block.IsObject() && len(block.Map()) > 0 {
				return true
			}
		}
	}
	return false
}

func anthropicBufferedErrorResponse(resp *http.Response, body []byte) *http.Response {
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil
	}
	status := anthropicBufferedErrorStatus(body)
	if status == 0 {
		return nil
	}
	normalized := new(http.Response)
	*normalized = *resp
	normalized.StatusCode = status
	normalized.Status = strconv.Itoa(status) + " " + http.StatusText(status)
	normalized.Body = io.NopCloser(bytes.NewReader(body))
	return normalized
}

func (s *GatewayService) handleAnthropicBufferedError(ctx context.Context, c *gin.Context, account *Account, resp *http.Response, body []byte, model string, passthrough bool) error {
	if failover := s.handleUpstreamModelNotFound(ctx, c, account, resp, model, OpsUpstreamErrorEvent{Passthrough: passthrough}); failover != nil {
		return failover
	}
	if !s.shouldFailoverUpstreamError(resp.StatusCode) && (resp.StatusCode != http.StatusBadRequest || !s.shouldFailoverBadRequest(body)) {
		_, err := s.handleErrorResponse(ctx, resp, c, account, model)
		return err
	}
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		ProxyID: opsUpstreamProxyID(account), ProxyName: opsUpstreamProxyName(account),
		Platform: account.Platform, AccountID: account.ID, AccountName: account.Name,
		UpstreamStatusCode: resp.StatusCode, UpstreamRequestID: resp.Header.Get("x-request-id"),
		Passthrough: passthrough, Kind: "embedded_error",
		Message: sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(body))),
	})
	if s.rateLimitService != nil {
		s.handleFailoverSideEffects(ctx, resp, account, model)
	}
	return &UpstreamFailoverError{
		StatusCode: resp.StatusCode, ResponseBody: body, ResponseHeaders: resp.Header,
		RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
	}
}
