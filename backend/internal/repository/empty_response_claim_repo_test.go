//go:build unit

package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
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
			"actual_cost", "compensated_cost", "created_at", "stream",
			"input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens",
			"group_enabled", "outcome_id",
			"http_status", "upstream_status", "has_text", "has_tool_call", "has_reasoning", "has_media",
			"output_bytes", "event_count", "stream_completed", "finish_reason", "disconnect_source",
			"upstream_error_kind", "collector_version",
		}).AddRow(
			100, 7, 8, 9, 10, nil, 1.25, 0, now, true, 1234, 0, 12, 34, true, 55,
			200, 200, false, true, false, false, 15, 2, true, "stop", "none", "none", 1,
		))

	evaluation, err := repo.LoadEvaluation(context.Background(), 7, 100)
	require.NoError(t, err)
	require.Equal(t, int64(100), evaluation.Usage.ID)
	require.Equal(t, 1.25, evaluation.Usage.ActualCost)
	require.Equal(t, 1234, evaluation.Usage.InputTokens)
	require.Equal(t, 0, evaluation.Usage.OutputTokens)
	require.Equal(t, 12, evaluation.Usage.CacheCreationTokens)
	require.Equal(t, 34, evaluation.Usage.CacheReadTokens)
	require.True(t, evaluation.Usage.Stream)
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
		"api_key_quota_refund", "evidence", "rule_version", "admin_note", "reviewed_by", "reviewed_at", "compensated_at", "created_at", "updated_at",
	}

	mock.ExpectQuery("INSERT INTO empty_response_claims").
		WithArgs(
			int64(100), &outcomeID, int64(7), int64(8), int64(9), sqlmock.AnyArg(), nil,
			service.EmptyResponseClaimApproved, service.EmptyResponseReasonPureEmpty, "empty reply", 1.25,
			sqlmock.AnyArg(), 1, service.EmptyResponseClaimDailyLimit,
		).
		WillReturnRows(sqlmock.NewRows(claimColumns))
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("FROM empty_response_claims") + `.*` + regexp.QuoteMeta("WHERE usage_log_id = $1")).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows(claimColumns).AddRow(
			201, 100, 55, 7, 8, 9, 10, nil,
			service.EmptyResponseClaimCompensated, service.EmptyResponseReasonPureEmpty, "empty reply", 1.25, 1.25, 0, 1.25,
			`{"http_status":200,"stream_completed":true}`, 1, "", nil, nil, now, now, now,
		))

	claim, created, err := repo.Create(context.Background(), input)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, int64(201), claim.ID)
	require.Equal(t, service.EmptyResponseClaimCompensated, claim.Status)
	require.True(t, claim.Evidence.StreamCompleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEmptyResponseClaimRepositoryCreateUsesBigintUserIDAcrossDailyGuard(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := newEmptyResponseClaimRepositoryWithSQL(db)
	now := time.Now().UTC()
	outcomeID := int64(55)
	input := &service.EmptyResponseClaimCreateInput{
		Evaluation: service.EmptyResponseClaimEvaluation{
			Usage:     service.UsageLog{ID: 100, UserID: 7, APIKeyID: 8, AccountID: 9, ActualCost: 1.25},
			OutcomeID: &outcomeID,
			Outcome:   &service.ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, CollectorVersion: 1},
			Group:     service.Group{ID: 10, EmptyResponseCompensationEnabled: true},
		},
		Decision: service.ClaimDecision{
			Status:      service.EmptyResponseClaimApproved,
			ReasonCode:  service.EmptyResponseReasonPureEmpty,
			RuleVersion: 1,
		},
		OriginalActualCost: 1.25,
		UserReason:         "empty reply",
	}
	claimColumns := []string{
		"id", "usage_log_id", "outcome_id", "user_id", "api_key_id", "account_id", "group_id", "subscription_id",
		"status", "reason_code", "user_reason", "original_actual_cost", "balance_refund", "subscription_refund",
		"api_key_quota_refund", "evidence", "rule_version", "admin_note", "reviewed_by", "reviewed_at", "compensated_at", "created_at", "updated_at",
	}
	typedUserIDQuery := `(?s)` +
		regexp.QuoteMeta("hashtextextended(($3::bigint)::text") + `.*` +
		regexp.QuoteMeta("WHERE user_id = $3::bigint") + `.*` +
		regexp.QuoteMeta("$1, $2, $3::bigint")
	mock.ExpectQuery(typedUserIDQuery).
		WithArgs(
			int64(100), &outcomeID, int64(7), int64(8), int64(9), sqlmock.AnyArg(), nil,
			service.EmptyResponseClaimApproved, service.EmptyResponseReasonPureEmpty, "empty reply", 1.25,
			sqlmock.AnyArg(), 1, service.EmptyResponseClaimDailyLimit,
		).
		WillReturnRows(sqlmock.NewRows(claimColumns).AddRow(
			201, 100, 55, 7, 8, 9, 10, nil,
			service.EmptyResponseClaimApproved, service.EmptyResponseReasonPureEmpty, "empty reply", 1.25, 0, 0, 0,
			`{"http_status":200}`, 1, "", nil, nil, nil, now, now,
		))

	claim, created, err := repo.Create(context.Background(), input)

	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, int64(201), claim.ID)
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

func TestEmptyResponseClaimRepositoryListsRecentEvaluationsWithExistingClaimState(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := newEmptyResponseClaimRepositoryWithSQL(db)
	now := time.Now().UTC()
	start := now.Add(-7 * 24 * time.Hour)

	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("SELECT COUNT(*)")+`.*`+regexp.QuoteMeta("FROM usage_logs ul")+`.*`+regexp.QuoteMeta("LEFT JOIN empty_response_claims erc ON erc.usage_log_id = ul.id")).
		WithArgs(int64(7), start, now, service.EmptyResponseClaimLowOutputTokenLimit).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(21))
	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("FROM usage_logs ul")+`.*`+regexp.QuoteMeta("LEFT JOIN usage_response_outcomes uro ON uro.usage_log_id = ul.id")+`.*`+regexp.QuoteMeta("LEFT JOIN empty_response_claims erc ON erc.usage_log_id = ul.id")+`.*`+regexp.QuoteMeta("COALESCE(g.empty_response_compensation_enabled, FALSE) = TRUE")+`.*`+regexp.QuoteMeta("AND ul.output_tokens <= $4")).
		WithArgs(int64(7), start, now, service.EmptyResponseClaimLowOutputTokenLimit, 10, 10).
		WillReturnRows(sqlmock.NewRows([]string{
			"usage_log_id", "user_id", "api_key_id", "account_id", "group_id", "subscription_id",
			"model", "actual_cost", "compensated_cost", "billing_type", "inbound_endpoint", "created_at", "stream",
			"input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens",
			"api_key_name", "group_name", "group_enabled", "outcome_id", "http_status", "upstream_status", "has_text",
			"has_tool_call", "has_reasoning", "has_media", "output_bytes", "event_count",
			"stream_completed", "finish_reason", "disconnect_source", "upstream_error_kind", "collector_version",
			"claim_id", "claim_status", "claim_reason_code", "refunded_amount",
		}).AddRow(
			100, 7, 8, 9, 10, nil,
			"claude-opus-4-6", 1.25, 0, service.BillingTypeBalance, "/v1/messages", now.Add(-time.Hour), true,
			1234, 0, 12, 34,
			"cli", "cc", true, 55, 200, 200, false,
			false, false, false, 0, 1,
			true, "stop", "none", "none", 1,
			201, service.EmptyResponseClaimCompensated, service.EmptyResponseReasonPureEmpty, 1.25,
		).AddRow(
			101, 7, 8, 9, 11, nil,
			"claude-opus-4-6", 0.75, 0, service.BillingTypeBalance, "/v1/messages", now.Add(-2*time.Hour), false,
			10, service.EmptyResponseClaimLowOutputTokenLimit, 0, 0,
			"cli", "enabled-cc", true, nil, nil, nil, nil,
			nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil,
			nil, nil, nil, 0,
		))

	candidates, result, err := repo.ListRecentEvaluations(context.Background(), 7, start, now, pagination.PaginationParams{Page: 2, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, candidates, 2)
	require.Equal(t, int64(21), result.Total)
	require.Equal(t, 2, result.Page)
	require.Equal(t, 10, result.PageSize)
	require.Equal(t, 3, result.Pages)
	got := candidates[0]
	require.Equal(t, int64(100), got.Evaluation.Usage.ID)
	require.Equal(t, "claude-opus-4-6", got.Evaluation.Usage.Model)
	require.Equal(t, 1234, got.Evaluation.Usage.InputTokens)
	require.Equal(t, 0, got.Evaluation.Usage.OutputTokens)
	require.True(t, got.Evaluation.Usage.Stream)
	require.Equal(t, 12, got.Evaluation.Usage.CacheCreationTokens)
	require.Equal(t, 34, got.Evaluation.Usage.CacheReadTokens)
	require.Equal(t, "cli", got.APIKeyName)
	require.Equal(t, "cc", got.GroupName)
	require.True(t, got.Evaluation.Group.EmptyResponseCompensationEnabled)
	require.Equal(t, "/v1/messages", got.InboundEndpoint)
	require.NotNil(t, got.ClaimID)
	require.Equal(t, int64(201), *got.ClaimID)
	require.Equal(t, service.EmptyResponseClaimCompensated, got.ClaimStatus)
	require.Equal(t, 1.25, got.RefundedAmount)
	require.NotNil(t, got.Evaluation.Outcome)
	require.True(t, got.Evaluation.Outcome.StreamCompleted)
	withoutOutcome := candidates[1]
	require.Equal(t, int64(101), withoutOutcome.Evaluation.Usage.ID)
	require.Equal(t, int64(11), *withoutOutcome.Evaluation.Usage.GroupID)
	require.True(t, withoutOutcome.Evaluation.Group.EmptyResponseCompensationEnabled)
	require.Equal(t, service.EmptyResponseClaimLowOutputTokenLimit, withoutOutcome.Evaluation.Usage.OutputTokens)
	require.Nil(t, withoutOutcome.Evaluation.OutcomeID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEmptyResponseClaimRepositoryListIncludesPrivacySafeReviewContext(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := newEmptyResponseClaimRepositoryWithSQL(db)
	now := time.Now().UTC()
	claimColumns := []string{
		"id", "usage_log_id", "outcome_id", "user_id", "api_key_id", "account_id", "group_id", "subscription_id",
		"status", "reason_code", "user_reason", "original_actual_cost", "balance_refund", "subscription_refund",
		"api_key_quota_refund", "evidence", "rule_version", "admin_note", "reviewed_by", "reviewed_at", "compensated_at", "created_at", "updated_at",
		"model", "user_email", "account_name", "group_name", "request_id", "usage_created_at", "input_tokens", "output_tokens",
		"cache_creation_tokens", "cache_read_tokens", "total_cost", "actual_cost", "compensated_cost", "billing_type", "request_type",
		"stream", "duration_ms", "first_token_ms", "inbound_endpoint", "upstream_endpoint",
	}
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM empty_response_claims").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("FROM empty_response_claims erc") + `.*` + regexp.QuoteMeta("JOIN usage_logs ul") + `.*` + regexp.QuoteMeta("ORDER BY erc.created_at DESC")).
		WillReturnRows(sqlmock.NewRows(claimColumns).AddRow(
			1, 100, nil, 7, 8, 9, 10, nil,
			service.EmptyResponseClaimManualReview, service.EmptyResponseReasonMissingEvidence, "empty reply", 1.25, 0, 0, 0,
			`{}`, 1, "", nil, nil, nil, now, now,
			"claude-opus-4-6", "user@example.com", "pool-1", "cc", "client:req-1", now, 1234, 0,
			12, 34, 1.5, 1.25, 0, int8(0), int16(service.RequestTypeStream), true, 1800, 320, "/v1/messages", "/v1/messages",
		))

	claims, _, err := repo.List(context.Background(), pagination.PaginationParams{Page: 1, PageSize: 20}, service.EmptyResponseClaimListFilters{})
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.Equal(t, "client:req-1", claims[0].RequestID)
	require.Equal(t, 1234, claims[0].InputTokens)
	require.Equal(t, 34, claims[0].CacheReadTokens)
	require.Equal(t, service.RequestTypeStream, claims[0].RequestType)
	require.NotNil(t, claims[0].DurationMs)
	require.Equal(t, 1800, *claims[0].DurationMs)
	require.Equal(t, "/v1/messages", claims[0].InboundEndpoint)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEmptyResponseClaimRepositoryGetAdminByIDReloadsMutationDetails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := newEmptyResponseClaimRepositoryWithSQL(db)
	now := time.Now().UTC()
	claimColumns := []string{
		"id", "usage_log_id", "outcome_id", "user_id", "api_key_id", "account_id", "group_id", "subscription_id",
		"status", "reason_code", "user_reason", "original_actual_cost", "balance_refund", "subscription_refund",
		"api_key_quota_refund", "evidence", "rule_version", "admin_note", "reviewed_by", "reviewed_at", "compensated_at", "created_at", "updated_at",
		"model", "user_email", "account_name", "group_name", "request_id", "usage_created_at", "input_tokens", "output_tokens",
		"cache_creation_tokens", "cache_read_tokens", "total_cost", "actual_cost", "compensated_cost", "billing_type", "request_type",
		"stream", "duration_ms", "first_token_ms", "inbound_endpoint", "upstream_endpoint",
	}
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("FROM empty_response_claims erc") + `.*` + regexp.QuoteMeta("WHERE erc.id = $1")).
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows(claimColumns).AddRow(
			12, 100, nil, 7, 8, 9, 10, nil,
			service.EmptyResponseClaimCompensated, service.EmptyResponseReasonPureEmpty, "empty reply", 1.25, 1.25, 0, 1.25,
			`{"http_status":200}`, 1, "verified", 99, now, now, now, now,
			"claude-opus-4-6", "user@example.com", "pool-1", "cc", "req-empty-response", now, 1234, 0,
			12, 34, 1.5, 1.25, 1.25, int8(0), int16(service.RequestTypeStream), true, 1800, 320, "/v1/messages", "/v1/messages",
		))

	claim, err := repo.GetAdminByID(context.Background(), 12)
	require.NoError(t, err)
	require.Equal(t, service.EmptyResponseClaimCompensated, claim.Status)
	require.Equal(t, "claude-opus-4-6", claim.Model)
	require.Equal(t, "user@example.com", claim.UserEmail)
	require.Equal(t, "req-empty-response", claim.RequestID)
	require.NotNil(t, claim.CompensatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}
