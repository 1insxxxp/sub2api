package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyErrorPassthroughRule_NoBoundService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	status, errType, errMsg, matched := applyErrorPassthroughRule(
		c,
		PlatformAnthropic,
		http.StatusUnprocessableEntity,
		[]byte(`{"error":{"message":"invalid schema"}}`),
		http.StatusBadGateway,
		"upstream_error",
		"Upstream request failed",
	)

	assert.False(t, matched)
	assert.Equal(t, http.StatusBadGateway, status)
	assert.Equal(t, "upstream_error", errType)
	assert.Equal(t, "Upstream request failed", errMsg)
}

func TestApplyErrorPassthroughRule_SanitizesUpstreamMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	ruleSvc := &ErrorPassthroughService{}
	rule := newNonFailoverPassthroughRule(http.StatusBadRequest, "provider failed", http.StatusBadRequest, "")
	rule.PassthroughBody = true
	ruleSvc.setLocalCache([]*model.ErrorPassthroughRule{rule})
	BindErrorPassthroughService(c, ruleSvc)

	_, _, message, matched := applyErrorPassthroughRule(
		c,
		PlatformOpenAI,
		http.StatusBadRequest,
		[]byte(`{"error":{"message":"provider failed at https://relay.example:8443/v1"}}`),
		http.StatusBadGateway,
		"upstream_error",
		"Upstream request failed",
	)

	require.True(t, matched)
	require.NotContains(t, message, "relay.example")
	require.Contains(t, message, "[upstream-url]")
}

func TestGatewayHandleErrorResponse_NoRuleKeepsDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	svc := &GatewayService{}
	respBody := []byte(`{"error":{"message":"Invalid schema for field messages"}}`)
	resp := &http.Response{
		StatusCode: http.StatusUnprocessableEntity,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
		Header:     http.Header{},
	}
	account := &Account{ID: 11, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}

	_, err := svc.handleErrorResponse(context.Background(), resp, c, account)
	require.Error(t, err)
	assert.Equal(t, http.StatusBadGateway, rec.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	errField, ok := payload["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "upstream_error", errField["type"])
	assert.Equal(t, "Upstream request failed", errField["message"])
}

func TestGatewayHandleErrorResponse_SanitizesBadRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	svc := &GatewayService{}
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"message":"invalid request at https://relay.example:8443/v1"}}`))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	account := &Account{ID: 15, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}

	_, err := svc.handleErrorResponse(context.Background(), resp, c, account)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.NotContains(t, rec.Body.String(), "relay.example")
	require.Contains(t, rec.Body.String(), "[upstream-url]")
}

func TestGatewayHandleErrorResponse_SanitizesBadRequestFallbackError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	svc := &GatewayService{}
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"details":"failed at https://relay.example:8443/v1"}`))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	account := &Account{ID: 19, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}

	_, err := svc.handleErrorResponse(context.Background(), resp, c, account)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "relay.example")
	require.NotContains(t, rec.Body.String(), "relay.example")
}

func TestOpenAIHandleErrorResponse_NoRuleKeepsDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	svc := &OpenAIGatewayService{}
	respBody := []byte(`{"error":{"message":"Invalid schema for field messages"}}`)
	resp := &http.Response{
		StatusCode: http.StatusUnprocessableEntity,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
		Header:     http.Header{},
	}
	account := &Account{ID: 12, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	_, err := svc.handleErrorResponse(context.Background(), resp, c, account, nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusBadGateway, rec.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	errField, ok := payload["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "upstream_error", errField["type"])
	assert.Equal(t, "Upstream request failed", errField["message"])
}

func TestOpenAIHandleErrorResponse_SanitizesCyberPolicyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	svc := &OpenAIGatewayService{}
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"code":"cyber_policy","message":"blocked by https://relay.example:8443/policy"}}`))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	account := &Account{ID: 16, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	_, err := svc.handleErrorResponse(context.Background(), resp, c, account, nil)
	require.Error(t, err)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.NotContains(t, err.Error(), "relay.example")
	require.NotContains(t, rec.Body.String(), "relay.example")
	if mark := GetOpsCyberPolicy(c); mark != nil {
		require.NotContains(t, mark.Message, "relay.example")
	}
	require.Contains(t, rec.Body.String(), "[upstream-url]")
}

func TestOpenAIHandleCompatErrorResponse_SanitizesCyberPolicyMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	svc := &OpenAIGatewayService{}
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"code":"cyber_policy","message":"blocked by https://relay.example:8443/policy"}}`))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	account := &Account{ID: 18, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	_, err := svc.handleCompatErrorResponse(resp, c, account, func(c *gin.Context, status int, errType, message string) {
		c.JSON(status, gin.H{"error": gin.H{"type": errType, "message": message}})
	})
	require.Error(t, err)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.NotContains(t, err.Error(), "relay.example")
	require.NotContains(t, rec.Body.String(), "relay.example")
	if mark := GetOpsCyberPolicy(c); mark != nil {
		require.NotContains(t, mark.Message, "relay.example")
	}
	require.Contains(t, rec.Body.String(), "[upstream-url]")
}

func TestOpenAIHandleErrorResponse_ContextWindow502KeepsMessageWithoutFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	svc := &OpenAIGatewayService{}
	respBody := []byte(`{"error":{"message":"Your input exceeds the context window of this model. Please adjust your input and try again.","type":"upstream_error","code":null}}`)
	resp := &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
		Header:     http.Header{},
	}
	account := &Account{ID: 14, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	_, err := svc.handleErrorResponse(context.Background(), resp, c, account, nil)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	assert.Equal(t, http.StatusBadGateway, rec.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	errField, ok := payload["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "upstream_error", errField["type"])
	assert.Equal(t, "Your input exceeds the context window of this model. Please adjust your input and try again.", errField["message"])
}

func TestGeminiWriteGeminiMappedError_NoRuleKeepsDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	svc := &GeminiMessagesCompatService{}
	respBody := []byte(`{"error":{"code":422,"message":"Invalid schema for field messages","status":"INVALID_ARGUMENT"}}`)
	account := &Account{ID: 13, Platform: PlatformGemini, Type: AccountTypeAPIKey}

	err := svc.writeGeminiMappedError(c, account, http.StatusUnprocessableEntity, "req-2", respBody)
	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	errField, ok := payload["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "invalid_request_error", errField["type"])
	assert.Equal(t, "Upstream request failed", errField["message"])
}

func TestGeminiWriteGeminiNativeUpstreamError_SanitizesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	svc := &GeminiMessagesCompatService{}
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	account := &Account{ID: 17, Platform: PlatformGemini, Type: AccountTypeAPIKey}
	body := []byte(`{"error":{"message":"invalid request at https://relay.example:8443/v1"}}`)

	err := svc.writeGeminiNativeUpstreamError(c, account, resp, body, "req-3", false)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.NotContains(t, rec.Body.String(), "relay.example")
	require.Contains(t, rec.Body.String(), "[upstream-url]")
}

func TestGatewayHandleErrorResponse_AppliesRuleFor422(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	ruleSvc := &ErrorPassthroughService{}
	ruleSvc.setLocalCache([]*model.ErrorPassthroughRule{newNonFailoverPassthroughRule(http.StatusUnprocessableEntity, "invalid schema", http.StatusTeapot, "上游请求失败")})
	BindErrorPassthroughService(c, ruleSvc)

	svc := &GatewayService{}
	respBody := []byte(`{"error":{"message":"Invalid schema for field messages"}}`)
	resp := &http.Response{
		StatusCode: http.StatusUnprocessableEntity,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
		Header:     http.Header{},
	}
	account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}

	_, err := svc.handleErrorResponse(context.Background(), resp, c, account)
	require.Error(t, err)
	assert.Equal(t, http.StatusTeapot, rec.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	errField, ok := payload["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "upstream_error", errField["type"])
	assert.Equal(t, "上游请求失败", errField["message"])
}

func TestOpenAIHandleErrorResponse_AppliesRuleFor422(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	ruleSvc := &ErrorPassthroughService{}
	ruleSvc.setLocalCache([]*model.ErrorPassthroughRule{newNonFailoverPassthroughRule(http.StatusUnprocessableEntity, "invalid schema", http.StatusTeapot, "OpenAI上游失败")})
	BindErrorPassthroughService(c, ruleSvc)

	svc := &OpenAIGatewayService{}
	respBody := []byte(`{"error":{"message":"Invalid schema for field messages"}}`)
	resp := &http.Response{
		StatusCode: http.StatusUnprocessableEntity,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
		Header:     http.Header{},
	}
	account := &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	_, err := svc.handleErrorResponse(context.Background(), resp, c, account, nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusTeapot, rec.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	errField, ok := payload["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "upstream_error", errField["type"])
	assert.Equal(t, "OpenAI上游失败", errField["message"])
}

func TestGeminiWriteGeminiMappedError_AppliesRuleFor422(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	ruleSvc := &ErrorPassthroughService{}
	ruleSvc.setLocalCache([]*model.ErrorPassthroughRule{newNonFailoverPassthroughRule(http.StatusUnprocessableEntity, "invalid schema", http.StatusTeapot, "Gemini上游失败")})
	BindErrorPassthroughService(c, ruleSvc)

	svc := &GeminiMessagesCompatService{}
	respBody := []byte(`{"error":{"code":422,"message":"Invalid schema for field messages","status":"INVALID_ARGUMENT"}}`)
	account := &Account{ID: 3, Platform: PlatformGemini, Type: AccountTypeAPIKey}

	err := svc.writeGeminiMappedError(c, account, http.StatusUnprocessableEntity, "req-1", respBody)
	require.Error(t, err)
	assert.Equal(t, http.StatusTeapot, rec.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	errField, ok := payload["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "upstream_error", errField["type"])
	assert.Equal(t, "Gemini上游失败", errField["message"])
}

func TestApplyErrorPassthroughRule_SkipMonitoringSetsContextKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	rule := newNonFailoverPassthroughRule(http.StatusBadRequest, "prompt is too long", http.StatusBadRequest, "上下文超限")
	rule.SkipMonitoring = true

	ruleSvc := &ErrorPassthroughService{}
	ruleSvc.setLocalCache([]*model.ErrorPassthroughRule{rule})
	BindErrorPassthroughService(c, ruleSvc)

	_, _, _, matched := applyErrorPassthroughRule(
		c,
		PlatformAnthropic,
		http.StatusBadRequest,
		[]byte(`{"error":{"message":"prompt is too long"}}`),
		http.StatusBadGateway,
		"upstream_error",
		"Upstream request failed",
	)

	assert.True(t, matched)
	v, exists := c.Get(OpsSkipPassthroughKey)
	assert.True(t, exists, "OpsSkipPassthroughKey should be set when skip_monitoring=true")
	boolVal, ok := v.(bool)
	assert.True(t, ok, "value should be a bool")
	assert.True(t, boolVal)
}

func TestApplyErrorPassthroughRule_NoSkipMonitoringDoesNotSetContextKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	rule := newNonFailoverPassthroughRule(http.StatusBadRequest, "prompt is too long", http.StatusBadRequest, "上下文超限")
	rule.SkipMonitoring = false

	ruleSvc := &ErrorPassthroughService{}
	ruleSvc.setLocalCache([]*model.ErrorPassthroughRule{rule})
	BindErrorPassthroughService(c, ruleSvc)

	_, _, _, matched := applyErrorPassthroughRule(
		c,
		PlatformAnthropic,
		http.StatusBadRequest,
		[]byte(`{"error":{"message":"prompt is too long"}}`),
		http.StatusBadGateway,
		"upstream_error",
		"Upstream request failed",
	)

	assert.True(t, matched)
	_, exists := c.Get(OpsSkipPassthroughKey)
	assert.False(t, exists, "OpsSkipPassthroughKey should NOT be set when skip_monitoring=false")
}

// ---- ResponseCommittedKey: service 层写完错误响应后标记，handler 层检查跳过兜底写入 ----

func TestHandleErrorResponse_SetsResponseCommitted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	svc := &GatewayService{}
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"message":"temperature: range: 0..1"}}`))),
		Header:     http.Header{},
	}
	account := &Account{ID: 100, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}

	_, err := svc.handleErrorResponse(context.Background(), resp, c, account)
	require.Error(t, err)
	assert.True(t, IsResponseCommitted(c), "non-failover error path must mark response committed")
	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
}

func TestHandleErrorResponse_PassthroughRuleSetsCommitted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	ruleSvc := &ErrorPassthroughService{}
	ruleSvc.setLocalCache([]*model.ErrorPassthroughRule{
		newNonFailoverPassthroughRule(http.StatusBadRequest, "temperature", http.StatusBadRequest, "参数错误"),
	})
	BindErrorPassthroughService(c, ruleSvc)

	svc := &GatewayService{}
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"message":"temperature: range: 0..1"}}`))),
		Header:     http.Header{},
	}
	account := &Account{ID: 200, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}

	_, err := svc.handleErrorResponse(context.Background(), resp, c, account)
	require.Error(t, err)
	assert.True(t, IsResponseCommitted(c), "passthrough rule path must mark response committed")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	errField, ok := payload["error"].(map[string]any)
	require.True(t, ok, "payload[\"error\"] should be map[string]any")
	assert.Equal(t, "参数错误", errField["message"])
}

func TestOpenAIHandleErrorResponse_SetsResponseCommitted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	svc := &OpenAIGatewayService{}
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"message":"rate limit exceeded"}}`))),
		Header:     http.Header{},
	}
	account := &Account{ID: 101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	_, err := svc.handleErrorResponse(context.Background(), resp, c, account, nil)
	require.Error(t, err)
	assert.True(t, IsResponseCommitted(c), "OpenAI non-failover path must mark response committed")
}

func TestGeminiWriteGeminiMappedError_SetsResponseCommitted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	svc := &GeminiMessagesCompatService{}
	body := []byte(`{"error":{"message":"invalid field"}}`)
	account := &Account{ID: 102, Platform: PlatformGemini, Type: AccountTypeAPIKey}

	err := svc.writeGeminiMappedError(c, account, http.StatusBadRequest, "req-99", body)
	require.Error(t, err)
	assert.True(t, IsResponseCommitted(c), "Gemini path must mark response committed")
}

func newNonFailoverPassthroughRule(statusCode int, keyword string, respCode int, customMessage string) *model.ErrorPassthroughRule {
	return &model.ErrorPassthroughRule{
		ID:              1,
		Name:            "non-failover-rule",
		Enabled:         true,
		Priority:        1,
		ErrorCodes:      []int{statusCode},
		Keywords:        []string{keyword},
		MatchMode:       model.MatchModeAll,
		PassthroughCode: false,
		ResponseCode:    &respCode,
		PassthroughBody: false,
		CustomMessage:   &customMessage,
	}
}
