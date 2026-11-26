package service

import (
	"strings"
	"testing"
)

func TestPublicGatewayErrorClassification(t *testing.T) {
	cases := []struct {
		status  int
		input   string
		want    int
		message string
	}{
		{400, "antigravity auth missing project_id: http 403", 502, "上游凭证或项目异常"},
		{400, "This service has been disabled in this account", 502, "上游凭证或项目异常"},
		{400, "一次最多支持 3 张图片，请减少后重试。", 400, "图片数量错误"},
		{400, "UNSUPPORTED_CONTENT_TYPE", 400, "内容类型不支持"},
		{400, "invalid_schema", 400, "工具或请求结构错误"},
		{401, "invalid authentication credentials", 502, "上游认证失败"},
		{402, "prepayment credits are depleted", 502, "上游额度耗尽"},
		{403, "Upstream access forbidden", 502, "上游访问被拒绝"},
		{403, "Insufficient account balance", 502, "余额不足"},
		{404, "Model alpha is not supported by any configured account", 404, "模型不可用"},
		{413, "REQUEST_TOO_LARGE", 413, "请求体过大"},
		{422, "Request failed. status_code=422", 422, "请求参数错误"},
		{429, "Upstream rate limit exceeded", 429, "上游限流"},
		{429, "Concurrency limit exceeded", 429, "并发超限"},
		{429, "Service temporarily unavailable at vendor.invalid", 503, "服务繁忙"},
		{500, "当前无可用凭证", 502, "没有可用凭证"},
		{502, "NATIVE_TURN_TIMEOUT", 504, "上游响应超时"},
		{502, "Upstream inference interrupted", 502, "上游请求失败"},
		{503, "No available accounts", 503, "没有可用上游资源"},
		{503, "The service is temporarily unavailable.", 503, "没有可用上游资源"},
		{503, "Service temporarily unavailable", 503, "服务繁忙"},
		{504, "The model did not respond in time", 504, "模型响应超时"},
		{524, "Recovered upstream error 524", 504, "上游连接超时"},
		{400, "unrecognized error from vendor.invalid", 400, "参数异常"},
		{500, "UNSUPPORTED_CONTENT_TYPE", 502, "模型服务请求失败，请稍后重试。如持续出现，请提供请求 ID 联系支持。"},
	}
	for _, c := range cases {
		got := ClassifyPublicGatewayError(c.status, c.input)
		if got.Status != c.want || got.Message != c.message {
			t.Errorf("%d %q: got %+v, want %d %q", c.status, c.input, got, c.want, c.message)
		}
	}
}

func TestPublicGatewayErrorEvent(t *testing.T) {
	payload := []byte(`{"type":"response.failed","sequence_number":9,"response":{"id":"resp_1","error":{"message":"antigravity auth missing project_id","vendor":"hidden"},"model":"secret-provider"}}`)
	got, ok := PublicGatewayErrorEvent("", payload)
	if !ok || strings.Contains(string(got), "antigravity") || strings.Contains(string(got), "secret-provider") || !strings.Contains(string(got), "service_unavailable") || strings.Contains(string(got), "resp_1") {
		t.Fatalf("%s", got)
	}
	success := []byte(`{"type":"response.output_text.delta","delta":"vendor.invalid"}`)
	if _, ok := PublicGatewayErrorEvent("", success); ok {
		t.Fatal("success classified as error")
	}
}
