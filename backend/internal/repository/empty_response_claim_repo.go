package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type emptyResponseClaimRepository struct {
	sql sqlExecutor
}

func NewEmptyResponseClaimRepository(sqlDB *sql.DB) *emptyResponseClaimRepository {
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
			ul.input_tokens, ul.output_tokens, ul.cache_creation_tokens, ul.cache_read_tokens,
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
	var inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens int
	var httpStatus, upstreamStatus, eventCount, collectorVersion sql.NullInt64
	var hasText, hasToolCall, hasReasoning, hasMedia, streamCompleted sql.NullBool
	var outputBytes sql.NullInt64
	var finishReason, disconnectSource, upstreamErrorKind sql.NullString
	err := scanSingleRow(ctx, r.sql, query, []any{userID, usageLogID},
		&usage.ID, &usage.UserID, &usage.APIKeyID, &usage.AccountID, &groupID, &subscriptionID,
		&usage.ActualCost, &usage.CompensatedCost, &usage.CreatedAt,
		&inputTokens, &outputTokens, &cacheCreationTokens, &cacheReadTokens,
		&groupEnabled,
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
	usage.InputTokens = inputTokens
	usage.OutputTokens = outputTokens
	usage.CacheCreationTokens = cacheCreationTokens
	usage.CacheReadTokens = cacheReadTokens
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

func (r *emptyResponseClaimRepository) ListRecentEvaluations(ctx context.Context, userID int64, start, end time.Time, limit int) ([]service.EmptyResponseRecentCandidate, error) {
	if limit <= 0 {
		limit = service.EmptyResponseRecentListLimit
	}
	query := `
		SELECT
			ul.id, ul.user_id, ul.api_key_id, ul.account_id, ul.group_id, ul.subscription_id,
			COALESCE(NULLIF(ul.requested_model, ''), ul.model), ul.actual_cost::float8,
			ul.compensated_cost::float8, ul.billing_type, COALESCE(ul.inbound_endpoint, ''), ul.created_at,
			ul.input_tokens, ul.output_tokens, ul.cache_creation_tokens, ul.cache_read_tokens,
			COALESCE(ak.name, ''), COALESCE(g.name, ''),
			uro.id, uro.http_status, uro.upstream_status, uro.has_text, uro.has_tool_call,
			uro.has_reasoning, uro.has_media, uro.output_bytes, uro.event_count,
			uro.stream_completed, uro.finish_reason, uro.disconnect_source,
			uro.upstream_error_kind, uro.collector_version,
			erc.id, erc.status, erc.reason_code,
			COALESCE(erc.balance_refund + erc.subscription_refund, 0)::float8
		FROM usage_logs ul
		JOIN usage_response_outcomes uro ON uro.usage_log_id = ul.id
		LEFT JOIN empty_response_claims erc ON erc.usage_log_id = ul.id
		LEFT JOIN api_keys ak ON ak.id = ul.api_key_id
		LEFT JOIN groups g ON g.id = ul.group_id
		WHERE ul.user_id = $1
			AND ul.created_at >= $2
			AND ul.created_at < $3
			AND ul.actual_cost > 0
		ORDER BY ul.created_at DESC, ul.id DESC
		LIMIT $4
	`
	rows, err := r.sql.QueryContext(ctx, query, userID, start, end, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent empty response evaluations: %w", err)
	}
	defer rows.Close()

	candidates := make([]service.EmptyResponseRecentCandidate, 0)
	for rows.Next() {
		var candidate service.EmptyResponseRecentCandidate
		var usage service.UsageLog
		var groupID, subscriptionID, outcomeID, claimID sql.NullInt64
		var inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens int
		var httpStatus, upstreamStatus, outputBytes, eventCount, collectorVersion sql.NullInt64
		var hasText, hasToolCall, hasReasoning, hasMedia, streamCompleted sql.NullBool
		var finishReason, disconnectSource, upstreamErrorKind, claimStatus, claimReasonCode sql.NullString
		var inboundEndpoint sql.NullString
		if err := rows.Scan(
			&usage.ID, &usage.UserID, &usage.APIKeyID, &usage.AccountID, &groupID, &subscriptionID,
			&usage.Model, &usage.ActualCost, &usage.CompensatedCost, &usage.BillingType, &inboundEndpoint, &usage.CreatedAt,
			&inputTokens, &outputTokens, &cacheCreationTokens, &cacheReadTokens,
			&candidate.APIKeyName, &candidate.GroupName,
			&outcomeID, &httpStatus, &upstreamStatus, &hasText, &hasToolCall,
			&hasReasoning, &hasMedia, &outputBytes, &eventCount, &streamCompleted,
			&finishReason, &disconnectSource, &upstreamErrorKind, &collectorVersion,
			&claimID, &claimStatus, &claimReasonCode, &candidate.RefundedAmount,
		); err != nil {
			return nil, fmt.Errorf("scan recent empty response evaluation: %w", err)
		}
		if groupID.Valid {
			usage.GroupID = &groupID.Int64
			candidate.Evaluation.Group.ID = groupID.Int64
		}
		usage.InputTokens = inputTokens
		usage.OutputTokens = outputTokens
		usage.CacheCreationTokens = cacheCreationTokens
		usage.CacheReadTokens = cacheReadTokens
		if subscriptionID.Valid {
			usage.SubscriptionID = &subscriptionID.Int64
		}
		if inboundEndpoint.Valid {
			candidate.InboundEndpoint = inboundEndpoint.String
		}
		if outcomeID.Valid {
			candidate.Evaluation.OutcomeID = &outcomeID.Int64
		}
		candidate.Evaluation.Usage = usage
		candidate.Evaluation.Outcome = &service.ResponseOutcome{
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
		if claimID.Valid {
			candidate.ClaimID = &claimID.Int64
			candidate.ClaimStatus = claimStatus.String
			candidate.ClaimReasonCode = claimReasonCode.String
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent empty response evaluations: %w", err)
	}
	return candidates, nil
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
		WITH claim_lock AS (
			SELECT pg_advisory_xact_lock(hashtextextended($3::text || ':' || to_char(NOW() AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD'), 0))
		),
		daily_claims AS (
			SELECT COUNT(*) AS claim_count
			FROM empty_response_claims, claim_lock
			WHERE user_id = $3
				AND created_at >= (date_trunc('day', NOW() AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai')
				AND created_at < ((date_trunc('day', NOW() AT TIME ZONE 'Asia/Shanghai') + INTERVAL '1 day') AT TIME ZONE 'Asia/Shanghai')
		)
		INSERT INTO empty_response_claims (
			usage_log_id, outcome_id, user_id, api_key_id, account_id, group_id, subscription_id,
			status, reason_code, user_reason, original_actual_cost, evidence, rule_version
		)
		SELECT
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12::jsonb, $13
		FROM daily_claims
		WHERE claim_count < $14
		ON CONFLICT (usage_log_id) DO NOTHING
		RETURNING ` + emptyResponseClaimSelectColumns
	claim, err := r.scanClaim(ctx, query, []any{
		usage.ID, evaluation.OutcomeID, usage.UserID, usage.APIKeyID, usage.AccountID, groupID, usage.SubscriptionID,
		input.Decision.Status, input.Decision.ReasonCode, input.UserReason, usage.ActualCost, evidence, input.Decision.RuleVersion,
		service.EmptyResponseClaimDailyLimit,
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, service.ErrEmptyResponseClaimDailyLimitExceeded
		}
		return nil, false, fmt.Errorf("load existing empty response claim: %w", err)
	}
	return claim, false, nil
}

const emptyResponseClaimSelectColumns = `
	id, usage_log_id, outcome_id, user_id, api_key_id, account_id, group_id, subscription_id,
	status, reason_code, user_reason, original_actual_cost::float8, balance_refund::float8,
	subscription_refund::float8, api_key_quota_refund::float8, evidence, rule_version,
	admin_note, reviewed_by, reviewed_at, compensated_at, created_at, updated_at`

func (r *emptyResponseClaimRepository) scanClaim(ctx context.Context, query string, args []any) (*service.EmptyResponseClaim, error) {
	claim := &service.EmptyResponseClaim{}
	var outcomeID, groupID, subscriptionID, reviewedBy sql.NullInt64
	var reviewedAt, compensatedAt sql.NullTime
	var evidence []byte
	err := scanSingleRow(ctx, r.sql, query, args,
		&claim.ID, &claim.UsageLogID, &outcomeID, &claim.UserID, &claim.APIKeyID, &claim.AccountID, &groupID, &subscriptionID,
		&claim.Status, &claim.ReasonCode, &claim.UserReason, &claim.OriginalActualCost, &claim.BalanceRefund,
		&claim.SubscriptionRefund, &claim.APIKeyQuotaRefund, &evidence, &claim.RuleVersion,
		&claim.AdminNote, &reviewedBy, &reviewedAt, &compensatedAt,
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
	if reviewedBy.Valid {
		claim.ReviewedBy = &reviewedBy.Int64
	}
	if reviewedAt.Valid {
		claim.ReviewedAt = &reviewedAt.Time
	}
	if compensatedAt.Valid {
		claim.CompensatedAt = &compensatedAt.Time
	}
	if len(evidence) > 0 && string(evidence) != "null" {
		if err := json.Unmarshal(evidence, &claim.Evidence); err != nil {
			return nil, fmt.Errorf("decode empty response evidence: %w", err)
		}
	}
	return claim, nil
}

func (r *emptyResponseClaimRepository) List(ctx context.Context, params pagination.PaginationParams, filters service.EmptyResponseClaimListFilters) (claims []service.EmptyResponseClaim, paginationResult *pagination.PaginationResult, err error) {
	where := []string{"1=1"}
	args := make([]any, 0, 8)
	add := func(condition string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(condition, len(args)))
	}
	if status := strings.TrimSpace(filters.Status); status != "" {
		add("erc.status = $%d", status)
	}
	if filters.UserID > 0 {
		add("erc.user_id = $%d", filters.UserID)
	}
	if filters.GroupID > 0 {
		add("erc.group_id = $%d", filters.GroupID)
	}
	if filters.AccountID > 0 {
		add("erc.account_id = $%d", filters.AccountID)
	}
	if model := strings.TrimSpace(filters.Model); model != "" {
		add("ul.model = $%d", model)
	}
	if filters.StartTime != nil {
		add("erc.created_at >= $%d", *filters.StartTime)
	}
	if filters.EndTime != nil {
		add("erc.created_at < $%d", *filters.EndTime)
	}
	whereSQL := strings.Join(where, " AND ")
	countQuery := "SELECT COUNT(*) FROM empty_response_claims erc JOIN usage_logs ul ON ul.id = erc.usage_log_id WHERE " + whereSQL
	var total int64
	if err := scanSingleRow(ctx, r.sql, countQuery, args, &total); err != nil {
		return nil, nil, fmt.Errorf("count empty response claims: %w", err)
	}

	limit, offset := params.Limit(), params.Offset()
	queryArgs := append(append([]any{}, args...), limit, offset)
	query := `
		SELECT ` + prefixedEmptyResponseClaimSelectColumns("erc") + `,
			ul.model, COALESCE(u.email, ''), COALESCE(a.name, ''), COALESCE(g.name, ''),
			COALESCE(ul.request_id, ''), ul.created_at,
			ul.input_tokens, ul.output_tokens, ul.cache_creation_tokens, ul.cache_read_tokens,
			ul.total_cost::float8, ul.actual_cost::float8, ul.compensated_cost::float8,
			ul.billing_type, ul.request_type, ul.stream, ul.duration_ms, ul.first_token_ms,
			COALESCE(ul.inbound_endpoint, ''), COALESCE(ul.upstream_endpoint, '')
		FROM empty_response_claims erc
		JOIN usage_logs ul ON ul.id = erc.usage_log_id
		LEFT JOIN users u ON u.id = erc.user_id
		LEFT JOIN accounts a ON a.id = erc.account_id
		LEFT JOIN groups g ON g.id = erc.group_id
		WHERE ` + whereSQL + fmt.Sprintf(" ORDER BY erc.created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	rows, err := r.sql.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("list empty response claims: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			claims = nil
			paginationResult = nil
		}
	}()
	claims = make([]service.EmptyResponseClaim, 0, limit)
	for rows.Next() {
		claim, scanErr := scanEmptyResponseClaimRow(rows, true)
		if scanErr != nil {
			return nil, nil, fmt.Errorf("scan empty response claim: %w", scanErr)
		}
		claims = append(claims, *claim)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate empty response claims: %w", err)
	}
	pages := int((total + int64(limit) - 1) / int64(limit))
	if pages < 1 {
		pages = 1
	}
	return claims, &pagination.PaginationResult{Total: total, Page: params.Page, PageSize: limit, Pages: pages}, nil
}

func (r *emptyResponseClaimRepository) GetAdminByID(ctx context.Context, id int64) (claim *service.EmptyResponseClaim, err error) {
	query := `
		SELECT ` + prefixedEmptyResponseClaimSelectColumns("erc") + `,
			ul.model, COALESCE(u.email, ''), COALESCE(a.name, ''), COALESCE(g.name, ''),
			COALESCE(ul.request_id, ''), ul.created_at,
			ul.input_tokens, ul.output_tokens, ul.cache_creation_tokens, ul.cache_read_tokens,
			ul.total_cost::float8, ul.actual_cost::float8, ul.compensated_cost::float8,
			ul.billing_type, ul.request_type, ul.stream, ul.duration_ms, ul.first_token_ms,
			COALESCE(ul.inbound_endpoint, ''), COALESCE(ul.upstream_endpoint, '')
		FROM empty_response_claims erc
		JOIN usage_logs ul ON ul.id = erc.usage_log_id
		LEFT JOIN users u ON u.id = erc.user_id
		LEFT JOIN accounts a ON a.id = erc.account_id
		LEFT JOIN groups g ON g.id = erc.group_id
		WHERE erc.id = $1`
	rows, err := r.sql.QueryContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("get admin empty response claim: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			claim = nil
		}
	}()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("get admin empty response claim: %w", err)
		}
		return nil, service.ErrEmptyResponseClaimNotFound
	}
	claim, err = scanEmptyResponseClaimRow(rows, true)
	if err != nil {
		return nil, fmt.Errorf("scan admin empty response claim: %w", err)
	}
	return claim, nil
}

func (r *emptyResponseClaimRepository) Review(ctx context.Context, id int64, status string, reviewerID int64, note string) (*service.EmptyResponseClaim, error) {
	allowed := "('manual_review','evaluating')"
	if status == service.EmptyResponseClaimApproved {
		allowed = "('manual_review','evaluating','approved')"
	}
	query := `UPDATE empty_response_claims SET status = $2, reviewed_by = $3, reviewed_at = NOW(), admin_note = $4, updated_at = NOW()
		WHERE id = $1 AND status IN ` + allowed + ` RETURNING ` + emptyResponseClaimSelectColumns
	claim, err := r.scanClaim(ctx, query, []any{id, status, reviewerID, note})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrEmptyResponseClaimNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("review empty response claim: %w", err)
	}
	return claim, nil
}

func prefixedEmptyResponseClaimSelectColumns(prefix string) string {
	columns := strings.Split(strings.TrimSpace(emptyResponseClaimSelectColumns), ",")
	for i := range columns {
		column := strings.TrimSpace(columns[i])
		if strings.Contains(column, "::") {
			parts := strings.SplitN(column, "::", 2)
			columns[i] = prefix + "." + parts[0] + "::" + parts[1]
		} else {
			columns[i] = prefix + "." + column
		}
	}
	return strings.Join(columns, ", ")
}

func scanEmptyResponseClaimRow(row rowScanner, withIdentity bool) (*service.EmptyResponseClaim, error) {
	claim := &service.EmptyResponseClaim{}
	var outcomeID, groupID, subscriptionID, reviewedBy sql.NullInt64
	var reviewedAt, compensatedAt sql.NullTime
	var evidence []byte
	var requestID, inboundEndpoint, upstreamEndpoint sql.NullString
	var usageCreatedAt time.Time
	var inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens int64
	var totalCost, actualCost, compensatedCost float64
	var billingType, requestType sql.NullInt64
	var stream sql.NullBool
	var durationMs, firstTokenMs sql.NullInt64
	dest := []any{
		&claim.ID, &claim.UsageLogID, &outcomeID, &claim.UserID, &claim.APIKeyID, &claim.AccountID, &groupID, &subscriptionID,
		&claim.Status, &claim.ReasonCode, &claim.UserReason, &claim.OriginalActualCost, &claim.BalanceRefund,
		&claim.SubscriptionRefund, &claim.APIKeyQuotaRefund, &evidence, &claim.RuleVersion,
		&claim.AdminNote, &reviewedBy, &reviewedAt, &compensatedAt, &claim.CreatedAt, &claim.UpdatedAt,
	}
	if withIdentity {
		dest = append(dest, &claim.Model, &claim.UserEmail, &claim.AccountName, &claim.GroupName)
		dest = append(dest,
			&requestID, &usageCreatedAt,
			&inputTokens, &outputTokens, &cacheCreationTokens, &cacheReadTokens,
			&totalCost, &actualCost, &compensatedCost,
			&billingType, &requestType, &stream, &durationMs, &firstTokenMs,
			&inboundEndpoint, &upstreamEndpoint,
		)
	}
	if err := row.Scan(dest...); err != nil {
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
	if reviewedBy.Valid {
		claim.ReviewedBy = &reviewedBy.Int64
	}
	if reviewedAt.Valid {
		claim.ReviewedAt = &reviewedAt.Time
	}
	if compensatedAt.Valid {
		claim.CompensatedAt = &compensatedAt.Time
	}
	if len(evidence) > 0 && string(evidence) != "null" {
		if err := json.Unmarshal(evidence, &claim.Evidence); err != nil {
			return nil, err
		}
	}
	if withIdentity {
		claim.RequestID = requestID.String
		claim.UsageCreatedAt = usageCreatedAt
		claim.InputTokens = int(inputTokens)
		claim.OutputTokens = int(outputTokens)
		claim.CacheCreationTokens = int(cacheCreationTokens)
		claim.CacheReadTokens = int(cacheReadTokens)
		claim.TotalCost = totalCost
		claim.ActualCost = actualCost
		claim.CompensatedCost = compensatedCost
		if billingType.Valid {
			claim.BillingType = int8(billingType.Int64)
		}
		if requestType.Valid {
			claim.RequestType = service.RequestTypeFromInt16(int16(requestType.Int64))
		}
		claim.Stream = stream.Bool
		if durationMs.Valid {
			value := int(durationMs.Int64)
			claim.DurationMs = &value
		}
		if firstTokenMs.Valid {
			value := int(firstTokenMs.Int64)
			claim.FirstTokenMs = &value
		}
		claim.InboundEndpoint = inboundEndpoint.String
		claim.UpstreamEndpoint = upstreamEndpoint.String
	}
	return claim, nil
}
