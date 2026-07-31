ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS model_mappings JSONB NOT NULL DEFAULT '{}'::jsonb;
