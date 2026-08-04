CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_custom_groups_user_id
    ON user_custom_groups(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_custom_groups_status
    ON user_custom_groups(status);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_custom_groups_deleted_at
    ON user_custom_groups(deleted_at);
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_user_custom_groups_owner_name_active
    ON user_custom_groups(user_id, name) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_custom_group_models_source_group_id
    ON user_custom_group_models(source_group_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_api_keys_custom_group_id
    ON api_keys(custom_group_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_custom_group_id
    ON usage_logs(custom_group_id);
