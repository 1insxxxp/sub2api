//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const geminiRealHTTPFinalUsage = `data: {"candidates":[{"content":{"parts":[{"text":" done"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":21916,"candidatesTokenCount":4055,"totalTokenCount":25971}}` + "\n\ndata: [DONE]\n\n"

// The inference transport still closes immediately. Exact usage is recovered
// separately from the provider's request ledger; generation is never resumed.
func TestGeminiUsageRecoveryRealHTTP_DownstreamAbortRetainsLateProviderUsage(t *testing.T) {
	for _, adapter := range []string{"chat", "messages", "native"} {
		for _, abort := range []string{"context_cancel", "write_failure"} {
			t.Run(adapter+"/"+abort, func(t *testing.T) {
				result, forwardErr, frozen, upstreamCanceledBeforeUsage := runGeminiRealHTTPUsageCase(t, adapter, abort)
				require.True(t, frozen, "the real downstream collector must observe the abort")
				require.NotNil(t, result, "forward error: %v", forwardErr)
				t.Logf("forward error=%v; upstream canceled before delayed usage=%t; measured input=%d, output=%d", forwardErr, upstreamCanceledBeforeUsage, result.Usage.InputTokens, result.Usage.OutputTokens)
				require.True(t, upstreamCanceledBeforeUsage, "ledger recovery must not keep the generation stream alive")
				assert.Equal(t, 21916, result.Usage.InputTokens, "recover the exact input from the matching provider ledger row")
				assert.Equal(t, 4055, result.Usage.OutputTokens, "ledger completion tokens replace any local estimate")
			})
		}
	}
}

// The control delivers the exact same final usage in the first network flush,
// without canceling, to distinguish transport loss from a usage-parser defect.
func TestGeminiUsageRecoveryRealHTTP_BufferedTailWithoutAbortPreservesProviderUsage(t *testing.T) {
	for _, adapter := range []string{"chat", "messages", "native"} {
		t.Run(adapter, func(t *testing.T) {
			result, err, frozen, _ := runGeminiRealHTTPUsageCase(t, adapter, "")
			require.NoError(t, err)
			require.NotNil(t, result)
			require.False(t, frozen)
			require.Equal(t, 21916, result.Usage.InputTokens)
			require.Equal(t, 4055, result.Usage.OutputTokens)
			require.Equal(t, UsageTokenSourceUpstream, result.InputTokensSource)
			require.True(t, result.MeasuredUsageComplete)
		})
	}
}

func runGeminiRealHTTPUsageCase(t *testing.T, adapter, abort string) (*ForwardResult, error, bool, bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	aborted := make(chan struct{})
	serverDone := make(chan bool, 1)
	var ledgerReady atomic.Bool
	var inferenceRequests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/log/token" {
			w.Header().Set("Content-Type", "application/json")
			if ledgerReady.Load() {
				_, _ = io.WriteString(w, geminiUsageRecoveryLedgerFixture)
			} else {
				_, _ = io.WriteString(w, `{"success":true,"data":[]}`)
			}
			return
		}
		inferenceRequests.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Request-ID", "local-delayed-usage-"+adapter)
		w.Header().Set("X-Oneapi-Request-Id", "provider-request-1")
		if abort == "" {
			_, _ = io.WriteString(w, geminiAbortTextChunk+geminiRealHTTPFinalUsage)
			w.(http.Flusher).Flush()
			serverDone <- false
			return
		}
		_, _ = io.WriteString(w, geminiAbortTextChunk)
		w.(http.Flusher).Flush()
		select {
		case <-aborted:
		case <-time.After(2 * time.Second):
			serverDone <- r.Context().Err() != nil
			return
		}
		// The usage is intentionally unavailable until after the downstream abort.
		// Unlike a strings.Reader, this live response respects HTTP cancellation.
		timer := time.NewTimer(100 * time.Millisecond)
		defer timer.Stop()
		<-timer.C
		canceled := r.Context().Err() != nil
		_, _ = io.WriteString(w, geminiRealHTTPFinalUsage)
		w.(http.Flusher).Flush()
		ledgerReady.Store(true)
		serverDone <- canceled
	}))
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	require.NoError(t, err)
	client := server.Client()
	client.Timeout = 3 * time.Second
	t.Cleanup(client.CloseIdleConnections)

	body := geminiInputUsageRegressionRequest(t, adapter)
	requestCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+adapter, bytes.NewReader(body)).WithContext(requestCtx)
	if abort != "" {
		c.Writer = &geminiRealHTTPAbortWriter{ResponseWriter: c.Writer, abort: abort, cancel: cancel, aborted: aborted}
	}
	collector, restore := AttachDownstreamOutputTokenCollector(c, geminiInputUsageRegressionModel)
	defer restore()
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	upstream := &geminiRealHTTPUsageUpstream{client: client, target: target}
	svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: cfg}
	account := &Account{ID: 5957, Platform: PlatformGemini, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"base_url": server.URL, "api_key": "local-ledger-key"}}
	result, forwardErr := forwardGeminiInputUsageRegression(t, adapter, svc, c, account, body)
	if abort != "" {
		require.NotNil(t, result)
		require.Zero(t, result.Usage.InputTokens, "the final stream usage was lost before recovery")
		recovered, recoveryErr := (&GatewayService{httpUpstream: upstream, cfg: cfg}).RecoverGeminiUsage(c.Request.Context(), &RecordUsageInput{Account: account, Result: result})
		require.NoError(t, recoveryErr)
		require.True(t, recovered)
	}
	require.EqualValues(t, 1, inferenceRequests.Load(), "recovery must never retry inference")
	select {
	case canceled := <-serverDone:
		return result, forwardErr, collector.Frozen(), canceled
	case <-time.After(3 * time.Second):
		t.Fatal("local upstream handler did not finish")
		return nil, nil, false, false
	}
}

type geminiRealHTTPAbortWriter struct {
	gin.ResponseWriter
	abort   string
	cancel  context.CancelFunc
	aborted chan struct{}
	once    sync.Once
}

func (w *geminiRealHTTPAbortWriter) Write(data []byte) (int, error) {
	if strings.Contains(string(data), "hello") {
		w.once.Do(func() {
			if w.abort == "context_cancel" {
				w.cancel()
			}
			close(w.aborted)
		})
		if w.abort == "write_failure" {
			return 0, io.ErrClosedPipe
		}
	}
	return w.ResponseWriter.Write(data)
}

func (w *geminiRealHTTPAbortWriter) WriteString(data string) (int, error) {
	return w.Write([]byte(data))
}

// Only the destination is redirected to our local test server. Request context,
// HTTP streaming, and transport cancellation all remain real; no external call
// can be made even though the production Vertex request builder is exercised.
type geminiRealHTTPUsageUpstream struct {
	client *http.Client
	target *url.URL
}

func (u *geminiRealHTTPUsageUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	local := req.Clone(req.Context())
	local.URL.Scheme = u.target.Scheme
	local.URL.Host = u.target.Host
	local.Host = u.target.Host
	client := *u.client
	if HTTPUpstreamRedirectsDisabled(req.Context()) {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	}
	return client.Do(local)
}

func (u *geminiRealHTTPUsageUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}
