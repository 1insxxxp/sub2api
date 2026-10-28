package service

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// geminiAIStudioActions 是允许出现在上游 URL 里的 action 集合，与 ForwardNative
// 的入站校验保持一致，同时避免 compat 侧把可变字符串直接拼进 path。
var geminiAIStudioActions = map[string]struct{}{
	"generateContent":       {},
	"streamGenerateContent": {},
	"countTokens":           {},
}

// Match the configured group alias target's character limit; the final
// upstream path segment retains its separate, stricter byte limit.
const maxGeminiRoutingModelRunes = 200

// buildGeminiAIStudioModelActionURL 组装 AI Studio 的
// /v1beta/models/{model}:{action} 上游 URL。
//
// model 是客户端可控的（native 路由取自 URL 片段，compat 路由取自请求体的 model
// 字段，之后可能再经渠道映射），因此必须先过路径片段护栏才能拼进 path，
// 见 upstream_path_guard.go。新增 AI Studio 端点请一律走本函数。
func buildGeminiAIStudioModelActionURL(baseURL, model, action string, stream bool) (string, error) {
	trimmedBase := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmedBase == "" {
		return "", errors.New("gemini base url is required")
	}
	trimmedModel := strings.TrimSpace(model)
	if trimmedModel == "" {
		return "", errors.New("gemini model is required")
	}
	if err := validateUpstreamPathSegment("gemini model", trimmedModel); err != nil {
		return "", err
	}
	trimmedAction := strings.TrimSpace(action)
	if _, ok := geminiAIStudioActions[trimmedAction]; !ok {
		return "", fmt.Errorf("unsupported gemini action: %s", trimmedAction)
	}

	fullURL := fmt.Sprintf("%s/v1beta/models/%s:%s", trimmedBase, trimmedModel, trimmedAction)
	if stream {
		fullURL += "?alt=sse"
	}
	return fullURL, nil
}

// IsSafeGeminiModelPathSegment validates a final upstream model path segment.
// Public routing aliases must be resolved before applying this stricter check.
func IsSafeGeminiModelPathSegment(model string) bool {
	return isSafeUpstreamPathSegment(strings.TrimSpace(model))
}

// IsValidGeminiRoutingModel accepts public routing names, not upstream URL
// paths. Account/group aliases may contain Unicode or namespace prefixes.
// Keep a closed character allowlist and reject path/encoding metacharacters;
// callers must still validate the mapped model before building an upstream URL.
func IsValidGeminiRoutingModel(model string) bool {
	if model == "" || utf8.RuneCountInString(model) > maxGeminiRoutingModelRunes {
		return false
	}
	segments := strings.Split(model, "/")
	if len(segments) > maxUpstreamPathSegments {
		return false
	}
	for _, segment := range segments {
		if segment == "" || strings.Trim(segment, ".") == "" {
			return false
		}
		for _, r := range segment {
			if r != '.' && r != '_' && r != '-' && !unicode.IsLetter(r) && !unicode.IsNumber(r) {
				return false
			}
		}
	}
	return true
}
