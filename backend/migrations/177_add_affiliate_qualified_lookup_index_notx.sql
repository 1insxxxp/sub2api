-- Non-transactional migration: avoid blocking writes while indexing qualified
-- invitee relationships used by inviter-level tier counts.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_affiliates_inviter_qualified
    ON user_affiliates (inviter_id)
    WHERE qualified_at IS NOT NULL;
