//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiLargeRequestSharedAttempts(t *testing.T) {
	for _, route := range []string{"native", "chat", "messages"} {
		t.Run(route, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			BeginGeminiLargeRequestBudget(c)
			svc, upstream, _ := newGeminiSingleCheckService(geminiSequenceUpstreamResponse{503, geminiRetryTest503Body})
			svc.cfg.Gateway.MaxSameAccountRetriesGemini = 8
			account := geminiSameAccountRetryTestAccount()
			for i := 0; i < 5; i++ {
				account.ID++
				_, err := callGeminiTraversalPath(t, route, svc, c, account, "gemini-2.5-flash", false)
				var failover *UpstreamFailoverError
				require.ErrorAs(t, err, &failover)
				require.Equal(t, http.StatusServiceUnavailable, failover.StatusCode)
				if i == 0 {
					require.Less(t, upstream.calls, 4, "allow another account to participate")
				}
			}
			require.Equal(t, 4, upstream.calls)
			require.False(t, c.Writer.Written())
		})
	}
}

type largeGeminiTestUpstream struct {
	HTTPUpstream
	send func(*http.Request) (*http.Response, error)
}

func (u largeGeminiTestUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return u.send(req)
}

func TestGeminiLargeRequestStreamOutlivesRetryWindow(t *testing.T) {
	c, rec := newGeminiNativeTestContext(t)
	beginGeminiLargeRequestBudget(c, 300*time.Millisecond)
	ctx := c.Request.Context()
	account := geminiSameAccountRetryTestAccount()
	svc, _, _ := newGeminiSingleCheckService()
	calls := 0
	svc.httpUpstream = largeGeminiTestUpstream{send: func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: 503, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(geminiRetryTest503Body))}, nil
		}
		r, w := io.Pipe()
		go func() {
			select {
			case <-req.Context().Done():
				_ = w.CloseWithError(req.Context().Err())
			case <-time.After(time.Until(geminiLargeBudget(ctx).deadline) + 30*time.Millisecond):
				_, _ = fmt.Fprint(w, "data: "+geminiRetryTestOKBody+"\n\n")
				_ = w.Close()
			}
		}()
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: r}, nil
	}}
	forward := func() error {
		_, err := svc.ForwardNative(ctx, c, account, "gemini-2.5-flash", "streamGenerateContent", true, []byte(`{"contents":[{"parts":[{"text":"hi"}]}]}`))
		return err
	}
	require.Error(t, forward())
	account.ID++
	require.NoError(t, forward())
	require.Contains(t, rec.Body.String(), "ok")
	require.Equal(t, 2, calls)
}

func TestGeminiLargeRequestDeadlineAndCancel(t *testing.T) {
	for _, cancelClient := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel=%t", cancelClient), func(t *testing.T) {
			c, _ := newGeminiNativeTestContext(t)
			parent, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()
			c.Request = c.Request.WithContext(parent)
			window := 100 * time.Millisecond
			if cancelClient {
				window = time.Second
			}
			beginGeminiLargeRequestBudget(c, window)
			ctx := c.Request.Context()
			attempts := 0
			upstream := largeGeminiTestUpstream{send: func(req *http.Request) (*http.Response, error) {
				attempts++
				if attempts == 1 {
					return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader(""))}, nil
				}
				if cancelClient {
					cancel()
				}
				<-req.Context().Done()
				return nil, req.Context().Err()
			}}
			account := geminiSameAccountRetryTestAccount()
			resp, err := doGeminiInference(upstream, c.Request, "", account, nil)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			_, err = doGeminiInference(upstream, c.Request, "", account, nil)
			if cancelClient {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				var exhausted *UpstreamFailoverError
				require.ErrorAs(t, err, &exhausted)
				require.False(t, exhausted.ShouldRetryNextAccount())
				require.NoError(t, ctx.Err())
			}
			_, err = doGeminiInference(upstream, c.Request, "", account, nil)
			require.Error(t, err)
			require.Equal(t, 2, attempts)

		})
	}
}

func TestGeminiLargeRequestSuccessfulContinuation(t *testing.T) {
	c, rec := accountContinuationTestContext()
	// The first call has already spent the whole window producing good output.
	beginGeminiLargeRequestBudget(c, 0)
	account := geminiSameAccountRetryTestAccount()
	account.Extra = accountContinuationTestAccount(true).Extra
	svc, upstream, _ := newGeminiSingleCheckService(
		geminiSequenceUpstreamResponse{200, strings.Replace(geminiRetryTestOKBody, "STOP", "MAX_TOKENS", 1)},
		geminiSequenceUpstreamResponse{200, strings.Replace(geminiRetryTestOKBody, "ok", "finished", 1)},
	)
	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"write"}]}]}`)
	result, err := svc.ForwardNative(c.Request.Context(), c, account, "long-text", "generateContent", false, body)
	require.NoError(t, err)
	require.Equal(t, 2, upstream.calls)
	require.Equal(t, 1, result.AntiTruncationRounds)
	require.EqualValues(t, 1, geminiLargeBudget(c.Request.Context()).attempts.Load())
	require.Contains(t, rec.Body.String(), "finished")
}

func TestGeminiLargeRequestAntigravityCapacity(t *testing.T) {
	c, _ := newGeminiNativeTestContext(t)
	BeginGeminiLargeRequestBudget(c)
	ctx := WithSingleAccountRetry(c.Request.Context(), true, false)
	upstream := &geminiSequenceUpstreamStub{responses: []geminiSequenceUpstreamResponse{{503,
		`{"error":{"status":"UNAVAILABLE","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","metadata":{"model":"gemini-3-pro"},"reason":"MODEL_CAPACITY_EXHAUSTED"}]}}`}}}
	svc := &AntigravityGatewayService{}
	p := antigravityRetryLoopParams{ctx: ctx, c: c, action: "streamGenerateContent", accessToken: "test",
		account: &Account{ID: 1, Platform: PlatformAntigravity, Type: AccountTypeOAuth}, httpUpstream: upstream, body: []byte("{}")}
	for i := 0; i < 4; i++ {
		p.account.ID++
		result, err := svc.antigravityRetryLoop(p)
		require.NoError(t, err)
		require.Equal(t, 503, result.resp.StatusCode)
		require.NoError(t, result.resp.Body.Close())
		require.Equal(t, i+1, upstream.calls)
	}
	_, err := svc.antigravityRetryLoop(p)
	require.Error(t, err)
	require.Equal(t, 4, upstream.calls)
}

func TestGeminiLargeRequestBoundAccountSharesRetryLimit(t *testing.T) {
	c, _ := newGeminiNativeTestContext(t)
	BeginGeminiLargeRequestBudget(c)
	ctx := WithGeminiCachedContentBound(c.Request.Context())
	c.Request = c.Request.WithContext(ctx)
	svc, upstream, _ := newGeminiSingleCheckService(geminiSequenceUpstreamResponse{503, geminiRetryTest503Body})
	svc.cfg.Gateway.MaxSameAccountRetriesGemini = 8
	// Simulate two earlier sends before the current account's local loop.
	geminiLargeBudget(ctx).attempts.Store(2)
	_, err := callGeminiTraversalPath(t, "messages", svc, c, geminiSameAccountRetryTestAccount(), "gemini-2.5-flash", false)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Equal(t, 503, failover.StatusCode)
	require.Equal(t, 2, upstream.calls)
	require.False(t, GeminiLargeRequestRetryAllowed(ctx, 0))
}

func TestGeminiLargeRequestFailedSendReleasesResponse(t *testing.T) {
	c, _ := newGeminiNativeTestContext(t)
	BeginGeminiLargeRequestBudget(c)
	r, w := io.Pipe()
	t.Cleanup(func() { _ = w.Close() })
	var attemptCtx context.Context
	upstream := largeGeminiTestUpstream{send: func(req *http.Request) (*http.Response, error) {
		attemptCtx = req.Context()
		return &http.Response{StatusCode: 503, Body: r}, io.ErrUnexpectedEOF
	}}
	resp, err := doGeminiInference(upstream, c.Request, "", geminiSameAccountRetryTestAccount(), nil)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	require.Nil(t, resp)
	require.ErrorIs(t, attemptCtx.Err(), context.Canceled)
	_, err = w.Write(nil)
	require.ErrorIs(t, err, io.ErrClosedPipe)
	require.NoError(t, c.Request.Context().Err())
}
