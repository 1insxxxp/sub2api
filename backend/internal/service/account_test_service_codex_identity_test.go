package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAccountTestService_OpenAIOAuthGPT6InvalidUAPreservesSyncedVersion(t *testing.T) {
	settings := NewSettingService(&codexVersionSettingRepoStub{values: map[string]string{
		SettingKeyOpenAICodexUserAgent:           "admin@example.com",
		SettingKeyOpenAICodexClientVersionSynced: "0.153.4",
	}}, nil)
	SetCodexCanonicalUserAgentResolver(func() string {
		return settings.GetOpenAICodexCanonicalUserAgent(context.Background())
	})
	t.Cleanup(func() { SetCodexCanonicalUserAgentResolver(nil) })
	svc, upstream, ctx := newCodexIdentityAccountProbe()
	account := &Account{
		ID: 90, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}

	require.NoError(t, svc.testOpenAIAccountConnection(ctx, account, "gpt-6-astra", "", ""))
	require.Len(t, upstream.requests, 1)
	req := upstream.lastReq
	require.Equal(t, "0.153.4", req.Header.Get("Version"))
	require.Equal(t, "codex-tui/0.153.4"+codexCLIUserAgentSuffix, req.Header.Get("User-Agent"))
	require.Equal(t, "codex-tui", req.Header.Get("Originator"))
	require.Equal(t, "gpt-6-astra", gjson.GetBytes(upstream.lastBody, "model").String())
}

func newCodexIdentityAccountProbe() (*AccountTestService, *httpUpstreamRecorder, *gin.Context) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/90/test", nil)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\"}\n\n")),
	}}
	return &AccountTestService{httpUpstream: upstream}, upstream, ctx
}
