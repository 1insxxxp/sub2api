SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

-- Extend display-only categories without reclassifying existing groups.
ALTER TABLE groups
    DROP CONSTRAINT IF EXISTS groups_key_display_category_check,
    ADD CONSTRAINT groups_key_display_category_check
    CHECK (key_display_category IN ('', 'anthropic', 'openai', 'gemini', 'grok', 'domestic', 'image', 'other'));
