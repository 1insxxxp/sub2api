package service

import (
	"context"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	geminiLargeRequestAttempts = 4
	geminiLargeRequestWindow   = 90 * time.Second
)

type geminiLargeRequestBudgetKey struct{}
type geminiLargeRequestBudget struct {
	deadline     time.Time
	attempts     atomic.Int32
	continuation atomic.Bool
}

// BeginGeminiLargeRequestBudget is called after ingress admits a large Gemini
// request. All derived scheduling and forwarding contexts share this state.
func BeginGeminiLargeRequestBudget(c *gin.Context) {
	beginGeminiLargeRequestBudget(c, geminiLargeRequestWindow)
}

func beginGeminiLargeRequestBudget(c *gin.Context, window time.Duration) {
	parent := c.Request.Context()
	if geminiLargeBudget(parent) != nil {
		return
	}
	b := &geminiLargeRequestBudget{deadline: time.Now().Add(window)}
	c.Request = c.Request.WithContext(context.WithValue(parent, geminiLargeRequestBudgetKey{}, b))
}

func geminiLargeBudget(ctx context.Context) *geminiLargeRequestBudget {
	if ctx == nil {
		return nil
	}
	b, _ := ctx.Value(geminiLargeRequestBudgetKey{}).(*geminiLargeRequestBudget)
	return b
}

// GeminiLargeRequestRetryAllowed checks a prospective retry, including its backoff.
func GeminiLargeRequestRetryAllowed(ctx context.Context, delay time.Duration) bool {
	b := geminiLargeBudget(ctx)
	return b == nil || (b.attempts.Load() < geminiLargeRequestAttempts && time.Now().Add(delay).Before(b.deadline))
}

// GeminiLargeRequestSwitchOn503 leaves the remaining budget for another account.
// Explicit cache references must stay on their owning account.
func GeminiLargeRequestSwitchOn503(ctx context.Context, status int) bool {
	return status == http.StatusServiceUnavailable && geminiLargeBudget(ctx) != nil && !geminiCachedContentBoundFromContext(ctx)
}

func geminiLargeBudgetExhausted() *UpstreamFailoverError {
	return &UpstreamFailoverError{
		StatusCode:        http.StatusServiceUnavailable,
		ResponseBody:      []byte(`{"error":{"code":503,"status":"UNAVAILABLE","message":"Gemini request retry budget exhausted"}}`),
		NextAccountAction: NextAccountStop,
	}
}

type geminiBudgetResponseBody struct {
	io.ReadCloser
	release func()
}

func (b *geminiBudgetResponseBody) Close() error {
	b.release()
	return b.ReadCloser.Close()
}

// doGeminiInference charges actual sends, across both service and handler retries.
// The initial call may wait for healthy output indefinitely under its caller's
// context. A retry must receive headers inside the window. Successful headers
// disarm its timer before handing the body to a buffered or streaming reader.
func doGeminiInference(upstream HTTPUpstream, req *http.Request, proxy string, account *Account, onSend func()) (*http.Response, error) {
	ctx := req.Context()
	b := geminiLargeBudget(ctx)
	send := func(r *http.Request) (*http.Response, error) {
		if onSend != nil {
			onSend()
		}
		return upstream.Do(r, proxy, account.ID, account.Concurrency)
	}
	if b == nil {
		return send(req)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// A successful truncated round advances the prompt. Only its first send
	// is a continuation; subsequent sends of that prompt are retries.
	retry := false
	if !b.continuation.Swap(false) {
		for {
			used := b.attempts.Load()
			retry = used > 0
			if used >= geminiLargeRequestAttempts || (retry && !time.Now().Before(b.deadline)) {
				return nil, geminiLargeBudgetExhausted()
			}
			if b.attempts.CompareAndSwap(used, used+1) {
				break
			}
		}
	}
	attemptCtx, cancel := context.WithCancel(ctx)
	var timer *time.Timer
	if retry {
		timer = time.AfterFunc(time.Until(b.deadline), cancel)
	}
	resp, err := send(req.WithContext(attemptCtx))
	if err != nil || resp == nil {
		expired := timer != nil && !timer.Stop()
		cancel()
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if expired {
			return nil, geminiLargeBudgetExhausted()
		}
		return nil, err
	}
	if resp.StatusCode < 400 && timer != nil {
		if !timer.Stop() {
			cancel()
			_ = resp.Body.Close()
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return nil, geminiLargeBudgetExhausted()
		}
		timer = nil
	} else if resp.StatusCode >= 400 && timer == nil {
		timer = time.AfterFunc(time.Until(b.deadline), cancel)
	}
	resp.Body = &geminiBudgetResponseBody{ReadCloser: resp.Body, release: func() {
		if timer != nil {
			timer.Stop()
		}
		cancel()
	}}
	return resp, nil
}

func (s *GeminiMessagesCompatService) sendGeminiInference(c *gin.Context, req *http.Request, proxy string, account *Account, model string, stream bool) (*http.Response, *GeminiUpstreamAttempt, error) {
	var attempt *GeminiUpstreamAttempt
	resp, err := doGeminiInference(s.httpUpstream, req, proxy, account, func() {
		attempt = BeginGeminiUpstreamAttempt(c, account, model, stream)
		attempt.ObserveRequest(req)
	})
	attempt.ObserveResponse(resp)
	return resp, attempt, err
}

func waitGeminiRetry(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !GeminiLargeRequestRetryAllowed(ctx, delay) {
		return geminiLargeBudgetExhausted()
	}
	return sleepWithContext(ctx, delay)
}
