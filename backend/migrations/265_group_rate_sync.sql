ALTER TABLE groups ADD COLUMN IF NOT EXISTS rate_sync_group_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN groups.rate_sync_group_ids IS 'Target groups receiving base rate changes from this group; one-way, no cascading';
