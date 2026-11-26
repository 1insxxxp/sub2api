ALTER TABLE usage_response_outcomes
    ADD COLUMN IF NOT EXISTS upstream_first_event_ms INTEGER,
    ADD COLUMN IF NOT EXISTS upstream_first_reasoning_ms INTEGER,
    ADD COLUMN IF NOT EXISTS upstream_first_text_ms INTEGER,
    ADD COLUMN IF NOT EXISTS downstream_first_content_ms INTEGER;
