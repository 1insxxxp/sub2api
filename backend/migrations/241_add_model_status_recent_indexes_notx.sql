-- Seek the newest evidence in each public group/model without scanning history.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_model_status_recent
ON usage_logs (
    group_id,
    (COALESCE(NULLIF(BTRIM(requested_model), ''), NULLIF(BTRIM(model), ''), 'unknown')),
    created_at DESC,
    id DESC
);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_error_logs_model_status_recent
ON ops_error_logs (
    group_id,
    (COALESCE(NULLIF(BTRIM(requested_model), ''), NULLIF(BTRIM(model), ''), 'unknown')),
    created_at DESC,
    id DESC
)
WHERE NOT is_count_tokens AND (status_code >= 400 OR error_type = 'cyber_policy');
