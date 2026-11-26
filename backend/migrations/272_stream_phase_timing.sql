ALTER TABLE usage_response_outcomes
    ADD COLUMN upstream_first_event_ms INTEGER,
    ADD COLUMN upstream_first_reasoning_ms INTEGER,
    ADD COLUMN upstream_first_text_ms INTEGER,
    ADD COLUMN downstream_first_content_ms INTEGER;
