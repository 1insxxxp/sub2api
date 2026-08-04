CREATE TABLE IF NOT EXISTS user_custom_groups (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS user_custom_group_models (
    id BIGSERIAL PRIMARY KEY,
    custom_group_id BIGINT NOT NULL REFERENCES user_custom_groups(id) ON DELETE CASCADE,
    public_model VARCHAR(200) NOT NULL,
    source_group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    source_model VARCHAR(200) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_user_custom_group_public_model UNIQUE(custom_group_id, public_model)
);

ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS custom_group_id BIGINT NULL;
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_custom_group_id_fkey;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_custom_group_id_fkey
    FOREIGN KEY (custom_group_id) REFERENCES user_custom_groups(id) ON DELETE RESTRICT NOT VALID;
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_single_group_binding;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_single_group_binding
    CHECK (NOT (group_id IS NOT NULL AND custom_group_id IS NOT NULL)) NOT VALID;

ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS custom_group_id BIGINT NULL;
ALTER TABLE usage_logs DROP CONSTRAINT IF EXISTS usage_logs_custom_group_id_fkey;
ALTER TABLE usage_logs ADD CONSTRAINT usage_logs_custom_group_id_fkey
    FOREIGN KEY (custom_group_id) REFERENCES user_custom_groups(id) ON DELETE SET NULL NOT VALID;
