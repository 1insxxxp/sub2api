ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS default_reasoning_effort VARCHAR(20) NOT NULL DEFAULT '';
