package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type emptyResponseClaimRepository struct {
	sql sqlExecutor
}

func NewEmptyResponseClaimRepository(sqlDB *sql.DB) service.EmptyResponseClaimRepository {
	return newEmptyResponseClaimRepositoryWithSQL(sqlDB)
}

func newEmptyResponseClaimRepositoryWithSQL(sqlq sqlExecutor) *emptyResponseClaimRepository {
	return &emptyResponseClaimRepository{sql: sqlq}
}

func (r *emptyResponseClaimRepository) LoadEvaluation(ctx context.Context, userID, usageLogID int64) (*service.EmptyResponseClaimEvaluation, error) {
	query := `
		SELECT
			ul.id, ul.user_id, ul.api_key_id, ul.account_id, ul.group_id, ul.subscription_id,
			ul.actual_cost::float8, ul.compensated_cost::float8, ul.created_at,
			COALESCE(g.empty_response_compensation_enabled, FALSE),
			uro.id, uro.http_status, uro.upstream_status, uro.has_text, uro.has_tool_call,
			uro.has_reasoning, uro.has_media, uro.output_bytes, uro.event_count,
			uro.stream_completed, uro.finish_reason, uro.disconnect_source,
			uro.upstream_error_kind, uro.collector_version
		FROM usage_logs ul
		LEFT JOIN groups g ON g.id = ul.group_id
		LEFT JOIN usage_response_outcomes uro
			ON uro.request_id = ul.request_id AND uro.api_key_id = ul.api_key_id
		WHERE ul.user_id = $1 AND ul.id = $2
	`

	var usage service.UsageLog
	var groupID, subscriptionID, outcomeID sql.NullInt64
	var groupEnabled bool
	var httpStatus, upstreamStatus, eventCount, collectorVersion sql.NullInt64
	var hasText, hasToolCall, hasReasoning, hasMedia, streamCompleted sql.NullBool
	var outputBytes sql.NullInt64
	var finishReason, disconnectSource, upstreamErrorKind sql.NullString
	err := scanSingleRow(ctx, r.sql, query, []any{userID, usageLogID},
		&usage.ID, &usage.UserID, &usage.APIKeyID, &usage.AccountID, &groupID, &subscriptionID,
		&usage.ActualCost, &usage.CompensatedCost, &usage.CreatedAt, &groupEnabled,
		&outcomeID, &httpStatus, &upstreamStatus, &hasText, &hasToolCall,
		&hasReasoning, &hasMedia, &outputBytes, &eventCount, &streamCompleted,
		&finishReason, &disconnectSource, &upstreamErrorKind, &collectorVersion,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrEmptyResponseClaimNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load empty response claim evaluation: %w", err)
	}
	if groupID.Valid {
		usage.GroupID = &groupID.Int64
	}
	if subscriptionID.Valid {
		usage.SubscriptionID = &subscriptionID.Int64
	}
	evaluation := &service.EmptyResponseClaimEvaluation{
		Usage: usage,
		Group: service.Group{EmptyResponseCompensationEnabled: groupEnabled},
	}
	if groupID.Valid {
		evaluation.Group.ID = groupID.Int64
	}
	if outcomeID.Valid {
		evaluation.OutcomeID = &outcomeID.Int64
		evaluation.Outcome = &service.ResponseOutcome{
			HTTPStatus:        int(httpStatus.Int64),
			UpstreamStatus:    int(upstreamStatus.Int64),
			HasText:           hasText.Bool,
			HasToolCall:       hasToolCall.Bool,
			HasReasoning:      hasReasoning.Bool,
			HasMedia:          hasMedia.Bool,
			OutputBytes:       outputBytes.Int64,
			EventCount:        int(eventCount.Int64),
			StreamCompleted:   streamCompleted.Bool,
			FinishReason:      finishReason.String,
			DisconnectSource:  service.DisconnectSource(disconnectSource.String),
			UpstreamErrorKind: service.UpstreamErrorKind(upstreamErrorKind.String),
			CollectorVersion:  int(collectorVersion.Int64),
		}
	}
	return evaluation, nil
}

func (r *emptyResponseClaimRepository) CountUserClaims(ctx context.Context, userID int64, start, end time.Time) (int, error) {
	var count int
	err := scanSingleRow(ctx, r.sql,
		"SELECT COUNT(*) FROM empty_response_claims WHERE user_id = $1 AND created_at >= $2 AND created_at < $3",
		[]any{userID, start, end}, &count)
	if err != nil {
		return 0, fmt.Errorf("count empty response claims: %w", err)
	}
	return count, nil
}

func (r *emptyResponseClaimRepository) Create(ctx context.Context, input *service.EmptyResponseClaimCreateInput) (*service.EmptyResponseClaim, bool, error) {
	if input == nil {
		return nil, false, service.ErrEmptyResponseClaimInvalidInput
	}
	evaluation := input.Evaluation
	usage := evaluation.Usage
	groupID := usage.GroupID
	if groupID == nil && evaluation.Group.ID > 0 {
		id := evaluation.Group.ID
		groupID = &id
	}
	evidence := []byte(`{}`)
	if evaluation.Outcome != nil {
		var err error
		evidence, err = json.Marshal(evaluation.Outcome)
		if err != nil {
			return nil, false, fmt.Errorf("marshal empty response evidence: %w", err)
		}
	}

	query := `
		INSERT INTO empty_response_claims (
			usage_log_id, outcome_id, user_id, api_key_id, account_id, group_id, subscription_id,
			status, reason_code, user_reason, original_actual_cost, evidence, rule_version
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12::jsonb, $13
		)
		ON CONFLICT (usage_log_id) DO NOTHING
		RETURNING ` + emptyResponseClaimSelectColumns
	claim, err := r.scanClaim(ctx, query, []any{
		usage.ID, evaluation.OutcomeID, usage.UserID, usage.APIKeyID, usage.AccountID, groupID, usage.SubscriptionID,
		input.Decision.Status, input.Decision.ReasonCode, input.UserReason, usage.ActualCost, evidence, input.Decision.RuleVersion,
	})
	if err == nil {
		return claim, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("create empty response claim: %w", err)
	}
	claim, err = r.scanClaim(ctx,
		"SELECT "+emptyResponseClaimSelectColumns+" FROM empty_response_claims WHERE usage_log_id = $1",
		[]any{usage.ID})
	if err != nil {
		return nil, false, fmt.Errorf("load existing empty response claim: %w", err)
	}
	return claim, false, nil
}

const emptyResponseClaimSelectColumns = `
	id, usage_log_id, outcome_id, user_id, api_key_id, account_id, group_id, subscription_id,
	status, reason_code, user_reason, original_actual_cost::float8, balance_refund::float8,
	subscription_refund::float8, api_key_quota_refund::float8, evidence, rule_version,
	created_at, updated_at`

func (r *emptyResponseClaimRepository) scanClaim(ctx context.Context, query string, args []any) (*service.EmptyResponseClaim, error) {
	claim := &service.EmptyResponseClaim{}
	var outcomeID, groupID, subscriptionID sql.NullInt64
	var evidence []byte
	err := scanSingleRow(ctx, r.sql, query, args,
		&claim.ID, &claim.UsageLogID, &outcomeID, &claim.UserID, &claim.APIKeyID, &claim.AccountID, &groupID, &subscriptionID,
		&claim.Status, &claim.ReasonCode, &claim.UserReason, &claim.OriginalActualCost, &claim.BalanceRefund,
		&claim.SubscriptionRefund, &claim.APIKeyQuotaRefund, &evidence, &claim.RuleVersion,
		&claim.CreatedAt, &claim.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if outcomeID.Valid {
		claim.OutcomeID = &outcomeID.Int64
	}
	if groupID.Valid {
		claim.GroupID = &groupID.Int64
	}
	if subscriptionID.Valid {
		claim.SubscriptionID = &subscriptionID.Int64
	}
	if len(evidence) > 0 && string(evidence) != "null" {
		if err := json.Unmarshal(evidence, &claim.Evidence); err != nil {
			return nil, fmt.Errorf("decode empty response evidence: %w", err)
		}
	}
	return claim, nil
}
