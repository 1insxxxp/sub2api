-- Deleting a source must remove the route even for older soft-delete paths.
-- A pause or a display-name change keeps the selection available for restoration.
CREATE FUNCTION user_custom_group_unlink_deleted_source() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM user_custom_group_models WHERE source_group_id = NEW.id;
    RETURN NEW;
END
$$;

CREATE TRIGGER user_custom_group_unlink_deleted_source
    AFTER UPDATE OF deleted_at ON groups
    FOR EACH ROW WHEN (OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL)
    EXECUTE FUNCTION user_custom_group_unlink_deleted_source();

DELETE FROM user_custom_group_models m
USING groups g
WHERE m.source_group_id = g.id AND g.deleted_at IS NOT NULL;
