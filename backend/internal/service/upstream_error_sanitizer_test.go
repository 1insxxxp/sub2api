package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestUpstreamErrorPrivacyMessage(t *testing.T) {
	tests := []struct {
		name, message string
		secrets       []string
	}{
		{"email", "Invalid request for owner+billing@example.invalid: max_tokens must exceed thinking.budget_tokens", []string{"owner+billing@example.invalid"}},
		{"bearer", "Upstream rejected Bearer opaqueAuditCredential123; retry later", []string{"opaqueAuditCredential123"}},
		{"basic", "Authentication failed: Basic dXNlcjpwYXNzd29yZA==", []string{"dXNlcjpwYXNzd29yZA=="}},
		{"keys", "Rejected sk-ant-api03-auditSecret123 and sk-proj-secondSecret456", []string{"sk-ant-api03-auditSecret123", "sk-proj-secondSecret456"}},
		{"jwt", "Invalid token eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhdWRpdCJ9.auditSignature123", []string{"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhdWRpdCJ9.auditSignature123"}},
		{"named credential", "api_key=opaqueAuditSecret; access_token: 'anotherAuditSecret'", []string{"opaqueAuditSecret", "anotherAuditSecret"}},
		{"unprefixed token", "Authentication rejected: token=opaqueAuditSecret", []string{"opaqueAuditSecret"}},
		{"cookie header", "Cookie: session=opaqueAuditSecret; custom_auth=anotherAuditSecret", []string{"opaqueAuditSecret", "anotherAuditSecret"}},
		{"escaped password", `Invalid max_tokens; password="FIRST_SECRET\"SECOND_SECRET"; retry`, []string{"FIRST_SECRET", "SECOND_SECRET"}},
		{"unterminated password", `Invalid max_tokens; password="FIRST_SECRET\"SECOND_SECRET`, []string{"FIRST_SECRET", "SECOND_SECRET"}},
		{"AWS authorization", "Authorization: AWS4-HMAC-SHA256 Credential=FIRST_SECRET, SignedHeaders=host;x-amz-date, Signature=SECOND_SECRET", []string{"FIRST_SECRET", "SECOND_SECRET"}},
		{"proxy authorization", "Proxy-Authorization: CustomAlgorithm Credential=FIRST_SECRET, Signature=SECOND_SECRET", []string{"FIRST_SECRET", "SECOND_SECRET"}},
		{"whole url", "See https://upstream.invalid/customer/private-path?auth=opaqueAuditSecret for details", []string{"upstream.invalid", "private-path", "opaqueAuditSecret"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeUpstreamErrorMessage(tt.message)
			for _, secret := range tt.secrets {
				require.NotContains(t, got, secret)
			}
			require.Equal(t, got, sanitizeUpstreamErrorMessage(got), "sanitization must be idempotent")
		})
	}
	diagnostic := "max_tokens must be greater than thinking.budget_tokens; invalid input[8].tools[1].parameters"
	require.Equal(t, diagnostic, sanitizeUpstreamErrorMessage(diagnostic))
}

func TestUpstreamErrorPrivacyTruncatedPayload(t *testing.T) {
	diagnostic := "invalid_schema: max_tokens must exceed thinking.budget_tokens; "
	for _, payload := range []string{
		`request_body={"messages":[{"content":"PRIVATE_REQUEST`,
		`requestBody: {"messages":[{"content":"PRIVATE_REQUEST`,
		`"messages": [{"content":"PRIVATE_REQUEST`,
		`prompt="PRIVATE_REQUEST`,
		`input: "PRIVATE_REQUEST`,
		`request_headers={"Custom-Auth":"PRIVATE_REQUEST`,
		`messages=[{"content":"PRIVATE_REQUEST"}]; param=max_tokens`,
		`prompt="PRIVATE_REQUEST"; param=max_tokens`,
	} {
		t.Run(payload[:strings.IndexAny(payload, ":=")], func(t *testing.T) {
			raw := diagnostic + payload
			for _, got := range []string{sanitizeUpstreamErrorMessage(raw), string(sanitizeUpstreamErrorBody([]byte(raw)))} {
				require.NotContains(t, got, "PRIVATE_REQUEST")
				require.Contains(t, got, diagnostic)
				if strings.HasSuffix(payload, "; param=max_tokens") {
					require.Contains(t, got, "; param=max_tokens")
				}
				require.Equal(t, got, sanitizeUpstreamErrorMessage(got))
			}
		})
	}
	for _, diagnostic := range []string{
		"max_tokens must be greater than thinking.budget_tokens; invalid input[8].tools[1].parameters",
		"invalid_schema; input: expected a string; messages: expected an array",
	} {
		require.Equal(t, diagnostic, sanitizeUpstreamErrorMessage(diagnostic))
	}
}

func TestUpstreamErrorPrivacyBody(t *testing.T) {
	raw := `{"error":{"type":"invalid_request_error","code":"invalid_schema","param":"messages[0].content","message":"Invalid schema for owner@example.invalid; Bearer opaqueAuditCredential123","api_key":"opaqueAuditKey","headers":{"Authorization":"Basic opaqueAuditBasic","Cookie":"opaqueAuditCookie"},"request_body":{"messages":[{"content":"PRIVATE_REQUEST_BODY"}]}},"prompt":"PRIVATE_PROMPT","usage":{"input_tokens":6,"output_tokens":0}}`
	got := sanitizeUpstreamErrorBody([]byte(raw))
	for _, secret := range []string{"owner@example.invalid", "opaqueAuditCredential123", "opaqueAuditKey", "opaqueAuditBasic", "opaqueAuditCookie", "PRIVATE_REQUEST_BODY", "PRIVATE_PROMPT"} {
		require.NotContains(t, string(got), secret)
	}
	require.True(t, json.Valid(got))
	require.Equal(t, "invalid_request_error", gjson.GetBytes(got, "error.type").String())
	require.Equal(t, "invalid_schema", gjson.GetBytes(got, "error.code").String())
	require.Equal(t, "messages[0].content", gjson.GetBytes(got, "error.param").String())
	require.Contains(t, gjson.GetBytes(got, "error.message").String(), "Invalid schema")
	require.EqualValues(t, 6, gjson.GetBytes(got, "usage.input_tokens").Int())
	require.Equal(t, string(got), string(sanitizeUpstreamErrorBody(got)))
}

func TestUpstreamErrorPrivacyEncodedAndSSEBody(t *testing.T) {
	tests := []struct {
		name, raw string
	}{
		{"camel case request", `{"error":{"code":"invalid_schema","requestBody":{"input":"PRIVATE_AUDIT_INPUT"},"key":"opaqueAuditKey"}}`},
		{"echoed headers", `{"error":{"code":"invalid_schema","headers":{"X-Customer-Name":"PRIVATE_AUDIT_INPUT","Custom-Auth":"opaqueAuditKey"}}}`},
		{"embedded json message", `{"error":{"message":"invalid_schema: {\"api_key\":\"opaqueAuditKey\"}"}}`},
		{"embedded request json message", `{"error":{"message":"invalid_schema: {\"request_body\":{\"prompt\":\"PRIVATE_AUDIT_INPUT\"},\"api_key\":\"opaqueAuditKey\"}"}}`},
		{"SSE error", "event: error\ndata: {\"error\":{\"code\":\"invalid_schema\",\"api_key\":\"opaqueAuditKey\",\"request_body\":{\"input\":\"PRIVATE_AUDIT_INPUT\"}}}\n\n"},
		{"multiline SSE error", "event: error\ndata: {\"error\":{\"code\":\"invalid_schema\",\ndata: \"api_key\":\"opaqueAuditKey\",\"request_body\":{\"input\":\"PRIVATE_AUDIT_INPUT\"}}}\n\n"},
		{"SSE comment", ": api_key=opaqueAuditKey\nevent: error\ndata: {\"error\":{\"code\":\"invalid_schema\"}}\n\n"},
		{"SSE separate comment frame", ": api_key=opaqueAuditKey\n\nevent: error\ndata: {\"error\":{\"code\":\"invalid_schema\"}}\n\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(sanitizeUpstreamErrorBody([]byte(tt.raw)))
			require.NotContains(t, got, "opaqueAuditKey")
			require.NotContains(t, got, "PRIVATE_AUDIT_INPUT")
			require.Contains(t, got, "invalid_schema")
			require.Equal(t, got, string(sanitizeUpstreamErrorBody([]byte(got))))
		})
	}
}

func TestUpstreamErrorPrivacyGateway400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	response := &http.Response{StatusCode: http.StatusBadRequest, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"max_tokens must exceed thinking.budget_tokens for owner@example.invalid; sk-auditSecret123","api_key":"opaqueAuditKey"}}`))}
	_, err := (&GatewayService{}).handleErrorResponse(context.Background(), response, c, &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey})
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "max_tokens must exceed thinking.budget_tokens")
	for _, secret := range []string{"owner@example.invalid", "sk-auditSecret123", "opaqueAuditKey"} {
		require.NotContains(t, rec.Body.String(), secret)
	}
}

func TestSanitizeUpstreamErrorBodyMasksUpstreamAddresses(t *testing.T) {
	body := []byte(`{
		"error":{
			"code":"not_found",
			"message":"[openclawroot.com] 请求的模型或接口不存在，请检查模型名称和请求路径 https://openclawroot.com/v1/embeddings?key=secret",
			"type":"api_error"
		}
	}`)

	sanitized := sanitizeUpstreamErrorBody(body)

	require.NotContains(t, string(sanitized), "openclawroot.com")
	require.Contains(t, string(sanitized), "[upstream]")
	require.Contains(t, string(sanitized), "[upstream-url]")
	require.NotContains(t, string(sanitized), "key=secret")
	require.Equal(t, "not_found", gjson.GetBytes(sanitized, "error.code").String())
}

func TestSanitizeUpstreamErrorBodyPreservesUnchangedJSONBytes(t *testing.T) {
	body := []byte(`{"type":"response.failed","response":{"status":"failed","error":{"code":"content_policy","message":"request blocked by policy"},"usage":{"input_tokens":6,"output_tokens":0,"total_tokens":6}}}`)

	require.Equal(t, string(body), string(sanitizeUpstreamErrorBody(body)))
}

func TestSanitizeUpstreamErrorBodyMasksNestedAndPlainTextAddresses(t *testing.T) {
	body := []byte(`{
		"error": {
			"message": "request failed at https://relay.example:8443/v1/responses?access_token=secret",
			"details": [{"endpoint": "[10.20.30.40:9443]"}]
		},
		"message": "see https://relay.example:8443/debug"
	}`)

	sanitized := sanitizeUpstreamErrorBody(body)

	require.NotContains(t, string(sanitized), "relay.example")
	require.NotContains(t, string(sanitized), "10.20.30.40")
	require.NotContains(t, string(sanitized), "access_token=secret")
	require.Contains(t, string(sanitized), "[upstream-url]")
	require.Contains(t, string(sanitized), "[upstream]")

	plainText := sanitizeUpstreamErrorBody([]byte("upstream https://relay.example:8443/v1 failed"))
	require.NotContains(t, string(plainText), "relay.example")
	require.Contains(t, string(plainText), "[upstream-url]")
}

func TestSanitizeErrorBodyForStorageMasksUpstreamAddresses(t *testing.T) {
	raw := `{"error":{"message":"provider failed at https://relay.example:8443/v1","api_key":"secret"}}`

	sanitized, _ := sanitizeErrorBodyForStorage(raw, 4096)

	require.NotContains(t, sanitized, "relay.example")
	require.NotContains(t, sanitized, "secret")
	require.Contains(t, sanitized, "[upstream-url]")
}

func TestSanitizeOpenAIResponseFailedEventForClientMasksAddress(t *testing.T) {
	payload := []byte(`{"type":"response.failed","response":{"status":"failed","error":{"code":"upstream_error","message":"blocked by https://relay.example:8443/policy"}}}`)

	sanitized, changed := sanitizeOpenAIResponseFailedEventForClient(payload, "response.failed", false)

	require.True(t, changed)
	require.NotContains(t, string(sanitized), "relay.example")
	require.Contains(t, string(sanitized), "[upstream-url]")
}

func TestTruncateForLogMasksUpstreamAddresses(t *testing.T) {
	logged := truncateForLog([]byte("upstream failed at https://relay.example:8443/v1"), 4096)

	require.NotContains(t, logged, "relay.example")
	require.Contains(t, logged, "[upstream-url]")
}

func TestForwardEmbeddingsSanitizesRawUpstreamErrorBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"按次反重力1/gemini-2.5-pro","input":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"code":"not_found","message":"[openclawroot.com] 请求的模型或接口不存在，请检查模型名称和请求路径","type":"api_error"}}`,
		)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       5439,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://openclawroot.com",
			"model_mapping": map[string]any{
				"按次反重力1/gemini-2.5-pro": "gemini-2.5-pro",
			},
		},
	}

	result, err := svc.ForwardEmbeddings(context.Background(), c, account, body, "")

	require.Error(t, err)
	require.Nil(t, result)
	require.NotContains(t, rec.Body.String(), "openclawroot.com")
	require.Contains(t, rec.Body.String(), "[upstream]")
}
