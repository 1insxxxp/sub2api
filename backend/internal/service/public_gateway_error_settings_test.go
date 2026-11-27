//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestPublicGatewayErrorSettingsPriorityAndFallback(t *testing.T) {
	settings := DefaultPublicGatewayErrorSettings()
	settings.Rules = []PublicGatewayErrorRule{
		{ID: "second", Name: "后匹配", Enabled: true, Priority: 20, UpstreamStatuses: []int{502}, Pattern: "incomplete_stream", Status: 502, Code: "second", Message: "后匹配"},
		{ID: "first", Name: "流中断", Enabled: true, Priority: 10, Pattern: "(?i)incomplete_stream", Status: 503, Code: "stream_interrupted", Message: "生成中断，请重试。"},
	}
	policy, err := CompilePublicGatewayErrorSettings(settings)
	require.NoError(t, err)
	got := policy.Classify(502, "INCOMPLETE_STREAM vendor=kiro sk-private123456")
	require.Equal(t, "first", got.RuleID)
	require.Equal(t, 503, got.Status)
	require.Equal(t, "生成中断，请重试。", got.Message)
	settings.Rules[1].Enabled = false
	policy, err = CompilePublicGatewayErrorSettings(settings)
	require.NoError(t, err)
	require.Equal(t, "second", policy.Classify(502, "incomplete_stream").RuleID)
	require.Equal(t, "invalid_request", policy.Classify(400, "private error").Code)
	require.Equal(t, "request_failed", policy.Classify(500, "private error").Code)
	require.Empty(t, policy.Classify(500, "private error").RuleID)
}

func TestPublicGatewayErrorSettingsStatusOnlyAndUnknownStreamStatus(t *testing.T) {
	s := DefaultPublicGatewayErrorSettings()
	s.Rules = []PublicGatewayErrorRule{
		{ID: "quota", Name: "限流", Enabled: true, Priority: 10, UpstreamStatuses: []int{429}, Status: 429, Code: "rate_limit", Message: "请求过于频繁"},
		{ID: "eof", Name: "中断", Enabled: true, Priority: 20, UpstreamStatuses: []int{502}, Pattern: "incomplete_stream", Status: 502, Code: "stream_interrupted", Message: "生成中断"},
	}
	policy, err := CompilePublicGatewayErrorSettings(s)
	require.NoError(t, err)
	require.Equal(t, "quota", policy.Classify(429, "anything").RuleID)
	require.Empty(t, policy.Classify(502, "anything").RuleID)
	require.Empty(t, policy.Classify(0, "anything").RuleID)
	require.Equal(t, "eof", policy.Classify(0, "incomplete_stream").RuleID)
	require.Empty(t, policy.Classify(500, "incomplete_stream").RuleID)
}

func TestPublicGatewayErrorSettingsUserHistoryAndLocalErrors(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewSettingService(repo, &config.Config{})
	s := DefaultPublicGatewayErrorSettings()
	s.Fallback.Message = "自定义客户提示"
	s.Fallback.Code = "custom_error"
	require.NoError(t, svc.SetPublicGatewayErrorSettings(context.Background(), s))
	ctx := WithPublicGatewayErrorSettings(context.Background(), svc)
	provider := &OpsErrorLog{StatusCode: 502, Phase: "upstream", Type: "api_error", Message: "secret"}
	require.Equal(t, s.Fallback.Message, ToUserErrorRequestContext(ctx, provider).Message)
	local := &OpsErrorLog{StatusCode: 402, Phase: "request", Type: "INSUFFICIENT_BALANCE", Message: "insufficient balance"}
	require.Equal(t, "账户余额或额度不足，请检查后重试。", ToUserErrorRequestContext(ctx, local).Message)
	require.Equal(t, "账户余额或额度不足，请检查后重试。", ToUserErrorRequestDetailContext(ctx, &OpsErrorLogDetail{OpsErrorLog: *local, UpstreamErrorMessage: "previous attempt failed"}).Message)
}

func TestPublicGatewayErrorSettingsValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*PublicGatewayErrorSettings)
	}{
		{"regex", func(s *PublicGatewayErrorSettings) { s.Rules[0].Pattern = "[" }},
		{"source status", func(s *PublicGatewayErrorSettings) { s.Rules[0].UpstreamStatuses = []int{200} }},
		{"response status", func(s *PublicGatewayErrorSettings) { s.Rules[0].Status = 200 }},
		{"code", func(s *PublicGatewayErrorSettings) { s.Rules[0].Code = "vendor https://private.invalid" }},
		{"blank message", func(s *PublicGatewayErrorSettings) { s.Fallback.Message = " " }},
		{"duplicate id", func(s *PublicGatewayErrorSettings) { s.Rules[1].ID = s.Rules[0].ID }},
		{"no condition", func(s *PublicGatewayErrorSettings) { s.Rules[0].Pattern = ""; s.Rules[0].UpstreamStatuses = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := DefaultPublicGatewayErrorSettings()
			tc.change(s)
			_, err := CompilePublicGatewayErrorSettings(s)
			require.Error(t, err)
		})
	}
}

func TestPublicGatewayErrorSettingsContextAndInstances(t *testing.T) {
	repo := newMockSettingRepo()
	first := NewSettingService(repo, &config.Config{})
	second := NewSettingService(repo, &config.Config{})
	ctx := WithPublicGatewayErrorSettings(context.Background(), second)
	require.Equal(t, 0, repo.getValueCalls)
	success := []byte(`{"type":"response.output_text.delta","delta":"hello"}`)
	_, ok := PublicGatewayErrorEventContext(ctx, "", success)
	require.False(t, ok)
	require.Equal(t, 0, repo.getValueCalls)
	s := DefaultPublicGatewayErrorSettings()
	s.Fallback.Message = "服务稍忙，请重试。"
	require.NoError(t, first.SetPublicGatewayErrorSettings(context.Background(), s))
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := ClassifyPublicGatewayErrorContext(ctx, 502, "unknown")
			require.Equal(t, s.Fallback.Message, got.Message)
		}()
	}
	wg.Wait()
	require.Equal(t, 1, repo.getValueCalls)
	s.Fallback.Message = "更新后的提示"
	require.NoError(t, first.SetPublicGatewayErrorSettings(context.Background(), s))
	require.Equal(t, "服务稍忙，请重试。", ClassifyPublicGatewayErrorContext(ctx, 502, "unknown").Message)
	next := WithPublicGatewayErrorSettings(context.Background(), second)
	require.Equal(t, "更新后的提示", ClassifyPublicGatewayErrorContext(next, 502, "unknown").Message)
}

func TestPublicGatewayErrorSettingsInvalidStoreUsesDefaults(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewSettingService(repo, &config.Config{})
	repo.data[SettingKeyPublicGatewayErrorSettings] = `{"rules":[],"fallback":{"status":200}}`
	settings, err := svc.GetPublicGatewayErrorSettings(context.Background())
	require.Error(t, err)
	require.Nil(t, settings)
	ctx := WithPublicGatewayErrorSettings(context.Background(), svc)
	require.Equal(t, ClassifyPublicGatewayError(502, "unknown"), ClassifyPublicGatewayErrorContext(ctx, 502, "unknown"))
	repo.getValueErr = errors.New("database unavailable")
	ctx = WithPublicGatewayErrorSettings(context.Background(), svc)
	require.Equal(t, "request_failed", ClassifyPublicGatewayErrorContext(ctx, 502, "private hostname").Code)
}

func TestPublicGatewayErrorSettingsProtocolPreview(t *testing.T) {
	s := DefaultPublicGatewayErrorSettings()
	s.Rules = []PublicGatewayErrorRule{{ID: "eof", Name: "流中断", Enabled: true, Pattern: "incomplete_stream", Status: 502, Code: "stream_interrupted", Message: "上游返回中断，请重试。"}}
	for _, format := range []string{"http", "sse", "responses", "gemini"} {
		t.Run(format, func(t *testing.T) {
			result, err := PreviewPublicGatewayError(s, 502, "incomplete_stream kiro private.invalid", format)
			require.NoError(t, err)
			require.Equal(t, "eof", result.RuleID)
			require.Contains(t, string(result.Payload), "上游返回中断")
			require.NotContains(t, string(result.Payload), "kiro")
			require.NotContains(t, string(result.Payload), "private.invalid")
		})
	}
	_, err := PreviewPublicGatewayError(s, 200, "foo", "http")
	require.Error(t, err)
	_, err = PreviewPublicGatewayError(s, 502, "foo", "unknown")
	require.Error(t, err)
}
