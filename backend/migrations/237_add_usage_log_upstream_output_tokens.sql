ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS upstream_output_tokens INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN usage_logs.upstream_output_tokens IS
    'Provider-reported output tokens; output_tokens can reflect text delivered to the downstream client for streaming requests';
