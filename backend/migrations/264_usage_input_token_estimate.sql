ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS input_token_estimate JSONB;

COMMENT ON COLUMN usage_logs.input_token_estimate IS
    'Temporary admin-only local tokenizer estimate for accuracy evaluation; never used for billing';
