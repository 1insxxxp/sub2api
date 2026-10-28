//go:build unit

package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Exercise both sides with real HTTP. The application observes the downstream
// cancellation before the deliberately detached upstream returns EOF/reset.
func TestRawChatOpsRealHTTPCancellationBeforeHeaders(t *testing.T) {
	for _, tc := range []struct {
		name      string
		stream    bool
		cancel    bool
		reset     bool
		httpError bool
	}{
		{name: "stream_eof_after_cancel", stream: true, cancel: true},
		{name: "sync_eof_after_cancel", cancel: true},
		{name: "stream_reset_after_cancel", stream: true, cancel: true, reset: true},
		{name: "eof_without_cancel_remains_provider_error", stream: true},
		{name: "http_502_after_cancel_remains_provider_error", stream: true, cancel: true, httpError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstreamStarted, releaseUpstream := make(chan struct{}), make(chan struct{})
			upstreamDetached := make(chan bool, 1)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				close(upstreamStarted)
				<-releaseUpstream
				upstreamDetached <- r.Context().Err() == nil
				if tc.httpError {
					w.Header().Set("X-Oneapi-Request-Id", "observed-provider-error")
					w.WriteHeader(http.StatusBadGateway)
					_, _ = io.WriteString(w, `{"error":{"message":"isolated provider failure"}}`)
					return
				}
				connection, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					return
				}
				if tc.reset {
					if tcp, ok := connection.(*net.TCPConn); ok {
						_ = tcp.SetLinger(0)
					}
				}
				_ = connection.Close()
			}))
			defer upstream.Close()
			defer func() {
				select {
				case <-releaseUpstream:
				default:
					close(releaseUpstream)
				}
			}()
			account := rawChatCompletionsTestAccount()
			account.Credentials["base_url"] = upstream.URL
			transport := &gatewayCCCancellationHTTPUpstream{client: upstream.Client()}
			transport.client.Timeout = 3 * time.Second
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: transport}
			body := []byte(fmt.Sprintf(`{"model":"gemini-4-argon","messages":[{"role":"user","content":"isolated"}],"stream":%t}`, tc.stream))
			type captured struct {
				result *OpenAIForwardResult
				events []*OpsUpstreamErrorEvent
				err    error
			}
			finished := make(chan captured, 1)
			canceledObserved := make(chan struct{})
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				c, _ := gin.CreateTestContext(w)
				c.Request = r
				if tc.cancel {
					go func() { <-r.Context().Done(); close(canceledObserved) }()
				}
				appendOpsUpstreamError(c, OpsUpstreamErrorEvent{AccountID: account.ID, Kind: "failover", Stage: "inference", Attempt: 1, UpstreamStatusCode: http.StatusServiceUnavailable})
				c.Set(openAIChatOpsAttemptSequenceKey, 1)
				result, err := svc.forwardAsRawChatCompletions(r.Context(), c, account, body, "")
				finished <- captured{result: result, events: geminiUpstreamAttemptEvents(c), err: err}
			}))
			defer gateway.Close()
			clientContext, cancel := context.WithCancel(context.Background())
			defer cancel()
			request, err := http.NewRequestWithContext(clientContext, http.MethodPost, gateway.URL, bytes.NewReader(body))
			require.NoError(t, err)
			clientFinished := make(chan struct{})
			go func() {
				response, _ := gateway.Client().Do(request)
				if response != nil {
					_ = response.Body.Close()
				}
				close(clientFinished)
			}()
			select {
			case <-upstreamStarted:
			case <-time.After(3 * time.Second):
				t.Fatal("real upstream request did not start")
			}
			if tc.cancel {
				cancel()
				select {
				case <-canceledObserved:
				case <-time.After(3 * time.Second):
					t.Fatal("application did not observe downstream cancellation")
				}
			}
			close(releaseUpstream)
			var got captured
			select {
			case got = <-finished:
			case <-time.After(4 * time.Second):
				t.Fatal("forwarding did not finish")
			}
			<-clientFinished
			require.True(t, <-upstreamDetached, "the existing detached transport policy must remain unchanged")
			require.Error(t, got.err)
			require.EqualValues(t, 1, transport.requests.Load(), "do not send another inference after cancellation")
			require.Len(t, got.events, 2)
			require.Equal(t, account.ID, got.events[0].AccountID)
			require.Equal(t, http.StatusServiceUnavailable, got.events[0].UpstreamStatusCode)
			require.Equal(t, "failover", got.events[0].Kind)
			require.False(t, got.events[0].ClientCanceled, "earlier provider failures must remain intact")
			last := got.events[1]
			require.Equal(t, account.ID, last.AccountID)
			require.Equal(t, 2, last.Attempt)
			if tc.cancel && !tc.httpError {
				require.ErrorIs(t, got.err, context.Canceled, "EOF/reset after known downstream cancellation must not request failover")
				require.NotNil(t, got.result)
				require.True(t, got.result.ClientDisconnect)
				require.Equal(t, DisconnectSourceClient, got.result.Outcome.DisconnectSource)
				require.True(t, last.IsClientCancelDiagnostic())
				require.Zero(t, last.UpstreamStatusCode)
				require.Equal(t, "unreported", last.UsageState)
			} else {
				require.Nil(t, got.result)
				require.False(t, last.ClientCanceled)
				if tc.httpError {
					require.Equal(t, http.StatusBadGateway, last.UpstreamStatusCode)
					require.Equal(t, "observed-provider-error", last.UpstreamRequestID)
				}
			}
		})
	}
}

func TestRawChatOpsCanceledResponseAndTransportErrorPreservesHTTPObservation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	body := []byte(`{"model":"gemini-4-argon","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)).WithContext(ctx)
	upstream := &rawChatResponseErrorUpstream{response: &http.Response{StatusCode: http.StatusBadGateway, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	result, err := svc.forwardAsRawChatCompletions(ctx, c, rawChatCompletionsTestAccount(), body, "")
	require.Error(t, err)
	require.NotErrorIs(t, err, context.Canceled)
	require.Nil(t, result)
	events := geminiUpstreamAttemptEvents(c)
	require.Len(t, events, 1)
	require.False(t, events[0].ClientCanceled)
	require.Equal(t, http.StatusBadGateway, events[0].UpstreamStatusCode)
}

type rawChatResponseErrorUpstream struct{ response *http.Response }

func (u *rawChatResponseErrorUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return u.response, io.EOF
}

func (u *rawChatResponseErrorUpstream) DoWithTLS(req *http.Request, proxy string, account int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, account, concurrency)
}
