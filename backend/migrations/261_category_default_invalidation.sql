-- Record declaration history at the database boundary, including native SQL
-- writes. A source that changes A -> B -> A between requests must not silently
-- recover a published model. Prices and transient scheduling state are excluded.
ALTER TABLE category_default_groups
    ADD COLUMN publication_revision BIGINT NOT NULL DEFAULT 1 CHECK (publication_revision > 0);

CREATE TABLE category_default_source_clocks (
    source_group_id BIGINT PRIMARY KEY REFERENCES groups(id) ON DELETE CASCADE,
    epoch BIGINT NOT NULL DEFAULT 0 CHECK (epoch >= 0)
);
INSERT INTO category_default_source_clocks(source_group_id) SELECT id FROM groups;

CREATE TABLE category_default_declaration_history (
    entry_group_id BIGINT NOT NULL REFERENCES category_default_groups(group_id) ON DELETE CASCADE,
    publication_revision BIGINT NOT NULL,
    source_group_id BIGINT NOT NULL,
    source_txid BIGINT NOT NULL,
    snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(entry_group_id, publication_revision, source_group_id, source_txid)
);

-- Only non-secret values needed by the local declaration parser are retained.
CREATE FUNCTION category_default_account_declaration(row_data JSONB) RETURNS JSONB
LANGUAGE SQL IMMUTABLE AS $$
    SELECT jsonb_build_object(
        'ID', row_data->'id', 'Platform', row_data->'platform',
        'Type', row_data->'type', 'Status', row_data->'status',
        'Schedulable', row_data->'schedulable', 'DeletedAt', row_data->'deleted_at',
        'Credentials', jsonb_strip_nulls(jsonb_build_object(
            'model_mapping', row_data->'credentials'->'model_mapping',
            'compact_model_mapping', row_data->'credentials'->'compact_model_mapping',
            'oauth_type', row_data->'credentials'->'oauth_type',
            'api_protocol', row_data->'credentials'->'api_protocol',
            'account_mode', row_data->'credentials'->'account_mode')),
        'Extra', jsonb_strip_nulls(jsonb_build_object(
            'openai_passthrough', row_data->'extra'->'openai_passthrough',
            'openai_oauth_passthrough', row_data->'extra'->'openai_oauth_passthrough',
            'openai_responses_mode', row_data->'extra'->'openai_responses_mode',
            'openai_responses_supported', row_data->'extra'->'openai_responses_supported')))
$$;

CREATE FUNCTION category_default_group_declaration(row_data JSONB) RETURNS JSONB
LANGUAGE SQL IMMUTABLE AS $$
    SELECT jsonb_build_object(
        'id', row_data->'id', 'name', row_data->'name',
        'platform', row_data->'platform', 'status', row_data->'status',
        'deleted_at', row_data->'deleted_at', 'tag', row_data->'tag',
        'key_display_category', row_data->'key_display_category',
        'subscription_type', row_data->'subscription_type',
        'is_category_default', row_data->'is_category_default',
        'system_custom_routing_enabled', row_data->'system_custom_routing_enabled',
        'model_aliases', row_data->'model_aliases',
        'model_allowlist', row_data->'model_allowlist',
        'allow_messages_dispatch', row_data->'allow_messages_dispatch',
        'messages_dispatch_model_config', row_data->'messages_dispatch_model_config',
        'default_mapped_model', row_data->'default_mapped_model')
$$;

-- Grok accounts without an explicit string mapping inherit these two public
-- settings. Freeze their values alongside each historical source declaration;
-- recomputing old snapshots from process-local settings would erase A -> B -> A.
CREATE FUNCTION category_default_grok_mapping_options() RETURNS JSONB
LANGUAGE SQL STABLE AS $$
    SELECT jsonb_build_object(
        'DefaultText', COALESCE(NULLIF(btrim((SELECT value FROM settings WHERE key='grok_default_text_model')), ''), 'grok-4.6'),
        'EnableCrossClientMap', COALESCE(lower(btrim((SELECT value FROM settings WHERE key='grok_cross_client_model_map_enabled'))), '') NOT IN ('false','0','off','disabled'))
$$;

CREATE FUNCTION category_default_source_snapshot(source_id BIGINT) RETURNS JSONB
LANGUAGE SQL STABLE AS $$
    SELECT jsonb_build_object(
        'group', (SELECT category_default_group_declaration(to_jsonb(g)) FROM groups g WHERE g.id=source_id),
        'grok_mapping_options', category_default_grok_mapping_options(),
        'accounts', COALESCE((
            SELECT jsonb_agg(category_default_account_declaration(to_jsonb(a)) ORDER BY a.id)
            FROM accounts a JOIN account_groups ag ON ag.account_id=a.id
            WHERE ag.group_id=source_id AND a.deleted_at IS NULL AND a.status='active' AND a.schedulable=TRUE
        ), '[]'::jsonb),
        'routes', COALESCE((
            SELECT jsonb_agg(to_jsonb(r) - 'created_at' - 'updated_at' - 'notes' ORDER BY r.priority,r.id)
            FROM composite_model_routes r WHERE r.group_id=source_id AND r.deleted_at IS NULL AND r.enabled=TRUE
        ), '[]'::jsonb),
        'channel', (SELECT jsonb_build_object('ID',c.id,'Status',c.status,'ModelMapping',c.model_mapping,'GroupIDs',jsonb_build_array(source_id))
            FROM channels c JOIN channel_groups cg ON cg.channel_id=c.id
            WHERE cg.group_id=source_id AND c.status='active' LIMIT 1))
$$;

-- Publication takes the config lock before the clock lock, then reads sources.
-- Holding the same locks until source commit creates a durable publication
-- boundary. The clock row also detects writes committed after a SERIALIZABLE
-- publisher acquired its snapshot, including account/route insertion phantoms.
CREATE FUNCTION category_default_lock_changed_source(source_id BIGINT) RETURNS VOID
LANGUAGE plpgsql AS $$
BEGIN
    IF source_id IS NULL OR source_id <= 0 THEN RETURN; END IF;
    PERFORM c.group_id FROM category_default_groups c
        JOIN category_default_group_sources s ON s.group_id=c.group_id
        WHERE s.source_group_id=source_id ORDER BY c.group_id FOR SHARE OF c;
    INSERT INTO category_default_source_clocks(source_group_id,epoch)
        SELECT id,1 FROM groups WHERE id=source_id
        ON CONFLICT(source_group_id) DO UPDATE SET epoch=category_default_source_clocks.epoch+1;
END $$;

CREATE FUNCTION category_default_record_changed_source(source_id BIGINT) RETURNS VOID
LANGUAGE plpgsql AS $$
BEGIN
    IF source_id IS NULL OR source_id <= 0 THEN RETURN; END IF;
    INSERT INTO category_default_declaration_history(entry_group_id,publication_revision,source_group_id,source_txid,snapshot)
        SELECT c.group_id,c.publication_revision,source_id,txid_current(),category_default_source_snapshot(source_id)
        FROM category_default_groups c JOIN category_default_group_sources s ON s.group_id=c.group_id
        WHERE s.source_group_id=source_id
        ON CONFLICT(entry_group_id,publication_revision,source_group_id,source_txid) DO NOTHING;
END $$;

-- The immediate trigger acquires locks and bumps the source clock. The deferred
-- trigger observes the transaction's final catalogue, so a bulk atomic update
-- does not invalidate a model merely because of an intermediate row state.
CREATE FUNCTION category_default_declaration_changed() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    before_row JSONB := CASE WHEN TG_OP='INSERT' THEN NULL ELSE to_jsonb(OLD) END;
    after_row JSONB := CASE WHEN TG_OP='DELETE' THEN NULL ELSE to_jsonb(NEW) END;
    source_ids BIGINT[] := ARRAY[]::BIGINT[];
    source_id BIGINT;
    entity_id BIGINT;
    projection_before JSONB;
    projection_after JSONB;
BEGIN
    IF TG_TABLE_NAME='groups' THEN
        IF TG_OP='INSERT' THEN
            INSERT INTO category_default_source_clocks(source_group_id) VALUES(NEW.id) ON CONFLICT DO NOTHING;
            RETURN NULL;
        END IF;
        -- A label rename has no bearing on the model's public identity.
        projection_before := category_default_group_declaration(before_row) - 'name';
        projection_after := category_default_group_declaration(after_row) - 'name';
        IF projection_before IS NOT DISTINCT FROM projection_after THEN RETURN NULL; END IF;
        source_ids := ARRAY[COALESCE(NEW.id,OLD.id)];
    ELSIF TG_TABLE_NAME='accounts' THEN
        projection_before := category_default_account_declaration(before_row);
        projection_after := category_default_account_declaration(after_row);
        IF projection_before IS NOT DISTINCT FROM projection_after THEN RETURN NULL; END IF;
        entity_id := COALESCE(NEW.id,OLD.id);
        SELECT COALESCE(array_agg(group_id ORDER BY group_id),ARRAY[]::BIGINT[]) INTO source_ids FROM account_groups WHERE account_id=entity_id;
    ELSIF TG_TABLE_NAME='account_groups' THEN
        source_ids := ARRAY[CASE WHEN TG_OP='INSERT' THEN NULL ELSE OLD.group_id END,CASE WHEN TG_OP='DELETE' THEN NULL ELSE NEW.group_id END];
    ELSIF TG_TABLE_NAME='composite_model_routes' THEN
        IF (before_row - 'created_at' - 'updated_at' - 'notes') IS NOT DISTINCT FROM (after_row - 'created_at' - 'updated_at' - 'notes') THEN RETURN NULL; END IF;
        source_ids := ARRAY[CASE WHEN TG_OP='INSERT' THEN NULL ELSE OLD.group_id END,CASE WHEN TG_OP='DELETE' THEN NULL ELSE NEW.group_id END];
    ELSIF TG_TABLE_NAME='channels' THEN
        projection_before := jsonb_build_object('mapping',before_row->'model_mapping','status',before_row->'status');
        projection_after := jsonb_build_object('mapping',after_row->'model_mapping','status',after_row->'status');
        IF projection_before IS NOT DISTINCT FROM projection_after THEN RETURN NULL; END IF;
        entity_id := COALESCE(NEW.id,OLD.id);
        SELECT COALESCE(array_agg(group_id ORDER BY group_id),ARRAY[]::BIGINT[]) INTO source_ids FROM channel_groups WHERE channel_id=entity_id;
    ELSIF TG_TABLE_NAME='channel_groups' THEN
        source_ids := ARRAY[CASE WHEN TG_OP='INSERT' THEN NULL ELSE OLD.group_id END,CASE WHEN TG_OP='DELETE' THEN NULL ELSE NEW.group_id END];
    ELSIF TG_TABLE_NAME='settings' THEN
        IF COALESCE(before_row->>'key','') NOT IN ('grok_default_text_model','grok_cross_client_model_map_enabled')
            AND COALESCE(after_row->>'key','') NOT IN ('grok_default_text_model','grok_cross_client_model_map_enabled') THEN RETURN NULL; END IF;
        IF jsonb_build_object('key',before_row->'key','value',before_row->'value') IS NOT DISTINCT FROM
            jsonb_build_object('key',after_row->'key','value',after_row->'value') THEN RETURN NULL; END IF;
        SELECT COALESCE(array_agg(DISTINCT ag.group_id ORDER BY ag.group_id),ARRAY[]::BIGINT[]) INTO source_ids
            FROM accounts a JOIN account_groups ag ON ag.account_id=a.id
            WHERE a.platform='grok' AND a.deleted_at IS NULL AND a.status='active' AND a.schedulable=TRUE
            AND NOT EXISTS (
                SELECT 1 FROM jsonb_each(CASE WHEN jsonb_typeof(a.credentials->'model_mapping')='object'
                    THEN a.credentials->'model_mapping' ELSE '{}'::jsonb END) AS m
                WHERE jsonb_typeof(m.value)='string');
    END IF;
    FOR source_id IN SELECT DISTINCT x FROM unnest(source_ids) AS x WHERE x IS NOT NULL ORDER BY x LOOP
        IF TG_ARGV[0]='record' THEN
            PERFORM category_default_record_changed_source(source_id);
        ELSE
            PERFORM category_default_lock_changed_source(source_id);
        END IF;
    END LOOP;
    RETURN NULL;
END $$;

CREATE TRIGGER category_default_groups_changed AFTER INSERT OR UPDATE OR DELETE ON groups
    FOR EACH ROW EXECUTE FUNCTION category_default_declaration_changed('lock');
CREATE CONSTRAINT TRIGGER category_default_groups_history AFTER UPDATE OR DELETE ON groups
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION category_default_declaration_changed('record');
CREATE TRIGGER category_default_accounts_changed AFTER INSERT OR UPDATE OR DELETE ON accounts
    FOR EACH ROW EXECUTE FUNCTION category_default_declaration_changed('lock');
CREATE CONSTRAINT TRIGGER category_default_accounts_history AFTER INSERT OR UPDATE OR DELETE ON accounts
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION category_default_declaration_changed('record');
CREATE TRIGGER category_default_account_groups_changed AFTER INSERT OR UPDATE OR DELETE ON account_groups
    FOR EACH ROW EXECUTE FUNCTION category_default_declaration_changed('lock');
CREATE CONSTRAINT TRIGGER category_default_account_groups_history AFTER INSERT OR UPDATE OR DELETE ON account_groups
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION category_default_declaration_changed('record');
CREATE TRIGGER category_default_routes_changed AFTER INSERT OR UPDATE OR DELETE ON composite_model_routes
    FOR EACH ROW EXECUTE FUNCTION category_default_declaration_changed('lock');
CREATE CONSTRAINT TRIGGER category_default_routes_history AFTER INSERT OR UPDATE OR DELETE ON composite_model_routes
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION category_default_declaration_changed('record');
CREATE TRIGGER category_default_channels_changed AFTER INSERT OR UPDATE OR DELETE ON channels
    FOR EACH ROW EXECUTE FUNCTION category_default_declaration_changed('lock');
CREATE CONSTRAINT TRIGGER category_default_channels_history AFTER INSERT OR UPDATE OR DELETE ON channels
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION category_default_declaration_changed('record');
CREATE TRIGGER category_default_channel_groups_changed AFTER INSERT OR UPDATE OR DELETE ON channel_groups
    FOR EACH ROW EXECUTE FUNCTION category_default_declaration_changed('lock');
CREATE CONSTRAINT TRIGGER category_default_channel_groups_history AFTER INSERT OR UPDATE OR DELETE ON channel_groups
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION category_default_declaration_changed('record');
CREATE TRIGGER category_default_grok_settings_changed AFTER INSERT OR UPDATE OR DELETE ON settings
    FOR EACH ROW EXECUTE FUNCTION category_default_declaration_changed('lock');
CREATE CONSTRAINT TRIGGER category_default_grok_settings_history AFTER INSERT OR UPDATE OR DELETE ON settings
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION category_default_declaration_changed('record');
