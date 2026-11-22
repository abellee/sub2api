ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS blocked_user_agents TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN groups.blocked_user_agents IS '拒绝的客户端 UA 关键词，英文逗号分隔，不区分大小写';
