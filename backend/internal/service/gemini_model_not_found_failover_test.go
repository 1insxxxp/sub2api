//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const geminiUnsupportedModelBody = `{"error":{"code":404,"message":"Model \"gemini-3.1-flash-lite\" is not supported by any configured account in this group","status":"NOT_FOUND"}}`

func TestGeminiModelNotFoundFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, fw := range geminiSingleCheckForwarders {
		for _, mode := range []string{"ordinary", "pool", "custom_excluded", "custom_matched"} {
			t.Run(fw.name+"/"+mode, func(t *testing.T) {
				svc, upstream, repo := newGeminiSingleCheckService(geminiSequenceUpstreamResponse{
					status: http.StatusNotFound, body: geminiUnsupportedModelBody,
				})
				account := geminiDefaultPolicyAPIKeyAccount()
				account.Credentials["model_mapping"] = map[string]any{
					"gemini-2.5-flash":      "gemini-3.1-flash-lite",
					"gemini-3.1-flash-lite": "do-not-map-twice",
				}
				if mode == "pool" {
					account.Credentials["pool_mode"] = true
					account.Credentials["pool_mode_retry_status_codes"] = []any{float64(http.StatusNotFound)}
				}
				if mode == "custom_excluded" || mode == "custom_matched" {
					account.Credentials["custom_error_codes_enabled"] = true
					code := http.StatusTooManyRequests
					if mode == "custom_matched" {
						code = http.StatusNotFound
					}
					account.Credentials["custom_error_codes"] = []any{float64(code)}
				}

				err, c := fw.forward(t, svc, account)

				var failoverErr *UpstreamFailoverError
				require.ErrorAs(t, err, &failoverErr)
				require.Equal(t, http.StatusNotFound, failoverErr.StatusCode)
				require.JSONEq(t, geminiUnsupportedModelBody, string(failoverErr.ResponseBody))
				require.True(t, failoverErr.ShouldRetryNextAccount())
				require.False(t, failoverErr.RetryableOnSameAccount)
				require.False(t, failoverErr.PreferNextAccount, "normal traversal budget must still apply")
				require.Equal(t, 1, upstream.calls, "missing models must not retry the same account")
				require.False(t, c.Writer.Written(), "the next account must still be able to respond")
				require.Zero(t, repo.setErrCalls, "do not disable the entire account")
				require.Zero(t, repo.tempCalls)
				if mode == "custom_excluded" {
					require.Empty(t, repo.modelRateLimitCalls, "custom-code exclusions retain control of persisted cooldowns")
				} else {
					require.Len(t, repo.modelRateLimitCalls, 1)
					require.Equal(t, "gemini-3.1-flash-lite", repo.modelRateLimitCalls[0].scope)
					require.Equal(t, upstreamModelNotFoundReason, repo.modelRateLimitCalls[0].reason)
					require.WithinDuration(t, time.Now().Add(upstreamModelNotFoundCooldown), repo.modelRateLimitCalls[0].resetAt, time.Second)
				}
				events := c.MustGet(OpsUpstreamErrorsKey).([]*OpsUpstreamErrorEvent)
				require.Len(t, events, 1)
				require.Equal(t, "failover", events[0].Kind)
				require.Equal(t, account.ID, events[0].AccountID)
				require.Equal(t, http.StatusNotFound, events[0].UpstreamStatusCode)
			})
		}
	}
}

func TestGeminiModelNotFoundUsesModelCooldownBeforeAccountRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, fw := range geminiSingleCheckForwarders {
		t.Run(fw.name, func(t *testing.T) {
			svc, upstream, repo := newGeminiSingleCheckService(geminiSequenceUpstreamResponse{status: http.StatusNotFound, body: geminiUnsupportedModelBody})
			account := geminiAPIKeyAccountWithTempRule(map[string]any{
				"error_code":       float64(http.StatusNotFound),
				"keywords":         []any{"not supported"},
				"duration_minutes": float64(10),
				"scope":            TempUnschedScopeAccount,
			})

			err, c := fw.forward(t, svc, account)

			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Zero(t, repo.tempCalls, "a missing model must not pause other models on this account")
			require.Len(t, repo.modelRateLimitCalls, 1)
			require.Equal(t, 1, upstream.calls)
			require.False(t, c.Writer.Written())
		})
	}
}

func TestGeminiModelNotFoundPreservesBoundCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream, repo := newGeminiSingleCheckService(geminiSequenceUpstreamResponse{status: http.StatusNotFound, body: geminiUnsupportedModelBody})
	c, _ := newGeminiNativeTestContext(t)
	ctx := WithGeminiCachedContentBound(context.Background())

	_, err := svc.ForwardNative(ctx, c, geminiDefaultPolicyAPIKeyAccount(), "gemini-2.5-flash", "generateContent", false,
		[]byte(geminiCachedContentRequestBody))

	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	require.Empty(t, repo.modelRateLimitCalls, "a cache binding mismatch must not cool normal model requests")
	require.Equal(t, 1, upstream.calls)
}

func TestGeminiModelNotFoundPreservesTerminalErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, fw := range geminiSingleCheckForwarders {
		for _, tc := range []struct {
			name   string
			status int
			body   string
		}{
			{"endpoint_missing", http.StatusNotFound, `{"error":{"message":"endpoint not found"}}`},
			{"cached_content_missing", http.StatusNotFound, geminiVertexCachedContentNotFoundBody},
			{"bad_parameter", http.StatusBadRequest, `{"error":{"message":"temperature is invalid"}}`},
		} {
			t.Run(fw.name+"/"+tc.name, func(t *testing.T) {
				svc, upstream, repo := newGeminiSingleCheckService(geminiSequenceUpstreamResponse{status: tc.status, body: tc.body})

				err, c := fw.forward(t, svc, geminiDefaultPolicyAPIKeyAccount())

				require.Error(t, err)
				var failoverErr *UpstreamFailoverError
				require.False(t, errors.As(err, &failoverErr))
				require.Equal(t, 1, upstream.calls)
				require.True(t, c.Writer.Written())
				require.Empty(t, repo.modelRateLimitCalls)
				require.Zero(t, repo.setErrCalls)
			})
		}
	}
}

func TestGeminiModelNotFoundFailoverWithoutCooldownStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, fw := range geminiSingleCheckForwarders {
		for _, unavailable := range []string{"no_service", "write_failed"} {
			t.Run(fw.name+"/"+unavailable, func(t *testing.T) {
				svc, upstream, _ := newGeminiSingleCheckService(geminiSequenceUpstreamResponse{status: http.StatusNotFound, body: geminiUnsupportedModelBody})
				if unavailable == "no_service" {
					svc.rateLimitService = nil
				} else {
					svc.rateLimitService.accountRepo = &modelNotFoundAccountRepoStub{modelRateLimitErr: errors.New("storage unavailable")}
				}

				err, c := fw.forward(t, svc, geminiDefaultPolicyAPIKeyAccount())

				var failoverErr *UpstreamFailoverError
				require.ErrorAs(t, err, &failoverErr)
				require.True(t, failoverErr.ShouldRetryNextAccount())
				require.False(t, failoverErr.RetryableOnSameAccount)
				require.Equal(t, 1, upstream.calls)
				require.False(t, c.Writer.Written())
			})
		}
	}
}

func TestGeminiModelNotFoundFailoverVertex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream, repo := newGeminiSingleCheckService(geminiSequenceUpstreamResponse{
		status: http.StatusNotFound,
		body:   `{"error":{"code":404,"message":"Publisher Model projects/proj-1/locations/global/publishers/google/models/gemini-3.1-flash-lite was not found.","status":"NOT_FOUND"}}`,
	})
	svc.tokenProvider = NewGeminiTokenProvider(nil, &fakeGeminiTokenCache{token: "ya29.test-token"}, nil)
	account := vertexServiceAccount()
	account.Credentials["model_mapping"] = map[string]any{"gemini-2.5-flash": "gemini-3.1-flash-lite"}
	c, rec := newGeminiNativeTestContext(t)

	_, err := svc.ForwardNative(context.Background(), c, account, "gemini-2.5-flash", "generateContent", false,
		[]byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, 1, upstream.calls)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Equal(t, "gemini-3.1-flash-lite", repo.modelRateLimitCalls[0].scope)
}
