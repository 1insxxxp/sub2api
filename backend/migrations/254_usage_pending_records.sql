-- Request-history evidence for calls whose upstream usage is incomplete.
-- Deliberately separate from usage_logs, billing dedup and wallet ledgers:
-- a pending observation must never prevent a later measured settlement.
-- Transaction-local bounds also protect the brief usage_logs trigger changes.
SET LOCAL lock_timeout = '3s';
SET LOCAL statement_timeout = '30s';

CREATE TABLE IF NOT EXISTS usage_pending_records (
    id BIGSERIAL PRIMARY KEY CHECK (id > 0),
    request_id VARCHAR(255) NOT NULL,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    model VARCHAR(200) NOT NULL,
    requested_model VARCHAR(200) NOT NULL,
    upstream_model VARCHAR(200),
    group_id BIGINT,
    source_group_id BIGINT,
    custom_group_id BIGINT,
    subscription_id BIGINT,
    input_tokens INTEGER NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens INTEGER NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    input_tokens_source VARCHAR(20) NOT NULL DEFAULT 'unreported' CHECK (input_tokens_source IN ('upstream', 'unreported')),
    output_tokens_source VARCHAR(20) NOT NULL DEFAULT 'unreported' CHECK (output_tokens_source IN ('upstream', 'unreported')),
    cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cache_creation_5m_tokens INTEGER NOT NULL DEFAULT 0,
    cache_creation_1h_tokens INTEGER NOT NULL DEFAULT 0,
    image_output_tokens INTEGER NOT NULL DEFAULT 0,
    billing_type SMALLINT NOT NULL DEFAULT 0,
    stream BOOLEAN NOT NULL DEFAULT FALSE,
    duration_ms INTEGER,
    first_token_ms INTEGER,
    user_agent TEXT,
    ip_address VARCHAR(64),
    inbound_endpoint VARCHAR(255),
    upstream_endpoint VARCHAR(255),
    upstream_request_id VARCHAR(255),
    session_id VARCHAR(255),
    channel_id BIGINT,
    model_mapping_chain TEXT,
    service_tier VARCHAR(64),
    reasoning_effort VARCHAR(64),
    requested_reasoning_effort VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    UNIQUE (request_id, api_key_id)
);

CREATE INDEX IF NOT EXISTS idx_usage_pending_user_created ON usage_pending_records (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_pending_key_created ON usage_pending_records (api_key_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_pending_account_created ON usage_pending_records (account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_pending_group_created ON usage_pending_records (group_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_pending_created ON usage_pending_records (created_at DESC);

COMMENT ON TABLE usage_pending_records IS 'Unsettled upstream-usage observations; no financial effect. Hidden from request history once matching usage_logs exists.';

-- Serialize pending observations against actual usage INSERTs before either
-- can miss the other's uncommitted row. Settlements share the lock so unrelated
-- batch writers do not deadlock on repeated identities in different orders.
-- The pending writer takes the exclusive form in a separate statement before
-- its INSERT, obtaining a fresh READ COMMITTED snapshot after the lock wait.
CREATE OR REPLACE FUNCTION lock_pending_usage_identity_before_settlement()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.request_id IS NOT NULL AND NEW.request_id <> '' THEN
        PERFORM pg_advisory_xact_lock_shared(
            hashtextextended('usage-pending:' || NEW.api_key_id::text || ':' || NEW.request_id, 0)
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS usage_logs_lock_pending_identity ON usage_logs;
CREATE TRIGGER usage_logs_lock_pending_identity
    BEFORE INSERT ON usage_logs
    FOR EACH ROW EXECUTE FUNCTION lock_pending_usage_identity_before_settlement();

-- Cover single inserts, batch inserts and durable-outbox replay atomically.
-- Keep the observation as audit evidence after usage-log retention/purging.
CREATE OR REPLACE FUNCTION resolve_pending_usage_after_settlement()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE usage_pending_records
    SET resolved_at = NOW()
    WHERE request_id = NEW.request_id AND api_key_id = NEW.api_key_id
      AND resolved_at IS NULL;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS usage_logs_resolve_pending ON usage_logs;
CREATE TRIGGER usage_logs_resolve_pending
    AFTER INSERT ON usage_logs
    FOR EACH ROW EXECUTE FUNCTION resolve_pending_usage_after_settlement();
