ALTER TABLE media_generation_files ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
