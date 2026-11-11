ALTER TABLE groups ADD COLUMN IF NOT EXISTS anthropic_thinking_compat JSONB NOT NULL DEFAULT '{}';
COMMENT ON COLUMN groups.anthropic_thinking_compat IS 'Anthropic legacy thinking conversion: enabled, all_models, models';
