//go:build unit

package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestEmptyResponseClaimRepositoryLoadEvaluationUsesOwnedUsageAndStructuredOutcome(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := newEmptyResponseClaimRepositoryWithSQL(db)
	now := time.Now().UTC()

	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("FROM usage_logs ul")+`.*`+regexp.QuoteMeta("LEFT JOIN usage_response_outcomes uro")+`.*`+regexp.QuoteMeta("WHERE ul.user_id = $1 AND ul.id = $2")).
		WithArgs(int64(7), int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{
			"usage_log_id", "user_id", "api_key_id", "account_id", "group_id", "subscription_id",
			"actual_cost", "compensated_cost", "created_at", "group_enabled", "outcome_id",
			"http_status", "upstream_status", "has_text", "has_tool_call", "has_reasoning", "has_media",
			"output_bytes", "event_count", "stream_completed", "finish_reason", "disconnect_source",
			"upstream_error_kind", "collector_version",
		}).AddRow(
			100, 7, 8, 9, 10, nil, 1.25, 0, now, true, 55,
			200, 200, false, true, false, false, 15, 2, true, "stop", "none", "none", 1,
		))

	evaluation, err := repo.LoadEvaluation(context.Background(), 7, 100)
	require.NoError(t, err)
	require.Equal(t, int64(100), evaluation.Usage.ID)
	require.Equal(t, 1.25, evaluation.Usage.ActualCost)
	require.True(t, evaluation.Group.EmptyResponseCompensationEnabled)
	require.NotNil(t, evaluation.OutcomeID)
	require.Equal(t, int64(55), *evaluation.OutcomeID)
	require.NotNil(t, evaluation.Outcome)
	require.True(t, evaluation.Outcome.HasToolCall)
	require.True(t, evaluation.Outcome.StreamCompleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEmptyResponseClaimRepositoryCreateReturnsExistingClaimOnUniqueUsage(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := newEmptyResponseClaimRepositoryWithSQL(db)
	now := time.Now().UTC()
	outcomeID := int64(55)
	evaluation := service.EmptyResponseClaimEvaluation{
		Usage:     service.UsageLog{ID: 100, UserID: 7, APIKeyID: 8, AccountID: 9, ActualCost: 1.25},
		OutcomeID: &outcomeID,
		Outcome:   &service.ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true, CollectorVersion: 1},
		Group:     service.Group{ID: 10, EmptyResponseCompensationEnabled: true},
	}
	input := &service.EmptyResponseClaimCreateInput{
		Evaluation:         evaluation,
		Decision:           service.ClaimDecision{Status: service.EmptyResponseClaimApproved, ReasonCode: service.EmptyResponseReasonPureEmpty, RuleVersion: 1},
		OriginalActualCost: 1.25,
		UserReason:         "empty reply",
	}
	claimColumns := []string{
		"id", "usage_log_id", "outcome_id", "user_id", "api_key_id", "account_id", "group_id", "subscription_id",
		"status", "reason_code", "user_reason", "original_actual_cost", "balance_refund", "subscription_refund",
		"api_key_quota_refund", "evidence", "rule_version", "created_at", "updated_at",
	}

	mock.ExpectQuery("INSERT INTO empty_response_claims").
		WithArgs(
			int64(100), &outcomeID, int64(7), int64(8), int64(9), sqlmock.AnyArg(), nil,
			service.EmptyResponseClaimApproved, service.EmptyResponseReasonPureEmpty, "empty reply", 1.25,
			sqlmock.AnyArg(), 1,
		).
		WillReturnRows(sqlmock.NewRows(claimColumns))
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("FROM empty_response_claims") + `.*` + regexp.QuoteMeta("WHERE usage_log_id = $1")).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows(claimColumns).AddRow(
			201, 100, 55, 7, 8, 9, 10, nil,
			service.EmptyResponseClaimCompensated, service.EmptyResponseReasonPureEmpty, "empty reply", 1.25, 1.25, 0, 1.25,
			`{"http_status":200,"stream_completed":true}`, 1, now, now,
		))

	claim, created, err := repo.Create(context.Background(), input)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, int64(201), claim.ID)
	require.Equal(t, service.EmptyResponseClaimCompensated, claim.Status)
	require.True(t, claim.Evidence.StreamCompleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEmptyResponseClaimRepositoryCountsShanghaiBusinessDayRange(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := newEmptyResponseClaimRepositoryWithSQL(db)
	start := time.Date(2026, 8, 7, 0, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	end := start.AddDate(0, 0, 1)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM empty_response_claims").
		WithArgs(int64(7), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))

	count, err := repo.CountUserClaims(context.Background(), 7, start, end)
	require.NoError(t, err)
	require.Equal(t, 10, count)
	require.NoError(t, mock.ExpectationsWereMet())
}
