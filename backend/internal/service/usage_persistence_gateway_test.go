//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type durableUsageBillingStub struct {
	openAIRecordUsageBillingRepoStub
}

func (*durableUsageBillingStub) UsagePersistenceEnabled() bool { return true }

func TestDurableUsageGateway_SnapshotIsTheOnlyDetailWriter(t *testing.T) {
	for _, gateway := range []string{"gateway", "openai"} {
		for _, state := range []string{"committed", "replayed", "commit_ack_lost", "simple_key_windows"} {
			t.Run(gateway+"/"+state, func(t *testing.T) {
				logs := &openAIRecordUsageBestEffortLogRepoStub{}
				billing := &durableUsageBillingStub{}
				if state == "replayed" {
					billing.result = &UsageBillingApplyResult{Applied: false}
				}
				if state == "commit_ack_lost" {
					billing.err = errors.New("commit acknowledgement lost")
				}
				key := &APIKey{ID: 11, Quota: 100, RateLimit5h: 10}
				user := &User{ID: 12}
				account := &Account{ID: 13, Type: AccountTypeAPIKey}
				delivered := 5
				outcome := &ResponseOutcome{HasText: true, OutputBytes: 16, DisconnectSource: DisconnectSourceClient}
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				var err error
				if gateway == "gateway" {
					svc := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, nil, nil)
					if state == "simple_key_windows" {
						svc.cfg.RunMode = config.RunModeSimple
						svc.cfg.SimpleModeKeyRateLimitEnabled = true
					}
					err = svc.RecordUsage(ctx, &RecordUsageInput{
						Result: &ForwardResult{RequestID: "durable-gateway", Model: "claude-sonnet-4", Stream: true,
							ClientDisconnect: true, DeliveredOutputTokens: &delivered, Outcome: outcome,
							Usage: ClaudeUsage{InputTokens: 1200, OutputTokens: 900}, Duration: time.Second},
						APIKey: key, User: user, Account: account, APIKeyService: &apiKeyQuotaUpdaterStub{},
					})
				} else {
					svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, billing, nil, nil, nil)
					if state == "simple_key_windows" {
						svc.cfg.RunMode = config.RunModeSimple
						svc.cfg.SimpleModeKeyRateLimitEnabled = true
					}
					err = svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
						Result: &OpenAIForwardResult{RequestID: "durable-openai", Model: "gpt-5.1", Stream: true,
							ClientDisconnect: true, DeliveredOutputTokens: &delivered, Outcome: outcome,
							Usage: OpenAIUsage{InputTokens: 1200, OutputTokens: 900}, Duration: time.Second},
						APIKey: key, User: user, Account: account, APIKeyService: &apiKeyQuotaUpdaterStub{},
					})
				}
				if billing.err != nil {
					require.ErrorIs(t, err, billing.err)
				} else {
					require.NoError(t, err)
				}
				require.Equal(t, 1, billing.calls)
				require.NoError(t, billing.lastCtxErr)
				require.Zero(t, logs.bestEffortCalls, "durable writer must not race a legacy usage insert")
				require.Zero(t, logs.createCalls, "uncertain commit must not insert a zero-cost placeholder")
				cmd := billing.lastCmd
				require.NotNil(t, cmd.UsageLog, "billing transaction must receive the complete usage snapshot")
				require.Equal(t, cmd.RequestID, cmd.UsageLog.RequestID)
				require.Equal(t, 1200, cmd.UsageLog.InputTokens)
				require.Equal(t, delivered, cmd.UsageLog.OutputTokens)
				require.Equal(t, 900, cmd.UsageLog.UpstreamOutputTokens)
				require.NotNil(t, cmd.UsageLog.Outcome)
				require.False(t, cmd.UsageLog.CreatedAt.IsZero())
				require.Positive(t, cmd.UsageLog.ActualCost)
				if state == "simple_key_windows" {
					require.Zero(t, cmd.BalanceCost)
					require.Zero(t, cmd.SubscriptionCost)
					require.Zero(t, cmd.AccountQuotaCost)
					require.Zero(t, cmd.APIKeyQuotaCost)
					require.Positive(t, cmd.APIKeyRateLimitCost)
				} else {
					require.Equal(t, cmd.BalanceCost, cmd.UsageLog.ActualCost)
				}
			})
		}
	}
}

func TestDurableUsageGateway_PureSimpleModeKeepsUnbilledLogging(t *testing.T) {
	logs := &openAIRecordUsageBestEffortLogRepoStub{}
	billing := &durableUsageBillingStub{}
	svc := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, nil, nil)
	svc.cfg.RunMode = config.RunModeSimple
	require.NoError(t, svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 12, OutputTokens: 5}},
		APIKey: &APIKey{ID: 11}, User: &User{ID: 12}, Account: &Account{ID: 13},
	}))
	require.Zero(t, billing.calls)
	require.Equal(t, 1, logs.bestEffortCalls)
}

type nearlyExpiredUsageWriter struct {
	UsageLogRepository
	remaining time.Duration
}

func (*nearlyExpiredUsageWriter) CreateBestEffort(context.Context, *UsageLog) error {
	time.Sleep(14 * time.Second)
	return context.DeadlineExceeded
}

func (r *nearlyExpiredUsageWriter) Create(ctx context.Context, _ *UsageLog) (bool, error) {
	deadline, _ := ctx.Deadline()
	r.remaining = time.Until(deadline)
	return true, ctx.Err()
}

func TestUsagePersistenceFallback_AlwaysHasIndependentBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := &nearlyExpiredUsageWriter{}
		writeUsageLogBestEffort(context.Background(), repo, &UsageLog{RequestID: "fresh-fallback"}, "test")
		require.Equal(t, postUsageBillingTimeout, repo.remaining, "fallback cannot inherit a nearly expired batch deadline")
	})
}
