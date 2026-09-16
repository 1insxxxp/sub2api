ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS tag VARCHAR(16) NOT NULL DEFAULT ''
    CHECK (tag IN ('', 'chat', 'image', 'airp'));
