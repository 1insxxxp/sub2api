//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type emptyResponseClaimRepoStub struct {
	evaluation *EmptyResponseClaimEvaluation
	recent     []EmptyResponseRecentCandidate
	loadErr    error
	dailyCount int
	claim      *EmptyResponseClaim
	created    bool
	createErr  error
	createIn   *EmptyResponseClaimCreateInput
	dayStart   time.Time
	dayEnd     time.Time
}

func (s *emptyResponseClaimRepoStub) LoadEvaluation(_ context.Context, _, _ int64) (*EmptyResponseClaimEvaluation, error) {
	return s.evaluation, s.loadErr
}

func (s *emptyResponseClaimRepoStub) ListRecentEvaluations(_ context.Context, _ int64, _, _ time.Time, _ int) ([]EmptyResponseRecentCandidate, error) {
	return s.recent, s.loadErr
}

func (s *emptyResponseClaimRepoStub) CountUserClaims(_ context.Context, _ int64, start, end time.Time) (int, error) {
	s.dayStart = start
	s.dayEnd = end
	return s.dailyCount, nil
}

func (s *emptyResponseClaimRepoStub) Create(_ context.Context, input *EmptyResponseClaimCreateInput) (*EmptyResponseClaim, bool, error) {
	s.createIn = input
	return s.claim, s.created, s.createErr
}

type emptyResponseClaimCompensatorStub struct {
	claimID int64
	err     error
}

func (s *emptyResponseClaimCompensatorStub) CompensateApprovedClaim(_ context.Context, claimID int64) error {
	s.claimID = claimID
	return s.err
}

func TestEvaluateEmptyResponseClaimDecisionTable(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	baseUsage := UsageLog{ActualCost: 1.25, CreatedAt: now.Add(-time.Hour)}
	aboveLowOutputLimitUsage := UsageLog{ActualCost: 1.25, OutputTokens: EmptyResponseClaimLowOutputTokenLimit + 1, CreatedAt: baseUsage.CreatedAt}
	baseGroup := Group{EmptyResponseCompensationEnabled: true}
	pureEmpty := &ResponseOutcome{
		HTTPStatus:        200,
		UpstreamStatus:    200,
		StreamCompleted:   true,
		DisconnectSource:  DisconnectSourceNone,
		UpstreamErrorKind: UpstreamErrorNone,
		CollectorVersion:  ResponseOutcomeCollectorVersion,
	}

	tests := []struct {
		name       string
		usage      UsageLog
		group      Group
		outcome    *ResponseOutcome
		dailyCount int
		status     string
		reason     string
	}{
		{name: "pure 200 empty response", usage: baseUsage, group: baseGroup, outcome: pureEmpty, status: EmptyResponseClaimApproved, reason: EmptyResponseReasonPureEmpty},
		{name: "low output ignores upstream 5xx", usage: baseUsage, group: baseGroup, outcome: &ResponseOutcome{HTTPStatus: 502, UpstreamStatus: 502, UpstreamErrorKind: UpstreamErrorHTTP5xx, CollectorVersion: 1}, status: EmptyResponseClaimApproved, reason: EmptyResponseReasonLowOutput},
		{name: "low output ignores upstream timeout", usage: baseUsage, group: baseGroup, outcome: &ResponseOutcome{HTTPStatus: 504, UpstreamStatus: 504, DisconnectSource: DisconnectSourceUpstream, UpstreamErrorKind: UpstreamErrorTimeout, CollectorVersion: 1}, status: EmptyResponseClaimApproved, reason: EmptyResponseReasonLowOutput},
		{name: "low output ignores interrupted stream", usage: baseUsage, group: baseGroup, outcome: &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, DisconnectSource: DisconnectSourceUpstream, UpstreamErrorKind: UpstreamErrorProtocol, CollectorVersion: 1}, status: EmptyResponseClaimApproved, reason: EmptyResponseReasonLowOutput},
		{name: "low output ignores client cancellation", usage: baseUsage, group: baseGroup, outcome: &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, DisconnectSource: DisconnectSourceClient, CollectorVersion: 1}, status: EmptyResponseClaimApproved, reason: EmptyResponseReasonLowOutput},
		{name: "low output ignores client cancellation after terminal signal", usage: baseUsage, group: baseGroup, outcome: &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true, DisconnectSource: DisconnectSourceClient, CollectorVersion: 1}, status: EmptyResponseClaimApproved, reason: EmptyResponseReasonLowOutput},
		{name: "low text output", usage: UsageLog{ActualCost: 1.25, OutputTokens: 10, CreatedAt: baseUsage.CreatedAt}, group: baseGroup, outcome: &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, HasText: true, StreamCompleted: true, CollectorVersion: 1}, status: EmptyResponseClaimApproved, reason: EmptyResponseReasonLowOutput},
		{name: "low output without response evidence", usage: UsageLog{ActualCost: 1.25, OutputTokens: 10, CreatedAt: baseUsage.CreatedAt}, group: baseGroup, outcome: nil, status: EmptyResponseClaimApproved, reason: EmptyResponseReasonLowOutput},
		{name: "zero output is low output", usage: UsageLog{ActualCost: 1.25, OutputTokens: 0, CreatedAt: baseUsage.CreatedAt}, group: baseGroup, outcome: nil, status: EmptyResponseClaimApproved, reason: EmptyResponseReasonLowOutput},
		{name: "low output overrides client cancellation", usage: UsageLog{ActualCost: 1.25, OutputTokens: 7, CreatedAt: baseUsage.CreatedAt}, group: baseGroup, outcome: &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, HasText: true, OutputBytes: 807, StreamCompleted: false, DisconnectSource: DisconnectSourceClient, CollectorVersion: 1}, status: EmptyResponseClaimApproved, reason: EmptyResponseReasonLowOutput},
		{name: "text output above low output limit", usage: aboveLowOutputLimitUsage, group: baseGroup, outcome: &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, HasText: true, StreamCompleted: true, CollectorVersion: 1}, status: EmptyResponseClaimRejected, reason: EmptyResponseReasonEffectiveOutput},
		{name: "effective output above limit overrides conflicting upstream signal", usage: aboveLowOutputLimitUsage, group: baseGroup, outcome: &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, HasText: true, StreamCompleted: true, DisconnectSource: DisconnectSourceUpstream, UpstreamErrorKind: UpstreamErrorProtocol, CollectorVersion: 1}, status: EmptyResponseClaimRejected, reason: EmptyResponseReasonEffectiveOutput},
		{name: "tool output above low output limit", usage: aboveLowOutputLimitUsage, group: baseGroup, outcome: &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, HasToolCall: true, StreamCompleted: true, CollectorVersion: 1}, status: EmptyResponseClaimRejected, reason: EmptyResponseReasonEffectiveOutput},
		{name: "reasoning output above low output limit", usage: aboveLowOutputLimitUsage, group: baseGroup, outcome: &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, HasReasoning: true, StreamCompleted: true, CollectorVersion: 1}, status: EmptyResponseClaimRejected, reason: EmptyResponseReasonEffectiveOutput},
		{name: "media output above low output limit", usage: aboveLowOutputLimitUsage, group: baseGroup, outcome: &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, HasMedia: true, StreamCompleted: true, CollectorVersion: 1}, status: EmptyResponseClaimRejected, reason: EmptyResponseReasonEffectiveOutput},
		{name: "no charge", usage: UsageLog{ActualCost: 0, CreatedAt: baseUsage.CreatedAt}, group: baseGroup, outcome: pureEmpty, status: EmptyResponseClaimRejected, reason: EmptyResponseReasonNotCharged},
		{name: "already compensated", usage: UsageLog{ActualCost: 1, CompensatedCost: 1, CreatedAt: baseUsage.CreatedAt}, group: baseGroup, outcome: pureEmpty, status: EmptyResponseClaimRejected, reason: EmptyResponseReasonAlreadyCompensated},
		{name: "output tokens over empty threshold", usage: UsageLog{ActualCost: 1, OutputTokens: EmptyResponseClaimLowOutputTokenLimit + 1, CreatedAt: baseUsage.CreatedAt}, group: baseGroup, outcome: pureEmpty, status: EmptyResponseClaimRejected, reason: EmptyResponseReasonEffectiveOutput},
		{name: "output tokens at empty threshold", usage: UsageLog{ActualCost: 1, OutputTokens: 10, CreatedAt: baseUsage.CreatedAt}, group: baseGroup, outcome: pureEmpty, status: EmptyResponseClaimApproved, reason: EmptyResponseReasonPureEmpty},
		{name: "low output requires group enabled", usage: baseUsage, group: Group{}, outcome: pureEmpty, status: EmptyResponseClaimRejected, reason: EmptyResponseReasonGroupDisabled},
		{name: "older than seven days", usage: UsageLog{ActualCost: 1, CreatedAt: now.Add(-7*24*time.Hour - time.Second)}, group: baseGroup, outcome: pureEmpty, status: EmptyResponseClaimRejected, reason: EmptyResponseReasonWindowExpired},
		{name: "exactly seven days remains eligible", usage: UsageLog{ActualCost: 1, CreatedAt: now.Add(-7 * 24 * time.Hour)}, group: baseGroup, outcome: pureEmpty, status: EmptyResponseClaimApproved, reason: EmptyResponseReasonPureEmpty},
		{name: "missing evidence with invalid token count", usage: UsageLog{ActualCost: 1, OutputTokens: -1, CreatedAt: baseUsage.CreatedAt}, group: baseGroup, outcome: nil, status: EmptyResponseClaimManualReview, reason: EmptyResponseReasonMissingEvidence},
		{name: "low output ignores conflicting evidence", usage: baseUsage, group: baseGroup, outcome: &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true, DisconnectSource: DisconnectSourceUpstream, UpstreamErrorKind: UpstreamErrorProtocol, CollectorVersion: 1}, status: EmptyResponseClaimApproved, reason: EmptyResponseReasonLowOutput},
		{name: "claim fifteen today still eligible", usage: baseUsage, group: baseGroup, outcome: pureEmpty, dailyCount: 14, status: EmptyResponseClaimApproved, reason: EmptyResponseReasonPureEmpty},
		{name: "claim sixteen today", usage: baseUsage, group: baseGroup, outcome: pureEmpty, dailyCount: 15, status: EmptyResponseClaimManualReview, reason: EmptyResponseReasonDailyLimit},
		{name: "historical row without outcome under token limit", usage: UsageLog{ActualCost: 1, CreatedAt: now.Add(-23 * time.Hour)}, group: baseGroup, outcome: nil, status: EmptyResponseClaimApproved, reason: EmptyResponseReasonLowOutput},
	}

	require.Equal(t, 7*24*time.Hour, EmptyResponseClaimWindow)
	require.Equal(t, 15, EmptyResponseClaimDailyLimit)
	require.Equal(t, 10, EmptyResponseClaimLowOutputTokenLimit)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := EvaluateEmptyResponseClaim(now, tt.usage, tt.outcome, tt.group, tt.dailyCount)
			require.Equal(t, tt.status, decision.Status)
			require.Equal(t, tt.reason, decision.ReasonCode)
			require.Equal(t, EmptyResponseClaimRuleVersion, decision.RuleVersion)
		})
	}
}

func TestEmptyResponseClaimServiceCreatesServerEvaluatedClaimAndCompensatesApproval(t *testing.T) {
	now := time.Date(2026, 8, 7, 23, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	outcome := &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true, CollectorVersion: 1}
	repo := &emptyResponseClaimRepoStub{
		evaluation: &EmptyResponseClaimEvaluation{
			Usage:   UsageLog{ID: 100, UserID: 7, APIKeyID: 8, AccountID: 9, ActualCost: 1.25, CreatedAt: now.Add(-time.Hour)},
			Outcome: outcome,
			Group:   Group{ID: 10, EmptyResponseCompensationEnabled: true},
		},
		claim:   &EmptyResponseClaim{ID: 200, Status: EmptyResponseClaimApproved},
		created: true,
	}
	compensator := &emptyResponseClaimCompensatorStub{}
	svc := NewEmptyResponseClaimService(repo, compensator)
	svc.now = func() time.Time { return now }

	claim, err := svc.Submit(context.Background(), EmptyResponseClaimSubmitInput{UserID: 7, UsageLogID: 100, UserReason: "empty reply"})

	require.NoError(t, err)
	require.Equal(t, int64(200), claim.ID)
	require.NotNil(t, repo.createIn)
	require.Equal(t, EmptyResponseClaimApproved, repo.createIn.Decision.Status)
	require.Equal(t, EmptyResponseReasonPureEmpty, repo.createIn.Decision.ReasonCode)
	require.Equal(t, 1.25, repo.createIn.OriginalActualCost)
	require.Equal(t, "empty reply", repo.createIn.UserReason)
	require.Equal(t, int64(200), compensator.claimID)
	shanghai, loadErr := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, loadErr)
	require.Equal(t, time.Date(2026, 8, 7, 0, 0, 0, 0, shanghai), repo.dayStart)
	require.Equal(t, repo.dayStart.AddDate(0, 0, 1), repo.dayEnd)
}

func TestEmptyResponseClaimServiceRejectsExistingClaimWithoutCompensatingAgain(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	repo := &emptyResponseClaimRepoStub{
		evaluation: &EmptyResponseClaimEvaluation{
			Usage:   UsageLog{ID: 100, UserID: 7, APIKeyID: 8, AccountID: 9, ActualCost: 1, CreatedAt: now.Add(-time.Hour)},
			Outcome: &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true, CollectorVersion: 1},
			Group:   Group{EmptyResponseCompensationEnabled: true},
		},
		claim:   &EmptyResponseClaim{ID: 201, Status: EmptyResponseClaimCompensated},
		created: false,
	}
	compensator := &emptyResponseClaimCompensatorStub{err: errors.New("must not be called")}
	svc := NewEmptyResponseClaimService(repo, compensator)
	svc.now = func() time.Time { return now }

	claim, err := svc.Submit(context.Background(), EmptyResponseClaimSubmitInput{UserID: 7, UsageLogID: 100})

	require.Same(t, repo.claim, claim)
	require.ErrorIs(t, err, ErrEmptyResponseClaimAlreadyExists)
	require.Zero(t, compensator.claimID)
}

func TestEmptyResponseClaimServiceRetriesAnExistingApprovedClaim(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	repo := &emptyResponseClaimRepoStub{
		evaluation: &EmptyResponseClaimEvaluation{
			Usage:   UsageLog{ID: 100, UserID: 7, APIKeyID: 8, AccountID: 9, ActualCost: 1, CreatedAt: now.Add(-time.Hour)},
			Outcome: &ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true, CollectorVersion: 1},
			Group:   Group{EmptyResponseCompensationEnabled: true},
		},
		claim:   &EmptyResponseClaim{ID: 202, Status: EmptyResponseClaimApproved, OriginalActualCost: 1},
		created: false,
	}
	compensator := &emptyResponseClaimCompensatorStub{}
	svc := NewEmptyResponseClaimService(repo, compensator)
	svc.now = func() time.Time { return now }

	claim, err := svc.Submit(context.Background(), EmptyResponseClaimSubmitInput{UserID: 7, UsageLogID: 100})

	require.NoError(t, err)
	require.Equal(t, int64(202), compensator.claimID)
	require.Equal(t, EmptyResponseClaimCompensated, claim.Status)
	require.Equal(t, 1.0, claim.BalanceRefund)
}
