package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/googleapi"
)

const SettingKeyPublicGatewayErrorSettings = "public_gateway_error_settings"

// PublicGatewayErrorRule matches diagnostics before constructing a client-safe
// response. Priority is ascending, with input order breaking ties.
type PublicGatewayErrorRule struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Enabled          bool   `json:"enabled"`
	Priority         int    `json:"priority"`
	UpstreamStatuses []int  `json:"upstream_statuses"`
	Pattern          string `json:"pattern"`
	Status           int    `json:"status"`
	Code             string `json:"code"`
	Message          string `json:"message"`
}

type PublicGatewayErrorSettings struct {
	Rules              []PublicGatewayErrorRule `json:"rules"`
	Fallback           PublicGatewayError       `json:"fallback"`
	BadRequestFallback PublicGatewayError       `json:"bad_request_fallback"`
}

type PublicGatewayErrorMatch struct {
	PublicGatewayError
	RuleID   string `json:"rule_id,omitempty"`
	RuleName string `json:"rule_name,omitempty"`
}

type compiledPublicGatewayErrorRule struct {
	rule    PublicGatewayErrorRule
	pattern *regexp.Regexp
}

type PublicGatewayErrorPolicy struct {
	rules              []compiledPublicGatewayErrorRule
	fallback           PublicGatewayError
	badRequestFallback PublicGatewayError
}

func DefaultPublicGatewayErrorSettings() *PublicGatewayErrorSettings {
	settings := &PublicGatewayErrorSettings{
		Rules:              make([]PublicGatewayErrorRule, 0, len(publicGatewayErrorRules)),
		Fallback:           PublicGatewayError{502, "request_failed", "模型服务请求失败，请稍后重试。如持续出现，请提供请求 ID 联系支持。"},
		BadRequestFallback: PublicGatewayError{400, "invalid_request", "参数异常"},
	}
	for i, rule := range publicGatewayErrorRules {
		settings.Rules = append(settings.Rules, PublicGatewayErrorRule{
			ID: rule.code, Name: rule.message, Enabled: true, Priority: (i + 1) * 10,
			UpstreamStatuses: append([]int{}, rule.upstreamStatuses...), Pattern: rule.pattern.String(),
			Status: rule.status, Code: rule.code, Message: rule.message,
		})
	}
	return settings
}

var publicGatewayIdentifier = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
var publicGatewayCode = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func validatePublicGatewayOutput(output PublicGatewayError) error {
	if output.Status < 400 || output.Status > 599 {
		return errors.New("返回状态码须为 400 至 599")
	}
	if !publicGatewayCode.MatchString(output.Code) {
		return errors.New("错误代码须为小写字母开头的字母、数字或下划线，最多 64 个字符")
	}
	if strings.TrimSpace(output.Message) == "" || len([]rune(output.Message)) > 2048 {
		return errors.New("客户提示须为 1 至 2048 个字符")
	}
	return nil
}

func CompilePublicGatewayErrorSettings(settings *PublicGatewayErrorSettings) (*PublicGatewayErrorPolicy, error) {
	if settings == nil {
		return nil, errors.New("请提供错误映射设置")
	}
	if len(settings.Rules) > 100 {
		return nil, errors.New("最多支持 100 条错误映射规则")
	}
	if err := validatePublicGatewayOutput(settings.Fallback); err != nil {
		return nil, fmt.Errorf("默认提示%w", err)
	}
	if err := validatePublicGatewayOutput(settings.BadRequestFallback); err != nil {
		return nil, fmt.Errorf("参数错误默认提示%w", err)
	}
	policy := &PublicGatewayErrorPolicy{fallback: safePublicGatewayOutput(settings.Fallback), badRequestFallback: safePublicGatewayOutput(settings.BadRequestFallback)}
	ids := make(map[string]bool, len(settings.Rules))
	for i, rule := range settings.Rules {
		if !publicGatewayIdentifier.MatchString(rule.ID) || ids[rule.ID] {
			return nil, fmt.Errorf("第 %d 条规则的标识无效或重复", i+1)
		}
		ids[rule.ID] = true
		if strings.TrimSpace(rule.Name) == "" || len([]rune(rule.Name)) > 128 {
			return nil, fmt.Errorf("第 %d 条规则须填写名称，最多 128 个字符", i+1)
		}
		if rule.Priority < -1000000 || rule.Priority > 1000000 {
			return nil, fmt.Errorf("规则 %s 的优先级超出范围", rule.Name)
		}
		if len(rule.UpstreamStatuses) > 200 || len(rule.Pattern) > 4096 {
			return nil, fmt.Errorf("规则 %s 的匹配条件过长", rule.Name)
		}
		for _, status := range rule.UpstreamStatuses {
			if status < 400 || status > 599 {
				return nil, fmt.Errorf("规则 %s 的来源状态码须为 400 至 599", rule.Name)
			}
		}
		if len(rule.UpstreamStatuses) == 0 && strings.TrimSpace(rule.Pattern) == "" {
			return nil, fmt.Errorf("规则 %s 至少需要状态码或匹配表达式", rule.Name)
		}
		output := PublicGatewayError{rule.Status, rule.Code, rule.Message}
		if err := validatePublicGatewayOutput(output); err != nil {
			return nil, fmt.Errorf("规则 %s %w", rule.Name, err)
		}
		pattern, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return nil, fmt.Errorf("规则 %s 的正则表达式无效 %w", rule.Name, err)
		}
		rule.UpstreamStatuses = append([]int(nil), rule.UpstreamStatuses...)
		rule.Message = safePublicGatewayOutput(output).Message
		policy.rules = append(policy.rules, compiledPublicGatewayErrorRule{rule, pattern})
	}
	sort.SliceStable(policy.rules, func(i, j int) bool { return policy.rules[i].rule.Priority < policy.rules[j].rule.Priority })
	return policy, nil
}

func safePublicGatewayOutput(output PublicGatewayError) PublicGatewayError {
	output.Message = sanitizeUpstreamErrorMessage(strings.TrimSpace(output.Message))
	return output
}

func (p *PublicGatewayErrorPolicy) Classify(status int, diagnostic string) PublicGatewayErrorMatch {
	for _, compiled := range p.rules {
		rule := compiled.rule
		if !rule.Enabled || (status != 0 && len(rule.UpstreamStatuses) > 0 && !slices.Contains(rule.UpstreamStatuses, status)) {
			continue
		}
		// Unknown stream status can be matched by diagnostic text. A status-only
		// rule requires a known status to avoid matching every stream error.
		if status == 0 && len(rule.UpstreamStatuses) > 0 && rule.Pattern == "" {
			continue
		}
		if compiled.pattern.MatchString(diagnostic) {
			return PublicGatewayErrorMatch{PublicGatewayError{rule.Status, rule.Code, rule.Message}, rule.ID, rule.Name}
		}
	}
	if status == 400 {
		return PublicGatewayErrorMatch{PublicGatewayError: p.badRequestFallback}
	}
	return PublicGatewayErrorMatch{PublicGatewayError: p.fallback}
}

var defaultPublicGatewayErrorPolicy = func() *PublicGatewayErrorPolicy {
	policy, err := CompilePublicGatewayErrorSettings(DefaultPublicGatewayErrorSettings())
	if err != nil {
		panic(err)
	}
	return policy
}()

func (s *SettingService) GetPublicGatewayErrorSettings(ctx context.Context) (*PublicGatewayErrorSettings, error) {
	if s == nil || s.settingRepo == nil {
		return DefaultPublicGatewayErrorSettings(), nil
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyPublicGatewayErrorSettings)
	if errors.Is(err, ErrSettingNotFound) || (err == nil && strings.TrimSpace(raw) == "") {
		return DefaultPublicGatewayErrorSettings(), nil
	}
	if err != nil {
		return nil, err
	}
	var settings PublicGatewayErrorSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return nil, err
	}
	if _, err := CompilePublicGatewayErrorSettings(&settings); err != nil {
		return nil, err
	}
	if settings.Rules == nil {
		settings.Rules = []PublicGatewayErrorRule{}
	}
	return &settings, nil
}

func (s *SettingService) SetPublicGatewayErrorSettings(ctx context.Context, settings *PublicGatewayErrorSettings) error {
	if _, err := CompilePublicGatewayErrorSettings(settings); err != nil {
		return err
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return s.settingRepo.Set(ctx, SettingKeyPublicGatewayErrorSettings, string(data))
}

type publicGatewaySettingsContextKey struct{}
type publicGatewaySettingsSnapshot struct {
	once    sync.Once
	service *SettingService
	policy  *PublicGatewayErrorPolicy
}

// WithPublicGatewayErrorSettings defers the shared-store read until the first
// error. A request, including its stream or WebSocket, uses one immutable policy.
func WithPublicGatewayErrorSettings(ctx context.Context, svc *SettingService) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, publicGatewaySettingsContextKey{}, &publicGatewaySettingsSnapshot{service: svc})
}

func publicGatewayPolicyFromContext(ctx context.Context) *PublicGatewayErrorPolicy {
	if ctx == nil {
		return defaultPublicGatewayErrorPolicy
	}
	snapshot, _ := ctx.Value(publicGatewaySettingsContextKey{}).(*publicGatewaySettingsSnapshot)
	if snapshot == nil {
		return defaultPublicGatewayErrorPolicy
	}
	snapshot.once.Do(func() {
		snapshot.policy = defaultPublicGatewayErrorPolicy
		if snapshot.service == nil {
			return
		}
		// Client cancellation must not select a different public policy.
		readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		settings, err := snapshot.service.GetPublicGatewayErrorSettings(readCtx)
		if err != nil {
			return
		}
		if policy, err := CompilePublicGatewayErrorSettings(settings); err == nil {
			snapshot.policy = policy
		}
	})
	return snapshot.policy
}

func ClassifyPublicGatewayErrorContext(ctx context.Context, status int, diagnostic string) PublicGatewayError {
	return publicGatewayPolicyFromContext(ctx).Classify(status, diagnostic).PublicGatewayError
}

type PublicGatewayErrorPreview struct {
	PublicGatewayErrorMatch
	Payload json.RawMessage `json:"payload"`
}

// PreviewPublicGatewayError uses the same classifier and event formatter as live
// requests. Diagnostics are input only, never copied into the result.
func PreviewPublicGatewayError(settings *PublicGatewayErrorSettings, status int, diagnostic, format string) (*PublicGatewayErrorPreview, error) {
	if status != 0 && (status < 400 || status > 599) {
		return nil, errors.New("来源状态码须为 400 至 599，未知时填 0")
	}
	if len(diagnostic) > 64<<10 {
		return nil, errors.New("测试错误内容最多 64KB")
	}
	policy, err := CompilePublicGatewayErrorSettings(settings)
	if err != nil {
		return nil, err
	}
	matched := policy.Classify(status, diagnostic)
	public := matched.PublicGatewayError
	var payload []byte
	switch format {
	case "http":
		payload, _ = json.Marshal(map[string]any{"error": map[string]any{"type": public.Type(), "code": public.Code, "message": public.Message}})
	case "sse", "responses", "gemini":
		eventName := "error"
		source := map[string]any{"error": map[string]any{"status_code": status, "message": diagnostic}}
		if format == "responses" {
			eventName = "response.failed"
		}
		raw, _ := json.Marshal(source)
		payload, _ = publicGatewayErrorEvent(eventName, raw, policy, status)
		if format == "gemini" {
			payload, _ = json.Marshal(map[string]any{"error": map[string]any{"code": public.Status, "status": googleapi.HTTPStatusToGoogleStatus(public.Status), "message": public.Message}})
		}
	default:
		return nil, errors.New("请选择 HTTP、SSE、Responses 或 Gemini 格式")
	}
	return &PublicGatewayErrorPreview{matched, payload}, nil
}
