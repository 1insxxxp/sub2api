package service

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/googleapi"
	"github.com/tidwall/gjson"
)

type PublicGatewayError struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

var publicGatewayErrorRules = []struct {
	upstreamStatuses []int
	pattern          *regexp.Regexp
	status           int
	code, message    string
}{
	{[]int{400}, regexp.MustCompile(`(?i)(?:auth missing project_id|service has been disabled in this account)`), 502, "service_unavailable", "上游凭证或项目异常"},
	{[]int{400}, regexp.MustCompile(`(?i)(?:一次最多支持\s*3\s*张图片)`), 400, "too_many_images", "图片数量错误"},
	{[]int{400}, regexp.MustCompile(`(?i)(?:unsupported_content_type|unsupported content type)`), 400, "unsupported_content_type", "内容类型不支持"},
	{[]int{400}, regexp.MustCompile(`(?i)(?:thought.?signature|invalid_schema|invalid tool (input|schema|arguments?)|工具参数错误)`), 400, "invalid_tool_request", "工具或请求结构错误"},
	{[]int{401}, regexp.MustCompile(`(?i)(?:invalid authentication credentials)`), 502, "upstream_authentication_failed", "上游认证失败"},
	{[]int{402}, regexp.MustCompile(`(?i)(?:prepayment credits are depleted)`), 502, "upstream_quota_exhausted", "上游额度耗尽"},
	{[]int{403}, regexp.MustCompile(`(?i)(?:insufficient (account )?balance)`), 502, "upstream_insufficient_balance", "余额不足"},
	{[]int{403}, regexp.MustCompile(`(?i)(?:upstream access forbidden)`), 502, "upstream_access_denied", "上游访问被拒绝"},
	{[]int{404}, regexp.MustCompile(`(?i)(?:model .{0,160}(is not supported by any configured account|is not available for this group)|model_not_found)`), 404, "model_not_found", "模型不可用"},
	{[]int{413}, regexp.MustCompile(`(?i)(?:request_too_large|request body is too large)`), 413, "request_too_large", "请求体过大"},
	{[]int{422}, regexp.MustCompile(`(?i)(?:request failed\. status_code=422)`), 422, "invalid_request", "请求参数错误"},
	{[]int{429}, regexp.MustCompile(`(?i)(?:concurrency limit exceeded|排队的请求已达上限)`), 429, "concurrency_limit_exceeded", "并发超限"},
	{[]int{429}, regexp.MustCompile(`(?i)(?:rate limit exceeded|rate_limit_exceeded)`), 429, "rate_limit_exceeded", "上游限流"},
	{[]int{503}, regexp.MustCompile(`(?i)(?:no available accounts|the service is temporarily unavailable|没有可用的上游资源)`), 503, "no_available_resources", "没有可用上游资源"},
	{[]int{429, 503}, regexp.MustCompile(`(?i)(?:service temporarily unavailable)`), 503, "service_overloaded", "服务繁忙"},
	{[]int{500}, regexp.MustCompile(`(?i)(?:当前无可用凭证)`), 502, "no_available_credentials", "没有可用凭证"},
	{[]int{500, 502}, regexp.MustCompile(`(?i)(?:upstream timeout|native_turn_timeout)`), 504, "response_timeout", "上游响应超时"},
	{[]int{502}, regexp.MustCompile(`(?i)(?:upstream inference interrupted|recovered upstream error 502)`), 502, "upstream_request_failed", "上游请求失败"},
	{[]int{504}, regexp.MustCompile(`(?i)(?:the model did not respond in time)`), 504, "model_response_timeout", "模型响应超时"},
	{[]int{524}, regexp.MustCompile(`(?i)(?:recovered upstream error 524)`), 504, "upstream_connection_timeout", "上游连接超时"},
	{[]int{400, 403, 422, 451, 502}, regexp.MustCompile(`(?i)(?:cyber_policy|content_policy_violation|请求触发敏感词拦截)`), 403, "content_policy_violation", "请求触发内容限制，请修改提示词后重试。"},
}

func ClassifyPublicGatewayError(status int, diagnostic string) PublicGatewayError {
	return defaultPublicGatewayErrorPolicy.Classify(status, diagnostic).PublicGatewayError
}

func (e PublicGatewayError) Type() string {
	switch e.Status {
	case 400, 404, 413, 422:
		return "invalid_request_error"
	case 403:
		return "permission_error"
	case 429:
		return "rate_limit_error"
	case 503:
		return "overloaded_error"
	default:
		return "api_error"
	}
}

// PublicGatewayErrorEvent rebuilds protocol error events from fixed public fields.
func PublicGatewayErrorEvent(eventName string, payload []byte, upstreamStatus ...int) ([]byte, bool) {
	return publicGatewayErrorEvent(eventName, payload, nil, upstreamStatus...)
}

func PublicGatewayErrorEventContext(ctx context.Context, eventName string, payload []byte, upstreamStatus ...int) ([]byte, bool) {
	// Successful frames avoid both policy loading and shared-store I/O.
	return publicGatewayErrorEventWithClassifier(eventName, payload, func(status int, diagnostic string) PublicGatewayError {
		return ClassifyPublicGatewayErrorContext(ctx, status, diagnostic)
	}, upstreamStatus...)
}

func publicGatewayErrorEvent(eventName string, payload []byte, policy *PublicGatewayErrorPolicy, upstreamStatus ...int) ([]byte, bool) {
	if policy == nil {
		policy = defaultPublicGatewayErrorPolicy
	}
	return publicGatewayErrorEventWithClassifier(eventName, payload, func(status int, diagnostic string) PublicGatewayError {
		return policy.Classify(status, diagnostic).PublicGatewayError
	}, upstreamStatus...)
}

func publicGatewayErrorEventWithClassifier(eventName string, payload []byte, classify func(int, string) PublicGatewayError, upstreamStatus ...int) ([]byte, bool) {
	parsed := gjson.ParseBytes(payload)
	kind := parsed.Get("type").String()
	if eventName == "response.failed" || kind == "response.failed" || parsed.Get("response.status").String() == "failed" {
		kind = "response.failed"
	} else if eventName == "error" || kind == "error" || (parsed.Get("error").Exists() && parsed.Get("error").Type != gjson.Null) {
		kind = "error"
	} else {
		return payload, false
	}
	source := parsed.Get("response.error")
	if !source.Exists() {
		source = parsed.Get("error")
	}
	if !source.IsObject() {
		source = parsed
	}
	status := int(source.Get("status_code").Int())
	if status == 0 {
		status = int(source.Get("code").Int())
	}
	if status == 0 {
		status = int(parsed.Get("status_code").Int())
	}
	if status == 0 && len(upstreamStatus) > 0 {
		status = upstreamStatus[0]
	}
	diagnostic := source.Get("code").String() + " " + source.Get("type").String() + " " + source.Get("message").String()
	if source.Get("message").String() == "" {
		diagnostic += " " + extractUpstreamErrorMessage(payload)
	}
	e := classify(status, diagnostic)
	errorObject := map[string]any{"type": e.Type(), "code": e.Code, "message": e.Message}
	var output any = map[string]any{"type": "error", "error": errorObject}
	if kind == "response.failed" {
		response := map[string]any{"object": "response", "status": "failed", "output": []any{}, "error": errorObject}
		output = map[string]any{"type": kind, "sequence_number": parsed.Get("sequence_number").Int(), "response": response}
	} else if source.Get("code").Type == gjson.Number && source.Get("status").Exists() {
		output = map[string]any{"error": map[string]any{"code": e.Status, "status": googleapi.HTTPStatusToGoogleStatus(e.Status), "message": e.Message}}
	} else if parsed.Get("message").Exists() && !parsed.Get("error").Exists() {
		output = map[string]any{"type": "error", "code": e.Code, "message": e.Message, "sequence_number": parsed.Get("sequence_number").Int()}
	}
	safe, _ := json.Marshal(output)
	return safe, true
}

// PublicLocalGatewayError formats errors known to arise at the gateway.
func PublicLocalGatewayError(status int, code, message string) (PublicGatewayError, bool) {
	switch status {
	case 401:
		switch strings.ToUpper(code) {
		case "API_KEY_REQUIRED", "INVALID_API_KEY", "API_KEY_DISABLED", "USER_NOT_FOUND", "USER_INACTIVE":
			return PublicGatewayError{401, "authentication_error", "身份验证失败，请检查 API Key。"}, true
		}
	case 402, 403:
		if status == 403 && message == "Seedance requires an OpenAI or composite group" {
			return PublicGatewayError{403, "permission_denied", "当前分组不支持此接口。"}, true
		}
		switch strings.ToUpper(code) {
		case "INSUFFICIENT_BALANCE", "INSUFFICIENT_QUOTA":
			return PublicGatewayError{status, "insufficient_balance", "账户余额或额度不足，请检查后重试。"}, true
		}
	case 404:
		switch message {
		case "Images API is not supported for this platform",
			"Embeddings API is not supported for this platform",
			"Voice API is not supported for this platform",
			"Realtime API is not supported for this platform",
			"Web Search API is not supported for this platform",
			"X Search API is not supported for this platform",
			"Codex alpha search is only available for OpenAI and Composite groups",
			"Codex models manifest is only available for OpenAI and Composite groups",
			"Videos API is not supported for this platform",
			"Unsupported responses subpath":
			return PublicGatewayError{404, "not_found", message}, true
		}
	}
	return PublicGatewayError{}, false
}
