ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS empty_response_compensation_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS compensated_cost DECIMAL(20,10) NOT NULL DEFAULT 0;

ALTER TABLE usage_logs
    DROP CONSTRAINT IF EXISTS usage_logs_compensated_cost_valid;

ALTER TABLE usage_logs
    ADD CONSTRAINT usage_logs_compensated_cost_valid
    CHECK (compensated_cost >= 0 AND compensated_cost <= actual_cost) NOT VALID;

CREATE TABLE IF NOT EXISTS usage_response_outcomes (
    id BIGSERIAL PRIMARY KEY,
    usage_log_id BIGINT UNIQUE REFERENCES usage_logs(id) ON DELETE CASCADE,
    request_id VARCHAR(64) NOT NULL,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    http_status INTEGER NOT NULL DEFAULT 0,
    upstream_status INTEGER NOT NULL DEFAULT 0,
    has_text BOOLEAN NOT NULL DEFAULT FALSE,
    has_tool_call BOOLEAN NOT NULL DEFAULT FALSE,
    has_reasoning BOOLEAN NOT NULL DEFAULT FALSE,
    has_media BOOLEAN NOT NULL DEFAULT FALSE,
    output_bytes BIGINT NOT NULL DEFAULT 0,
    event_count INTEGER NOT NULL DEFAULT 0,
    stream_completed BOOLEAN NOT NULL DEFAULT FALSE,
    finish_reason VARCHAR(100) NOT NULL DEFAULT '',
    disconnect_source VARCHAR(20) NOT NULL DEFAULT 'none'
        CHECK (disconnect_source IN ('none', 'client', 'upstream', 'server')),
    upstream_error_kind VARCHAR(32) NOT NULL DEFAULT 'none',
    collector_version SMALLINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (request_id, api_key_id)
);

CREATE TABLE IF NOT EXISTS empty_response_claims (
    id BIGSERIAL PRIMARY KEY,
    usage_log_id BIGINT NOT NULL UNIQUE REFERENCES usage_logs(id) ON DELETE RESTRICT,
    outcome_id BIGINT REFERENCES usage_response_outcomes(id) ON DELETE SET NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE RESTRICT,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    subscription_id BIGINT REFERENCES user_subscriptions(id) ON DELETE SET NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'evaluating'
        CHECK (status IN ('evaluating', 'manual_review', 'approved', 'rejected', 'compensated')),
    reason_code VARCHAR(64) NOT NULL DEFAULT '',
    user_reason VARCHAR(255) NOT NULL DEFAULT '',
    admin_note TEXT NOT NULL DEFAULT '',
    original_actual_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    balance_refund DECIMAL(20,10) NOT NULL DEFAULT 0,
    subscription_refund DECIMAL(20,10) NOT NULL DEFAULT 0,
    api_key_quota_refund DECIMAL(20,10) NOT NULL DEFAULT 0,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    rule_version INTEGER NOT NULL DEFAULT 1,
    reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    compensated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_empty_response_claims_status_created
    ON empty_response_claims(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_empty_response_claims_user_created
    ON empty_response_claims(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_empty_response_claims_group_created
    ON empty_response_claims(group_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_empty_response_claims_account_created
    ON empty_response_claims(account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_response_outcomes_user_created
    ON usage_response_outcomes(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_response_outcomes_group_created
    ON usage_response_outcomes(group_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_response_outcomes_account_created
    ON usage_response_outcomes(account_id, created_at DESC);
