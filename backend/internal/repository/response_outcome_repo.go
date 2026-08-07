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
	if r == nil || r.sql == nil || log == nil || log.Outcome == nil {
		return nil
	}
	requestID := strings.TrimSpace(log.RequestID)
	if requestID == "" || log.APIKeyID <= 0 {
		return fmt.Errorf("response outcome requires usage identity")
	}

	outcome := *log.Outcome
	if outcome.DisconnectSource == "" {
		outcome.DisconnectSource = service.DisconnectSourceNone
	}
	if outcome.UpstreamErrorKind == "" {
		outcome.UpstreamErrorKind = service.UpstreamErrorNone
	}
	if outcome.CollectorVersion <= 0 {
		outcome.CollectorVersion = service.ResponseOutcomeCollectorVersion
	}

	query := `
		INSERT INTO usage_response_outcomes (
			usage_log_id, request_id, api_key_id, user_id, account_id, group_id,
			http_status, upstream_status, has_text, has_tool_call, has_reasoning,
			has_media, output_bytes, event_count, stream_completed, finish_reason,
			disconnect_source, upstream_error_kind, collector_version
		)
		SELECT
			ul.id, $1::varchar(64), $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15,
			$16, $17, $18
		FROM usage_logs ul
		WHERE ul.request_id = $1::varchar(64) AND ul.api_key_id = $2
		ON CONFLICT (request_id, api_key_id) DO UPDATE SET
			usage_log_id = EXCLUDED.usage_log_id,
			user_id = EXCLUDED.user_id,
			account_id = EXCLUDED.account_id,
			group_id = EXCLUDED.group_id,
			http_status = EXCLUDED.http_status,
			upstream_status = EXCLUDED.upstream_status,
			has_text = EXCLUDED.has_text,
			has_tool_call = EXCLUDED.has_tool_call,
			has_reasoning = EXCLUDED.has_reasoning,
			has_media = EXCLUDED.has_media,
			output_bytes = EXCLUDED.output_bytes,
			event_count = EXCLUDED.event_count,
			stream_completed = EXCLUDED.stream_completed,
			finish_reason = EXCLUDED.finish_reason,
			disconnect_source = EXCLUDED.disconnect_source,
			upstream_error_kind = EXCLUDED.upstream_error_kind,
			collector_version = EXCLUDED.collector_version
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
