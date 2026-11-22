CREATE TABLE IF NOT EXISTS media_generation_files (
    id VARCHAR(66) PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id BIGINT,
    task_id VARCHAR(128) NOT NULL,
    file_id VARCHAR(32) NOT NULL,
    model VARCHAR(256) NOT NULL DEFAULT '',
    download_url TEXT NOT NULL,
    content_type VARCHAR(256) NOT NULL DEFAULT 'application/octet-stream',
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
    sha256 VARCHAR(64) NOT NULL,
    ready BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (task_id, file_id)
);
CREATE INDEX IF NOT EXISTS media_generation_files_user_created_idx
    ON media_generation_files (user_id, created_at DESC, id);
COMMENT ON TABLE media_generation_files IS '媒体文件由中转服务保存；主站仅记录用户归属和下载链接';
