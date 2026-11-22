ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS force_openai_ultrafast BOOLEAN NOT NULL DEFAULT FALSE;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'groups_forced_openai_tiers_exclusive' AND conrelid = 'groups'::regclass
    ) THEN
        ALTER TABLE groups ADD CONSTRAINT groups_forced_openai_tiers_exclusive
            CHECK (NOT (force_openai_fast AND force_openai_ultrafast));
    END IF;
END $$;

COMMENT ON COLUMN groups.force_openai_ultrafast IS
    'Force service_tier=ultrafast before global policy evaluation; bill tokens at 6x Standard';
