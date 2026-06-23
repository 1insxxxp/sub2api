CREATE TABLE IF NOT EXISTS user_image_tasks (
    id                    BIGSERIAL PRIMARY KEY,
    user_id               BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id            BIGINT,
    group_id              BIGINT,
    image_id              BIGINT REFERENCES user_images(id) ON DELETE SET NULL,
    mode                  VARCHAR(32) NOT NULL,
    status                VARCHAR(32) NOT NULL,
    model                 VARCHAR(128) NOT NULL,
    prompt                TEXT,
    aspect_ratio          VARCHAR(16) NOT NULL,
    quality               VARCHAR(32) NOT NULL,
    size                  VARCHAR(32) NOT NULL,
    estimated_cost        DECIMAL(20,8) NOT NULL DEFAULT 0,
    source_image_count    INTEGER NOT NULL DEFAULT 0,
    reference_object_keys TEXT,
    error_reason          VARCHAR(128),
    error_message         TEXT,
    started_at            TIMESTAMPTZ,
    completed_at          TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_image_tasks_user_created_at
    ON user_image_tasks (user_id, created_at);

CREATE INDEX IF NOT EXISTS idx_user_image_tasks_status_created_at
    ON user_image_tasks (status, created_at);

CREATE INDEX IF NOT EXISTS idx_user_image_tasks_image_id
    ON user_image_tasks (image_id);
