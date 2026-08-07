CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_user_custom_group_public_model_casefold
    ON user_custom_group_models (custom_group_id, LOWER(public_model));
