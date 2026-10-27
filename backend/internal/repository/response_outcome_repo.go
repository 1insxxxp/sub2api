package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// UpsertResponseOutcome persists privacy-safe response evidence only after the
// corresponding usage row exists. The usage identity makes retries idempotent.
func (r *usageLogRepository) UpsertResponseOutcome(ctx context.Context, log *service.UsageLog) error {
	return r.upsertResponseOutcome(ctx, log)
}

func (r *usageLogRepository) upsertResponseOutcome(ctx context.Context, log *service.UsageLog) error {
	if r == nil || r.sql == nil || log == nil || !log.HasResponseMetadata() {
		return nil
	}
	requestID := strings.TrimSpace(log.RequestID)
	if requestID == "" || log.APIKeyID <= 0 {
		return fmt.Errorf("response outcome requires usage identity")
	}

	var outcome service.ResponseOutcome
	if log.Outcome != nil {
		outcome = *log.Outcome
	}
	if outcome.DisconnectSource == "" {
		outcome.DisconnectSource = service.DisconnectSourceNone
	}
	if outcome.UpstreamErrorKind == "" {
		outcome.UpstreamErrorKind = service.UpstreamErrorNone
	}
	if log.Outcome != nil && outcome.CollectorVersion <= 0 {
		outcome.CollectorVersion = service.ResponseOutcomeCollectorVersion
	}

	query := `
		INSERT INTO usage_response_outcomes (
			usage_log_id, request_id, api_key_id, user_id, account_id, group_id,
			http_status, upstream_status, has_text, has_tool_call, has_reasoning,
			has_media, output_bytes, event_count, stream_completed, finish_reason,
			disconnect_source, upstream_error_kind, collector_version, anti_truncation_rounds,
			input_tokens_source, output_tokens_source
		)
		SELECT
			ul.id, $1::varchar(64), $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21
		FROM usage_logs ul
		WHERE ul.request_id = $1::varchar(64) AND ul.api_key_id = $2
		ON CONFLICT (request_id, api_key_id) DO UPDATE SET
			usage_log_id = EXCLUDED.usage_log_id,
			user_id = EXCLUDED.user_id,
			account_id = EXCLUDED.account_id,
			group_id = EXCLUDED.group_id,
			http_status = CASE WHEN EXCLUDED.collector_version > 0 THEN EXCLUDED.http_status ELSE usage_response_outcomes.http_status END,
			upstream_status = CASE WHEN EXCLUDED.collector_version > 0 THEN EXCLUDED.upstream_status ELSE usage_response_outcomes.upstream_status END,
			has_text = CASE WHEN EXCLUDED.collector_version > 0 THEN EXCLUDED.has_text ELSE usage_response_outcomes.has_text END,
			has_tool_call = CASE WHEN EXCLUDED.collector_version > 0 THEN EXCLUDED.has_tool_call ELSE usage_response_outcomes.has_tool_call END,
			has_reasoning = CASE WHEN EXCLUDED.collector_version > 0 THEN EXCLUDED.has_reasoning ELSE usage_response_outcomes.has_reasoning END,
			has_media = CASE WHEN EXCLUDED.collector_version > 0 THEN EXCLUDED.has_media ELSE usage_response_outcomes.has_media END,
			output_bytes = CASE WHEN EXCLUDED.collector_version > 0 THEN EXCLUDED.output_bytes ELSE usage_response_outcomes.output_bytes END,
			event_count = CASE WHEN EXCLUDED.collector_version > 0 THEN EXCLUDED.event_count ELSE usage_response_outcomes.event_count END,
			stream_completed = CASE WHEN EXCLUDED.collector_version > 0 THEN EXCLUDED.stream_completed ELSE usage_response_outcomes.stream_completed END,
			finish_reason = CASE WHEN EXCLUDED.collector_version > 0 THEN EXCLUDED.finish_reason ELSE usage_response_outcomes.finish_reason END,
			disconnect_source = CASE WHEN EXCLUDED.collector_version > 0 THEN EXCLUDED.disconnect_source ELSE usage_response_outcomes.disconnect_source END,
			upstream_error_kind = CASE WHEN EXCLUDED.collector_version > 0 THEN EXCLUDED.upstream_error_kind ELSE usage_response_outcomes.upstream_error_kind END,
			collector_version = CASE WHEN EXCLUDED.collector_version > 0 THEN EXCLUDED.collector_version ELSE usage_response_outcomes.collector_version END,
			anti_truncation_rounds = GREATEST(usage_response_outcomes.anti_truncation_rounds, EXCLUDED.anti_truncation_rounds),
			input_tokens_source = CASE WHEN EXCLUDED.input_tokens_source <> 'unknown' THEN EXCLUDED.input_tokens_source ELSE usage_response_outcomes.input_tokens_source END,
			output_tokens_source = CASE WHEN EXCLUDED.output_tokens_source <> 'unknown' THEN EXCLUDED.output_tokens_source ELSE usage_response_outcomes.output_tokens_source END
	`
	result, err := r.sql.ExecContext(ctx, query,
		requestID,
		log.APIKeyID,
		log.UserID,
		log.AccountID,
		log.GroupID,
		outcome.HTTPStatus,
		outcome.UpstreamStatus,
		outcome.HasText,
		outcome.HasToolCall,
		outcome.HasReasoning,
		outcome.HasMedia,
		outcome.OutputBytes,
		outcome.EventCount,
		outcome.StreamCompleted,
		strings.TrimSpace(outcome.FinishReason),
		string(outcome.DisconnectSource),
		string(outcome.UpstreamErrorKind),
		int16(outcome.CollectorVersion),
		log.AntiTruncationRounds,
		string(log.InputTokensSource.Normalize()),
		string(log.OutputTokensSource.Normalize()),
	)
	if err != nil {
		return fmt.Errorf("upsert response outcome: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read response outcome upsert result: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("usage log not found for response outcome request_id=%q api_key_id=%d", requestID, log.APIKeyID)
	}
	return nil
}
