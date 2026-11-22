-- Isolation children keep their own rate links instead of inheriting self-links.
CREATE OR REPLACE FUNCTION propagate_sync_isolation_group() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE assignments TEXT; child_id BIGINT;
BEGIN
    IF NEW.sync_isolation_source_group_id>0 THEN RETURN NULL; END IF;
    SELECT string_agg(format('%I=s.%I',attname,attname), ',' ORDER BY attnum) INTO assignments
    FROM pg_attribute WHERE attrelid='groups'::regclass AND attnum>0 AND NOT attisdropped
        AND attname NOT IN ('id','name','i18n','created_at','updated_at','is_exclusive',
            'duplicate_operation_id','rate_sync_group_ids','sync_isolation_enabled','sync_isolation_source_group_id');
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

-- Serialize automatic names across sources whose long names share a prefix.
CREATE FUNCTION sync_isolation_group_name(source_name TEXT) RETURNS TEXT LANGUAGE plpgsql AS $$
DECLARE candidate TEXT; suffix TEXT; attempt INTEGER := 1;
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended('sync_isolation_group_names',0));
    LOOP
        suffix := CASE WHEN attempt=1 THEN ' [同步]' ELSE ' [同步 '||attempt||']' END;
        candidate := left(source_name,100-char_length(suffix))||suffix;
        IF NOT EXISTS (SELECT 1 FROM groups WHERE name=candidate AND deleted_at IS NULL) THEN
            RETURN candidate;
        END IF;
        attempt := attempt+1;
    END LOOP;
END;
$$;

-- Repair existing automatic children, preserving manual names and other links.
DO $$
DECLARE child RECORD;
BEGIN
    FOR child IN SELECT c.id,c.name,c.sync_isolation_source_group_id AS source_id,s.name AS source_name
        FROM groups c JOIN groups s ON s.id=c.sync_isolation_source_group_id
        WHERE c.deleted_at IS NULL AND s.deleted_at IS NULL ORDER BY c.id LOOP
        IF right(child.name,char_length(' [同步-'||child.source_id||']'))=' [同步-'||child.source_id||']' THEN
            UPDATE groups SET name=sync_isolation_group_name(child.source_name),updated_at=NOW() WHERE id=child.id;
        END IF;
        UPDATE groups SET rate_sync_group_ids=(SELECT COALESCE(jsonb_agg(value),'[]'::jsonb)
            FROM jsonb_array_elements(rate_sync_group_ids) WHERE value<>to_jsonb(child.id)),updated_at=NOW()
            WHERE id=child.id AND rate_sync_group_ids @> jsonb_build_array(child.id);
        UPDATE groups SET rate_sync_group_ids=rate_sync_group_ids||jsonb_build_array(child.id),updated_at=NOW()
            WHERE id=child.source_id AND NOT rate_sync_group_ids @> jsonb_build_array(child.id);
        INSERT INTO scheduler_outbox(event_type,group_id) VALUES ('group_changed',child.id),('group_changed',child.source_id);
    END LOOP;
END;
$$;
