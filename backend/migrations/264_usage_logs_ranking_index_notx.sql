-- Speed up admin dashboard user spending ranking over recent usage logs.
-- The index is intentionally concurrent because usage_logs is a large live table.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_billing_created_user_perf
    ON usage_logs (billing_type, created_at, user_id)
    INCLUDE (actual_cost, compensated_cost, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens);
