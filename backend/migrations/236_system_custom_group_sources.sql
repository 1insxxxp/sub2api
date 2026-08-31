CREATE TABLE IF NOT EXISTS system_custom_group_sources (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    source_group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    priority INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT system_custom_group_sources_no_self_reference
        CHECK (group_id <> source_group_id),
    CONSTRAINT system_custom_group_sources_priority_nonnegative
        CHECK (priority >= 0),
    UNIQUE (group_id, source_group_id),
    UNIQUE (group_id, priority)
);

CREATE INDEX IF NOT EXISTS idx_system_custom_group_sources_source_group_id
    ON system_custom_group_sources(source_group_id);

INSERT INTO system_custom_group_sources (group_id, source_group_id, priority)
SELECT group_id,
       source_group_id,
       ROW_NUMBER() OVER (PARTITION BY group_id ORDER BY first_route_id) - 1
FROM (
    SELECT group_id, source_group_id, MIN(id) AS first_route_id
    FROM system_custom_group_models
    GROUP BY group_id, source_group_id
) AS existing_sources
ON CONFLICT (group_id, source_group_id) DO NOTHING;
