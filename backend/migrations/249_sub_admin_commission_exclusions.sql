-- Exclude only usage whose request timestamp falls inside a recorded interval.
-- Closed intervals are retained when a customer is included again.
CREATE TABLE IF NOT EXISTS sub_admin_commission_exclusions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    ended_at TIMESTAMPTZ NULL,
    created_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    CHECK (ended_at IS NULL OR ended_at >= started_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sub_admin_commission_exclusions_active_unique
    ON sub_admin_commission_exclusions (user_id)
    WHERE ended_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_sub_admin_commission_exclusions_user_started
    ON sub_admin_commission_exclusions (user_id, started_at)
    INCLUDE (ended_at);

-- The existing gross cost remains the common source for other dashboards.
-- Existing buckets predate this feature and therefore have no exclusions.
ALTER TABLE usage_group_daily_rollups
    ADD COLUMN IF NOT EXISTS commission_excluded_cost DECIMAL(20, 10) NOT NULL DEFAULT 0;

COMMENT ON TABLE sub_admin_commission_exclusions IS 'Prospective customer exclusions from commission reporting; closed intervals remain effective for historical usage.';
COMMENT ON COLUMN sub_admin_commission_exclusions.user_id IS 'Stable customer ID, intentionally retained after customer deletion.';
COMMENT ON COLUMN usage_group_daily_rollups.commission_excluded_cost IS 'Subset of actual_cost excluded only from administrator commission reporting.';

-- Moving a historical usage row between customers can change only the
-- commission subset, even when its group, timestamp and gross cost stay equal.
DROP TRIGGER IF EXISTS usage_logs_group_rollup_invalidate_update ON usage_logs;
CREATE TRIGGER usage_logs_group_rollup_invalidate_update
AFTER UPDATE OF created_at, group_id, actual_cost, user_id ON usage_logs
FOR EACH ROW
WHEN (
    (
        OLD.created_at IS DISTINCT FROM NEW.created_at
        OR OLD.group_id IS DISTINCT FROM NEW.group_id
        OR OLD.actual_cost IS DISTINCT FROM NEW.actual_cost
        OR OLD.user_id IS DISTINCT FROM NEW.user_id
    )
    AND (OLD.group_id IS NOT NULL OR NEW.group_id IS NOT NULL)
)
EXECUTE FUNCTION invalidate_group_usage_rollup_state();
