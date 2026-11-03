-- Category defaults are published standard balance entries. Existing groups,
-- keys, prices and subscription contracts remain unchanged.
ALTER TABLE groups ADD COLUMN IF NOT EXISTS is_category_default BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS category_default_group_id BIGINT NULL;
ALTER TABLE usage_pending_records ADD COLUMN IF NOT EXISTS category_default_group_id BIGINT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_groups_category_default_category
    ON groups (key_display_category)
    WHERE is_category_default AND deleted_at IS NULL;

ALTER TABLE groups ADD CONSTRAINT groups_category_default_shape CHECK (
    NOT is_category_default OR (
        platform = 'composite' AND subscription_type = 'standard'
        AND NOT system_custom_routing_enabled
        AND key_display_category IN ('anthropic','openai','gemini','grok','domestic','other')
    )
);

CREATE TABLE IF NOT EXISTS category_default_groups (
    category VARCHAR(32) PRIMARY KEY CHECK (category IN ('anthropic','openai','gemini','grok','domestic','other')),
    group_id BIGINT NOT NULL UNIQUE REFERENCES groups(id),
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS category_default_group_sources (
    group_id BIGINT NOT NULL REFERENCES category_default_groups(group_id) ON DELETE CASCADE,
    source_group_id BIGINT NOT NULL REFERENCES groups(id),
    PRIMARY KEY (group_id, source_group_id),
    CHECK (group_id <> source_group_id)
);
CREATE INDEX IF NOT EXISTS idx_category_default_sources_reverse
    ON category_default_group_sources(source_group_id, group_id);

CREATE TABLE IF NOT EXISTS category_default_group_models (
    group_id BIGINT NOT NULL,
    public_model VARCHAR(200) NOT NULL CHECK (btrim(public_model) <> ''),
    source_group_id BIGINT NOT NULL,
    source_model VARCHAR(200) NOT NULL CHECK (btrim(source_model) <> ''),
    fingerprint CHAR(64) NOT NULL,
    FOREIGN KEY (group_id, source_group_id)
        REFERENCES category_default_group_sources(group_id, source_group_id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_category_default_public_model
    ON category_default_group_models(group_id, lower(btrim(public_model)));
