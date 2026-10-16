ALTER TABLE usage_response_outcomes
    ADD COLUMN IF NOT EXISTS anti_truncation_rounds INTEGER NOT NULL DEFAULT 0
    CHECK (anti_truncation_rounds >= 0);
