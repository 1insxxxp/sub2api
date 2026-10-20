//go:build unit

package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const streamEOFFirstContent = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_first\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-5\",\"content\":[],\"usage\":{\"input_tokens\":2}}}\n\n" +
	"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"first account partial answer\"}}\n\n"

const streamEOFSecondSuccess = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_second\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-5\",\"content\":[],\"usage\":{\"input_tokens\":3}}}\n\n" +
	"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
	"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"second account answer\"}}\n\n" +
	"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
	"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":4}}\n\n" +
	"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"

// Hold the first response open until the real stream handler has sent its own
// keepalive. This exercises SafeToFailoverAfterWrite without injecting bytes
// outside the service's accounting or relying on a fixed upstream sleep.
type streamEOFHeartbeatBody struct {
	heartbeat <-chan struct{}
	ctx       context.Context
}

func (b *streamEOFHeartbeatBody) Read([]byte) (int, error) {
	timer := time.NewTimer(4 * time.Second)
	defer timer.Stop()
	select {
	case <-b.heartbeat:
		return 0, io.EOF
	case <-b.ctx.Done():
		return 0, b.ctx.Err()
	case <-timer.C:
		return 0, fmt.Errorf("test upstream did not observe the gateway keepalive")
	}
}

func (*streamEOFHeartbeatBody) Close() error { return nil }

type streamEOFHeartbeatWriter struct {
	gin.ResponseWriter
	once      sync.Once
	heartbeat chan struct{}
}

func (w *streamEOFHeartbeatWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if err == nil && bytes.Contains(p[:n], []byte("event: ping\n")) {
		w.once.Do(func() { close(w.heartbeat) })
	}
	return n, err
}

type streamEOFFailoverUpstream struct {
	service.HTTPUpstream
	accountIDs []int64
	firstBody  string
	alwaysEOF  bool
	heartbeat  <-chan struct{}
}

func (u *streamEOFFailoverUpstream) DoWithTLS(req *http.Request, _ string, accountID int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.accountIDs = append(u.accountIDs, accountID)
	var body io.ReadCloser
	if len(u.accountIDs) == 1 {
		if u.heartbeat != nil {
			body = &streamEOFHeartbeatBody{heartbeat: u.heartbeat, ctx: req.Context()}
		} else {
			body = io.NopCloser(strings.NewReader(u.firstBody))
		}
	} else if u.alwaysEOF {
		body = io.NopCloser(strings.NewReader(""))
	} else {
		body = io.NopCloser(strings.NewReader(streamEOFSecondSuccess))
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": {"text/event-stream"},
			"X-Request-Id": {fmt.Sprintf("stream-eof-attempt-%d", accountID)},
		},
		Body: body,
	}, nil
}

// Run the real Messages handler, scheduler and stream processors together.
// Distinct account IDs prove EOF skips same-account retry; the third available
// account catches accidental use of the unlimited first-output traversal path.
func TestGatewayMessagesStreamEOFFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		for _, tc := range []struct {
			name           string
			maxSwitches    int
			firstBody      string
			heartbeat      bool
			alwaysEOF      bool
			wantAccountIDs []int64
			wantSuccess    bool
		}{
			{name: "empty EOF selects second account", maxSwitches: 1, wantAccountIDs: []int64{9801, 9802}, wantSuccess: true},
			{name: "only gateway heartbeat selects second account", maxSwitches: 1, heartbeat: true, wantAccountIDs: []int64{9801, 9802}, wantSuccess: true},
			{name: "disabled switching stops after first account", wantAccountIDs: []int64{9801}},
			{name: "EOF respects switch limit", maxSwitches: 1, alwaysEOF: true, wantAccountIDs: []int64{9801, 9802}},
			{name: "committed content is never replayed", maxSwitches: 1, firstBody: streamEOFFirstContent, wantAccountIDs: []int64{9801}},
		} {
			t.Run(fmt.Sprintf("passthrough=%t/%s", passthrough, tc.name), func(t *testing.T) {
				groupID := int64(9800)
				group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive, RateMultiplier: 1}
				accounts := make([]*service.Account, 0, 3)
				for i := 1; i <= 3; i++ {
					accountID := groupID + int64(i)
					accounts = append(accounts, &service.Account{
						ID: accountID, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
						Status: service.StatusActive, Schedulable: true, Concurrency: 1, Priority: i,
						Credentials: map[string]any{
							"api_key": "local-test-only", "base_url": "https://api.anthropic.com",
							"pool_mode": true, "pool_mode_retry_count": float64(3),
						},
						Extra:         map[string]any{"anthropic_passthrough": passthrough},
						AccountGroups: []service.AccountGroup{{AccountID: accountID, GroupID: groupID}},
					})
				}
				upstream := &streamEOFFailoverUpstream{firstBody: tc.firstBody, alwaysEOF: tc.alwaysEOF}
				cfg := &config.Config{RunMode: config.RunModeSimple}
				cfg.Default.RateMultiplier = 1
				if tc.heartbeat {
					cfg.Gateway.StreamKeepaliveInterval = 1
				}
				usageRepo := &partialChatUsageRepo{}
				snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: accounts}, nil, nil, nil, nil)
				gateway := service.NewGatewayService(
					nil, &fakeGroupRepo{group: group}, usageRepo, nil, nil, nil, nil, nil, cfg,
					snapshot, nil, service.NewBillingService(cfg, nil), &service.RateLimitService{}, nil, nil, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil,
					&service.TLSFingerprintProfileService{}, nil, nil, nil, nil, nil,
				)
				billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
				t.Cleanup(billing.Stop)
				h := &GatewayHandler{
					gatewayService: gateway, billingCacheService: billing, cfg: cfg,
					concurrencyHelper:  NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0),
					maxAccountSwitches: tc.maxSwitches,
				}
				apiKey := &service.APIKey{
					ID: 9850, UserID: 9851, GroupID: &groupID, Group: group, Status: service.StatusActive,
					User: &service.User{ID: 9851, Concurrency: 10, Balance: 100},
				}
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				ctx := context.WithValue(context.Background(), ctxkey.Group, group)
				body := `{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hello"}],"stream":true,"max_tokens":100}`
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)).WithContext(ctx)
				c.Request.Header.Set("Content-Type", "application/json")
				c.Set(string(middleware.ContextKeyAPIKey), apiKey)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})
				if tc.heartbeat {
					writer := &streamEOFHeartbeatWriter{ResponseWriter: c.Writer, heartbeat: make(chan struct{})}
					c.Writer = writer
					upstream.heartbeat = writer.heartbeat
				}

				h.Messages(c)

				require.Equal(t, tc.wantAccountIDs, upstream.accountIDs)
				response := recorder.Body.String()
				if tc.heartbeat {
					require.Contains(t, response, "event: ping\n")
				}
				if tc.wantSuccess {
					require.Equal(t, http.StatusOK, recorder.Code)
					require.Contains(t, response, "second account answer")
					require.Equal(t, 1, strings.Count(response, "event: message_start\n"), "the failed stream must not leak a second response")
					require.Equal(t, 1, strings.Count(response, "event: message_stop\n"))
					require.NotContains(t, response, "event: error\n")
					require.Len(t, usageRepo.logs, 1, "only the successful account is settled")
					require.Equal(t, int64(9802), usageRepo.logs[0].AccountID)
				} else {
					require.NotContains(t, response, "second account answer")
					if tc.firstBody != "" {
						require.Contains(t, response, "first account partial answer")
						require.Equal(t, 1, strings.Count(response, "event: message_start\n"))
						require.Len(t, usageRepo.logs, 1, "already metered partial output must retain its usage")
						require.Equal(t, int64(9801), usageRepo.logs[0].AccountID)
						require.Equal(t, 2, usageRepo.logs[0].InputTokens)
					} else {
						if passthrough {
							// Passthrough preserves its EOF Flush, which has already
							// committed HTTP 200 even when no data was received.
							// Exhaustion must therefore end the stream with an error.
							require.Equal(t, http.StatusOK, recorder.Code)
							require.Contains(t, response, "event: error\n")
							require.Contains(t, response, `"type":"upstream_error"`)
						} else {
							require.Equal(t, http.StatusBadGateway, recorder.Code)
						}
						require.Empty(t, usageRepo.logs, "empty failed attempts must not create billable usage")
					}
				}
			})
		}
	}
}
