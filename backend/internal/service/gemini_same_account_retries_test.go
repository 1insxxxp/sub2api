//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	geminiRetryTest503Body       = `{"error":{"code":503,"message":"The model is overloaded.","status":"UNAVAILABLE"}}`
	geminiRetryTestSignatureBody = `{"error":{"code":400,"message":"Function call is missing a thought_signature in functionCall parts.","status":"INVALID_ARGUMENT"}}`
	geminiRetryTestOKBody        = `{"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":1,"totalTokenCount":4}}`
)

func geminiSameAccountRetryTestAccount() *Account {
	return &Account{
		ID:          840,
		Platform:    PlatformGemini,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "test-key"},
	}
}

func TestGeminiForward_SameAccountRetriesFollowConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, retries := range []int{0, 1} {
		for _, fw := range geminiSingleCheckForwarders {
			t.Run(fmt.Sprintf("%s/retries=%d", fw.name, retries), func(t *testing.T) {
				svc, upstream, _ := newGeminiSingleCheckService(geminiSequenceUpstreamResponse{status: http.StatusServiceUnavailable, body: geminiRetryTest503Body})
				svc.cfg.Gateway.MaxSameAccountRetriesGemini = retries

				err, _ := fw.forward(t, svc, geminiSameAccountRetryTestAccount())

				var failoverErr *UpstreamFailoverError
				require.True(t, errors.As(err, &failoverErr), "同号重试耗尽应换号，got %T: %v", err, err)
				require.Equal(t, http.StatusServiceUnavailable, failoverErr.StatusCode)
				require.Equal(t, retries+1, upstream.calls)
			})
		}
	}
}

func TestGeminiClaudeCompat_SignatureDowngradeDoesNotConsumeSameAccountRetries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	forward := func(svc *GeminiMessagesCompatService) (*ForwardResult, error) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		body := []byte(`{"model":"gemini-2.5-flash","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
		return svc.Forward(context.Background(), c, geminiSameAccountRetryTestAccount(), body)
	}

	t.Run("both downgrade stages run without same-account retries", func(t *testing.T) {
		svc, upstream, _ := newGeminiSingleCheckService(
			geminiSequenceUpstreamResponse{status: http.StatusBadRequest, body: geminiRetryTestSignatureBody},
			geminiSequenceUpstreamResponse{status: http.StatusBadRequest, body: geminiRetryTestSignatureBody},
			geminiSequenceUpstreamResponse{status: http.StatusOK, body: geminiRetryTestOKBody},
		)
		svc.cfg.Gateway.MaxSameAccountRetriesGemini = 0

		result, err := forward(svc)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, 3, upstream.calls)
	})

	t.Run("same-account retries are counted apart from downgrades", func(t *testing.T) {
		svc, upstream, _ := newGeminiSingleCheckService(
			geminiSequenceUpstreamResponse{status: http.StatusServiceUnavailable, body: geminiRetryTest503Body},
			geminiSequenceUpstreamResponse{status: http.StatusBadRequest, body: geminiRetryTestSignatureBody},
			geminiSequenceUpstreamResponse{status: http.StatusBadRequest, body: geminiRetryTestSignatureBody},
			geminiSequenceUpstreamResponse{status: http.StatusServiceUnavailable, body: geminiRetryTest503Body},
		)
		svc.cfg.Gateway.MaxSameAccountRetriesGemini = 1

		_, err := forward(svc)

		var failoverErr *UpstreamFailoverError
		require.True(t, errors.As(err, &failoverErr), "同号重试耗尽应换号，got %T: %v", err, err)
		require.Equal(t, http.StatusServiceUnavailable, failoverErr.StatusCode)
		require.Equal(t, 4, upstream.calls)
	})
}

func TestGeminiForward_ConfiguredRetriesPreserve429BoundAndAttemptEvidence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, retries := range []int{0, 1, 4} {
		for _, fw := range geminiSingleCheckForwarders {
			t.Run(fmt.Sprintf("%s/retries=%d", fw.name, retries), func(t *testing.T) {
				svc, upstream, _ := newGeminiSingleCheckService(geminiSequenceUpstreamResponse{
					status: http.StatusTooManyRequests,
					body:   `{"error":{"code":429,"message":"Resource exhausted","status":"RESOURCE_EXHAUSTED"}}`,
				})
				svc.cfg.Gateway.MaxSameAccountRetriesGemini = retries
				account := geminiSameAccountRetryTestAccount()

				err, c := fw.forward(t, svc, account)

				var failoverErr *UpstreamFailoverError
				require.ErrorAs(t, err, &failoverErr)
				require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
				expectedAttempts := min(retries+1, 2)
				require.Equal(t, expectedAttempts, upstream.calls)
				var attempts []*OpsUpstreamErrorEvent
				for _, event := range geminiAttemptTestEvents(t, c) {
					if event.Stage == "inference" && event.Attempt > 0 {
						attempts = append(attempts, event)
					}
				}
				require.Len(t, attempts, expectedAttempts, "every actual attempt must retain its own diagnostics")
				for i, event := range attempts {
					require.Equal(t, i+1, event.Attempt)
					require.Equal(t, account.ID, event.AccountID)
					require.Equal(t, http.StatusTooManyRequests, event.UpstreamStatusCode)
					require.Equal(t, "unreported", event.UsageState)
				}
			})
		}
	}
}
