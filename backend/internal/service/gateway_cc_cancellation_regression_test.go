//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const gatewayCCCancelTimeout = 2 * time.Second

const gatewayCCThinkingSSE = "event: message_start\n" +
	`data: {"type":"message_start","message":{"id":"msg_cancel","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5","usage":{"input_tokens":20}}}` + "\n\n" +
	"event: content_block_start\n" +
	`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}` + "\n\n" +
	"event: content_block_delta\n" +
	`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"still thinking"}}` + "\n\n"

const gatewayCCFinalUsageSSE = "event: content_block_stop\n" +
	`data: {"type":"content_block_stop","index":0}` + "\n\n" +
	"event: message_delta\n" +
	`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":321}}` + "\n\n" +
	"event: message_stop\n" +
	`data: {"type":"message_stop"}` + "\n\n"

// Use a real HTTP transport so cancellation must reach the server-side request,
// including while Client.Do is waiting for headers or the SSE scanner is blocked.
type gatewayCCCancellationHTTPUpstream struct {
	client       *http.Client
	bodyReady    chan struct{}
	initialBytes int
	requests     atomic.Int32
}

func (u *gatewayCCCancellationHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.requests.Add(1)
	resp, err := u.client.Do(req)
	if err == nil && u.bodyReady != nil {
		resp.Body = &gatewayCCCancellationBody{
			ReadCloser: resp.Body,
			ready:      u.bodyReady,
			remaining:  u.initialBytes,
		}
	}
	return resp, err
}

func (u *gatewayCCCancellationHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

type gatewayCCCancellationBody struct {
	io.ReadCloser
	ready     chan struct{}
	remaining int
	once      sync.Once
}

func (b *gatewayCCCancellationBody) Read(p []byte) (int, error) {
	// Signal on the next read, after the scanner consumed the initial SSE bytes.
	if b.remaining <= 0 {
		b.once.Do(func() { close(b.ready) })
	}
	n, err := b.ReadCloser.Read(p)
	b.remaining -= n
	return n, err
}

type gatewayCCCancellationResult struct {
	result          *ForwardResult
	err             error
	responseBody    string
	deliveredText   string
	deliveredTokens int
}

func TestGatewayCC_ClientCancellationReachesAnthropicUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{true, false} {
		for _, phase := range []string{"before_headers", "before_first_sse", "during_thinking"} {
			t.Run(fmt.Sprintf("stream_%t/%s", stream, phase), func(t *testing.T) {
				upstreamStarted := make(chan struct{})
				upstreamCanceled := make(chan struct{})
				release := make(chan struct{})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.Copy(io.Discard, r.Body)
					close(upstreamStarted)
					if phase != "before_headers" {
						w.Header().Set("Content-Type", "text/event-stream")
						w.WriteHeader(http.StatusOK)
						if phase == "during_thinking" {
							_, _ = io.WriteString(w, gatewayCCThinkingSSE)
						}
						w.(http.Flusher).Flush()
					}
					select {
					case <-r.Context().Done():
						close(upstreamCanceled)
					case <-release:
					}
				}))

				bodyReady := make(chan struct{})
				upstream := &gatewayCCCancellationHTTPUpstream{client: server.Client(), bodyReady: bodyReady}
				if phase == "during_thinking" {
					upstream.initialBytes = len(gatewayCCThinkingSSE)
				}
				ctx, cancel := context.WithCancel(context.Background())
				resultCh, finished := startGatewayCCCancellationForward(t, ctx, server.URL, upstream, stream, nil)
				t.Cleanup(func() {
					cancel()
					close(release)
					server.CloseClientConnections()
					server.Close()
					awaitGatewayCCSignal(t, finished, "forwarder did not stop during cleanup")
				})

				awaitGatewayCCSignal(t, upstreamStarted, "upstream request was not dispatched")
				if phase != "before_headers" {
					awaitGatewayCCSignal(t, bodyReady, "forwarder did not reach the expected SSE read")
				}
				cancel()
				awaitGatewayCCSignal(t, upstreamCanceled, "client cancellation did not reach the Anthropic upstream promptly")
				select {
				case got := <-resultCh:
					require.NotContains(t, got.responseBody, "[DONE]", "cancellation must not emit a completion marker")
					require.NotContains(t, got.responseBody, `"finish_reason":"stop"`, "cancellation must not synthesize a finish reason")
					if !stream {
						require.Empty(t, got.responseBody, "a canceled buffered request must not emit JSON")
					}
					if got.result != nil {
						require.NotNil(t, got.result.Outcome)
						require.False(t, got.result.Outcome.StreamCompleted, "cancellation must not synthesize stream completion")
						require.True(t, got.result.ClientDisconnect)
						require.Equal(t, DisconnectSourceClient, got.result.Outcome.DisconnectSource)
					}
					if phase == "during_thinking" {
						require.NoError(t, got.err)
						require.NotNil(t, got.result)
						require.Equal(t, 20, got.result.Usage.InputTokens, "retain observed input usage after cancellation")
						require.Zero(t, got.result.Usage.OutputTokens, "final upstream output usage was never received")
						require.NotNil(t, got.result.DeliveredOutputTokens)
						if stream {
							require.Equal(t, "still thinking", got.deliveredText)
							require.Greater(t, got.deliveredTokens, 0)
							require.Less(t, got.deliveredTokens, 321)
							require.Equal(t, got.deliveredTokens, *got.result.DeliveredOutputTokens)
							require.Equal(t, got.deliveredTokens, recordedOutputTokens(got.result), "record only output successfully delivered before cancellation")
						} else {
							require.Zero(t, *got.result.DeliveredOutputTokens)
							require.Zero(t, recordedOutputTokens(got.result), "the buffered client received no output")
						}
					}
				case <-time.After(gatewayCCCancelTimeout):
					t.Fatal("forwarder kept waiting for Anthropic SSE after client cancellation")
				}
			})
		}
	}
}

func TestGatewayCC_CompletedResponsePreservesProviderUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{true, false} {
		t.Run(fmt.Sprintf("stream_%t", stream), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, gatewayCCThinkingSSE+gatewayCCFinalUsageSSE)
			}))
			upstream := &gatewayCCCancellationHTTPUpstream{client: server.Client()}
			resultCh, finished := startGatewayCCCancellationForward(t, context.Background(), server.URL, upstream, stream, nil)
			t.Cleanup(func() {
				server.CloseClientConnections()
				server.Close()
				awaitGatewayCCSignal(t, finished, "forwarder did not stop during cleanup")
			})

			select {
			case got := <-resultCh:
				require.NoError(t, got.err)
				require.NotNil(t, got.result)
				require.False(t, got.result.ClientDisconnect)
				require.Equal(t, 20, got.result.Usage.InputTokens)
				require.Equal(t, 321, got.result.Usage.OutputTokens)
				require.Equal(t, 321, recordedOutputTokens(got.result), "completed requests retain provider output usage")
				require.NotNil(t, got.result.Outcome)
				require.True(t, got.result.Outcome.StreamCompleted)
				require.Equal(t, DisconnectSourceNone, got.result.Outcome.DisconnectSource)
				if stream {
					require.Contains(t, got.responseBody, "[DONE]")
					require.NotNil(t, got.result.DeliveredOutputTokens)
					require.Greater(t, *got.result.DeliveredOutputTokens, 0)
					require.Less(t, *got.result.DeliveredOutputTokens, 321)
				} else {
					var response struct {
						Usage struct {
							CompletionTokens int `json:"completion_tokens"`
						} `json:"usage"`
					}
					require.NoError(t, json.Unmarshal([]byte(got.responseBody), &response))
					require.Equal(t, 321, response.Usage.CompletionTokens)
				}
			case <-time.After(gatewayCCCancelTimeout):
				t.Fatal("forwarder did not finish the completed Anthropic response")
			}
		})
	}
}

func TestGatewayCC_PreCanceledRequestDoesNotDispatchUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{true, false} {
		t.Run(fmt.Sprintf("stream_%t", stream), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, gatewayCCThinkingSSE+gatewayCCFinalUsageSSE)
			}))
			upstream := &gatewayCCCancellationHTTPUpstream{client: server.Client()}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			resultCh, finished := startGatewayCCCancellationForward(t, ctx, server.URL, upstream, stream, nil)
			t.Cleanup(func() {
				server.CloseClientConnections()
				server.Close()
				awaitGatewayCCSignal(t, finished, "forwarder did not stop during cleanup")
			})

			select {
			case got := <-resultCh:
				require.ErrorIs(t, got.err, context.Canceled)
				require.Nil(t, got.result)
				require.Empty(t, got.responseBody)
				require.Zero(t, upstream.requests.Load(), "a canceled request must not enter upstream dispatch")
			case <-time.After(gatewayCCCancelTimeout):
				t.Fatal("forwarder did not stop the pre-canceled request")
			}
		})
	}
}

type gatewayCCDisconnectWriter struct {
	gin.ResponseWriter
	failed chan struct{}
	once   sync.Once
}

func (w *gatewayCCDisconnectWriter) Write(_ []byte) (int, error) {
	w.once.Do(func() { close(w.failed) })
	return 0, errors.New("client disconnected")
}

func (w *gatewayCCDisconnectWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

type gatewayCCDoneFailureWriter struct {
	gin.ResponseWriter
	failed              bool
	flushesAfterFailure int
}

func (w *gatewayCCDoneFailureWriter) Write(data []byte) (int, error) {
	if strings.Contains(string(data), "data: [DONE]") {
		w.failed = true
		return 0, errors.New("client disconnected while writing completion marker")
	}
	return w.ResponseWriter.Write(data)
}

func (w *gatewayCCDoneFailureWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *gatewayCCDoneFailureWriter) Flush() {
	if w.failed {
		w.flushesAfterFailure++
	}
	w.ResponseWriter.Flush()
}

func TestGatewayCC_FinalDoneWriteFailurePreservesCompletedUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	writer := &gatewayCCDoneFailureWriter{ResponseWriter: c.Writer}
	c.Writer = writer
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(gatewayCCThinkingSSE + gatewayCCFinalUsageSSE)),
	}

	result, err := (&GatewayService{}).handleCCStreamingFromAnthropic(resp, c, "claude-sonnet-4-5", "claude-sonnet-4-5", "", nil, time.Now())

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, writer.failed, "the completion marker must exercise the failing write")
	require.Equal(t, 321, result.Usage.OutputTokens)
	require.Equal(t, 321, recordedOutputTokens(result), "terminal upstream usage remains authoritative")
	require.NotNil(t, result.Outcome)
	require.True(t, result.Outcome.StreamCompleted)
	if !result.ClientDisconnect {
		t.Error("completion-marker write failure must mark the client as disconnected")
	}
	if result.Outcome.DisconnectSource != DisconnectSourceClient {
		t.Errorf("disconnect source = %q, want %q", result.Outcome.DisconnectSource, DisconnectSourceClient)
	}
	if writer.flushesAfterFailure != 0 {
		t.Errorf("flushed %d times after the completion-marker write failed", writer.flushesAfterFailure)
	}
}

func TestGatewayCC_DownstreamWriteFailureStopsBeforeFinalUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstreamCanceled := make(chan struct{})
	sendFinalUsage := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(sendFinalUsage) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, gatewayCCThinkingSSE)
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
			close(upstreamCanceled)
		case <-sendFinalUsage:
			_, _ = io.WriteString(w, gatewayCCFinalUsageSSE)
		}
	}))
	failed := make(chan struct{})
	upstream := &gatewayCCCancellationHTTPUpstream{client: server.Client()}
	resultCh, finished := startGatewayCCCancellationForward(t, context.Background(), server.URL, upstream, true, failed)
	t.Cleanup(func() {
		release()
		server.CloseClientConnections()
		server.Close()
		awaitGatewayCCSignal(t, finished, "forwarder did not stop during cleanup")
	})

	awaitGatewayCCSignal(t, failed, "forwarder did not attempt the downstream write")
	select {
	case <-upstreamCanceled:
	case <-time.After(gatewayCCCancelTimeout):
		t.Error("downstream write failure did not promptly close the Anthropic upstream")
	}
	// On the regression path, let the detached request finish so the test also
	// detects accidental billing of final usage and never leaves a stuck handler.
	release()
	select {
	case got := <-resultCh:
		require.NoError(t, got.err)
		require.NotNil(t, got.result)
		require.True(t, got.result.ClientDisconnect)
		require.Equal(t, 20, got.result.Usage.InputTokens, "retain usage already received before the failed write")
		require.Zero(t, got.result.Usage.OutputTokens, "must not drain the final usage after the failed write")
		require.NotNil(t, got.result.Outcome)
		require.False(t, got.result.Outcome.StreamCompleted)
	case <-time.After(gatewayCCCancelTimeout):
		t.Fatal("forwarder kept reading after downstream write failure")
	}
}

func startGatewayCCCancellationForward(t *testing.T, ctx context.Context, baseURL string, upstream *gatewayCCCancellationHTTPUpstream, stream bool, failed chan struct{}) (<-chan gatewayCCCancellationResult, <-chan struct{}) {
	t.Helper()
	body := []byte(fmt.Sprintf(`{"model":"claude-sonnet-4-5","max_tokens":1024,"messages":[{"role":"user","content":"hello"}],"stream":%t}`, stream))
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body))).WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")
	if failed != nil {
		c.Writer = &gatewayCCDisconnectWriter{ResponseWriter: c.Writer, failed: failed}
	}
	var collector *DownstreamOutputTokenCollector
	restoreCollector := func() {}
	if stream && failed == nil {
		// Mirror handler accounting while keeping the write-failure regression
		// independent of the collector's own upstream cancellation behavior.
		collector, restoreCollector = AttachDownstreamOutputTokenCollector(c, "claude-sonnet-4-5")
	}
	account := newAnthropicAPIKeyAccountForTest()
	account.Credentials["base_url"] = baseURL
	svc := &GatewayService{
		cfg:                 rawChatCompletionsTestConfig(),
		httpUpstream:        upstream,
		tlsFPProfileService: &TLSFingerprintProfileService{},
	}
	resultCh := make(chan gatewayCCCancellationResult, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer restoreCollector()
		result, err := svc.ForwardAsChatCompletions(c.Request.Context(), c, account, body, nil)
		ApplyDeliveredOutputTokens(result, collector)
		resultCh <- gatewayCCCancellationResult{
			result: result, err: err, responseBody: recorder.Body.String(),
			deliveredText: collector.VisibleText(), deliveredTokens: collector.TokenCount(),
		}
	}()
	return resultCh, finished
}

func awaitGatewayCCSignal(t *testing.T, signal <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(gatewayCCCancelTimeout):
		t.Fatal(message)
	}
}
