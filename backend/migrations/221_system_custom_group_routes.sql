ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS system_custom_routing_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS system_custom_group_models (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    public_model VARCHAR(200) NOT NULL,
    source_group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    source_model VARCHAR(200) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT system_custom_group_models_no_self_reference
        CHECK (group_id <> source_group_id)
);

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS source_group_id BIGINT NULL;
ALTER TABLE usage_logs
    DROP CONSTRAINT IF EXISTS usage_logs_source_group_id_fkey;
ALTER TABLE usage_logs
    ADD CONSTRAINT usage_logs_source_group_id_fkey
    FOREIGN KEY (source_group_id) REFERENCES groups(id) ON DELETE SET NULL NOT VALID;
