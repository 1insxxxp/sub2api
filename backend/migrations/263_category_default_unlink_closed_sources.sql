-- Closing a source removes its saved links in the same transaction. Restoring
-- the source does not restore those links; an administrator must select it again.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

CREATE OR REPLACE FUNCTION category_default_unlink_sources(source_ids BIGINT[]) RETURNS VOID
LANGUAGE plpgsql AS $$
DECLARE
    entry_id BIGINT;
BEGIN
    IF COALESCE(cardinality(source_ids), 0) = 0 THEN RETURN; END IF;
    -- Lock entries before deleting links, matching publication's config/clock
    -- ordering. In particular, do not upgrade the old declaration SHARE lock.
    FOR entry_id IN
        SELECT c.group_id FROM category_default_groups c
        WHERE EXISTS (
            SELECT 1 FROM category_default_group_sources s
            WHERE s.group_id=c.group_id AND s.source_group_id=ANY(source_ids)
        )
        ORDER BY c.group_id FOR UPDATE OF c
    LOOP
        DELETE FROM category_default_group_sources
            WHERE group_id=entry_id AND source_group_id=ANY(source_ids);
        IF FOUND THEN
            -- Published model rows follow the source-link foreign key cascade.
            DELETE FROM category_default_declaration_history
                WHERE entry_group_id=entry_id AND source_group_id=ANY(source_ids);
            UPDATE category_default_groups
                SET revision=revision+1,updated_at=NOW() WHERE group_id=entry_id;
            INSERT INTO scheduler_outbox(event_type,group_id) VALUES('group_changed',entry_id);
            INSERT INTO auth_cache_invalidation_outbox(cache_key)
                SELECT encode(sha256(convert_to(k.key,'UTF8')),'hex') FROM api_keys k
                WHERE k.group_id=entry_id AND k.deleted_at IS NULL AND k.key<>'';
        END IF;
    END LOOP;
END $$;

CREATE OR REPLACE FUNCTION category_default_unlink_closed_source() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        PERFORM category_default_unlink_sources(ARRAY[OLD.id]);
        RETURN OLD;
    END IF;
    IF NEW.status<>'active' OR NEW.deleted_at IS NOT NULL THEN
        PERFORM category_default_unlink_sources(ARRAY[NEW.id]);
    END IF;
    RETURN NEW;
END $$;

-- The name places this UPDATE trigger before category_default_groups_changed,
-- whose declaration clock takes a shared config lock. Both operations are atomic.
DROP TRIGGER IF EXISTS category_default_auto_unlink_source_update ON groups;
CREATE TRIGGER category_default_auto_unlink_source_update
    AFTER UPDATE OF status,deleted_at ON groups
    FOR EACH ROW EXECUTE FUNCTION category_default_unlink_closed_source();
DROP TRIGGER IF EXISTS category_default_auto_unlink_source_delete ON groups;
CREATE TRIGGER category_default_auto_unlink_source_delete
    BEFORE DELETE ON groups
    FOR EACH ROW EXECUTE FUNCTION category_default_unlink_closed_source();

-- Repair existing closed/deleted links, including old disabled sources that
-- have never been edited since the automatic-follow feature was introduced.
SELECT category_default_unlink_sources(ARRAY(
    SELECT DISTINCT s.source_group_id FROM category_default_group_sources s
    JOIN groups g ON g.id=s.source_group_id
    WHERE g.status<>'active' OR g.deleted_at IS NOT NULL
));
