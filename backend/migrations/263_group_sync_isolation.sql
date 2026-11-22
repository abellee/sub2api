ALTER TABLE groups ADD COLUMN sync_isolation_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE groups ADD COLUMN sync_isolation_source_group_id BIGINT NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX idx_groups_sync_isolation_source ON groups (sync_isolation_source_group_id)
    WHERE sync_isolation_source_group_id > 0 AND deleted_at IS NULL;

CREATE TABLE sync_isolation_windows (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    state JSONB NOT NULL DEFAULT '{"isolated":false,"events":[]}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, group_id)
);

-- Copy all routing/pricing bindings, retaining their column defaults and priorities.
-- The account/channel/composite triggers below keep later admin changes in sync.
CREATE FUNCTION sync_isolation_bindings(source_id BIGINT, child_id BIGINT, only_table TEXT DEFAULT NULL) RETURNS VOID
LANGUAGE plpgsql AS $$
DECLARE relation_name TEXT; columns TEXT;
BEGIN
    FOREACH relation_name IN ARRAY ARRAY['account_groups','channel_groups','composite_model_routes','user_group_rate_multipliers'] LOOP
        IF only_table IS NOT NULL AND relation_name<>only_table THEN CONTINUE; END IF;
        SELECT string_agg(quote_ident(attname), ',' ORDER BY attnum) INTO columns
        FROM pg_attribute WHERE attrelid=relation_name::regclass AND attnum>0 AND NOT attisdropped
            AND attname<>'id';
        EXECUTE format('DELETE FROM %I WHERE group_id=$1',relation_name) USING child_id;
        EXECUTE format('INSERT INTO %I (%s) SELECT %s FROM %I r
            CROSS JOIN LATERAL jsonb_populate_record(NULL::%I, to_jsonb(r) || jsonb_build_object(''group_id'',$2)) c
            WHERE r.group_id=$1',relation_name,columns,
            (SELECT string_agg('c.'||quote_ident(attname), ',' ORDER BY attnum)
                FROM pg_attribute WHERE attrelid=relation_name::regclass AND attnum>0 AND NOT attisdropped AND attname<>'id'),
            relation_name,relation_name) USING source_id,child_id;
    END LOOP;
END;
$$;

CREATE FUNCTION propagate_sync_isolation_bindings() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE source_id BIGINT; child_id BIGINT;
BEGIN
    FOR source_id IN SELECT DISTINCT x FROM unnest(ARRAY[
        CASE WHEN TG_OP<>'INSERT' THEN OLD.group_id END,
        CASE WHEN TG_OP<>'DELETE' THEN NEW.group_id END]) x WHERE x IS NOT NULL LOOP
        IF NOT EXISTS (SELECT 1 FROM groups WHERE sync_isolation_source_group_id=source_id AND deleted_at IS NULL) THEN CONTINUE; END IF;
        PERFORM 1 FROM groups WHERE id=source_id FOR UPDATE;
        FOR child_id IN SELECT id FROM groups WHERE sync_isolation_source_group_id=source_id AND deleted_at IS NULL LOOP
            PERFORM sync_isolation_bindings(source_id,child_id,TG_TABLE_NAME);
            INSERT INTO scheduler_outbox (event_type,group_id) VALUES ('group_changed',child_id);
        END LOOP;
    END LOOP;
    RETURN NULL;
END;
$$;
CREATE TRIGGER trg_sync_isolation_accounts AFTER INSERT OR UPDATE OR DELETE ON account_groups
    FOR EACH ROW EXECUTE FUNCTION propagate_sync_isolation_bindings();
CREATE TRIGGER trg_sync_isolation_channels AFTER INSERT OR UPDATE OR DELETE ON channel_groups
    FOR EACH ROW EXECUTE FUNCTION propagate_sync_isolation_bindings();
CREATE TRIGGER trg_sync_isolation_routes AFTER INSERT OR UPDATE OR DELETE ON composite_model_routes
    FOR EACH ROW EXECUTE FUNCTION propagate_sync_isolation_bindings();
CREATE TRIGGER trg_sync_isolation_rates AFTER INSERT OR UPDATE OR DELETE ON user_group_rate_multipliers
    FOR EACH ROW EXECUTE FUNCTION propagate_sync_isolation_bindings();

-- Source fields are copied by column name so future group settings are inherited too.
CREATE FUNCTION propagate_sync_isolation_group() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE assignments TEXT; child_id BIGINT;
BEGIN
    IF NEW.sync_isolation_source_group_id>0 THEN RETURN NULL; END IF;
    SELECT string_agg(format('%I=s.%I',attname,attname), ',' ORDER BY attnum) INTO assignments
    FROM pg_attribute WHERE attrelid='groups'::regclass AND attnum>0 AND NOT attisdropped
        AND attname NOT IN ('id','name','i18n','created_at','updated_at','is_exclusive',
            'duplicate_operation_id','sync_isolation_enabled','sync_isolation_source_group_id');
    FOR child_id IN SELECT id FROM groups WHERE sync_isolation_source_group_id=NEW.id AND deleted_at IS NULL LOOP
        EXECUTE format('UPDATE groups c SET %s, sync_isolation_enabled=FALSE, updated_at=NOW()
            FROM groups s WHERE s.id=$1 AND c.id=$2',assignments) USING NEW.id,child_id;
        INSERT INTO auth_cache_invalidation_outbox (cache_key)
            SELECT encode(sha256(convert_to(key,'UTF8')),'hex') FROM api_keys
            WHERE group_id=child_id AND deleted_at IS NULL;
        INSERT INTO scheduler_outbox (event_type,group_id) VALUES ('group_changed',child_id);
    END LOOP;
    IF OLD.sync_isolation_enabled IS DISTINCT FROM NEW.sync_isolation_enabled THEN
        INSERT INTO auth_cache_invalidation_outbox (cache_key)
            SELECT encode(sha256(convert_to(key,'UTF8')),'hex') FROM api_keys
            WHERE group_id=NEW.id AND deleted_at IS NULL;
    END IF;
    IF OLD.sync_isolation_enabled AND NOT NEW.sync_isolation_enabled THEN
        UPDATE api_keys SET group_id=NEW.id,updated_at=NOW()
            WHERE group_id IN (SELECT id FROM groups WHERE sync_isolation_source_group_id=NEW.id)
            AND deleted_at IS NULL;
        DELETE FROM sync_isolation_windows WHERE group_id=NEW.id;
    END IF;
    RETURN NULL;
END;
$$;
CREATE TRIGGER trg_sync_isolation_group AFTER UPDATE ON groups
    FOR EACH ROW EXECUTE FUNCTION propagate_sync_isolation_group();

-- Revoking source access must also revoke access granted by automatic isolation.
CREATE FUNCTION revoke_sync_isolation_access() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM user_allowed_groups WHERE user_id=OLD.user_id AND group_id IN
        (SELECT id FROM groups WHERE sync_isolation_source_group_id=OLD.group_id);
    RETURN NULL;
END;
$$;
CREATE TRIGGER trg_sync_isolation_revoke AFTER DELETE ON user_allowed_groups
    FOR EACH ROW EXECUTE FUNCTION revoke_sync_isolation_access();
