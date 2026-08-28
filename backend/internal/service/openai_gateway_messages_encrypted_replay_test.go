//go:build unit

package service

import (
	"bytes"
	"context"
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

func TestForwardAsAnthropic_InvalidEncryptedContentStripsSignatureAndRetries(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{
		"model":"gpt-5.6-sol",
		"max_tokens":128,
		"stream":false,
		"messages":[
			{"role":"user","content":"first"},
			{"role":"assistant","content":[
				{"type":"thinking","thinking":"private reasoning","signature":"sig_foreign_account"},
				{"type":"text","text":"previous answer"}
			]},
			{"role":"user","content":"continue"}
		]
	}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	bindPassthroughRule(c, PlatformOpenAI, []string{"encrypted content"}, http.StatusBadRequest)

	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"error":{"code":"invalid_encrypted_content","message":"The encrypted content sig_... could not be verified.","type":"invalid_request_error"}}`,
			)),
		},
		openAICompatSSECompletedResponse("resp_recovered", "gpt-5.6-sol"),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := rawGPT56ResponsesOAuthAccount("gpt-5.6-sol", "gpt-5.6-sol")

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "gpt-5.6-sol")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_recovered", result.ResponseID)
	require.Len(t, upstream.bodies, 2)
	require.Equal(t, "sig_foreign_account", gjson.GetBytes(upstream.bodies[0], `input.#(type=="reasoning").encrypted_content`).String())
	require.False(t, gjson.GetBytes(upstream.bodies[1], `input.#(type=="reasoning").encrypted_content`).Exists())
	require.Contains(t, string(upstream.bodies[1]), "previous answer")
	require.Contains(t, string(upstream.bodies[1]), "continue")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "resp_recovered", gjson.Get(rec.Body.String(), "id").String())
	require.NotContains(t, rec.Body.String(), "invalid_encrypted_content", "the recoverable upstream 400 must not be committed to the client")
}
