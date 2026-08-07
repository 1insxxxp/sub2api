package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	EmptyResponseClaimEvaluating   = "evaluating"
	EmptyResponseClaimManualReview = "manual_review"
	EmptyResponseClaimApproved     = "approved"
	EmptyResponseClaimRejected     = "rejected"
	EmptyResponseClaimCompensated  = "compensated"

	EmptyResponseClaimRuleVersion = 1
	EmptyResponseClaimDailyLimit  = 10
)

var (
	ErrEmptyResponseClaimNotFound      = infraerrors.NotFound("EMPTY_RESPONSE_CLAIM_NOT_FOUND", "usage record is not available for an empty response claim")
	ErrEmptyResponseClaimAlreadyExists = infraerrors.Conflict("EMPTY_RESPONSE_CLAIM_ALREADY_EXISTS", "an empty response claim already exists for this usage record")
	ErrEmptyResponseClaimInvalidInput  = infraerrors.BadRequest("EMPTY_RESPONSE_CLAIM_INVALID_INPUT", "user_id and usage_log_id must be greater than zero")
)

const (
	EmptyResponseReasonPureEmpty           = "pure_empty"
	EmptyResponseReasonUpstreamHTTP5xx     = "upstream_http_5xx"
	EmptyResponseReasonUpstreamTimeout     = "upstream_timeout"
	EmptyResponseReasonUpstreamInterrupted = "upstream_interrupted"
	EmptyResponseReasonClientCancelled     = "client_cancelled"
	EmptyResponseReasonEffectiveOutput     = "effective_output"
	EmptyResponseReasonNotCharged          = "not_charged"
	EmptyResponseReasonAlreadyCompensated  = "already_compensated"
	EmptyResponseReasonGroupDisabled       = "group_disabled"
	EmptyResponseReasonWindowExpired       = "claim_window_expired"
	EmptyResponseReasonMissingEvidence     = "missing_evidence"
	EmptyResponseReasonConflictingEvidence = "conflicting_evidence"
	EmptyResponseReasonDailyLimit          = "daily_limit_manual_review"
	EmptyResponseReasonAlreadyClaimed      = "already_claimed"
)

type ClaimDecision struct {
	Status      string
	ReasonCode  string
	RuleVersion int
}

type EmptyResponseClaim struct {
	ID                 int64
	UsageLogID         int64
	OutcomeID          *int64
	UserID             int64
	APIKeyID           int64
	AccountID          int64
	GroupID            *int64
	SubscriptionID     *int64
	Status             string
	ReasonCode         string
	UserReason         string
	OriginalActualCost float64
	BalanceRefund      float64
	SubscriptionRefund float64
	APIKeyQuotaRefund  float64
	Evidence           ResponseOutcome
	RuleVersion        int
	AdminNote          string
	ReviewedBy         *int64
	ReviewedAt         *time.Time
	CompensatedAt      *time.Time
	Model              string
	UserEmail          string
	AccountName        string
	GroupName          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type EmptyResponseClaimEvaluation struct {
	Usage     UsageLog
	OutcomeID *int64
	Outcome   *ResponseOutcome
	Group     Group
}

type EmptyResponseClaimCreateInput struct {
	Evaluation         EmptyResponseClaimEvaluation
	Decision           ClaimDecision
	OriginalActualCost float64
	UserReason         string
}

type EmptyResponseClaimSubmitInput struct {
	UserID     int64
	UsageLogID int64
	UserReason string
}

type EmptyResponseClaimRepository interface {
	LoadEvaluation(ctx context.Context, userID, usageLogID int64) (*EmptyResponseClaimEvaluation, error)
	CountUserClaims(ctx context.Context, userID int64, start, end time.Time) (int, error)
	Create(ctx context.Context, input *EmptyResponseClaimCreateInput) (*EmptyResponseClaim, bool, error)
}

type EmptyResponseClaimCompensator interface {
	CompensateApprovedClaim(ctx context.Context, claimID int64) error
}

type EmptyResponseClaimService struct {
	repo        EmptyResponseClaimRepository
	compensator EmptyResponseClaimCompensator
	now         func() time.Time
}

func NewEmptyResponseClaimService(repo EmptyResponseClaimRepository, compensator EmptyResponseClaimCompensator) *EmptyResponseClaimService {
	return &EmptyResponseClaimService{repo: repo, compensator: compensator, now: time.Now}
}

func (s *EmptyResponseClaimService) Submit(ctx context.Context, input EmptyResponseClaimSubmitInput) (*EmptyResponseClaim, error) {
	if input.UserID <= 0 || input.UsageLogID <= 0 {
		return nil, ErrEmptyResponseClaimInvalidInput
	}
	if s == nil || s.repo == nil {
		return nil, ErrEmptyResponseClaimNotFound
	}

	evaluation, err := s.repo.LoadEvaluation(ctx, input.UserID, input.UsageLogID)
	if err != nil {
		return nil, err
	}
	if evaluation == nil {
		return nil, ErrEmptyResponseClaimNotFound
	}
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	dayStart, dayEnd := emptyResponseClaimBusinessDay(now)
	dailyCount, err := s.repo.CountUserClaims(ctx, input.UserID, dayStart, dayEnd)
	if err != nil {
		return nil, err
	}
	decision := EvaluateEmptyResponseClaim(now, evaluation.Usage, evaluation.Outcome, evaluation.Group, dailyCount)
	createInput := &EmptyResponseClaimCreateInput{
		Evaluation:         *evaluation,
		Decision:           decision,
		OriginalActualCost: evaluation.Usage.ActualCost,
		UserReason:         trimEmptyResponseClaimUserReason(input.UserReason),
	}
	claim, created, err := s.repo.Create(ctx, createInput)
	if err != nil {
		return nil, err
	}
	if !created {
		if claim != nil && claim.Status == EmptyResponseClaimApproved && s.compensator != nil {
			if err := s.compensateClaim(ctx, claim, evaluation.Usage.BillingType); err != nil {
				return claim, err
			}
			return claim, nil
		}
		return claim, ErrEmptyResponseClaimAlreadyExists
	}
	if claim != nil && decision.Status == EmptyResponseClaimApproved && s.compensator != nil {
		if err := s.compensateClaim(ctx, claim, evaluation.Usage.BillingType); err != nil {
			return claim, err
		}
	}
	return claim, nil
}

func (s *EmptyResponseClaimService) compensateClaim(ctx context.Context, claim *EmptyResponseClaim, billingType int8) error {
	if err := s.compensator.CompensateApprovedClaim(ctx, claim.ID); err != nil {
		return err
	}
	claim.Status = EmptyResponseClaimCompensated
	claim.APIKeyQuotaRefund = claim.OriginalActualCost
	if billingType == BillingTypeSubscription {
		claim.SubscriptionRefund = claim.OriginalActualCost
		claim.BalanceRefund = 0
	} else {
		claim.BalanceRefund = claim.OriginalActualCost
		claim.SubscriptionRefund = 0
	}
	return nil
}

func emptyResponseClaimBusinessDay(now time.Time) (time.Time, time.Time) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return start, start.AddDate(0, 0, 1)
}

func trimEmptyResponseClaimUserReason(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > 255 {
		return string(runes[:255])
	}
	return value
}

func EvaluateEmptyResponseClaim(now time.Time, usage UsageLog, outcome *ResponseOutcome, group Group, dailyCount int) ClaimDecision {
	decision := func(status, reason string) ClaimDecision {
		return ClaimDecision{Status: status, ReasonCode: reason, RuleVersion: EmptyResponseClaimRuleVersion}
	}

	if usage.ActualCost <= 0 {
		return decision(EmptyResponseClaimRejected, EmptyResponseReasonNotCharged)
	}
	if usage.CompensatedCost > 0 {
		return decision(EmptyResponseClaimRejected, EmptyResponseReasonAlreadyCompensated)
	}
	if !group.EmptyResponseCompensationEnabled {
		return decision(EmptyResponseClaimRejected, EmptyResponseReasonGroupDisabled)
	}
	if usage.CreatedAt.IsZero() || now.Before(usage.CreatedAt) {
		return decision(EmptyResponseClaimManualReview, EmptyResponseReasonConflictingEvidence)
	}
	if now.Sub(usage.CreatedAt) > 24*time.Hour {
		return decision(EmptyResponseClaimRejected, EmptyResponseReasonWindowExpired)
	}
	if outcome == nil || outcome.CollectorVersion <= 0 {
		return decision(EmptyResponseClaimManualReview, EmptyResponseReasonMissingEvidence)
	}
	if outcome.DisconnectSource == DisconnectSourceClient {
		return decision(EmptyResponseClaimRejected, EmptyResponseReasonClientCancelled)
	}
	if outcome.HasEffectiveOutput() {
		return decision(EmptyResponseClaimRejected, EmptyResponseReasonEffectiveOutput)
	}
	if outcome.StreamCompleted && (outcome.DisconnectSource != "" && outcome.DisconnectSource != DisconnectSourceNone || outcome.UpstreamErrorKind != "" && outcome.UpstreamErrorKind != UpstreamErrorNone) {
		return decision(EmptyResponseClaimManualReview, EmptyResponseReasonConflictingEvidence)
	}
	if dailyCount >= EmptyResponseClaimDailyLimit {
		return decision(EmptyResponseClaimManualReview, EmptyResponseReasonDailyLimit)
	}
	if outcome.UpstreamErrorKind == UpstreamErrorTimeout {
		return decision(EmptyResponseClaimApproved, EmptyResponseReasonUpstreamTimeout)
	}
	if outcome.UpstreamStatus >= 500 || outcome.HTTPStatus >= 500 || outcome.UpstreamErrorKind == UpstreamErrorHTTP5xx {
		return decision(EmptyResponseClaimApproved, EmptyResponseReasonUpstreamHTTP5xx)
	}
	if outcome.DisconnectSource == DisconnectSourceUpstream || outcome.UpstreamErrorKind == UpstreamErrorProtocol {
		return decision(EmptyResponseClaimApproved, EmptyResponseReasonUpstreamInterrupted)
	}
	if outcome.StreamCompleted && outcome.HTTPStatus >= 200 && outcome.HTTPStatus < 300 && outcome.UpstreamStatus >= 200 && outcome.UpstreamStatus < 300 {
		return decision(EmptyResponseClaimApproved, EmptyResponseReasonPureEmpty)
	}
	return decision(EmptyResponseClaimManualReview, EmptyResponseReasonConflictingEvidence)
}
