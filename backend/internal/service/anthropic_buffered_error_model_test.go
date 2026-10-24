//go:build unit

package service

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicBufferedModelNotFoundFailsOverWithoutPoolRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		mode := "normal"
		if passthrough {
			mode = "passthrough"
		}
		for _, poolMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/pool=%t", mode, poolMode), func(t *testing.T) {
				const upstreamBody = `{"type":"error","error":{"type":"not_found_error","status_code":404,"message":"Model claude-sonnet-4-5 not found"}}`
				svc, account, c, recorder, parsed := newEmbeddedErrorForwardFixture(t, passthrough, upstreamBody)
				account.Credentials["pool_mode"] = poolMode
				account.Credentials["pool_mode_retry_status_codes"] = []any{float64(http.StatusNotFound)}
				repo := &modelNotFoundAccountRepoStub{}
				svc.rateLimitService = &RateLimitService{accountRepo: repo}

				result, err := svc.Forward(c.Request.Context(), c, account, parsed)

				require.Nil(t, result)
				var failoverErr *UpstreamFailoverError
				require.ErrorAs(t, err, &failoverErr)
				require.Equal(t, http.StatusNotFound, failoverErr.StatusCode)
				require.JSONEq(t, upstreamBody, string(failoverErr.ResponseBody))
				require.Equal(t, "rid-embedded-error", failoverErr.ResponseHeaders.Get("x-request-id"))
				require.True(t, failoverErr.ShouldRetryNextAccount())
				require.False(t, failoverErr.RetryableOnSameAccount, "a missing model cannot recover by retrying the same pool account")
				require.False(t, c.Writer.Written())
				require.Empty(t, recorder.Body.String())

				value, ok := c.Get(OpsUpstreamErrorsKey)
				require.True(t, ok)
				events, ok := value.([]*OpsUpstreamErrorEvent)
				require.True(t, ok)
				require.Len(t, events, 1)
				require.Equal(t, account.ID, events[0].AccountID)
				require.Equal(t, http.StatusNotFound, events[0].UpstreamStatusCode)
				require.Equal(t, "rid-embedded-error", events[0].UpstreamRequestID)
				require.Equal(t, passthrough, events[0].Passthrough)

				require.Zero(t, repo.tempCalls, "model availability must not disable the entire account")
				require.Len(t, repo.modelRateLimitCalls, 1)
				call := repo.modelRateLimitCalls[0]
				require.Equal(t, account.ID, call.accountID)
				require.Equal(t, "claude-sonnet-4-5", call.scope)
				require.Equal(t, upstreamModelNotFoundReason, call.reason)
				require.WithinDuration(t, time.Now().Add(upstreamModelNotFoundCooldown), call.resetAt, time.Second)
			})
		}
	}
}
