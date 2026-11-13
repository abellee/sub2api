ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS required_protocol VARCHAR(20) NOT NULL DEFAULT ''
    CHECK (required_protocol IN ('', 'anthropic', 'openai', 'google'));

COMMENT ON COLUMN groups.required_protocol IS '强制客户端协议；空值表示不限制';
