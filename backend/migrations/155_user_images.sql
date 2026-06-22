CREATE TABLE IF NOT EXISTS user_images (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode               VARCHAR(32) NOT NULL,
    model              VARCHAR(128) NOT NULL,
    prompt             TEXT,
    aspect_ratio       VARCHAR(16) NOT NULL,
    size               VARCHAR(32) NOT NULL,
    image_url          VARCHAR(2048) NOT NULL,
    storage_driver     VARCHAR(32) NOT NULL DEFAULT 'local',
    storage_object_key VARCHAR(1024) NOT NULL,
    mime_type          VARCHAR(128) NOT NULL DEFAULT 'image/png',
    bytes              BIGINT NOT NULL DEFAULT 0,
    cost               DECIMAL(20,8) NOT NULL DEFAULT 0,
    usage_log_id       BIGINT,
    source_image_count INTEGER NOT NULL DEFAULT 0,
    expires_at         TIMESTAMPTZ,
    deleted_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_images_user_created_at
    ON user_images (user_id, created_at);

CREATE INDEX IF NOT EXISTS idx_user_images_user_deleted_at
    ON user_images (user_id, deleted_at);

CREATE INDEX IF NOT EXISTS idx_user_images_deleted_at
    ON user_images (deleted_at);

CREATE INDEX IF NOT EXISTS idx_user_images_expires_at
    ON user_images (expires_at);

CREATE INDEX IF NOT EXISTS idx_user_images_storage_object_key
    ON user_images (storage_object_key);
