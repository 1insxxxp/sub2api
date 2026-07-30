ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS model_system_prompts JSONB NOT NULL DEFAULT '{}'::jsonb;
