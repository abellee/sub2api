CREATE TABLE announcement_reaction_counts (
    announcement_id BIGINT NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    emoji_id SMALLINT NOT NULL CHECK (emoji_id BETWEEN 1 AND 30),
    reaction_count BIGINT NOT NULL CHECK (reaction_count >= 0),
    PRIMARY KEY (announcement_id, emoji_id)
);

CREATE TABLE announcement_reactions (
    announcement_id BIGINT NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji_id SMALLINT NOT NULL CHECK (emoji_id BETWEEN 1 AND 30),
    PRIMARY KEY (announcement_id, user_id)
);

-- Keep shared totals in the same transaction as each user's reaction.
CREATE FUNCTION update_announcement_reaction_counts() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    change RECORD;
    target_id BIGINT;
BEGIN
    IF TG_OP = 'DELETE' THEN target_id := OLD.announcement_id;
    ELSE target_id := NEW.announcement_id;
    END IF;
    -- Lock totals in a stable order when users switch between two emoji.
    FOR change IN
        SELECT emoji_id, SUM(delta) AS delta FROM (
            SELECT OLD.emoji_id AS emoji_id, -1 AS delta WHERE TG_OP IN ('UPDATE', 'DELETE')
            UNION ALL
            SELECT NEW.emoji_id, 1 WHERE TG_OP IN ('INSERT', 'UPDATE')
        ) AS contributions
        GROUP BY emoji_id HAVING SUM(delta) <> 0 ORDER BY emoji_id
    LOOP
        IF change.delta < 0 THEN
            UPDATE announcement_reaction_counts
            SET reaction_count = reaction_count + change.delta
            WHERE announcement_id = target_id AND emoji_id = change.emoji_id;
            DELETE FROM announcement_reaction_counts
            WHERE announcement_id = target_id AND emoji_id = change.emoji_id AND reaction_count = 0;
        ELSE
            INSERT INTO announcement_reaction_counts(announcement_id, emoji_id, reaction_count)
            VALUES(target_id, change.emoji_id, change.delta)
            ON CONFLICT(announcement_id, emoji_id) DO UPDATE
            SET reaction_count = announcement_reaction_counts.reaction_count + EXCLUDED.reaction_count;
        END IF;
    END LOOP;
    RETURN NULL;
END;
$$;

CREATE TRIGGER announcement_reaction_counts_update
AFTER INSERT OR UPDATE OF emoji_id OR DELETE ON announcement_reactions
FOR EACH ROW EXECUTE FUNCTION update_announcement_reaction_counts();
