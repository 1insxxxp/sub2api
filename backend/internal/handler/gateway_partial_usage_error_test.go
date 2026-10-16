//go:build unit

package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type partialChatUsageRepo struct {
	service.UsageLogRepository
	logs []*service.UsageLog
}

func (r *partialChatUsageRepo) CreateBestEffort(_ context.Context, usage *service.UsageLog) error {
	clone := *usage
	r.logs = append(r.logs, &clone)
	return nil
}

type partialChatUpstream struct {
	service.HTTPUpstream
	accountIDs []int64
	failure    string
	cancel     context.CancelFunc
}

func (u *partialChatUpstream) DoWithTLS(_ *http.Request, _ string, accountID int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.accountIDs = append(u.accountIDs, accountID)
	if len(u.accountIDs) == 1 && u.failure != "initial transport" {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"X-Request-Id": {"first-billable-round"}},
			Body: io.NopCloser(strings.NewReader(
				"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_partial\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-5\",\"content\":[],\"usage\":{\"input_tokens\":12,\"output_tokens\":0}}}\n\n" +
					"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
					"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"partial answer\"}}\n\n" +
					"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
					"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"max_tokens\"},\"usage\":{\"output_tokens\":3}}\n\n" +
					"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")),
		}, nil
	}
	switch u.failure {
	case "continuation 5xx":
		return &http.Response{StatusCode: http.StatusBadGateway, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"message":"provider unavailable"}}`))}, nil
	case "client cancellation":
		u.cancel()
		return nil, context.Canceled
	default:
		return nil, errors.New("upstream connection reset")
	}
}

// Exercise scheduling, continuation, the real Chat Completions handler and
// RecordUsage together: an interrupted continuation must retain the first
// round's usage without replaying already delivered content on another account.
func TestGatewayChatContinuationFailureRetainsPartialUsageOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, failure := range []string{"continuation 5xx", "continuation transport", "client cancellation", "initial transport"} {
		t.Run(failure, func(t *testing.T) {
			groupID := int64(9700)
			group := &service.Group{
				ID: groupID, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive, RateMultiplier: 1,
				ModelAliases: []domain.ModelAlias{{Alias: "story-continue", UpstreamModel: "claude-sonnet-4-5", AntiTruncation: domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1, MaxDurationSeconds: 30}}},
			}
			account := &service.Account{
				ID: 9701, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
				Status: service.StatusActive, Schedulable: true, Concurrency: 1,
				Credentials:   map[string]any{"api_key": "local-test-only", "base_url": "https://api.anthropic.com"},
				AccountGroups: []service.AccountGroup{{AccountID: 9701, GroupID: groupID}},
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			upstream := &partialChatUpstream{failure: failure, cancel: cancel}
			usageRepo := &partialChatUsageRepo{}
			cfg := &config.Config{RunMode: config.RunModeSimple}
			cfg.Default.RateMultiplier = 1
			snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: []*service.Account{account}}, nil, nil, nil, nil)
			gateway := service.NewGatewayService(
				nil, &fakeGroupRepo{group: group}, usageRepo, nil, nil, nil, nil, nil, cfg,
				snapshot, nil, service.NewBillingService(cfg, nil), nil, nil, nil, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil,
				&service.TLSFingerprintProfileService{}, nil, nil, nil, nil, nil,
			)
			billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(billing.Stop)
			h := &GatewayHandler{
				gatewayService: gateway, billingCacheService: billing, cfg: cfg,
				concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0),
			}
			apiKey := &service.APIKey{
				ID: 9702, UserID: 9703, GroupID: &groupID, Group: group, Status: service.StatusActive,
				User: &service.User{ID: 9703, Concurrency: 10, Balance: 100},
			}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			ctx = context.WithValue(ctx, ctxkey.Group, group)
			body := `{"model":"story-continue","messages":[{"role":"user","content":"hello"}],"stream":true,"max_tokens":100}`
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body)).WithContext(ctx)
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set(string(middleware.ContextKeyAPIKey), apiKey)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})

			h.ChatCompletions(c)

			if failure == "initial transport" {
				require.Equal(t, []int64{account.ID}, upstream.accountIDs)
				require.Empty(t, usageRepo.logs, "an error without a forward result has no usage to settle")
				return
			}
			require.Equal(t, []int64{account.ID, account.ID}, upstream.accountIDs, "only the original round and its continuation may run")
			require.Equal(t, 1, strings.Count(recorder.Body.String(), `"content":"partial answer"`), "delivered content must not be replayed")
			require.Len(t, usageRepo.logs, 1, "partial usage must be recorded exactly once")
			usage := usageRepo.logs[0]
			require.Equal(t, 12, usage.InputTokens)
			require.Equal(t, 3, usage.UpstreamOutputTokens)
			require.Positive(t, usage.OutputTokens, "the first round's delivered text remains billable")
			require.Equal(t, account.ID, usage.AccountID)
			require.NotNil(t, usage.Outcome)
			require.NotEmpty(t, usage.Outcome.UpstreamErrorKind, "the continuation error must remain visible in the outcome")
		})
	}
}
