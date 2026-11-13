-- Speed up admin dashboard model distribution statistics over time ranges.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_created_model_cost_perf
    ON usage_logs (created_at, model)
    INCLUDE (total_cost, actual_cost, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens);
