SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS key_display_category VARCHAR(20) NOT NULL DEFAULT '';

ALTER TABLE groups
    ADD CONSTRAINT groups_key_display_category_check
    CHECK (key_display_category IN ('', 'anthropic', 'openai', 'domestic', 'other'));

COMMENT ON COLUMN groups.key_display_category IS
    'API key group selection display category; empty follows platform; does not affect routing or billing';
