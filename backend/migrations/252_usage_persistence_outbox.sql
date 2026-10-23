-- Durable usage detail snapshots committed atomically with wallet/quota billing.
-- No cascading identity foreign keys: administrative cleanup must never silently
-- discard a charged request awaiting persistence. Only successful delivery deletes.
CREATE TABLE IF NOT EXISTS usage_persistence_outbox (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(255) NOT NULL,
    api_key_id BIGINT NOT NULL,
    payload JSONB NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error TEXT,
    claimed_at TIMESTAMPTZ,
    claimed_by VARCHAR(128),
    UNIQUE (request_id, api_key_id),
    CHECK ((claimed_at IS NULL) = (claimed_by IS NULL))
);

CREATE INDEX IF NOT EXISTS idx_usage_persistence_outbox_ready
    ON usage_persistence_outbox (available_at, id) WHERE claimed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_usage_persistence_outbox_lease
    ON usage_persistence_outbox (claimed_at) WHERE claimed_at IS NOT NULL;

COMMENT ON TABLE usage_persistence_outbox IS
    'Committed billed-request usage snapshots; explicit scalar DTO only, no prompt, response content, credentials, or domain relations. Retained until atomic usage/outcome delivery succeeds.';
