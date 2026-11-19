-- Filter exact counts and read recent pages for one user's API key in index order.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_user_key_history
    ON usage_logs (user_id, api_key_id, created_at DESC, id DESC);
