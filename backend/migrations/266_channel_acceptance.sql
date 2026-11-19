-- Synthetic administrator audit data only. No foreign keys to live accounts:
-- historical evidence must survive account deletion and never affect billing.
CREATE TABLE IF NOT EXISTS channel_acceptance_previews (
    id TEXT PRIMARY KEY,
    actor_id BIGINT NOT NULL,
    body JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS channel_acceptance_runs (
    id TEXT PRIMARY KEY,
    preview_id TEXT NOT NULL UNIQUE REFERENCES channel_acceptance_previews(id),
    actor_id BIGINT NOT NULL,
    rule_version TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending','running','completed','cancelled','budget_exhausted')),
    budget_usd NUMERIC(24,12) CHECK (budget_usd >= 0),
    estimated_cost_usd NUMERIC(24,12) NOT NULL CHECK (estimated_cost_usd >= 0),
    reserved_cost_usd NUMERIC(24,12) NOT NULL DEFAULT 0 CHECK (reserved_cost_usd >= 0),
    spent_cost_usd NUMERIC(24,12) NOT NULL DEFAULT 0 CHECK (spent_cost_usd >= 0),
    total INTEGER NOT NULL CHECK (total > 0),
    completed INTEGER NOT NULL DEFAULT 0 CHECK (completed >= 0 AND completed <= total),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS channel_acceptance_samples (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES channel_acceptance_runs(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL,
    ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
    body JSONB NOT NULL,
    estimated_cost_usd NUMERIC(24,12) NOT NULL CHECK (estimated_cost_usd >= 0),
    timeout_seconds INTEGER NOT NULL CHECK (timeout_seconds > 0 AND timeout_seconds <= 360),
    status TEXT NOT NULL CHECK (status IN ('pending','running','passed','anomaly','unsupported','inconclusive','error','cancelled','unknown')),
    worker_id TEXT NOT NULL DEFAULT '',
    lease_token TEXT NOT NULL DEFAULT '',
    lease_until TIMESTAMPTZ,
    result JSONB,
    UNIQUE (run_id, account_id, ordinal),
    CHECK (status <> 'running' OR (lease_token <> '' AND lease_until IS NOT NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS channel_acceptance_one_running_account
    ON channel_acceptance_samples(account_id) WHERE status = 'running';
CREATE INDEX IF NOT EXISTS channel_acceptance_pending
    ON channel_acceptance_samples(run_id, ordinal) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS channel_acceptance_account_history
    ON channel_acceptance_samples(account_id, run_id);
CREATE INDEX IF NOT EXISTS channel_acceptance_running_lease
    ON channel_acceptance_samples(lease_until) WHERE status = 'running';
CREATE INDEX IF NOT EXISTS channel_acceptance_run_history
    ON channel_acceptance_runs(created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS channel_acceptance_run_retention
    ON channel_acceptance_runs(updated_at) WHERE status IN ('completed','cancelled','budget_exhausted');
CREATE INDEX IF NOT EXISTS channel_acceptance_preview_expiry
    ON channel_acceptance_previews(expires_at);
