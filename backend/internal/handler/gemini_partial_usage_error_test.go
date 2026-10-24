//go:build unit

package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type geminiPartialBillingRepo struct {
	service.UsageBillingRepository
	commands []*service.UsageBillingCommand
}

func (r *geminiPartialBillingRepo) Apply(_ context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	copy := *cmd
	r.commands = append(r.commands, &copy)
	return &service.UsageBillingApplyResult{Applied: true}, nil
}

type geminiPartialBody struct {
	reader *strings.Reader
	err    error
	cancel context.CancelFunc
}

func (b *geminiPartialBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	if err == io.EOF && b.err != nil {
		if b.cancel != nil {
			b.cancel()
		}
		return n, b.err
	}
	return n, err
}

func (*geminiPartialBody) Close() error { return nil }

type geminiPartialUpstream struct {
	service.HTTPUpstream
	sse             string
	readErr         error
	cancel          context.CancelFunc
	accounts        []int64
	initialCancel   bool
	metadataTimeout bool
}

func (u *geminiPartialUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.accounts = append(u.accounts, accountID)
	if u.initialCancel {
		u.cancel()
		return nil, context.Canceled
	}
	if u.metadataTimeout && len(u.accounts) == 1 {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"text/event-stream"}},
			Body:       io.NopCloser(io.MultiReader(strings.NewReader("data: {\"usageMetadata\":{\"promptTokenCount\":99999,\"candidatesTokenCount\":99999}}\n\n"), geminiPartialWaitForCancel{ctx: req.Context()})),
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}, "X-Request-Id": {"gemini-partial-only-once"}},
		Body:       &geminiPartialBody{reader: strings.NewReader(u.sse), err: u.readErr, cancel: u.cancel},
	}, nil
}

type geminiPartialWaitForCancel struct{ ctx context.Context }

func (r geminiPartialWaitForCancel) Read([]byte) (int, error) {
	<-r.ctx.Done()
	return 0, r.ctx.Err()
}

type geminiPartialTimeoutSettings struct{ service.SettingRepository }

func (*geminiPartialTimeoutSettings) GetValue(context.Context, string) (string, error) {
	return `{"enabled":true,"default":{"enabled":true,"target_seconds":1,"switch_seconds":1,"hard_cap_seconds":5}}`, nil
}

type geminiPartialRejectWriter struct{ gin.ResponseWriter }

func (w *geminiPartialRejectWriter) Write([]byte) (int, error)       { return 0, io.ErrClosedPipe }
func (w *geminiPartialRejectWriter) WriteString(string) (int, error) { return 0, io.ErrClosedPipe }

type geminiPartialRejectAnswerWriter struct {
	gin.ResponseWriter
	rejectedText string
}

func (w *geminiPartialRejectAnswerWriter) rejects(data string) bool {
	rejected := w.rejectedText
	if rejected == "" {
		rejected = "partial answer"
	}
	return strings.Contains(data, rejected)
}

func (w *geminiPartialRejectAnswerWriter) Write(data []byte) (int, error) {
	if w.rejects(string(data)) {
		return 0, io.ErrClosedPipe
	}
	return w.ResponseWriter.Write(data)
}

func (w *geminiPartialRejectAnswerWriter) WriteString(data string) (int, error) {
	if w.rejects(data) {
		return 0, io.ErrClosedPipe
	}
	return w.ResponseWriter.WriteString(data)
}

type geminiPartialCancelAfterTerminalWriter struct {
	gin.ResponseWriter
	cancel    context.CancelFunc
	written   strings.Builder
	terminal  string
	completed bool
}

func (w *geminiPartialCancelAfterTerminalWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.written.Write(data[:n])
	if strings.HasSuffix(w.written.String(), "\n\n") && strings.Contains(w.written.String(), w.terminal) {
		w.completed = true
	}
	return n, err
}
func (w *geminiPartialCancelAfterTerminalWriter) WriteString(data string) (int, error) {
	return w.Write([]byte(data))
}
func (w *geminiPartialCancelAfterTerminalWriter) Flush() {
	w.ResponseWriter.Flush()
	if w.completed {
		w.cancel()
	}
}

// Exercise all three public protocols, the real Gemini adapter, RecordUsage and the
// normal-mode wallet command. A failed attempt with metadata cannot debit a
// wallet; a partial answer must settle once using only downstream output.
func TestGeminiHandlers_InterruptedUsageBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const usage = `"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":10000,"totalTokenCount":10012}`
	for _, protocol := range []string{"chat", "native", "messages"} {
		for _, tc := range []struct {
			name                       string
			parts                      string
			finish                     string
			readErr                    error
			clientGone                 bool
			wantBilling                bool
			completed                  bool
			omitUsage                  bool
			initialCancel              bool
			rejectWrites               bool
			metadataTimeout            bool
			rejectAnswerAfterThought   bool
			rejectTerminalAfterPartial bool
			cancelAfterTerminal        bool
			upstreamTerminalReadError  bool
			protocolOnly               string
			zeroOutputUsage            bool
		}{
			{name: "partial EOF", parts: `{"text":"partial answer"}`, wantBilling: true},
			{name: "partial EOF without usage", parts: `{"text":"partial answer"}`, wantBilling: true, omitUsage: true},
			{name: "partial reset", parts: `{"text":"partial answer"}`, readErr: io.ErrUnexpectedEOF, wantBilling: true},
			{name: "partial client cancellation", parts: `{"text":"partial answer"}`, readErr: context.Canceled, clientGone: true, wantBilling: true},
			{name: "metadata only cancellation", readErr: context.Canceled, clientGone: true},
			{name: "metadata only cancellation without usage", readErr: context.Canceled, clientGone: true, omitUsage: true},
			{name: "thought only cancellation", parts: `{"text":"private reasoning","thought":true}`, readErr: context.Canceled, clientGone: true},
			{name: "thought only cancellation without usage", parts: `{"text":"private reasoning","thought":true}`, readErr: context.Canceled, clientGone: true, omitUsage: true},
			{name: "cancellation before upstream response", clientGone: true, initialCancel: true},
			{name: "failed downstream write", parts: `{"text":"partial answer"}`, readErr: io.ErrUnexpectedEOF, rejectWrites: true},
			{name: "failed downstream write without usage", parts: `{"text":"partial answer"}`, readErr: io.ErrUnexpectedEOF, rejectWrites: true, omitUsage: true},
			{name: "thought delivered then answer write fails", parts: `{"text":"partial answer"}`, readErr: io.ErrUnexpectedEOF, rejectAnswerAfterThought: true},
			{name: "partial delivered then terminal answer write fails", parts: `{"text":"hidden final answer"}`, finish: `,"finishReason":"STOP"`, wantBilling: true, rejectTerminalAfterPartial: true},
			{name: "thought delivered then terminal answer write fails", parts: `{"text":"partial answer"}`, finish: `,"finishReason":"STOP"`, rejectAnswerAfterThought: true},
			{name: "terminal delivered then client cancellation", parts: `{"text":"partial answer"}`, finish: `,"finishReason":"STOP"`, wantBilling: true, completed: true, cancelAfterTerminal: true},
			{name: "upstream STOP then read reset", parts: `{"text":"partial answer"}`, finish: `,"finishReason":"STOP"`, readErr: io.ErrUnexpectedEOF, wantBilling: true, upstreamTerminalReadError: true},
			{name: "metadata timeout then next account completes", parts: `{"text":"partial answer"}`, finish: `,"finishReason":"STOP"`, wantBilling: true, completed: true, metadataTimeout: true},
			{name: "normal completed", parts: `{"text":"partial answer"}`, finish: `,"finishReason":"STOP"`, wantBilling: true, completed: true},
			{name: "safety failure empty answer", finish: `,"finishReason":"SAFETY"`, zeroOutputUsage: true},
			{name: "safety failure partial answer", parts: `{"text":"partial answer"}`, finish: `,"finishReason":"SAFETY"`, wantBilling: true},
			{name: "normal empty STOP", finish: `,"finishReason":"STOP"`, protocolOnly: "native", zeroOutputUsage: true, wantBilling: true, completed: true},
		} {
			if tc.protocolOnly != "" && tc.protocolOnly != protocol {
				continue
			}
			t.Run(protocol+"/"+tc.name, func(t *testing.T) {
				groupID := int64(9800)
				group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformGemini, Status: service.StatusActive, RateMultiplier: 1}
				account := &service.Account{
					ID: 9801, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey,
					Status: service.StatusActive, Schedulable: true, Concurrency: 1,
					Credentials:   map[string]any{"api_key": "local-test-only"},
					AccountGroups: []service.AccountGroup{{AccountID: 9801, GroupID: groupID}},
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				usageFragment := "," + usage
				providerOutput := 10000
				if tc.zeroOutputUsage {
					usageFragment = `,"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":0,"totalTokenCount":12}`
					providerOutput = 0
				}
				if tc.omitUsage {
					usageFragment = ""
				}
				sse := `data: {"candidates":[{"content":{"parts":[` + tc.parts + `],"role":"model"}` + tc.finish + `}]` + usageFragment + "}\n\n"
				if tc.rejectAnswerAfterThought {
					sse = "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"private reasoning\",\"thought\":true}],\"role\":\"model\"}}]}\n\n" + sse
				}
				if tc.rejectTerminalAfterPartial {
					sse = "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"partial answer\"}],\"role\":\"model\"}}]}\n\n" + sse
				}
				upstream := &geminiPartialUpstream{sse: sse, readErr: tc.readErr, initialCancel: tc.initialCancel, metadataTimeout: tc.metadataTimeout}
				if tc.clientGone {
					upstream.cancel = cancel
				}
				usageRepo := &partialChatUsageRepo{}
				wallet := &geminiPartialBillingRepo{}
				cfg := &config.Config{}
				cfg.Default.RateMultiplier = 1
				cfg.Pricing.DataDir = t.TempDir()
				cfg.Pricing.UpdateIntervalHours = 100000
				require.NoError(t, os.WriteFile(filepath.Join(cfg.Pricing.DataDir, "model_pricing.json"), []byte(`{"gemini-2.5-flash":{"litellm_provider":"gemini","mode":"chat","input_cost_per_token":0.000001,"output_cost_per_token":0.00001}}`), 0o600))
				pricing := service.NewPricingService(cfg, nil)
				require.NoError(t, pricing.Initialize())
				t.Cleanup(pricing.Stop)
				userRepo := &cachedBillingUserRepo{balance: 100}
				billing := service.NewBillingCacheService(nil, userRepo, nil, nil, nil, nil, cfg, nil)
				t.Cleanup(billing.Stop)
				accounts := []*service.Account{account}
				var settings *service.SettingService
				if tc.metadataTimeout {
					other := *account
					other.ID = 9804
					other.AccountGroups = []service.AccountGroup{{AccountID: other.ID, GroupID: groupID}}
					accounts = append(accounts, &other)
					settings = service.NewSettingService(&geminiPartialTimeoutSettings{}, cfg)
				}
				snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: accounts}, nil, nil, nil, nil)
				gateway := service.NewGatewayService(
					nil, &fakeGroupRepo{group: group}, usageRepo, wallet, userRepo, nil, nil, nil, cfg,
					snapshot, nil, service.NewBillingService(cfg, pricing), nil, billing, nil, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil,
					&service.TLSFingerprintProfileService{}, nil, nil, nil, nil, nil,
				)
				h := &GatewayHandler{
					gatewayService: gateway, billingCacheService: billing, cfg: cfg, settingService: settings, maxAccountSwitchesGemini: 2,
					geminiCompatService: service.NewGeminiMessagesCompatService(nil, nil, nil, nil, nil, nil, upstream, nil, cfg, settings),
					concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0),
				}
				apiKey := &service.APIKey{ID: 9802, UserID: 9803, GroupID: &groupID, Group: group, Status: service.StatusActive,
					User: &service.User{ID: 9803, Concurrency: 10, Balance: 100}}
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				if tc.rejectWrites {
					c.Writer = &geminiPartialRejectWriter{ResponseWriter: c.Writer}
				} else if tc.rejectAnswerAfterThought {
					c.Writer = &geminiPartialRejectAnswerWriter{ResponseWriter: c.Writer}
				} else if tc.rejectTerminalAfterPartial {
					c.Writer = &geminiPartialRejectAnswerWriter{ResponseWriter: c.Writer, rejectedText: "hidden final answer"}
				} else if tc.cancelAfterTerminal {
					terminal := map[string]string{"chat": "[DONE]", "messages": "message_stop", "native": "\"finishReason\":\"STOP\""}[protocol]
					c.Writer = &geminiPartialCancelAfterTerminalWriter{ResponseWriter: c.Writer, cancel: cancel, terminal: terminal}
				}
				ctx = context.WithValue(ctx, ctxkey.Group, group)
				path := "/v1/chat/completions"
				body := `{"model":"gemini-2.5-flash","messages":[{"role":"user","content":"hello"}],"stream":true}`
				if protocol == "native" {
					path = "/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse"
					body = `{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`
					c.Params = gin.Params{{Key: "modelAction", Value: "/gemini-2.5-flash:streamGenerateContent"}}
				} else if protocol == "messages" {
					path = "/v1/messages"
					body = `{"model":"gemini-2.5-flash","messages":[{"role":"user","content":"hello"}],"stream":true,"max_tokens":100}`
				}
				c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body)).WithContext(ctx)
				c.Request.Header.Set("Content-Type", "application/json")
				c.Set(string(middleware.ContextKeyAPIKey), apiKey)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})
				if protocol == "native" {
					h.GeminiV1BetaModels(c)
				} else if protocol == "messages" {
					h.Messages(c)
				} else {
					h.ChatCompletions(c)
				}

				if tc.metadataTimeout {
					require.Len(t, upstream.accounts, 2, "metadata-only first-output timeout must switch once")
					require.NotEqual(t, upstream.accounts[0], upstream.accounts[1])
				} else {
					require.Equal(t, []int64{account.ID}, upstream.accounts, "output and client cancellation must never replay on another account; response=%d %s", recorder.Code, recorder.Body.String())
				}
				if tc.rejectAnswerAfterThought {
					require.Contains(t, recorder.Body.String(), "private reasoning", "reasoning reached the downstream writer")
					require.NotContains(t, recorder.Body.String(), "partial answer", "answer write was rejected")
				}
				if tc.rejectTerminalAfterPartial {
					require.Contains(t, recorder.Body.String(), "partial answer")
					require.NotContains(t, recorder.Body.String(), "hidden final answer")
				}
				if !tc.wantBilling {
					require.Empty(t, wallet.commands, "metadata or reasoning without delivered answer must not mutate the wallet")
					require.Empty(t, usageRepo.logs, "failed outputless attempts must never enter customer usage settlement")
					return
				}
				require.Len(t, wallet.commands, 1, "settle the delivered response exactly once in normal billing mode")
				require.Len(t, usageRepo.logs, 1)
				log := usageRepo.logs[0]
				if !tc.omitUsage {
					require.Equal(t, 12, log.InputTokens)
					require.Equal(t, providerOutput, log.UpstreamOutputTokens, "keep provider measurement separately")
				} else {
					require.Positive(t, log.InputTokens, "delivered Gemini responses without provider usage must settle the prepared input estimate")
					require.Zero(t, log.UpstreamOutputTokens, "unknown provider usage must not be fabricated")
				}
				require.Equal(t, upstream.accounts[len(upstream.accounts)-1], log.AccountID)
				require.Positive(t, wallet.commands[0].BalanceCost)
				if tc.completed || (tc.upstreamTerminalReadError && protocol == "native") {
					require.Equal(t, providerOutput, log.OutputTokens, "normal complete responses retain existing provider usage billing")
				} else {
					collector := service.NewDownstreamOutputTokenCollector("gemini-2.5-flash")
					collector.ObserveWritten([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial answer\"}}]}\n\n"))
					require.Positive(t, collector.TokenCount())
					require.Equal(t, collector.TokenCount(), log.OutputTokens, "never charge the provider's unseen 10000 output tokens")
					require.NotNil(t, log.Outcome)
					require.False(t, log.Outcome.StreamCompleted)
				}
				require.InDelta(t, float64(log.InputTokens)*0.000001+float64(log.OutputTokens)*0.00001, wallet.commands[0].BalanceCost, 0.00000001,
					"wallet charge must use customer-visible output, not the larger provider output")
			})
		}
	}
}
