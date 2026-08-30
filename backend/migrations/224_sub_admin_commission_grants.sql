CREATE TABLE IF NOT EXISTS sub_admin_commission_grants (
    id BIGSERIAL PRIMARY KEY,
    sub_admin_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    granted_date DATE NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sub_admin_commission_grants_active_unique
    ON sub_admin_commission_grants (sub_admin_user_id, group_id)
    WHERE enabled = TRUE;

CREATE INDEX IF NOT EXISTS idx_sub_admin_commission_grants_sub_admin_enabled
    ON sub_admin_commission_grants (sub_admin_user_id, enabled, granted_date);

CREATE INDEX IF NOT EXISTS idx_sub_admin_commission_grants_group_enabled
    ON sub_admin_commission_grants (group_id, enabled);

COMMENT ON TABLE sub_admin_commission_grants IS 'Secondary admin financial visibility grants for group commission reporting.';
COMMENT ON COLUMN sub_admin_commission_grants.granted_date IS 'Local natural date from which the assigned group is visible.';
