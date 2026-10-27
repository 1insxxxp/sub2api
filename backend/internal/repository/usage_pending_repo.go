package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// UpsertPendingUsage accepts only explicit scalar observations. In particular it
// never serializes UsageLog associations, request bodies, credentials, or costs.
func (r *usageLogRepository) UpsertPendingUsage(ctx context.Context, log *service.UsageLog) error {
	if log == nil || log.UserID <= 0 || log.APIKeyID <= 0 || log.AccountID <= 0 || strings.TrimSpace(log.RequestID) == "" || strings.TrimSpace(log.Model) == "" {
		return errors.New("invalid pending usage identity")
	}
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return r.upsertPendingUsage(ctx, tx.Client(), log)
	}
	if db, ok := r.sql.(*sql.DB); ok {
		tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()
		if err := r.upsertPendingUsage(ctx, tx, log); err != nil {
			return err
		}
		return tx.Commit()
	}
	// Existing ent/sql transactions remain owned by their caller.
	switch r.sql.(type) {
	case *sql.Tx, *dbent.Tx:
		return r.upsertPendingUsage(ctx, r.sql, log)
	default:
		return errors.New("pending usage persistence requires a transaction-capable repository")
	}
}

func (r *usageLogRepository) upsertPendingUsage(ctx context.Context, sqlq sqlExecutor, log *service.UsageLog) error {
	var isolation string
	if err := scanSingleRow(ctx, sqlq, "SHOW transaction_isolation", nil, &isolation); err != nil {
		return err
	}
	if isolation != "read committed" && isolation != "read uncommitted" {
		return errors.New("pending usage persistence requires READ COMMITTED isolation")
	}
	// This must be a separate statement from INSERT: after waiting for a real
	// settlement, the INSERT must use a fresh snapshot that can see its commit.
	if _, err := sqlq.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('usage-pending:' || $1::bigint::text || ':' || $2::text, 0))`, log.APIKeyID, log.RequestID); err != nil {
		return err
	}
	inputSource, outputSource := service.UsageTokenSourceUnreported, service.UsageTokenSourceUnreported
	input, output, cacheRead, cacheCreate, cache5m, cache1h, imageOutput := 0, 0, 0, 0, 0, 0, 0
	if log.InputTokensSource == service.UsageTokenSourceUpstream {
		inputSource = service.UsageTokenSourceUpstream
		input, cacheRead, cacheCreate = max(0, log.InputTokens), max(0, log.CacheReadTokens), max(0, log.CacheCreationTokens)
		cache5m, cache1h = max(0, log.CacheCreation5mTokens), max(0, log.CacheCreation1hTokens)
	}
	if log.OutputTokensSource == service.UsageTokenSourceUpstream {
		outputSource = service.UsageTokenSourceUpstream
		output, imageOutput = max(0, log.OutputTokens), max(0, log.ImageOutputTokens)
	}
	createdAt := log.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	requestedModel := strings.TrimSpace(log.RequestedModel)
	if requestedModel == "" {
		requestedModel = log.Model
	}
	columns := []string{"request_id", "user_id", "api_key_id", "account_id", "model", "requested_model", "upstream_model", "group_id", "source_group_id", "custom_group_id", "subscription_id", "input_tokens", "output_tokens", "input_tokens_source", "output_tokens_source", "cache_creation_tokens", "cache_read_tokens", "cache_creation_5m_tokens", "cache_creation_1h_tokens", "image_output_tokens", "billing_type", "stream", "duration_ms", "first_token_ms", "user_agent", "ip_address", "inbound_endpoint", "upstream_endpoint", "upstream_request_id", "session_id", "channel_id", "model_mapping_chain", "service_tier", "reasoning_effort", "requested_reasoning_effort", "created_at"}
	args := []any{log.RequestID, log.UserID, log.APIKeyID, log.AccountID, log.Model, requestedModel, log.UpstreamModel, log.GroupID, log.SourceGroupID, log.CustomGroupID, log.SubscriptionID, input, output, inputSource, outputSource, cacheCreate, cacheRead, cache5m, cache1h, imageOutput, log.BillingType, log.Stream, log.DurationMs, log.FirstTokenMs, log.UserAgent, log.IPAddress, log.InboundEndpoint, log.UpstreamEndpoint, log.UpstreamRequestID, log.SessionID, log.ChannelID, log.ModelMappingChain, log.ServiceTier, log.ReasoningEffort, log.RequestedReasoningEffort, createdAt}
	placeholders := make([]string, len(args))
	updates := make([]string, 0, len(columns))
	for i, column := range columns {
		// INSERT ... SELECT also reuses the identity parameters in subqueries.
		// Explicit types avoid PostgreSQL inferring text in SELECT but varchar
		// from a comparison/assignment for the very same parameter.
		columnType := "varchar"
		switch column {
		case "user_id", "api_key_id", "account_id", "group_id", "source_group_id", "custom_group_id", "subscription_id", "channel_id":
			columnType = "bigint"
		case "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "cache_creation_5m_tokens", "cache_creation_1h_tokens", "image_output_tokens", "duration_ms", "first_token_ms":
			columnType = "integer"
		case "billing_type":
			columnType = "smallint"
		case "stream":
			columnType = "boolean"
		case "user_agent", "model_mapping_chain":
			columnType = "text"
		case "created_at":
			columnType = "timestamptz"
		}
		placeholders[i] = fmt.Sprintf("$%d::%s", i+1, columnType)
		switch column {
		case "request_id", "user_id", "api_key_id", "created_at":
			// Preserve request identity and original history ordering on retries.
		case "input_tokens", "input_tokens_source", "cache_creation_tokens", "cache_read_tokens", "cache_creation_5m_tokens", "cache_creation_1h_tokens":
			updates = append(updates, fmt.Sprintf("%[1]s = CASE WHEN EXCLUDED.input_tokens_source = 'upstream' THEN EXCLUDED.%[1]s ELSE usage_pending_records.%[1]s END", column))
		case "output_tokens", "output_tokens_source", "image_output_tokens":
			updates = append(updates, fmt.Sprintf("%[1]s = CASE WHEN EXCLUDED.output_tokens_source = 'upstream' THEN EXCLUDED.%[1]s ELSE usage_pending_records.%[1]s END", column))
		default:
			updates = append(updates, column+" = EXCLUDED."+column)
		}
	}
	// A late observation must not recreate a pending request after real usage
	// retention. Committed billing dedup and its archive are settlement evidence.
	settled := "EXISTS (SELECT 1 FROM usage_logs WHERE request_id = $1::varchar AND api_key_id = $3::bigint) OR EXISTS (SELECT 1 FROM usage_billing_dedup WHERE request_id = $1::varchar AND api_key_id = $3::bigint) OR EXISTS (SELECT 1 FROM usage_billing_dedup_archive WHERE request_id = $1::varchar AND api_key_id = $3::bigint)"
	query := "INSERT INTO usage_pending_records (" + strings.Join(columns, ", ") + ") SELECT " + strings.Join(placeholders, ", ") + " WHERE NOT (" + settled + ") ON CONFLICT (request_id, api_key_id) DO UPDATE SET " + strings.Join(updates, ", ") + " WHERE usage_pending_records.user_id = EXCLUDED.user_id AND usage_pending_records.account_id = EXCLUDED.account_id RETURNING id"
	var id int64
	if err := scanSingleRow(ctx, sqlq, query, args, &id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Distinguish an already settled request from an identity conflict.
			var alreadySettled bool
			if checkErr := scanSingleRow(ctx, sqlq, "SELECT "+strings.ReplaceAll(settled, "$3", "$2"), []any{log.RequestID, log.APIKeyID}, &alreadySettled); checkErr != nil {
				return checkErr
			}
			if alreadySettled {
				return nil
			}
		}
		return fmt.Errorf("upsert pending usage: %w", err)
	}
	log.ID = -id
	log.BillingStatus = service.UsageBillingStatusPendingReview
	log.BillingPendingReason = service.UsagePendingReasonUpstreamMissing
	return nil
}

// These expressions produce the existing typed read projection, not billing
// rows. Financial columns are intentionally absent from the pending store.
func pendingUsageSelectColumns() string {
	values := make(map[string]string)
	for _, column := range strings.Fields("user_id api_key_id account_id request_id model requested_model upstream_model group_id source_group_id custom_group_id subscription_id input_tokens output_tokens cache_creation_tokens cache_read_tokens cache_creation_5m_tokens cache_creation_1h_tokens image_output_tokens billing_type stream duration_ms first_token_ms user_agent ip_address inbound_endpoint upstream_endpoint upstream_request_id session_id channel_id model_mapping_chain service_tier reasoning_effort requested_reasoning_effort created_at") {
		values[column] = "p." + column
	}
	for _, column := range strings.Fields("image_input_tokens image_count video_count") {
		values[column] = "0"
	}
	for _, column := range strings.Fields("image_output_cost image_input_cost input_cost output_cost cache_creation_cost cache_read_cost total_cost actual_cost threshold_exempt_cost compensated_cost") {
		values[column] = "0::numeric"
	}
	for _, column := range strings.Fields("openai_ws_mode cache_ttl_overridden long_context_billing_applied native_compaction_v2") {
		values[column] = "FALSE"
	}
	values["id"] = "-p.id"
	values["upstream_output_tokens"] = "p.output_tokens"
	// Preserve the settled branch's SQL types so PostgreSQL can pull up the
	// UNION and use its history indexes before LIMIT instead of casting/scanning
	// every usage row. Pending TEXT metadata also projects as existing VARCHAR.
	values["rate_multiplier"] = "1::numeric"
	values["request_type"] = "(CASE WHEN p.stream THEN 2 ELSE 1 END)::smallint"
	values["billing_mode"] = "'token'::varchar"
	values["user_agent"] = "p.user_agent::varchar"
	values["model_mapping_chain"] = "p.model_mapping_chain::varchar"
	columns := strings.Split(usageLogSelectColumns, ", ")
	for i, column := range columns {
		value, ok := values[column]
		if !ok {
			value = "NULL"
		}
		columns[i] = value + " AS " + column
	}
	return strings.Join(columns, ", ")
}

const pendingUsageUnsettledPredicate = `p.resolved_at IS NULL
AND NOT EXISTS (SELECT 1 FROM usage_logs settled WHERE settled.request_id = p.request_id AND settled.api_key_id = p.api_key_id)
AND NOT EXISTS (SELECT 1 FROM usage_billing_dedup billed WHERE billed.request_id = p.request_id AND billed.api_key_id = p.api_key_id)
AND NOT EXISTS (SELECT 1 FROM usage_billing_dedup_archive billed WHERE billed.request_id = p.request_id AND billed.api_key_id = p.api_key_id)`

func (r *usageLogRepository) listUsageHistoryWithPending(ctx context.Context, where string, args []any, params pagination.PaginationParams, fast bool) ([]service.UsageLog, *pagination.PaginationResult, error) {
	from := " FROM (SELECT " + usageLogSelectColumns + " FROM usage_logs UNION ALL SELECT " + pendingUsageSelectColumns() + " FROM usage_pending_records p WHERE " + pendingUsageUnsettledPredicate + ") usage_logs "
	var total int64
	if !fast {
		if err := scanSingleRow(ctx, r.sql, "SELECT COUNT(*)"+from+where, args, &total); err != nil {
			return nil, nil, err
		}
	}
	limit := params.Limit()
	if fast {
		limit++
	}
	orderParams := params
	// Numeric IDs come from separate namespaces; chronological order is the
	// meaningful default for a combined request history.
	if orderParams.SortBy == "" || strings.EqualFold(orderParams.SortBy, "id") {
		orderParams.SortBy = "created_at"
	}
	query := fmt.Sprintf("SELECT %s%s%s ORDER BY %s LIMIT $%d OFFSET $%d", usageLogSelectColumns, from, where, usageLogOrderBy(orderParams), len(args)+1, len(args)+2)
	listArgs := append(append([]any{}, args...), limit, params.Offset())
	logs, err := r.queryUsageLogs(ctx, query, listArgs...)
	if err != nil {
		return nil, nil, err
	}
	if fast {
		total = int64(params.Offset() + len(logs))
		if len(logs) > params.Limit() {
			logs = logs[:params.Limit()]
		}
	}
	if err := r.hydratePendingUsageMetadata(ctx, logs); err != nil {
		return nil, nil, err
	}
	return logs, paginationResultFromTotal(total, params), nil
}

func (r *usageLogRepository) getPendingUsageByID(ctx context.Context, id int64) (*service.UsageLog, error) {
	if id >= 0 || id == math.MinInt64 {
		return nil, service.ErrUsageLogNotFound
	}
	// Include the real usage projection as the first UNION branch solely to
	// retain its SQL types for NULL/default columns in the pending projection.
	query := "SELECT " + usageLogSelectColumns + " FROM usage_logs WHERE FALSE UNION ALL SELECT " + pendingUsageSelectColumns() + " FROM usage_pending_records p WHERE p.id = $1 AND " + pendingUsageUnsettledPredicate
	logs, err := r.queryUsageLogs(ctx, query, -id)
	if err != nil {
		return nil, err
	}
	if len(logs) == 0 {
		return nil, service.ErrUsageLogNotFound
	}
	if err := r.hydrateUsageLogAssociations(ctx, logs); err != nil {
		return nil, err
	}
	if err := r.hydratePendingUsageMetadata(ctx, logs); err != nil {
		return nil, err
	}
	return &logs[0], nil
}

func (r *usageLogRepository) hydratePendingUsageMetadata(ctx context.Context, logs []service.UsageLog) (err error) {
	ids := make([]int64, 0)
	byID := make(map[int64]*service.UsageLog)
	for i := range logs {
		if logs[i].ID < 0 && logs[i].ID != math.MinInt64 {
			id := -logs[i].ID
			ids = append(ids, id)
			byID[id] = &logs[i]
			logs[i].BillingStatus = service.UsageBillingStatusPendingReview
			logs[i].BillingPendingReason = service.UsagePendingReasonUpstreamMissing
			logs[i].InputTokensSource = service.UsageTokenSourceUnreported
			logs[i].OutputTokensSource = service.UsageTokenSourceUnreported
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := r.sql.QueryContext(ctx, "SELECT id, input_tokens_source, output_tokens_source, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, cache_creation_5m_tokens, cache_creation_1h_tokens, image_output_tokens FROM usage_pending_records WHERE id = ANY($1)", pq.Array(ids))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var id int64
		var input, output string
		var inputTokens, outputTokens, cacheRead, cacheCreate, cache5m, cache1h, imageOutput int
		if err := rows.Scan(&id, &input, &output, &inputTokens, &outputTokens, &cacheRead, &cacheCreate, &cache5m, &cache1h, &imageOutput); err != nil {
			return err
		}
		if log := byID[id]; log != nil {
			log.InputTokensSource = service.UsageTokenSource(input).Normalize()
			log.OutputTokensSource = service.UsageTokenSource(output).Normalize()
			// Read counts and their evidence in the same snapshot if a repeated
			// observation was persisted between the list and metadata queries.
			log.InputTokens, log.OutputTokens, log.UpstreamOutputTokens = inputTokens, outputTokens, outputTokens
			log.CacheReadTokens, log.CacheCreationTokens = cacheRead, cacheCreate
			log.CacheCreation5mTokens, log.CacheCreation1hTokens = cache5m, cache1h
			log.ImageOutputTokens = imageOutput
		}
	}
	return rows.Err()
}

var _ interface {
	UpsertPendingUsage(context.Context, *service.UsageLog) error
} = (*usageLogRepository)(nil)
