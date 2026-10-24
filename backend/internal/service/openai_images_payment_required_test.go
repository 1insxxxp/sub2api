//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRateLimitServiceImagePaymentRequiredPreservesScheduling(t *testing.T) {
	imageCtx := WithOpenAIImagesEndpoint(WithOpenAIImageGenerationIntent(context.Background()))
	for _, tc := range []struct {
		name        string
		ctx         context.Context
		status      int
		body        string
		wantDisable bool
	}{
		{name: "Anlas exhausted", ctx: imageCtx, status: 402, body: `{"error":{"message":"Not enough Anlas. Required 26, Available 2"}}`},
		{name: "plain payment rejection", ctx: imageCtx, status: 402, body: `Payment required`},
		{name: "text payment rejection", ctx: context.Background(), status: 402, body: `{"error":{"message":"insufficient balance"}}`, wantDisable: true},
		{name: "intent alone is not image endpoint", ctx: WithOpenAIImageGenerationIntent(context.Background()), status: 402, body: `{"error":{"message":"insufficient balance"}}`, wantDisable: true},
		{name: "workspace deactivated", ctx: imageCtx, status: 402, body: `{"detail":{"code":"deactivated_workspace"}}`, wantDisable: true},
		{name: "organization deactivated", ctx: imageCtx, status: 402, body: `{"error":{"code":"organization_deactivated"}}`, wantDisable: true},
		{name: "account disabled", ctx: imageCtx, status: 402, body: `{"code":"account_disabled"}`, wantDisable: true},
		{name: "invalid credential", ctx: imageCtx, status: 401, body: `{"error":{"message":"invalid api key"}}`, wantDisable: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &errorPolicyRepoStub{}
			svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			account := newOpenAIImagesAPIKeyAccount()
			account.Status, account.Schedulable = StatusActive, true

			disabled := svc.HandleUpstreamError(tc.ctx, account, tc.status, http.Header{}, []byte(tc.body), "gpt-image-2")

			require.Equal(t, tc.wantDisable, disabled)
			if tc.wantDisable {
				require.Equal(t, 1, repo.setErrCalls)
			} else {
				require.Zero(t, repo.setErrCalls)
			}
			require.Zero(t, repo.tempCalls)
			require.Empty(t, repo.modelRateLimitCalls)
		})
	}
}

func TestOpenAIImagesPaymentRequiredKeepsExplicitTemporaryPolicy(t *testing.T) {
	for _, model := range []string{"", "gpt-image-2"} {
		t.Run("model="+model, func(t *testing.T) {
			repo := &errorPolicyRepoStub{}
			svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			account := newOpenAIImagesAPIKeyAccount()
			account.Credentials["temp_unschedulable_enabled"] = true
			account.Credentials["temp_unschedulable_rules"] = []any{map[string]any{
				"error_code": float64(402), "keywords": []any{"Anlas"}, "duration_minutes": float64(5),
			}}
			disabled := svc.HandleUpstreamError(WithOpenAIImagesEndpoint(context.Background()), account,
				http.StatusPaymentRequired, http.Header{}, []byte(`{"error":{"message":"Not enough Anlas"}}`), model)
			require.True(t, disabled)
			require.Zero(t, repo.setErrCalls)
			if model == "" {
				require.Equal(t, 1, repo.tempCalls)
				require.Empty(t, repo.modelRateLimitCalls)
			} else {
				require.Zero(t, repo.tempCalls)
				require.Len(t, repo.modelRateLimitCalls, 1)
				require.Equal(t, model, repo.modelRateLimitCalls[0].scope)
			}
		})
	}
}

func TestForwardImagesPaymentRequiredAllowsFailoverWithoutDisablingAccount(t *testing.T) {
	for _, native := range []bool{false, true} {
		name := "OpenAI compatible"
		if native {
			name = "native NovelAI"
		}
		t.Run(name, func(t *testing.T) {
			repo := &errorPolicyRepoStub{}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusPaymentRequired,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Not enough Anlas. Required 26, Available 2"}}`)),
			}}
			svc := newOpenAIImagesTestService(upstream)
			svc.accountRepo = repo
			svc.rateLimitService = NewRateLimitService(repo, nil, svc.cfg, nil, nil)
			svc.rateLimitService.SetAccountRuntimeBlocker(svc)
			account := newOpenAIImagesAPIKeyAccount()
			account.Status, account.Schedulable = StatusActive, true
			body := []byte(`{"model":"gpt-image-2","prompt":"blue square"}`)
			if native {
				body = []byte(`{"model":"nai-diffusion-5-full","input":"blue square","parameters":{}}`)
				account.Credentials["image_protocol"] = "novelai"
			}
			c, _ := newOpenAIImagesTestContext(t, body)
			ctx := WithOpenAIImagesEndpoint(WithOpenAIImageGenerationIntent(c.Request.Context()))
			c.Request = c.Request.WithContext(ctx)
			var err error
			if native {
				var request *NovelAIImageRequest
				request, err = ParseNovelAIImageRequest(body)
				require.NoError(t, err)
				_, err = svc.ForwardNovelAI(ctx, c, account, request, "")
			} else {
				var parsed *OpenAIImagesRequest
				parsed, err = svc.ParseOpenAIImagesRequest(c, body)
				require.NoError(t, err)
				_, err = svc.ForwardImages(ctx, c, account, body, parsed, "")
			}
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, http.StatusPaymentRequired, failoverErr.StatusCode)
			require.True(t, failoverErr.ShouldRetryNextAccount())
			require.False(t, failoverErr.RetryableOnSameAccount)
			require.False(t, c.Writer.Written(), "failover must not commit an error response")
			require.Zero(t, repo.setErrCalls)
			require.Zero(t, repo.tempCalls)
			require.Empty(t, repo.modelRateLimitCalls)
			require.Equal(t, StatusActive, account.Status)
			require.True(t, account.Schedulable)
			require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
		})
	}
}
