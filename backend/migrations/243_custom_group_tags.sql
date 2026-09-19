ALTER TABLE groups
    DROP CONSTRAINT IF EXISTS groups_tag_check;

ALTER TABLE groups
    ALTER COLUMN tag TYPE VARCHAR(80),
    ADD COLUMN IF NOT EXISTS tag_color VARCHAR(7) NOT NULL DEFAULT '';

ALTER TABLE groups
    ADD CONSTRAINT groups_tag_color_check
    CHECK (tag_color = '' OR tag_color ~ '^#[0-9a-fA-F]{6}$');
