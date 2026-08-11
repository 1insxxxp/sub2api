CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_system_custom_group_public_model_ci
    ON system_custom_group_models(group_id, LOWER(public_model));

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_system_custom_group_source_model_ci
    ON system_custom_group_models(group_id, source_group_id, LOWER(source_model));

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_system_custom_group_models_source_group_id
    ON system_custom_group_models(source_group_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_source_group_id
    ON usage_logs(source_group_id);
