package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

type pendingUsageWriterStub struct {
	UsageLogRepository
	log        *UsageLog
	err        error
	contextErr error
}

func (s *pendingUsageWriterStub) UpsertPendingUsage(ctx context.Context, log *UsageLog) error {
	s.log = log
	s.contextErr = ctx.Err()
	return s.err
}

func TestRecordPendingUsageIsDetachedAndHasNoFinancialOrEstimateEffects(t *testing.T) {
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), ctxkey.ClientRequestID, "server-issued-request"))
	cancel()
	repo := &pendingUsageWriterStub{}
	svc := &GatewayService{usageLogRepo: repo}
	groupID, estimate := int64(4), 9876
	start := time.Now().Add(-time.Minute)
	input := &RecordUsageInput{
		Result: &ForwardResult{RequestID: "provider-id", Model: "mapped-gemini", UpstreamModel: "upstream-gemini", Stream: true, Duration: time.Second,
			InputTokensSource: UsageTokenSourceUnreported, OutputTokensSource: UsageTokenSourceEstimated,
			Usage: ClaudeUsage{InputTokens: 9000, OutputTokens: 8000}, DeliveredOutputTokens: &estimate},
		User: &User{ID: 1, Balance: 123}, APIKey: &APIKey{ID: 2, UserID: 1, GroupID: &groupID, Key: "secret-not-persisted"},
		Account: &Account{ID: 3}, PricingAt: start,
		ChannelUsageFields: ChannelUsageFields{OriginalModel: "requested-gemini"},
		InboundEndpoint:    "/v1/chat/completions", UpstreamEndpoint: "/v1beta/models",
	}
	require.NoError(t, svc.RecordPendingUsage(ctx, input))
	require.NoError(t, repo.contextErr)
	require.Equal(t, "client:server-issued-request", repo.log.RequestID)
	require.Equal(t, "requested-gemini", repo.log.RequestedModel)
	require.Equal(t, UsageBillingStatusPendingReview, repo.log.BillingStatus)
	require.Equal(t, UsagePendingReasonUpstreamMissing, repo.log.BillingPendingReason)
	require.Equal(t, UsageTokenSourceUnreported, repo.log.InputTokensSource)
	require.Equal(t, UsageTokenSourceUnreported, repo.log.OutputTokensSource)
	require.Zero(t, repo.log.InputTokens)
	require.Zero(t, repo.log.OutputTokens)
	require.Zero(t, repo.log.ActualCost)
	require.Zero(t, repo.log.TotalCost)
	require.Nil(t, repo.log.User)
	require.Nil(t, repo.log.APIKey)
	require.Nil(t, repo.log.Account)
	require.Equal(t, start, repo.log.CreatedAt)
	require.Equal(t, 123.0, input.User.Balance)
	require.Zero(t, input.APIKey.QuotaUsed)
}

func TestRecordPendingUsagePreservesOnlyReportedObservations(t *testing.T) {
	repo := &pendingUsageWriterStub{}
	svc := &GatewayService{usageLogRepo: repo}
	input := &RecordUsageInput{
		Result: &ForwardResult{Model: "gemini", InputTokensSource: UsageTokenSourceUpstream, OutputTokensSource: UsageTokenSourceUnreported,
			Usage: ClaudeUsage{InputTokens: 0, CacheReadInputTokens: 20, OutputTokens: 17}},
		User: &User{ID: 1}, APIKey: &APIKey{ID: 2, UserID: 1}, Account: &Account{ID: 3},
	}
	require.NoError(t, svc.RecordPendingUsage(context.Background(), input))
	require.Equal(t, UsageTokenSourceUpstream, repo.log.InputTokensSource, "reported zero is evidence")
	require.Zero(t, repo.log.InputTokens)
	require.Equal(t, 20, repo.log.CacheReadTokens)
	require.Zero(t, repo.log.OutputTokens, "unreported numeric values must not masquerade as measurements")
	input.Result.InputTokensSource = UsageTokenSourceUnreported
	input.Result.OutputTokensSource = UsageTokenSourceUpstream
	require.NoError(t, svc.RecordPendingUsage(context.Background(), input))
	require.Zero(t, repo.log.CacheReadTokens)
	require.Equal(t, 17, repo.log.OutputTokens)
	require.Equal(t, UsageTokenSourceUpstream, repo.log.OutputTokensSource)
}

func TestRecordPendingUsageRejectsMismatchedOwnershipAndSurfacesPersistenceFailure(t *testing.T) {
	repo := &pendingUsageWriterStub{}
	svc := &GatewayService{usageLogRepo: repo}
	input := &RecordUsageInput{Result: &ForwardResult{Model: "gemini"}, User: &User{ID: 1}, APIKey: &APIKey{ID: 2, UserID: 99}, Account: &Account{ID: 3}}
	require.ErrorContains(t, svc.RecordPendingUsage(context.Background(), input), "identity mismatch")
	require.Nil(t, repo.log)
	input.APIKey.UserID = 1
	repo.err = errors.New("database unavailable")
	require.ErrorIs(t, svc.RecordPendingUsage(context.Background(), input), repo.err)
}
