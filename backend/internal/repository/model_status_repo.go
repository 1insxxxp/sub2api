package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type modelStatusRepository struct{ db *sql.DB }

func NewModelStatusRepository(db *sql.DB) service.ModelStatusRepository {
	return &modelStatusRepository{db: db}
}

func (r *modelStatusRepository) Aggregate(ctx context.Context, end time.Time, scopes []service.ModelStatusScope) ([]service.ModelStatusAggregate, error) {
	result := []service.ModelStatusAggregate{}
	if len(scopes) == 0 {
		return result, nil
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, fmt.Errorf("start model status read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for limit := service.ModelStatusRecentLimit; len(scopes) > 0; limit *= 2 {
		rows, pending, err := queryModelStatusCandidates(ctx, tx, end, scopes, limit)
		if err != nil {
			return nil, err
		}
		result = append(result, rows...)
		scopes = pending
	}
	return result, nil
}

func queryModelStatusCandidates(ctx context.Context, tx *sql.Tx, end time.Time, scopes []service.ModelStatusScope, limit int) ([]service.ModelStatusAggregate, []service.ModelStatusScope, error) {
	scopeJSON, err := json.Marshal(scopes)
	if err != nil {
		return nil, nil, fmt.Errorf("encode model status scope: %w", err)
	}
	rows, err := tx.QueryContext(ctx, modelStatusAggregateSQL, string(scopeJSON), end, limit, service.ModelStatusRecentLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("query model status: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := []service.ModelStatusAggregate{}
	pending := []service.ModelStatusScope{}
	for rows.Next() {
		var row service.ModelStatusAggregate
		var ttft, duration sql.NullFloat64
		var recent, buckets []byte
		var needsMore bool
		if err := rows.Scan(&row.GroupID, &row.Platform, &row.Model,
			&row.Metrics.Total, &row.Metrics.Success, &row.Metrics.Failure, &row.Metrics.Empty, &row.Metrics.Unknown,
			&ttft, &duration, &row.Metrics.TTFTSamples, &row.Metrics.DurationSamples, &recent, &buckets, &needsMore); err != nil {
			return nil, nil, fmt.Errorf("scan model status: %w", err)
		}
		if needsMore {
			pending = append(pending, service.ModelStatusScope{GroupID: row.GroupID, Platform: row.Platform, Model: row.Model})
			continue
		}
		if ttft.Valid {
			row.Metrics.AvgTTFTMs = &ttft.Float64
		}
		if duration.Valid {
			row.Metrics.AvgDurationMs = &duration.Float64
		}
		if err := json.Unmarshal(recent, &row.Recent); err != nil {
			return nil, nil, fmt.Errorf("decode model status recent results: %w", err)
		}
		if err := json.Unmarshal(buckets, &row.Buckets); err != nil {
			return nil, nil, fmt.Errorf("decode model status buckets: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("read model status: %w", err)
	}
	return result, pending, nil
}

// Indexed per-scope reads bound the initial evidence set. If deduplication or
// terminal timestamps leave an unseen candidate capable of entering the last
// 30, only that scope is retried with more candidates. Equality also retries so
// ties use the final identity ordering, never an arbitrary raw-record limit.
const modelStatusAggregateSQL = `
WITH scope AS MATERIALIZED (
  SELECT DISTINCT s.group_id, s.platform, s.model
  FROM jsonb_to_recordset($1::jsonb) AS s(group_id bigint, platform text, model text)
  JOIN groups g ON g.id = s.group_id AND g.platform = s.platform
  WHERE g.status = 'active' AND NOT g.is_exclusive AND g.deleted_at IS NULL
), usage_candidates AS MATERIALIZED (
  SELECT scope.*, ul.id, ul.api_key_id, TRIM(ul.request_id) AS request_id, ul.created_at
  FROM scope CROSS JOIN LATERAL (
    SELECT ul.id, ul.api_key_id, ul.request_id, ul.created_at
    FROM usage_logs ul
    WHERE ul.group_id = scope.group_id
      AND COALESCE(NULLIF(TRIM(ul.requested_model), ''), NULLIF(TRIM(ul.model), ''), 'unknown') = scope.model
      AND ul.created_at < $2 AND ul.created_at >= date_bin(INTERVAL '15 minutes', $2, TIMESTAMPTZ '1970-01-01') - INTERVAL '4 hours 45 minutes'
    ORDER BY ul.created_at DESC, ul.id DESC LIMIT $3
  ) ul
), error_candidates AS MATERIALIZED (
  SELECT scope.*, e.id, e.api_key_id, TRIM(e.request_id) AS request_id,
    TRIM(e.client_request_id) AS client_request_id, e.created_at
  FROM scope CROSS JOIN LATERAL (
    SELECT e.id, e.api_key_id, e.request_id, e.client_request_id, e.created_at
    FROM ops_error_logs e
    WHERE e.group_id = scope.group_id
      AND COALESCE(NULLIF(TRIM(e.requested_model), ''), NULLIF(TRIM(e.model), ''), 'unknown') = scope.model
      AND e.created_at < $2 AND e.created_at >= date_bin(INTERVAL '15 minutes', $2, TIMESTAMPTZ '1970-01-01') - INTERVAL '4 hours 45 minutes' AND NOT e.is_count_tokens
      AND (e.status_code >= 400 OR e.error_type = 'cyber_policy')
    ORDER BY e.created_at DESC, e.id DESC LIMIT $3
  ) e
), request_candidates AS MATERIALIZED (
  SELECT api_key_id,
    CASE WHEN request_id LIKE 'client:%' THEN 'client' ELSE 'local' END AS namespace,
    CASE WHEN request_id LIKE 'client:%' THEN SUBSTRING(request_id FROM 8)
      WHEN request_id LIKE 'local:%' THEN SUBSTRING(request_id FROM 7) ELSE request_id END AS request_id
  FROM usage_candidates WHERE api_key_id > 0 AND NULLIF(request_id, '') IS NOT NULL
  UNION
  SELECT api_key_id, CASE WHEN NULLIF(client_request_id, '') IS NOT NULL THEN 'client' ELSE 'local' END,
    COALESCE(NULLIF(client_request_id, ''), request_id)
  FROM error_candidates WHERE api_key_id > 0 AND COALESCE(NULLIF(client_request_id, ''), NULLIF(request_id, '')) IS NOT NULL
), usage_ids AS (
  SELECT id FROM usage_candidates
  UNION
  SELECT ul.id FROM request_candidates candidate JOIN usage_logs ul
    ON ul.api_key_id = candidate.api_key_id
    AND (ul.request_id = candidate.namespace || ':' || candidate.request_id
      OR (candidate.namespace = 'local' AND ul.request_id = candidate.request_id))
), error_ids AS (
  SELECT id FROM error_candidates
  UNION
  SELECT e.id FROM request_candidates candidate JOIN ops_error_logs e
    ON e.api_key_id = candidate.api_key_id
    AND ((candidate.namespace = 'client' AND e.client_request_id = candidate.request_id)
      OR (candidate.namespace = 'local' AND NULLIF(e.client_request_id, '') IS NULL AND e.request_id = candidate.request_id))
  WHERE NOT e.is_count_tokens AND (e.status_code >= 400 OR e.error_type = 'cyber_policy')
), usage_evidence AS (
  SELECT ul.id, ul.group_id, scope.platform, scope.model,
    CASE WHEN NULLIF(TRIM(ul.request_id), '') IS NOT NULL AND ul.api_key_id > 0
      THEN 'request:' || ul.api_key_id::text || ':' ||
        CASE WHEN TRIM(ul.request_id) LIKE 'local:%' OR TRIM(ul.request_id) LIKE 'client:%'
          THEN TRIM(ul.request_id) ELSE 'local:' || TRIM(ul.request_id) END
      ELSE 'usage:' || ul.id::text END AS identity,
    ul.created_at, 1 AS source_rank,
    CASE
      WHEN outcome.disconnect_source = 'client' THEN 'unknown'
      WHEN outcome.id IS NOT NULL AND (
        outcome.http_status >= 400 OR outcome.upstream_status >= 400
        OR outcome.upstream_error_kind NOT IN ('', 'none')
        OR outcome.disconnect_source IN ('upstream', 'server')) THEN 'failure'
      WHEN ul.request_type = 4 THEN 'failure'
      WHEN outcome.id IS NOT NULL AND ul.stream AND NOT outcome.stream_completed THEN 'unknown'
      WHEN outcome.id IS NOT NULL AND (outcome.has_text OR outcome.has_tool_call OR outcome.has_reasoning OR outcome.has_media) THEN 'success'
      WHEN outcome.id IS NOT NULL THEN 'empty'
      WHEN ul.output_tokens > 0 OR ul.image_output_tokens > 0 OR ul.image_count > 0 OR claim.reason_code = 'effective_output' THEN 'success'
      ELSE 'unknown'
    END AS outcome,
    COALESCE(NULLIF(outcome.upstream_status, 0), NULLIF(outcome.http_status, 0), 0) AS status_code,
    CASE WHEN outcome.disconnect_source = 'client' THEN 3 ELSE 1 END AS priority,
    CASE WHEN ul.first_token_ms >= 0 THEN ul.first_token_ms END AS ttft_ms,
    CASE WHEN ul.duration_ms >= 0 THEN ul.duration_ms END AS duration_ms
  FROM usage_logs ul
  JOIN usage_ids ON usage_ids.id = ul.id
  JOIN scope ON scope.group_id = ul.group_id
    AND scope.model = COALESCE(NULLIF(TRIM(ul.requested_model), ''), NULLIF(TRIM(ul.model), ''), 'unknown')
  LEFT JOIN usage_response_outcomes outcome ON outcome.usage_log_id = ul.id
  LEFT JOIN empty_response_claims claim ON claim.usage_log_id = ul.id
), error_evidence AS (
  SELECT e.id, e.group_id, scope.platform, scope.model,
    CASE WHEN e.api_key_id > 0 AND NULLIF(TRIM(e.client_request_id), '') IS NOT NULL
      THEN 'request:' || e.api_key_id::text || ':client:' || TRIM(e.client_request_id)
      WHEN e.api_key_id > 0 AND NULLIF(TRIM(e.request_id), '') IS NOT NULL
      THEN 'request:' || e.api_key_id::text || ':local:' || TRIM(e.request_id)
      ELSE 'error:' || e.id::text END AS identity,
    e.created_at, 2 AS source_rank,
    CASE WHEN e.status_code = 499 THEN 'unknown' ELSE 'failure' END AS outcome,
    COALESCE(NULLIF(e.upstream_status_code, 0), NULLIF(e.status_code, 0), 0) AS status_code,
    CASE WHEN e.status_code = 499 THEN 3 ELSE 2 END AS priority,
    CASE WHEN e.time_to_first_token_ms >= 0 THEN e.time_to_first_token_ms END AS ttft_ms,
    CASE WHEN e.duration_ms >= 0 THEN e.duration_ms END AS duration_ms
  FROM ops_error_logs e
  JOIN error_ids ON error_ids.id = e.id
  JOIN scope ON scope.group_id = e.group_id
    AND scope.model = COALESCE(NULLIF(TRIM(e.requested_model), ''), NULLIF(TRIM(e.model), ''), 'unknown')
), evidence AS (
  SELECT * FROM usage_evidence
  UNION ALL
  SELECT * FROM error_evidence
), timed AS (
  SELECT *, MAX(created_at) FILTER (WHERE source_rank = 2) OVER (PARTITION BY group_id, platform, model, identity) AS terminal_at
  FROM evidence
), deduplicated AS (
  SELECT DISTINCT ON (group_id, platform, model, identity) identity, group_id, platform, model,
    COALESCE(terminal_at, created_at) AS created_at, outcome, status_code, ttft_ms, duration_ms
  FROM timed
  ORDER BY group_id, platform, model, identity, priority DESC, created_at DESC, source_rank DESC, id DESC
), ranked AS (
  SELECT *, ROW_NUMBER() OVER (PARTITION BY group_id, platform, model ORDER BY created_at DESC, identity DESC) AS recent_rank
  FROM deduplicated WHERE created_at < $2 AND created_at >= date_bin(INTERVAL '15 minutes', $2, TIMESTAMPTZ '1970-01-01') - INTERVAL '4 hours 45 minutes'
), samples AS (
  SELECT * FROM ranked
), aggregates AS (
  SELECT group_id, platform, model,
    COUNT(*) AS total, COUNT(*) FILTER (WHERE outcome = 'success') AS success,
    COUNT(*) FILTER (WHERE outcome = 'failure') AS failure, COUNT(*) FILTER (WHERE outcome = 'empty') AS empty,
    COUNT(*) FILTER (WHERE outcome = 'unknown') AS unknown,
    (AVG(ttft_ms) FILTER (WHERE outcome = 'success'))::double precision AS ttft,
    (AVG(duration_ms) FILTER (WHERE outcome = 'success'))::double precision AS duration,
    COUNT(ttft_ms) FILTER (WHERE outcome = 'success') AS ttft_samples,
    COUNT(duration_ms) FILTER (WHERE outcome = 'success') AS duration_samples,
    (SELECT JSONB_AGG(JSONB_BUILD_OBJECT('at', r.created_at, 'outcome', r.outcome, 'status_code', NULLIF(r.status_code, 0)) ORDER BY r.created_at, r.identity)
      FROM ranked r WHERE r.group_id = s.group_id AND r.platform = s.platform AND r.model = s.model AND r.recent_rank <= $4) AS recent,
    MIN(created_at) AS oldest
  FROM samples s GROUP BY group_id, platform, model
), bucket_rows AS (
  SELECT *, date_bin(INTERVAL '15 minutes', created_at, TIMESTAMPTZ '1970-01-01') AS bucket_start
  FROM samples
), bucket_stats AS (
  SELECT group_id, platform, model, bucket_start,
    COUNT(*) AS total, COUNT(*) FILTER (WHERE outcome = 'success') AS success,
    COUNT(*) FILTER (WHERE outcome = 'failure') AS failure, COUNT(*) FILTER (WHERE outcome = 'empty') AS empty,
    COUNT(*) FILTER (WHERE outcome = 'unknown') AS unknown,
    (SELECT JSONB_AGG(JSONB_BUILD_OBJECT('at', q.created_at, 'outcome', q.outcome, 'status_code', NULLIF(q.status_code, 0)) ORDER BY q.created_at DESC, q.identity DESC)
      FROM (SELECT br.created_at, br.outcome, br.status_code, br.identity FROM bucket_rows br
        WHERE br.group_id = b.group_id AND br.platform = b.platform AND br.model = b.model AND br.bucket_start = b.bucket_start
        ORDER BY br.created_at DESC, br.identity DESC LIMIT 100) q) AS requests
  FROM bucket_rows b GROUP BY group_id, platform, model, bucket_start
), bucket_aggregates AS (
  SELECT group_id, platform, model,
    JSONB_AGG(JSONB_BUILD_OBJECT('start_at', bucket_start, 'end_at', bucket_start + INTERVAL '15 minutes',
      'total', total, 'success', success, 'failure', failure, 'empty', empty, 'unknown', unknown,
      'requests', COALESCE(requests, '[]'::jsonb)) ORDER BY bucket_start) AS buckets
  FROM bucket_stats GROUP BY group_id, platform, model
), boundaries AS (
  SELECT group_id, platform, model, MIN(created_at) AS oldest FROM usage_candidates
  GROUP BY group_id, platform, model HAVING COUNT(*) = $3
  UNION ALL
  SELECT group_id, platform, model, MIN(created_at) AS oldest FROM error_candidates
  GROUP BY group_id, platform, model HAVING COUNT(*) = $3
)
SELECT scope.group_id, scope.platform, scope.model,
  COALESCE(a.total, 0), COALESCE(a.success, 0), COALESCE(a.failure, 0), COALESCE(a.empty, 0), COALESCE(a.unknown, 0),
  a.ttft, a.duration, COALESCE(a.ttft_samples, 0), COALESCE(a.duration_samples, 0), COALESCE(a.recent, '[]'::jsonb), COALESCE(ba.buckets, '[]'::jsonb),
  EXISTS (SELECT 1 FROM boundaries b WHERE b.group_id = scope.group_id AND b.platform = scope.platform AND b.model = scope.model
    AND (COALESCE(a.total, 0) < $4 OR b.oldest >= a.oldest)) AS needs_more
FROM scope LEFT JOIN aggregates a USING (group_id, platform, model)
  LEFT JOIN bucket_aggregates ba USING (group_id, platform, model)
ORDER BY scope.group_id, scope.platform, scope.model`
