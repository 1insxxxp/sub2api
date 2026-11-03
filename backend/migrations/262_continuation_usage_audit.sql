-- Private bounded per-provider-call accounting evidence. It contains no
-- request/response bodies, credentials or headers and is not a public DTO.
ALTER TABLE usage_response_outcomes
    ADD COLUMN IF NOT EXISTS continuation_usage JSONB;
COMMENT ON COLUMN usage_response_outcomes.continuation_usage IS
    'Internal per-round scalar billing evidence for account anti-truncation; durable outbox v5';
