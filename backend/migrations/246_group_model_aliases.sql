-- Group-level public model aliases and bounded continuation policy.
ALTER TABLE groups ADD COLUMN IF NOT EXISTS model_aliases JSONB NOT NULL DEFAULT '[]'::jsonb;
