//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type geminiTraversalUpstream struct {
	HTTPUpstream
	calls       int
	firstStatus int
	firstError  error
}

func (s *geminiTraversalUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	s.calls++
	if s.calls == 1 && s.firstError != nil {
		return nil, s.firstError
	}
	status := http.StatusBadRequest
	if s.calls == 1 {
		status = s.firstStatus
	}
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"test upstream failure"}}`)),
	}, nil
}

func callGeminiTraversalPath(t *testing.T, path string, svc *GeminiMessagesCompatService, c *gin.Context, account *Account, model string, stream bool) (*ForwardResult, error) {
	t.Helper()
	if path == "native" {
		action := "generateContent"
		if stream {
			action = "streamGenerateContent"
		}
		return svc.ForwardNative(c.Request.Context(), c, account, model, action, stream, []byte(geminiTransportTestNativeBody))
	}
	body := []byte(`{"model":"` + model + `","messages":[{"role":"user","content":"hi"}],"max_tokens":100,"stream":` + map[bool]string{true: "true", false: "false"}[stream] + `}`)
	if path == "messages" {
		return svc.Forward(c.Request.Context(), c, account, body)
	}
	return svc.ForwardAsChatCompletions(c.Request.Context(), c, account, body)
}

func TestGeminiFirstOutputTraversal_HTTPFailureMovesToNextAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"chat", "messages", "native"} {
		t.Run(path, func(t *testing.T) {
			upstream := &geminiTraversalUpstream{firstStatus: http.StatusBadGateway}
			svc := &GeminiMessagesCompatService{
				httpUpstream:   upstream,
				cfg:            &config.Config{},
				settingService: &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":60,"switch_seconds":60,"hard_cap_seconds":300}}`}},
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+path, nil).WithContext(context.Background())
			account := geminiPoolModeAPIKeyAccount()
			delete(account.Credentials, "pool_mode")

			result, err := callGeminiTraversalPath(t, path, svc, c, account, "gemini-3.1-pro-preview", true)

			require.Equal(t, 1, upstream.calls, "the first failing upstream must yield to the next account before retrying itself")
			require.Nil(t, result)
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
			require.True(t, failoverErr.PreferNextAccount)
			require.False(t, failoverErr.RetryableOnSameAccount)
			require.False(t, failoverErr.SafeToFailoverAfterWrite, "HTTP retry policy must not authorize replay after semantic output")
			require.False(t, c.Writer.Written(), "the handler must retain ownership of the final client response")
		})
	}
}

func TestGeminiFirstOutputTraversal_PoolFailureDoesNotRetrySameAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"chat", "messages", "native"} {
		t.Run(path, func(t *testing.T) {
			upstream := &geminiTraversalUpstream{firstStatus: http.StatusTooManyRequests}
			account := geminiPoolModeAPIKeyAccount()
			svc := &GeminiMessagesCompatService{
				httpUpstream:     upstream,
				cfg:              &config.Config{},
				rateLimitService: NewRateLimitService(&geminiErrorPolicyRepo{}, nil, &config.Config{}, nil, nil),
				settingService:   &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":60,"switch_seconds":60,"hard_cap_seconds":300}}`}},
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+path, nil)

			_, err := callGeminiTraversalPath(t, path, svc, c, account, "gemini-3.1-pro-preview", true)

			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, 1, upstream.calls)
			require.True(t, failoverErr.PreferNextAccount)
			require.False(t, failoverErr.RetryableOnSameAccount, "a pool credential must not consume the first-output traversal on same-account retries")
		})
	}
}

func TestGeminiFirstOutputTraversal_PreservesUnprotectedRetryAndBadRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"chat", "messages", "native"} {
		for _, tc := range []struct {
			name      string
			enabled   bool
			stream    bool
			model     string
			status    int
			wantCalls int
		}{
			{"disabled", false, true, "gemini-3.1-pro-preview", 502, 2},
			{"non_streaming", true, false, "gemini-3.1-pro-preview", 502, 2},
			{"image", true, true, "gemini-3-pro-image-preview", 502, 2},
			{"invalid_request", true, true, "gemini-3.1-pro-preview", 400, 1},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				settings := `{"enabled":` + map[bool]string{true: "true", false: "false"}[tc.enabled] + `,"default":{"enabled":true,"target_seconds":60,"switch_seconds":60,"hard_cap_seconds":300}}`
				upstream := &geminiTraversalUpstream{firstStatus: tc.status}
				svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}, settingService: &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: settings}}}
				account := geminiPoolModeAPIKeyAccount()
				delete(account.Credentials, "pool_mode")
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+path, nil)
				_, err := callGeminiTraversalPath(t, path, svc, c, account, tc.model, tc.stream)
				require.Equal(t, tc.wantCalls, upstream.calls)
				var failoverErr *UpstreamFailoverError
				require.False(t, errors.As(err, &failoverErr), "the terminal 400 must remain a client error")
			})
		}
	}
}

type geminiTraversalRefreshExecutor struct{ OAuthRefreshExecutor }

func (geminiTraversalRefreshExecutor) CacheKey(*Account) string { return "gemini-traversal-test" }

func TestGeminiFirstOutputTraversal_CredentialWaitTimeoutMovesToNextAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"chat", "messages", "native"} {
		t.Run(path, func(t *testing.T) {
			api := NewOAuthRefreshAPI(&geminiErrorPolicyRepo{}, nil)
			lock := api.getLocalLock("gemini-traversal-test")
			require.NoError(t, lock.Lock(context.Background()))
			defer lock.Unlock()
			upstream := &geminiTraversalUpstream{firstStatus: 502}
			svc := &GeminiMessagesCompatService{
				httpUpstream: upstream, cfg: &config.Config{},
				settingService: &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":1,"switch_seconds":1,"hard_cap_seconds":5}}`}},
				tokenProvider:  &GeminiTokenProvider{refreshAPI: api, executor: geminiTraversalRefreshExecutor{}, refreshPolicy: ProviderRefreshPolicy{OnRefreshError: ProviderRefreshErrorReturn}},
			}
			account := &Account{ID: 701, Platform: PlatformGemini, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test-token"}}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+path, nil)
			started := time.Now()
			_, err := callGeminiTraversalPath(t, path, svc, c, account, "gemini-3.1-pro-preview", true)
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr, "first-output timeout while waiting for credentials must release this account for failover")
			require.Equal(t, GatewayFailureReason("first_output_timeout"), failoverErr.Reason)
			require.True(t, failoverErr.PreferNextAccount)
			require.Zero(t, upstream.calls)
			require.Less(t, time.Since(started), 3*time.Second)
			require.NoError(t, c.Request.Context().Err(), "only the timed-out account attempt is canceled")
		})
	}
}

func TestPreferNextAccountBeforeFirstOutput_DoesNotReplaySemanticOutput(t *testing.T) {
	guard := &modelFirstOutputGuard{}
	guard.Stop()
	upstreamErr := &UpstreamFailoverError{StatusCode: 502, RetryableOnSameAccount: true}
	require.Same(t, upstreamErr, preferNextAccountBeforeFirstOutput(upstreamErr, guard))
	require.False(t, upstreamErr.PreferNextAccount)
	require.True(t, upstreamErr.RetryableOnSameAccount)
	require.False(t, upstreamErr.SafeToFailoverAfterWrite)
}

func TestGeminiFirstOutputTraversal_TransportFailurePrefersNextAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"chat", "messages", "native"} {
		t.Run(path, func(t *testing.T) {
			upstream := &geminiTraversalUpstream{firstError: io.ErrUnexpectedEOF}
			svc := &GeminiMessagesCompatService{
				httpUpstream: upstream, cfg: &config.Config{},
				settingService: &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":60,"switch_seconds":60,"hard_cap_seconds":300}}`}},
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+path, nil)
			_, err := callGeminiTraversalPath(t, path, svc, c, geminiPoolModeAPIKeyAccount(), "gemini-3.1-pro-preview", true)
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.True(t, failoverErr.PreferNextAccount)
			require.False(t, failoverErr.RetryableOnSameAccount)
			require.False(t, failoverErr.SafeToFailoverAfterWrite)
			require.Equal(t, 1, upstream.calls)
			require.False(t, c.Writer.Written())
		})
	}
}

type geminiTraversalBlockingTokenCache struct{ GeminiTokenCache }

func (geminiTraversalBlockingTokenCache) GetAccessToken(ctx context.Context, _ string) (string, error) {
	<-ctx.Done()
	return "late-cached-token", nil
}

func TestGeminiFirstOutputTraversal_LateCredentialDoesNotSendUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"chat", "messages", "native"} {
		t.Run(path, func(t *testing.T) {
			upstream := &geminiTraversalUpstream{firstStatus: http.StatusBadGateway}
			svc := &GeminiMessagesCompatService{
				httpUpstream: upstream, cfg: &config.Config{},
				settingService: &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":1,"switch_seconds":1,"hard_cap_seconds":5}}`}},
				tokenProvider:  &GeminiTokenProvider{tokenCache: geminiTraversalBlockingTokenCache{}},
			}
			account := &Account{ID: 702, Platform: PlatformGemini, Type: AccountTypeOAuth}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+path, nil)
			_, err := callGeminiTraversalPath(t, path, svc, c, account, "gemini-3.1-pro-preview", true)
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, GatewayFailureReason("first_output_timeout"), failoverErr.Reason)
			require.Zero(t, upstream.calls, "expired credential waits must not start a new HTTP attempt")
		})
	}
}

func TestGeminiFirstOutputTraversal_MappedImageReleasesTextBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"chat", "messages", "native"} {
		t.Run(path, func(t *testing.T) {
			upstream := &geminiTraversalUpstream{firstStatus: http.StatusBadRequest}
			svc := &GeminiMessagesCompatService{
				httpUpstream: upstream, cfg: &config.Config{},
				settingService: &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":1,"switch_seconds":1,"hard_cap_seconds":5}}`}},
			}
			account := geminiPoolModeAPIKeyAccount()
			delete(account.Credentials, "pool_mode")
			account.Credentials["model_mapping"] = map[string]any{"gemini-3.1-pro-preview": "gemini-3-pro-image-preview"}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+path, nil)
			closeTraversal := startModelFirstOutputTraversal(c, time.Now().Add(time.Hour))
			defer closeTraversal()
			state := modelFirstOutputTraversalFromContext(c.Request.Context())

			_, err := callGeminiTraversalPath(t, path, svc, c, account, "gemini-3.1-pro-preview", true)

			require.Error(t, err)
			state.mu.Lock()
			active := state.active
			state.mu.Unlock()
			require.False(t, active, "an image alias must not retain the text traversal cancellation timer")
		})
	}
}

func TestGeminiFirstOutputTraversal_ErrorAfterHeartbeatKeepsSSEEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GeminiMessagesCompatService{cfg: &config.Config{}}
	cases := []struct {
		name     string
		messages bool
		write    func(*gin.Context) error
	}{
		{"chat", false, func(c *gin.Context) error {
			return svc.writeChatCompletionsError(c, 400, "invalid_request_error", "request rejected")
		}},
		{"messages", true, func(c *gin.Context) error {
			return svc.writeClaudeError(c, 400, "invalid_request_error", "request rejected")
		}},
		{"google", false, func(c *gin.Context) error { return svc.writeGoogleError(c, 400, "request rejected") }},
		{"mapped_messages", true, func(c *gin.Context) error {
			return svc.writeGeminiMappedError(c, geminiPoolModeAPIKeyAccount(), 400, "request-test", []byte(`{"error":{"message":"request rejected"}}`))
		}},
		{"native_upstream", false, func(c *gin.Context) error {
			return svc.writeGeminiNativeUpstreamError(c, geminiPoolModeAPIKeyAccount(), &http.Response{StatusCode: 400, Header: make(http.Header)}, []byte("{\n\"error\":{\"code\":400,\"message\":\"request rejected\"}\n}"), "request-test", false)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/test", nil)
			closeKeepalive := startModelFirstOutputKeepalive(c, time.Hour)
			defer closeKeepalive()
			require.True(t, modelFirstOutputKeepaliveStateFromContext(c).active.beat())
			err := tc.write(c)
			require.Error(t, err)
			require.Equal(t, http.StatusOK, rec.Code, "already-sent heartbeat status cannot be replaced")
			require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
			body := strings.TrimPrefix(rec.Body.String(), ": keepalive\n\n")
			if tc.messages {
				require.True(t, strings.HasPrefix(body, "event: error\n"), body)
				body = strings.TrimPrefix(body, "event: error\n")
			}
			require.True(t, strings.HasPrefix(body, "data: "), body)
			require.True(t, strings.HasSuffix(body, "\n\n"), body)
			payload := strings.TrimSuffix(strings.TrimPrefix(body, "data: "), "\n\n")
			require.NotContains(t, payload, "\n", "upstream pretty JSON must be encoded as a single SSE data line")
			var parsed map[string]any
			require.NoError(t, json.Unmarshal([]byte(payload), &parsed))
			require.Contains(t, parsed, "error")
		})
	}
}
